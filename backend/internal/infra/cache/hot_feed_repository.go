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
	hotFeedSnapshotSize = 100
	hotFeedTTL          = 30 * time.Second
	hotFeedJitter       = 30 * time.Second
)

type hotFeedRef struct {
	QuestionID      uint64  `json:"question_id"`
	HotScore        float64 `json:"hot_score"`
	AnswerCount24H  uint64  `json:"answer_count_24h"`
	LikeCount24H    uint64  `json:"like_count_24h"`
	CommentCount24H uint64  `json:"comment_count_24h"`
	ViewCount24H    uint64  `json:"view_count_24h"`
}

type hotFeedSnapshot struct {
	RankedAt time.Time    `json:"ranked_at"`
	Items    []hotFeedRef `json:"items"`
}

type hotFeedHydrator interface {
	Find_HotQuestionsByIDs(ctx context.Context, questionIDs []uint64) ([]domainfeed.HotItem, error)
	Find_HotItemsByQuestionIDs(ctx context.Context, questionIDs []uint64, since, asOf time.Time) ([]domainfeed.HotItem, error)
}

type hotWindowStore interface {
	FindHotWindow(ctx context.Context, asOf time.Time, limit int) ([]HotCandidate, error)
}

// HotFeedRepository 缓存由 MySQL 计算的 24 小时热门榜快照。
type HotFeedRepository struct {
	next     domainfeed.Repository
	hydrator hotFeedHydrator
	store    Store
	hotStore hotWindowStore
	group    singleflight.Group
}

func NewHotFeedRepository(next domainfeed.Repository, hydrator hotFeedHydrator, store Store, hotStore hotWindowStore) *HotFeedRepository {
	return &HotFeedRepository{next: next, hydrator: hydrator, store: store, hotStore: hotStore}
}

