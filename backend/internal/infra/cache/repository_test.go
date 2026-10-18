package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domainanswerlike "askflow/internal/domain/answerlike"
	domainanswerranking "askflow/internal/domain/answerranking"
	domainfeed "askflow/internal/domain/feed"
	domainfollow "askflow/internal/domain/follow"
	domainquestion "askflow/internal/domain/question"
	domainuser "askflow/internal/domain/user"
)

type memoryStore struct {
	mu        sync.Mutex
	values    map[string][]byte
	getErr    error
	setErr    error
	deleteErr error
	gets      int
	sets      int
	deletes   int
}

func newMemoryStore() *memoryStore { return &memoryStore{values: map[string][]byte{}} }

func (s *memoryStore) Get(_ context.Context, stringKey string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gets++
	if s.getErr != nil {
		return nil, false, s.getErr
	}
	value, ok := s.values[stringKey]
	return append([]byte(nil), value...), ok, nil
}
func (s *memoryStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
	if s.setErr != nil {
		return s.setErr
	}
	s.values[key] = append([]byte(nil), value...)
	return nil
}
func (s *memoryStore) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for _, key := range keys {
		delete(s.values, key)
	}
	return nil
}

type countingQuestionRepository struct {
	mu       sync.Mutex
	question *domainquestion.Question
	err      error
	delay    time.Duration
	finds    int
}

func (r *countingQuestionRepository) Create_Question(context.Context, *domainquestion.Question) error {
	return nil
}
func (r *countingQuestionRepository) Find_QuestionByID(context.Context, uint64) (*domainquestion.Question, error) {
	r.mu.Lock()
	r.finds++
	r.mu.Unlock()
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.question, r.err
}
func (r *countingQuestionRepository) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finds
}

func TestQuestionRepositoryUsesLocalAndSharedCache(t *testing.T) {
	store := newMemoryStore()
	mysql := &countingQuestionRepository{question: &domainquestion.Question{ID: 1, Title: "问题"}}
	repository := NewQuestionRepository(mysql, store)

	for range 2 {
		question, err := repository.Find_QuestionByID(context.Background(), 1)
		if err != nil || question.Title != "问题" {
			t.Fatalf("读取问题失败: %+v, %v", question, err)
		}
	}
	if mysql.count() != 1 || store.sets != 1 {
		t.Fatalf("L1 应避免重复回源: mysql=%d sets=%d", mysql.count(), store.sets)
	}

	secondMySQL := &countingQuestionRepository{err: errors.New("不应访问 MySQL")}
	secondRepository := NewQuestionRepository(secondMySQL, store)
	if _, err := secondRepository.Find_QuestionByID(context.Background(), 1); err != nil {
		t.Fatalf("新实例应命中 Redis: %v", err)
	}
	if secondMySQL.count() != 0 {
		t.Fatalf("Redis 命中时不应访问 MySQL")
	}
}

func TestQuestionRepositoryCachesNotFoundAndDegrades(t *testing.T) {
	store := newMemoryStore()
	mysql := &countingQuestionRepository{err: domainquestion.ErrNotFound}
	repository := NewQuestionRepository(mysql, store)
	for range 2 {
		if _, err := repository.Find_QuestionByID(context.Background(), 99); !errors.Is(err, domainquestion.ErrNotFound) {
			t.Fatalf("不存在问题应返回指定错误: %v", err)
		}
	}
	if mysql.count() != 1 {
		t.Fatalf("空值缓存应避免重复回源，实际回源 %d 次", mysql.count())
	}

	brokenStore := newMemoryStore()
	brokenStore.getErr, brokenStore.setErr = errors.New("redis down"), errors.New("redis down")
	availableMySQL := &countingQuestionRepository{question: &domainquestion.Question{ID: 1}}
	if _, err := NewQuestionRepository(availableMySQL, brokenStore).Find_QuestionByID(context.Background(), 1); err != nil {
		t.Fatalf("Redis 故障时应降级到 MySQL: %v", err)
	}
}

