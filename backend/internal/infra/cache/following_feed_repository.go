package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	domainfeed "askflow/internal/domain/feed"
	domainfollow "askflow/internal/domain/follow"

	"golang.org/x/sync/singleflight"
)

const (
	followingFeedTTL    = 15 * time.Second
	followingFeedJitter = 15 * time.Second
)

type followingFeedRef struct {
	QuestionID uint64    `json:"question_id"`
	AnswerID   uint64    `json:"answer_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type followingFeedSnapshot struct {
	Items []followingFeedRef `json:"items"`
}

type followingFeedHydrator interface {
	Find_FollowingItemsByAnswerIDs(ctx context.Context, answerIDs []uint64) ([]domainfeed.FollowingItem, error)
}

// FollowingFeedRepository 只缓存关注 Feed 首页的轻量有序引用；后续页仍由 MySQL 查询。
type FollowingFeedRepository struct {
	next     domainfeed.Repository
	hydrator followingFeedHydrator
	store    Store
	group    singleflight.Group
}

func NewFollowingFeedRepository(next domainfeed.Repository, hydrator followingFeedHydrator, store Store) *FollowingFeedRepository {
	return &FollowingFeedRepository{next: next, hydrator: hydrator, store: store}
}

func (r *FollowingFeedRepository) Find_FollowingFeed(ctx context.Context, userID uint64, cursor *domainfeed.Cursor, limit int) ([]domainfeed.FollowingItem, error) {
	if cursor != nil {
		return r.next.Find_FollowingFeed(ctx, userID, cursor, limit)
	}
	key := r.key(userID, limit)
	if snapshot, ok := r.read(ctx, key); ok {
		return r.hydrate(ctx, snapshot)
	}

	value, err, shared := r.group.Do(key, func() (any, error) {
		if snapshot, ok := r.read(ctx, key); ok {
			return r.hydrate(ctx, snapshot)
		}
		DefaultMetrics.CacheFallback("following_feed")
		startedAt := time.Now()
		items, err := r.next.Find_FollowingFeed(ctx, userID, nil, limit)
		DefaultMetrics.ObserveLoad("following_feed", time.Since(startedAt))
		if err != nil {
			return nil, err
		}
		r.write(ctx, key, snapshotFromFollowingItems(items))
		return items, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("following_feed")
	}
	if err != nil {
		return nil, err
	}
	return value.([]domainfeed.FollowingItem), nil
}

func (r *FollowingFeedRepository) Find_HotFeed(ctx context.Context, since, asOf time.Time, cursor *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	return r.next.Find_HotFeed(ctx, since, asOf, cursor, limit)
}

func (r *FollowingFeedRepository) Find_RecommendedFeed(ctx context.Context, userID uint64, asOf time.Time, cursor *domainfeed.RecommendedCursor, limit int) ([]domainfeed.RecommendedItem, error) {
	return r.next.Find_RecommendedFeed(ctx, userID, asOf, cursor, limit)
}

func (r *FollowingFeedRepository) Invalidate_FollowingFeed(ctx context.Context, userID uint64) {
	keys := make([]string, 0, 50)
	for queryLimit := 2; queryLimit <= 51; queryLimit++ {
		keys = append(keys, r.key(userID, queryLimit))
	}
	if err := r.store.Delete(ctx, keys...); err != nil {
		slog.Warn("删除 Redis 关注 Feed 缓存失败", "user_id", userID, "error", err)
	}
}

func (r *FollowingFeedRepository) hydrate(ctx context.Context, snapshot followingFeedSnapshot) ([]domainfeed.FollowingItem, error) {
	answerIDs := make([]uint64, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		answerIDs = append(answerIDs, item.AnswerID)
	}
	loaded, err := r.hydrator.Find_FollowingItemsByAnswerIDs(ctx, answerIDs)
	if err != nil {
		return nil, err
	}
	byAnswerID := make(map[uint64]domainfeed.FollowingItem, len(loaded))
	for _, item := range loaded {
		byAnswerID[item.AnswerID] = item
	}
	items := make([]domainfeed.FollowingItem, 0, len(snapshot.Items))
	for _, ref := range snapshot.Items {
		item, ok := byAnswerID[ref.AnswerID]
		if !ok {
			continue
		}
		item.QuestionID = ref.QuestionID
		item.AnswerCreatedAt = ref.CreatedAt
		items = append(items, item)
	}
	return items, nil
}

func (r *FollowingFeedRepository) read(ctx context.Context, key string) (followingFeedSnapshot, bool) {
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 关注 Feed 失败，降级到 MySQL", "key", key, "error", err)
		return followingFeedSnapshot{}, false
	}
	if !ok {
		return followingFeedSnapshot{}, false
	}
	var snapshot followingFeedSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil || snapshot.Items == nil {
		_ = r.store.Delete(ctx, key)
		return followingFeedSnapshot{}, false
	}
	return snapshot, true
}

func (r *FollowingFeedRepository) write(ctx context.Context, key string, snapshot followingFeedSnapshot) {
	content, err := json.Marshal(snapshot)
	if err == nil {
		if err := r.store.Set(ctx, key, content, ttlWithJitter(key, followingFeedTTL, followingFeedJitter)); err != nil {
			slog.Warn("回填 Redis 关注 Feed 失败", "key", key, "error", err)
		}
	}
}

func (r *FollowingFeedRepository) key(userID uint64, limit int) string {
	return fmt.Sprintf("askflow:feed:following:v1:%d:first:%d", userID, limit)
}

func snapshotFromFollowingItems(items []domainfeed.FollowingItem) followingFeedSnapshot {
	snapshot := followingFeedSnapshot{Items: make([]followingFeedRef, 0, len(items))}
	for _, item := range items {
		snapshot.Items = append(snapshot.Items, followingFeedRef{QuestionID: item.QuestionID, AnswerID: item.AnswerID, CreatedAt: item.AnswerCreatedAt})
	}
	return snapshot
}

var _ domainfeed.Repository = (*FollowingFeedRepository)(nil)

type followingFeedInvalidator interface {
	Invalidate_FollowingFeed(ctx context.Context, userID uint64)
}

// FollowRepository 在关注关系写入成功后失效当前用户的关注 Feed 首页。
type FollowRepository struct {
	next                   domainfollow.Repository
	followingInvalidator   followingFeedInvalidator
	recommendedInvalidator recommendedFeedInvalidator
}

func NewFollowRepository(next domainfollow.Repository, following followingFeedInvalidator, recommended recommendedFeedInvalidator) *FollowRepository {
	return &FollowRepository{next: next, followingInvalidator: following, recommendedInvalidator: recommended}
}

func (r *FollowRepository) Create_Follow(ctx context.Context, follow *domainfollow.Follow) error {
	if err := r.next.Create_Follow(ctx, follow); err != nil {
		return err
	}
	r.invalidateFeeds(ctx, follow.FollowerID)
	return nil
}

func (r *FollowRepository) Delete_Follow(ctx context.Context, followerID, followeeID uint64) error {
	if err := r.next.Delete_Follow(ctx, followerID, followeeID); err != nil {
		return err
	}
	r.invalidateFeeds(ctx, followerID)
	return nil
}

func (r *FollowRepository) invalidateFeeds(ctx context.Context, userID uint64) {
	r.followingInvalidator.Invalidate_FollowingFeed(ctx, userID)
	r.recommendedInvalidator.Invalidate_RecommendedFeed(ctx, userID)
}

func (r *FollowRepository) Exists_Follow(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	return r.next.Exists_Follow(ctx, followerID, followeeID)
}

func (r *FollowRepository) Find_FolloweeIDsByFollowerID(ctx context.Context, followerID uint64) ([]uint64, error) {
	return r.next.Find_FolloweeIDsByFollowerID(ctx, followerID)
}

var _ domainfollow.Repository = (*FollowRepository)(nil)
