package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	domainfeed "askflow/internal/domain/feed"

	"golang.org/x/sync/singleflight"
)

const (
	recommendedUserTTL         = 30 * time.Second
	recommendedUserJitter      = 15 * time.Second
	recommendedAnonymousTTL    = 30 * time.Second
	recommendedAnonymousJitter = 30 * time.Second
)

type recommendedFeedRef struct {
	QuestionID      uint64  `json:"question_id"`
	AnswerID        uint64  `json:"answer_id"`
	RecommendScore  float64 `json:"recommend_score"`
	FollowingAuthor bool    `json:"following_author"`
}

type recommendedFeedSnapshot struct {
	RankedAt time.Time            `json:"ranked_at"`
	Items    []recommendedFeedRef `json:"items"`
}

type recommendedFeedHydrator interface {
	Find_RecommendedItemsByAnswerIDs(ctx context.Context, userID uint64, answerIDs []uint64) ([]domainfeed.RecommendedItem, error)
}

type recommendedFeedInvalidator interface {
	Invalidate_RecommendedFeed(ctx context.Context, userID uint64)
}

// RecommendedFeedRepository 分别缓存匿名共享首页和登录用户个性化首页。
type RecommendedFeedRepository struct {
	next     domainfeed.Repository
	hydrator recommendedFeedHydrator
	store    Store
	group    singleflight.Group
}

func NewRecommendedFeedRepository(next domainfeed.Repository, hydrator recommendedFeedHydrator, store Store) *RecommendedFeedRepository {
	return &RecommendedFeedRepository{next: next, hydrator: hydrator, store: store}
}

func (r *RecommendedFeedRepository) Find_RecommendedFeed(ctx context.Context, userID uint64, asOf time.Time, cursor *domainfeed.RecommendedCursor, limit int) ([]domainfeed.RecommendedItem, error) {
	if cursor != nil {
		return r.next.Find_RecommendedFeed(ctx, userID, asOf, cursor, limit)
	}
	key := r.key(userID, limit)
	if snapshot, ok := r.read(ctx, key); ok {
		return r.hydrate(ctx, userID, snapshot)
	}
	value, err, shared := r.group.Do(key, func() (any, error) {
		if snapshot, ok := r.read(ctx, key); ok {
			return r.hydrate(ctx, userID, snapshot)
		}
		DefaultMetrics.CacheFallback("recommended_feed")
		startedAt := time.Now()
		items, err := r.next.Find_RecommendedFeed(ctx, userID, asOf, nil, limit)
		DefaultMetrics.ObserveLoad("recommended_feed", time.Since(startedAt))
		if err != nil {
			return nil, err
		}
		r.write(ctx, key, userID, snapshotFromRecommendedItems(items, asOf))
		return items, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("recommended_feed")
	}
	if err != nil {
		return nil, err
	}
	return value.([]domainfeed.RecommendedItem), nil
}

func (r *RecommendedFeedRepository) Find_FollowingFeed(ctx context.Context, userID uint64, cursor *domainfeed.Cursor, limit int) ([]domainfeed.FollowingItem, error) {
	return r.next.Find_FollowingFeed(ctx, userID, cursor, limit)
}

func (r *RecommendedFeedRepository) Find_HotFeed(ctx context.Context, since, asOf time.Time, cursor *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	return r.next.Find_HotFeed(ctx, since, asOf, cursor, limit)
}

func (r *RecommendedFeedRepository) Invalidate_RecommendedFeed(ctx context.Context, userID uint64) {
	if userID == 0 {
		return
	}
	keys := make([]string, 0, 50)
	for queryLimit := 2; queryLimit <= 51; queryLimit++ {
		keys = append(keys, r.key(userID, queryLimit))
	}
	if err := r.store.Delete(ctx, keys...); err != nil {
		slog.Warn("删除 Redis 个性化推荐 Feed 缓存失败", "user_id", userID, "error", err)
	}
}

func (r *RecommendedFeedRepository) hydrate(ctx context.Context, userID uint64, snapshot recommendedFeedSnapshot) ([]domainfeed.RecommendedItem, error) {
	answerIDs := make([]uint64, 0, len(snapshot.Items))
	for _, ref := range snapshot.Items {
		answerIDs = append(answerIDs, ref.AnswerID)
	}
	loaded, err := r.hydrator.Find_RecommendedItemsByAnswerIDs(ctx, userID, answerIDs)
	if err != nil {
		return nil, err
	}
	byAnswerID := make(map[uint64]domainfeed.RecommendedItem, len(loaded))
	for _, item := range loaded {
		byAnswerID[item.AnswerID] = item
	}
	items := make([]domainfeed.RecommendedItem, 0, len(snapshot.Items))
	for _, ref := range snapshot.Items {
		item, ok := byAnswerID[ref.AnswerID]
		if !ok {
			continue
		}
		item.QuestionID = ref.QuestionID
		item.RecommendScore = ref.RecommendScore
		item.FollowingAuthor = ref.FollowingAuthor
		item.RankedAt = snapshot.RankedAt
		items = append(items, item)
	}
	return items, nil
}

func (r *RecommendedFeedRepository) read(ctx context.Context, key string) (recommendedFeedSnapshot, bool) {
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 推荐 Feed 失败，降级到 MySQL", "key", key, "error", err)
		return recommendedFeedSnapshot{}, false
	}
	if !ok {
		return recommendedFeedSnapshot{}, false
	}
	var snapshot recommendedFeedSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil || snapshot.RankedAt.IsZero() || snapshot.Items == nil {
		_ = r.store.Delete(ctx, key)
		return recommendedFeedSnapshot{}, false
	}
	return snapshot, true
}

func (r *RecommendedFeedRepository) write(ctx context.Context, key string, userID uint64, snapshot recommendedFeedSnapshot) {
	content, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	ttl := ttlWithJitter(key, recommendedUserTTL, recommendedUserJitter)
	if userID == 0 {
		ttl = ttlWithJitter(key, recommendedAnonymousTTL, recommendedAnonymousJitter)
	}
	if err := r.store.Set(ctx, key, content, ttl); err != nil {
		slog.Warn("回填 Redis 推荐 Feed 失败", "key", key, "error", err)
	}
}

func (r *RecommendedFeedRepository) key(userID uint64, limit int) string {
	if userID == 0 {
		return fmt.Sprintf("askflow:feed:recommended:v1:anonymous:first:%d", limit)
	}
	return fmt.Sprintf("askflow:feed:recommended:v1:user:%d:first:%d", userID, limit)
}

func snapshotFromRecommendedItems(items []domainfeed.RecommendedItem, asOf time.Time) recommendedFeedSnapshot {
	snapshot := recommendedFeedSnapshot{RankedAt: asOf, Items: make([]recommendedFeedRef, 0, len(items))}
	for _, item := range items {
		snapshot.Items = append(snapshot.Items, recommendedFeedRef{QuestionID: item.QuestionID, AnswerID: item.AnswerID, RecommendScore: item.RecommendScore, FollowingAuthor: item.FollowingAuthor})
	}
	return snapshot
}

var _ domainfeed.Repository = (*RecommendedFeedRepository)(nil)