func TestQuestionRepositorySingleflightMergesConcurrentLoads(t *testing.T) {
	store := newMemoryStore()
	mysql := &countingQuestionRepository{question: &domainquestion.Question{ID: 1}, delay: 20 * time.Millisecond}
	repository := NewQuestionRepository(mysql, store)

	var wait sync.WaitGroup
	for range 20 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, err := repository.Find_QuestionByID(context.Background(), 1); err != nil {
				t.Errorf("并发读取失败: %v", err)
			}
		}()
	}
	wait.Wait()
	if mysql.count() != 1 {
		t.Fatalf("singleflight 应只回源一次，实际为 %d", mysql.count())
	}
}

type fakeUserRepository struct {
	user    *domainuser.User
	updates int
}

func (r *fakeUserRepository) Create_User(context.Context, *domainuser.User) error { return nil }
func (r *fakeUserRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return r.user, nil
}
func (r *fakeUserRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return r.user, nil
}
func (r *fakeUserRepository) Update_User_Profile(context.Context, *domainuser.User) error {
	r.updates++
	return nil
}

func TestUserRepositoryDoesNotCachePasswordAndInvalidatesProfile(t *testing.T) {
	store := newMemoryStore()
	mysql := &fakeUserRepository{user: &domainuser.User{ID: 1, Username: "alice", PasswordHash: "secret-hash"}}
	repository := NewUserRepository(mysql, store)
	user, err := repository.Find_UserByID(context.Background(), 1)
	if err != nil || user.PasswordHash != "" {
		t.Fatalf("公开用户缓存不应返回密码摘要: %+v, %v", user, err)
	}
	for _, value := range store.values {
		if strings.Contains(string(value), "secret-hash") {
			t.Fatal("Redis 用户卡片不应包含密码摘要")
		}
	}
	if err := repository.Update_User_Profile(context.Background(), &domainuser.User{ID: 1}); err != nil {
		t.Fatalf("更新用户资料失败: %v", err)
	}
	if store.deletes != 1 || mysql.updates != 1 {
		t.Fatalf("资料更新后应删除共享缓存: deletes=%d updates=%d", store.deletes, mysql.updates)
	}
}

func TestTTLJitterIsStableAndBounded(t *testing.T) {
	base, jitter := 10*time.Minute, time.Minute
	first := ttlWithJitter("key", base, jitter)
	second := ttlWithJitter("key", base, jitter)
	if first != second || first < base || first > base+jitter {
		t.Fatalf("TTL 抖动不稳定或越界: first=%v second=%v", first, second)
	}
}

type countingAnswerRepository struct {
	answer *domainanswer.Answer
	err    error
	finds  int
}

func (r *countingAnswerRepository) Create_Answer(_ context.Context, answer *domainanswer.Answer) error {
	if r.err != nil {
		return r.err
	}
	r.answer = answer
	return nil
}
func (r *countingAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	r.finds++
	if r.err != nil {
		return nil, r.err
	}
	copy := *r.answer
	return &copy, nil
}
func (r *countingAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return []*domainanswer.Answer{r.answer}, r.err
}

func TestAnswerRepositorySeparatesCardAndStatsAndInvalidatesStats(t *testing.T) {
	store := newMemoryStore()
	mysql := &countingAnswerRepository{answer: &domainanswer.Answer{
		ID: 7, QuestionID: 2, AuthorID: 3, Content: "回答", LikeCount: 4, CommentCount: 5, FavoriteCount: 6,
	}}
	repository := NewAnswerRepository(mysql, store)

	first, err := repository.Find_AnswerByID(context.Background(), 7)
	if err != nil || first.LikeCount != 4 || mysql.finds != 1 {
		t.Fatalf("首次回源结果错误: answer=%+v finds=%d err=%v", first, mysql.finds, err)
	}
	cardContent := string(store.values[repository.cardKey(7)])
	if strings.Contains(cardContent, "like_count") || strings.Contains(cardContent, "comment_count") || strings.Contains(cardContent, "favorite_count") {
		t.Fatalf("回答卡片不应包含互动计数: %s", cardContent)
	}
	if _, ok := store.values[repository.statsKey(7)]; !ok {
		t.Fatal("首次回源后应写入回答统计缓存")
	}

	mysql.answer.LikeCount = 8
	repository.Invalidate_AnswerStats(context.Background(), 7)
	second, err := repository.Find_AnswerByID(context.Background(), 7)
	if err != nil || second.Content != "回答" || second.LikeCount != 8 || mysql.finds != 2 {
		t.Fatalf("统计失效后应保留卡片并回源最新计数: answer=%+v finds=%d err=%v", second, mysql.finds, err)
	}
}