func (r *HotFeedRepository) Find_HotFeed(ctx context.Context, since, asOf time.Time, cursor *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	if cursor != nil {
		return r.next.Find_HotFeed(ctx, since, asOf, cursor, limit)
	}
	if r.hotStore != nil {
		candidates, err := r.hotStore.FindHotWindow(ctx, asOf, limit)
		if err != nil {
			slog.Warn("读取 Redis 小时热榜失败，降级到 MySQL 快照", "error", err)
		} else if len(candidates) > 0 {
			return r.hydrateCandidates(ctx, candidates, since, asOf)
		}
	}
	key := r.key(asOf)
	if snapshot, ok := r.read(ctx, key); ok {
		return r.hydrate(ctx, snapshot, limit)
	}
	value, err, shared := r.group.Do(key, func() (any, error) {
		if snapshot, ok := r.read(ctx, key); ok {
			return snapshot, nil
		}
		DefaultMetrics.CacheFallback("hot_feed")
		startedAt := time.Now()
		items, err := r.next.Find_HotFeed(ctx, since, asOf, nil, hotFeedSnapshotSize)
		DefaultMetrics.ObserveLoad("hot_feed", time.Since(startedAt))
		if err != nil {
			return nil, err
		}
		snapshot := snapshotFromHotItems(items, asOf)
		r.write(ctx, key, snapshot)
		return snapshot, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("hot_feed")
	}
	if err != nil {
		return nil, err
	}
	return r.hydrate(ctx, value.(hotFeedSnapshot), limit)
}

func (r *HotFeedRepository) hydrateCandidates(ctx context.Context, candidates []HotCandidate, since, asOf time.Time) ([]domainfeed.HotItem, error) {
	questionIDs := make([]uint64, 0, len(candidates))
	for _, candidate := range candidates {
		questionIDs = append(questionIDs, candidate.QuestionID)
	}
	loaded, err := r.hydrator.Find_HotItemsByQuestionIDs(ctx, questionIDs, since, asOf)
	if err != nil {
		return nil, err
	}
	byQuestionID := make(map[uint64]domainfeed.HotItem, len(loaded))
	for _, item := range loaded {
		byQuestionID[item.QuestionID] = item
	}
	items := make([]domainfeed.HotItem, 0, len(candidates))
	for _, candidate := range candidates {
		item, ok := byQuestionID[candidate.QuestionID]
		if !ok {
			continue
		}
		item.HotScore = candidate.Score
		item.RankedAt = asOf
		items = append(items, item)
	}
	return items, nil
}

func (r *HotFeedRepository) Find_FollowingFeed(ctx context.Context, userID uint64, cursor *domainfeed.Cursor, limit int) ([]domainfeed.FollowingItem, error) {
	return r.next.Find_FollowingFeed(ctx, userID, cursor, limit)
}

func (r *HotFeedRepository) Find_RecommendedFeed(ctx context.Context, userID uint64, asOf time.Time, cursor *domainfeed.RecommendedCursor, limit int) ([]domainfeed.RecommendedItem, error) {
	return r.next.Find_RecommendedFeed(ctx, userID, asOf, cursor, limit)
}

func (r *HotFeedRepository) hydrate(ctx context.Context, snapshot hotFeedSnapshot, limit int) ([]domainfeed.HotItem, error) {
	refs := snapshot.Items
	if len(refs) > limit {
		refs = refs[:limit]
	}
	questionIDs := make([]uint64, 0, len(refs))
	for _, ref := range refs {
		questionIDs = append(questionIDs, ref.QuestionID)
	}
	loaded, err := r.hydrator.Find_HotQuestionsByIDs(ctx, questionIDs)
	if err != nil {
		return nil, err
	}
	byQuestionID := make(map[uint64]domainfeed.HotItem, len(loaded))
	for _, item := range loaded {
		byQuestionID[item.QuestionID] = item
	}
	items := make([]domainfeed.HotItem, 0, len(refs))
	for _, ref := range refs {
		item, ok := byQuestionID[ref.QuestionID]
		if !ok {
			continue
		}
		item.HotScore = ref.HotScore
		item.AnswerCount24H = ref.AnswerCount24H
		item.LikeCount24H = ref.LikeCount24H
		item.CommentCount24H = ref.CommentCount24H
		item.ViewCount24H = ref.ViewCount24H
		item.RankedAt = snapshot.RankedAt
		items = append(items, item)
	}
	return items, nil
}

func (r *HotFeedRepository) read(ctx context.Context, key string) (hotFeedSnapshot, bool) {
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 热门 Feed 失败，降级到 MySQL", "key", key, "error", err)
		return hotFeedSnapshot{}, false
	}
	if !ok {
		return hotFeedSnapshot{}, false
	}
	var snapshot hotFeedSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil || snapshot.RankedAt.IsZero() || snapshot.Items == nil {
		_ = r.store.Delete(ctx, key)
		return hotFeedSnapshot{}, false
	}
	return snapshot, true
}

func (r *HotFeedRepository) write(ctx context.Context, key string, snapshot hotFeedSnapshot) {
	content, err := json.Marshal(snapshot)
	if err == nil {
		if err := r.store.Set(ctx, key, content, ttlWithJitter(key, hotFeedTTL, hotFeedJitter)); err != nil {
			slog.Warn("回填 Redis 热门 Feed 失败", "key", key, "error", err)
		}
	}
}

func (r *HotFeedRepository) key(asOf time.Time) string {
	windowEnd := asOf.UTC().Truncate(time.Minute)
	return fmt.Sprintf("askflow:feed:hot:v1:snapshot:%d", windowEnd.Unix())
}

func snapshotFromHotItems(items []domainfeed.HotItem, asOf time.Time) hotFeedSnapshot {
	snapshot := hotFeedSnapshot{RankedAt: asOf, Items: make([]hotFeedRef, 0, len(items))}
	for _, item := range items {
		snapshot.Items = append(snapshot.Items, hotFeedRef{QuestionID: item.QuestionID, HotScore: item.HotScore, AnswerCount24H: item.AnswerCount24H, LikeCount24H: item.LikeCount24H, CommentCount24H: item.CommentCount24H, ViewCount24H: item.ViewCount24H})
	}
	return snapshot
}

var _ domainfeed.Repository = (*HotFeedRepository)(nil)