func TestAnswerRepositoryCachesNotFound(t *testing.T) {
	store := newMemoryStore()
	mysql := &countingAnswerRepository{err: domainanswer.ErrNotFound}
	repository := NewAnswerRepository(mysql, store)
	for range 2 {
		if _, err := repository.Find_AnswerByID(context.Background(), 99); !errors.Is(err, domainanswer.ErrNotFound) {
			t.Fatalf("不存在回答应返回指定错误: %v", err)
		}
	}
	if mysql.finds != 1 {
		t.Fatalf("回答空值缓存应避免重复回源，实际回源 %d 次", mysql.finds)
	}
}

type fakeLikeRepository struct{ err error }

func (r fakeLikeRepository) Create_AnswerLike(context.Context, *domainanswerlike.AnswerLike) error {
	return r.err
}
func (r fakeLikeRepository) Delete_AnswerLike(context.Context, uint64, uint64) error { return r.err }
func (r fakeLikeRepository) Exists_AnswerLike(context.Context, uint64, uint64) (bool, error) {
	return false, r.err
}

type countingInvalidator struct{ calls int }

func (i *countingInvalidator) Invalidate_AnswerStats(context.Context, uint64) { i.calls++ }
func (i *countingInvalidator) Invalidate_AnswerRank(context.Context, uint64)  { i.calls++ }

func TestInteractionInvalidatesStatsOnlyAfterTransactionSuccess(t *testing.T) {
	in := &countingInvalidator{}
	answers := &countingAnswerRepository{answer: &domainanswer.Answer{ID: 7, QuestionID: 2}}
	success := NewAnswerLikeRepository(fakeLikeRepository{}, answers, in, in)
	if err := success.Create_AnswerLike(context.Background(), &domainanswerlike.AnswerLike{AnswerID: 7}); err != nil || in.calls != 2 {
		t.Fatalf("点赞成功后应失效统计缓存: calls=%d err=%v", in.calls, err)
	}

	failure := NewAnswerLikeRepository(fakeLikeRepository{err: errors.New("transaction failed")}, answers, in, in)
	if err := failure.Delete_AnswerLike(context.Background(), 1, 7); err == nil || in.calls != 2 {
		t.Fatalf("事务失败时不应失效统计缓存: calls=%d err=%v", in.calls, err)
	}
}

type fakeRankingRepository struct {
	items       []domainanswerranking.Item
	calls       int
	excludeID   uint64
	receivedMax int
}

func (r *fakeRankingRepository) Find_AnswerForQuestion(context.Context, uint64, uint64) (*domainanswerranking.Item, error) {
	return nil, errors.New("不应直接查询置顶回答")
}
func (r *fakeRankingRepository) Find_RankedAnswers(_ context.Context, _ uint64, excludeID uint64, _ time.Time, cursor *domainanswerranking.Cursor, limit int) ([]domainanswerranking.Item, error) {
	r.calls++
	r.excludeID, r.receivedMax = excludeID, limit
	items := make([]domainanswerranking.Item, 0, limit)
	for _, item := range r.items {
		if item.ID == excludeID || (cursor != nil && cursor.Score > 0 && !(item.QualityScore < cursor.Score || item.QualityScore == cursor.Score && item.ID < cursor.AnswerID)) {
			continue
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}

func TestAnswerRankingRepositoryCachesBaseRankAndAppliesAnchorDynamically(t *testing.T) {
	store := newMemoryStore()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	mysqlRank := &fakeRankingRepository{items: []domainanswerranking.Item{
		{ID: 3, QuestionID: 1, AuthorID: 9, Content: "第三", QualityScore: 30},
		{ID: 2, QuestionID: 1, AuthorID: 9, Content: "第二", QualityScore: 20},
		{ID: 1, QuestionID: 1, AuthorID: 9, Content: "第一", QualityScore: 10},
	}}
	mysqlAnswers := &countingAnswerRepository{err: errors.New("卡片应由排序查询结果预热")}
	answers := NewAnswerRepository(mysqlAnswers, store)
	users := NewUserRepository(&fakeUserRepository{user: &domainuser.User{ID: 9, Username: "alice"}}, store)
	repository := NewAnswerRankingRepository(mysqlRank, answers, users, store)

	first, err := repository.Find_RankedAnswers(context.Background(), 1, 3, now, nil, 2)
	if err != nil || len(first) != 2 || first[0].ID != 2 || first[1].ID != 1 || !first[0].RankedAt.Equal(now) {
		t.Fatalf("首次排序结果错误: items=%+v err=%v", first, err)
	}
	if mysqlRank.calls != 1 || mysqlRank.excludeID != 0 || mysqlRank.receivedMax != answerRankSize {
		t.Fatalf("基础排名应忽略 anchor 并预取固定窗口: calls=%d exclude=%d limit=%d", mysqlRank.calls, mysqlRank.excludeID, mysqlRank.receivedMax)
	}

	second, err := repository.Find_RankedAnswers(context.Background(), 1, 2, now.Add(time.Second), nil, 2)
	if err != nil || len(second) != 2 || second[0].ID != 3 || second[1].ID != 1 || mysqlRank.calls != 1 {
		t.Fatalf("不同 anchor 应复用基础排名: items=%+v calls=%d err=%v", second, mysqlRank.calls, err)
	}
}

func TestAnswerRankingRepositoryFallsBackForDifferentCursorSnapshot(t *testing.T) {
	store := newMemoryStore()
	firstSnapshot := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	mysqlRank := &fakeRankingRepository{items: []domainanswerranking.Item{{ID: 2, QuestionID: 1, AuthorID: 9, QualityScore: 20}, {ID: 1, QuestionID: 1, AuthorID: 9, QualityScore: 10}}}
	answers := NewAnswerRepository(&countingAnswerRepository{}, store)
	users := NewUserRepository(&fakeUserRepository{user: &domainuser.User{ID: 9}}, store)
	repository := NewAnswerRankingRepository(mysqlRank, answers, users, store)
	if _, err := repository.Find_RankedAnswers(context.Background(), 1, 0, firstSnapshot, nil, 1); err != nil {
		t.Fatal(err)
	}

	cursor := &domainanswerranking.Cursor{Score: 20, AnswerID: 2, AsOf: firstSnapshot.Add(time.Second)}
	if _, err := repository.Find_RankedAnswers(context.Background(), 1, 0, cursor.AsOf, cursor, 1); err != nil {
		t.Fatal(err)
	}
	if mysqlRank.calls != 2 {
		t.Fatalf("游标快照不同时应回退 MySQL，实际调用 %d 次", mysqlRank.calls)
	}
}

type fakeFeedRepository struct {
	followingCalls   int
	items            []domainfeed.FollowingItem
	hotCalls         int
	hotItems         []domainfeed.HotItem
	hotLimit         int
	recommendedCalls int
	recommendedItems []domainfeed.RecommendedItem
}

func (r *fakeFeedRepository) Find_FollowingFeed(context.Context, uint64, *domainfeed.Cursor, int) ([]domainfeed.FollowingItem, error) {
	r.followingCalls++
	return append([]domainfeed.FollowingItem(nil), r.items...), nil
}
func (r *fakeFeedRepository) Find_HotFeed(_ context.Context, _, _ time.Time, _ *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	r.hotCalls++
	r.hotLimit = limit
	return append([]domainfeed.HotItem(nil), r.hotItems...), nil
}
func (r *fakeFeedRepository) Find_RecommendedFeed(context.Context, uint64, time.Time, *domainfeed.RecommendedCursor, int) ([]domainfeed.RecommendedItem, error) {
	r.recommendedCalls++
	return append([]domainfeed.RecommendedItem(nil), r.recommendedItems...), nil
}

type fakeFollowingHydrator struct {
	calls int
	items []domainfeed.FollowingItem
}

type fakeRecommendedHydrator struct {
	calls int
	items []domainfeed.RecommendedItem
}

type fakeHotHydrator struct {
	calls int
	items []domainfeed.HotItem
}

type fakeHotWindowStore struct {
	candidates []HotCandidate
	err        error
}

func (s fakeHotWindowStore) FindHotWindow(context.Context, time.Time, int) ([]HotCandidate, error) {
	return s.candidates, s.err
}

func (h *fakeHotHydrator) Find_HotQuestionsByIDs(context.Context, []uint64) ([]domainfeed.HotItem, error) {
	h.calls++
	return append([]domainfeed.HotItem(nil), h.items...), nil
}
func (h *fakeHotHydrator) Find_HotItemsByQuestionIDs(context.Context, []uint64, time.Time, time.Time) ([]domainfeed.HotItem, error) {
	h.calls++
	return append([]domainfeed.HotItem(nil), h.items...), nil
}

func (h *fakeRecommendedHydrator) Find_RecommendedItemsByAnswerIDs(context.Context, uint64, []uint64) ([]domainfeed.RecommendedItem, error) {
	h.calls++
	return append([]domainfeed.RecommendedItem(nil), h.items...), nil
}

func (h *fakeFollowingHydrator) Find_FollowingItemsByAnswerIDs(context.Context, []uint64) ([]domainfeed.FollowingItem, error) {
	h.calls++
	return append([]domainfeed.FollowingItem(nil), h.items...), nil
}

func TestFollowingFeedCachesOnlyFirstPageAndRestoresOrder(t *testing.T) {
	store := newMemoryStore()
	createdAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	items := []domainfeed.FollowingItem{
		{QuestionID: 2, AnswerID: 20, AnswerContent: "较新", AnswerCreatedAt: createdAt},
		{QuestionID: 1, AnswerID: 10, AnswerContent: "较旧", AnswerCreatedAt: createdAt.Add(-time.Minute)},
	}
	mysql := &fakeFeedRepository{items: items}
	hydrator := &fakeFollowingHydrator{items: []domainfeed.FollowingItem{items[1], items[0]}}
	repository := NewFollowingFeedRepository(mysql, hydrator, store)

	first, err := repository.Find_FollowingFeed(context.Background(), 7, nil, 21)
	if err != nil || len(first) != 2 || mysql.followingCalls != 1 || hydrator.calls != 0 {
		t.Fatalf("首次 Feed 回源错误: items=%+v mysql=%d hydrate=%d err=%v", first, mysql.followingCalls, hydrator.calls, err)
	}
	second, err := repository.Find_FollowingFeed(context.Background(), 7, nil, 21)
	if err != nil || len(second) != 2 || second[0].AnswerID != 20 || second[1].AnswerID != 10 || mysql.followingCalls != 1 || hydrator.calls != 1 {
		t.Fatalf("缓存 Feed 应按轻量引用恢复顺序: items=%+v mysql=%d hydrate=%d err=%v", second, mysql.followingCalls, hydrator.calls, err)
	}

	cursor := &domainfeed.Cursor{CreatedAt: createdAt, AnswerID: 20}
	if _, err := repository.Find_FollowingFeed(context.Background(), 7, cursor, 21); err != nil || mysql.followingCalls != 2 {
		t.Fatalf("后续页应直接查询 MySQL: calls=%d err=%v", mysql.followingCalls, err)
	}
}

type fakeFollowRepository struct{ err error }

func (r fakeFollowRepository) Create_Follow(context.Context, *domainfollow.Follow) error {
	return r.err
}
func (r fakeFollowRepository) Delete_Follow(context.Context, uint64, uint64) error { return r.err }
func (r fakeFollowRepository) Exists_Follow(context.Context, uint64, uint64) (bool, error) {
	return false, r.err
}
func (r fakeFollowRepository) Find_FolloweeIDsByFollowerID(context.Context, uint64) ([]uint64, error) {
	return nil, r.err
}

func TestFollowChangesInvalidateFollowingFeedOnlyAfterSuccess(t *testing.T) {
	store := newMemoryStore()
	feed := NewFollowingFeedRepository(&fakeFeedRepository{}, &fakeFollowingHydrator{}, store)
	for limit := 2; limit <= 51; limit++ {
		store.values[feed.key(7, limit)] = []byte("cached")
	}
	recommended := NewRecommendedFeedRepository(feed, &fakeRecommendedHydrator{}, store)
	for limit := 2; limit <= 51; limit++ {
		store.values[recommended.key(7, limit)] = []byte("cached")
	}
	repository := NewFollowRepository(fakeFollowRepository{}, feed, recommended)
	if err := repository.Create_Follow(context.Background(), &domainfollow.Follow{FollowerID: 7, FolloweeID: 8}); err != nil {
		t.Fatal(err)
	}
	if len(store.values) != 0 || store.deletes != 2 {
		t.Fatalf("关注成功后应一次删除全部首页规格: remaining=%d deletes=%d", len(store.values), store.deletes)
	}

	failed := NewFollowRepository(fakeFollowRepository{err: errors.New("write failed")}, feed, recommended)
	if err := failed.Delete_Follow(context.Background(), 7, 8); err == nil || store.deletes != 2 {
		t.Fatalf("关注事务失败时不应删除缓存: deletes=%d err=%v", store.deletes, err)
	}
}

func TestRecommendedFeedCachesAnonymousAndPersonalizedFirstPages(t *testing.T) {
	store := newMemoryStore()
	rankedAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	items := []domainfeed.RecommendedItem{
		{QuestionID: 2, AnswerID: 20, AnswerContent: "高分", RecommendScore: 8, FollowingAuthor: true},
		{QuestionID: 1, AnswerID: 10, AnswerContent: "低分", RecommendScore: 6},
	}
	mysql := &fakeFeedRepository{recommendedItems: items}
	hydrator := &fakeRecommendedHydrator{items: []domainfeed.RecommendedItem{items[1], items[0]}}
	repository := NewRecommendedFeedRepository(mysql, hydrator, store)

	first, err := repository.Find_RecommendedFeed(context.Background(), 0, rankedAt, nil, 21)
	if err != nil || len(first) != 2 || mysql.recommendedCalls != 1 || hydrator.calls != 0 {
		t.Fatalf("匿名推荐首次回源错误: items=%+v calls=%d err=%v", first, mysql.recommendedCalls, err)
	}
	second, err := repository.Find_RecommendedFeed(context.Background(), 0, rankedAt.Add(time.Second), nil, 21)
	if err != nil || len(second) != 2 || second[0].AnswerID != 20 || second[0].RecommendScore != 8 || !second[0].RankedAt.Equal(rankedAt) || mysql.recommendedCalls != 1 || hydrator.calls != 1 {
		t.Fatalf("匿名推荐缓存恢复错误: items=%+v mysql=%d hydrate=%d err=%v", second, mysql.recommendedCalls, hydrator.calls, err)
	}

	if _, err := repository.Find_RecommendedFeed(context.Background(), 7, rankedAt, nil, 21); err != nil || mysql.recommendedCalls != 2 {
		t.Fatalf("个性化推荐不应复用匿名缓存: calls=%d err=%v", mysql.recommendedCalls, err)
	}
	cursor := &domainfeed.RecommendedCursor{Score: 8, AnswerID: 20, AsOf: rankedAt}
	if _, err := repository.Find_RecommendedFeed(context.Background(), 0, rankedAt, cursor, 21); err != nil || mysql.recommendedCalls != 3 {
		t.Fatalf("推荐后续页应直接查询 MySQL: calls=%d err=%v", mysql.recommendedCalls, err)
	}
}

func TestHotFeedCachesMinuteSnapshotAndRestoresStatsOrder(t *testing.T) {
	store := newMemoryStore()
	rankedAt := time.Date(2026, 10, 18, 12, 0, 10, 0, time.UTC)
	items := []domainfeed.HotItem{
		{QuestionID: 2, QuestionTitle: "最热", HotScore: 9, AnswerCount24H: 3, LikeCount24H: 4},
		{QuestionID: 1, QuestionTitle: "次热", HotScore: 6, ViewCount24H: 5},
		{QuestionID: 3, QuestionTitle: "第三", HotScore: 3},
	}
	mysql := &fakeFeedRepository{hotItems: items}
	hydrator := &fakeHotHydrator{items: []domainfeed.HotItem{items[2], items[1], items[0]}}
	repository := NewHotFeedRepository(mysql, hydrator, store, nil)

	first, err := repository.Find_HotFeed(context.Background(), rankedAt.Add(-24*time.Hour), rankedAt, nil, 2)
	if err != nil || len(first) != 2 || first[0].QuestionID != 2 || mysql.hotCalls != 1 || mysql.hotLimit != hotFeedSnapshotSize {
		t.Fatalf("热门快照首次加载错误: items=%+v calls=%d limit=%d err=%v", first, mysql.hotCalls, mysql.hotLimit, err)
	}
	secondAsOf := rankedAt.Add(20 * time.Second)
	second, err := repository.Find_HotFeed(context.Background(), secondAsOf.Add(-24*time.Hour), secondAsOf, nil, 3)
	if err != nil || len(second) != 3 || second[0].QuestionID != 2 || second[0].AnswerCount24H != 3 || !second[0].RankedAt.Equal(rankedAt) || mysql.hotCalls != 1 {
		t.Fatalf("同一分钟应复用热门快照并恢复顺序统计: items=%+v calls=%d err=%v", second, mysql.hotCalls, err)
	}

	cursor := &domainfeed.HotCursor{Score: 9, QuestionID: 2, AsOf: rankedAt}
	if _, err := repository.Find_HotFeed(context.Background(), rankedAt.Add(-24*time.Hour), rankedAt, cursor, 2); err != nil || mysql.hotCalls != 2 {
		t.Fatalf("热门后续页应直接查询 MySQL: calls=%d err=%v", mysql.hotCalls, err)
	}
}

func TestHotFeedUsesRedisHourWindowBeforeMySQLSnapshot(t *testing.T) {
	store := newMemoryStore()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	mysql := &fakeFeedRepository{}
	hydrator := &fakeHotHydrator{items: []domainfeed.HotItem{
		{QuestionID: 1, QuestionTitle: "第一", AnswerCount24H: 2},
		{QuestionID: 2, QuestionTitle: "第二", ViewCount24H: 3},
	}}
	hotWindow := fakeHotWindowStore{candidates: []HotCandidate{{QuestionID: 2, Score: 9}, {QuestionID: 1, Score: 5}}}
	repository := NewHotFeedRepository(mysql, hydrator, store, hotWindow)

	items, err := repository.Find_HotFeed(context.Background(), now.Add(-24*time.Hour), now, nil, 20)
	if err != nil || len(items) != 2 || items[0].QuestionID != 2 || items[0].HotScore != 9 || items[0].ViewCount24H != 3 {
		t.Fatalf("Redis 小时窗口组装错误: items=%+v err=%v", items, err)
	}
	if mysql.hotCalls != 0 || hydrator.calls != 1 {
		t.Fatalf("ZSET 命中不应执行全量 MySQL 聚合: mysql=%d hydrate=%d", mysql.hotCalls, hydrator.calls)
	}
}
