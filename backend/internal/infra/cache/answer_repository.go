package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainanswer "askflow/internal/domain/answer"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

const (
	answerStatTTL    = 15 * time.Second
	answerStatJitter = 15 * time.Second
)

type answerCard struct {
	ID         uint64    `json:"id"`
	QuestionID uint64    `json:"question_id"`
	AuthorID   uint64    `json:"author_id"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type answerCardEntry struct {
	Found bool        `json:"found"`
	Value *answerCard `json:"value,omitempty"`
}

type answerStats struct {
	LikeCount     uint64 `json:"like_count"`
	CommentCount  uint64 `json:"comment_count"`
	FavoriteCount uint64 `json:"favorite_count"`
}

// AnswerStatsInvalidator 用于在互动事务提交后失效回答计数缓存。
type AnswerStatsInvalidator interface {
	Invalidate_AnswerStats(ctx context.Context, answerID uint64)
}

// AnswerRepository 将稳定的回答正文与短期互动计数拆分缓存。
type AnswerRepository struct {
	next            domainanswer.Repository
	store           Store
	local           *gocache.Cache
	group           singleflight.Group
	rankInvalidator AnswerRankInvalidator
}

func (r *AnswerRepository) SetAnswerRankInvalidator(invalidator AnswerRankInvalidator) {
	r.rankInvalidator = invalidator
}

func NewAnswerRepository(next domainanswer.Repository, store Store) *AnswerRepository {
	return &AnswerRepository{next: next, store: store, local: gocache.New(cardLocalTTL, time.Minute)}
}

func (r *AnswerRepository) Create_Answer(ctx context.Context, answer *domainanswer.Answer) error {
	if err := r.next.Create_Answer(ctx, answer); err != nil {
		return err
	}
	r.writeCard(ctx, answer)
	r.writeStats(ctx, answer.ID, statsFromAnswer(answer))
	if r.rankInvalidator != nil {
		r.rankInvalidator.Invalidate_AnswerRank(ctx, answer.QuestionID)
	}
	return nil
}

func (r *AnswerRepository) Find_AnswerByID(ctx context.Context, answerID uint64) (*domainanswer.Answer, error) {
	card, cardCached, cardFound := r.readCard(ctx, answerID)
	if cardCached && !cardFound {
		return nil, domainanswer.ErrNotFound
	}
	stats, statsFound := r.readStats(ctx, answerID)
	if cardFound && statsFound {
		return assembleAnswer(card, stats), nil
	}

	key := r.cardKey(answerID)
	value, err, shared := r.group.Do(key, func() (any, error) {
		cachedCard, cached, found := r.readCard(ctx, answerID)
		if cached && !found {
			return nil, domainanswer.ErrNotFound
		}
		statsCached := false
		if found {
			if cachedStats, ok := r.readStats(ctx, answerID); ok {
				return assembleAnswer(cachedCard, cachedStats), nil
			} else {
				statsCached = false
			}
		}
		if !found {
			DefaultMetrics.CacheFallback("answer_card")
		}
		if !statsCached {
			DefaultMetrics.CacheFallback("answer_stat")
		}
		startedAt := time.Now()
		answer, err := r.next.Find_AnswerByID(ctx, answerID)
		elapsed := time.Since(startedAt)
		if !found {
			DefaultMetrics.ObserveLoad("answer_card", elapsed)
		}
		if !statsCached {
			DefaultMetrics.ObserveLoad("answer_stat", elapsed)
		}
		if errors.Is(err, domainanswer.ErrNotFound) {
			r.writeNegativeCard(ctx, answerID)
			return nil, domainanswer.ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		r.writeCard(ctx, answer)
		r.writeStats(ctx, answerID, statsFromAnswer(answer))
		copy := *answer
		return &copy, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("answer_card")
	}
	if err != nil {
		return nil, err
	}
	return value.(*domainanswer.Answer), nil
}

func (r *AnswerRepository) Find_AnswersByQuestionID(ctx context.Context, questionID uint64) ([]*domainanswer.Answer, error) {
	answers, err := r.next.Find_AnswersByQuestionID(ctx, questionID)
	if err != nil {
		return nil, err
	}
	for _, answer := range answers {
		r.writeCard(ctx, answer)
		r.writeStats(ctx, answer.ID, statsFromAnswer(answer))
	}
	return answers, nil
}

func (r *AnswerRepository) Invalidate_AnswerStats(ctx context.Context, answerID uint64) {
	key := r.statsKey(answerID)
	if err := r.store.Delete(ctx, key); err != nil {
		slog.Warn("删除 Redis 回答统计缓存失败", "key", key, "error", err)
	}
}

func (r *AnswerRepository) readCard(ctx context.Context, answerID uint64) (answerCard, bool, bool) {
	key := r.cardKey(answerID)
	if value, ok := r.local.Get(key); ok {
		entry, valid := value.(answerCardEntry)
		if valid {
			DefaultMetrics.CacheRequest("answer_card", "hit")
		}
		if valid && !entry.Found {
			return answerCard{}, true, false
		}
		if valid && entry.Value != nil {
			return *entry.Value, true, true
		}
	}
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 回答卡片失败，降级到 MySQL", "key", key, "error", err)
		return answerCard{}, false, false
	}
	if !ok {
		return answerCard{}, false, false
	}
	var entry answerCardEntry
	if err := json.Unmarshal(content, &entry); err != nil || (entry.Found && entry.Value == nil) {
		_ = r.store.Delete(ctx, key)
		return answerCard{}, false, false
	}
	ttl := cardLocalTTL
	if !entry.Found {
		ttl = negativeLocalTTL
	}
	r.local.Set(key, entry, ttl)
	if !entry.Found {
		return answerCard{}, true, false
	}
	return *entry.Value, true, true
}

func (r *AnswerRepository) readStats(ctx context.Context, answerID uint64) (answerStats, bool) {
	key := r.statsKey(answerID)
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 回答统计失败，降级到 MySQL", "key", key, "error", err)
		return answerStats{}, false
	}
	if !ok {
		return answerStats{}, false
	}
	var stats answerStats
	if err := json.Unmarshal(content, &stats); err != nil {
		_ = r.store.Delete(ctx, key)
		return answerStats{}, false
	}
	return stats, true
}

func (r *AnswerRepository) writeCard(ctx context.Context, answer *domainanswer.Answer) {
	card := answerCard{ID: answer.ID, QuestionID: answer.QuestionID, AuthorID: answer.AuthorID, Content: answer.Content, CreatedAt: answer.CreatedAt, UpdatedAt: answer.UpdatedAt}
	r.writeCardEntry(ctx, answer.ID, answerCardEntry{Found: true, Value: &card})
}

func (r *AnswerRepository) writeNegativeCard(ctx context.Context, answerID uint64) {
	r.writeCardEntry(ctx, answerID, answerCardEntry{Found: false})
}

func (r *AnswerRepository) writeCardEntry(ctx context.Context, answerID uint64, entry answerCardEntry) {
	key := r.cardKey(answerID)
	localTTL, redisTTL := cardLocalTTL, ttlWithJitter(key, cardRedisTTL, cardTTLJitter)
	if !entry.Found {
		localTTL, redisTTL = negativeLocalTTL, negativeRedisTTL
	}
	r.local.Set(key, entry, localTTL)
	content, err := json.Marshal(entry)
	if err == nil {
		if err := r.store.Set(ctx, key, content, redisTTL); err != nil {
			slog.Warn("回填 Redis 回答卡片失败", "key", key, "error", err)
		}
	}
}

func (r *AnswerRepository) writeStats(ctx context.Context, answerID uint64, stats answerStats) {
	key := r.statsKey(answerID)
	content, err := json.Marshal(stats)
	if err == nil {
		if err := r.store.Set(ctx, key, content, ttlWithJitter(key, answerStatTTL, answerStatJitter)); err != nil {
			slog.Warn("回填 Redis 回答统计失败", "key", key, "error", err)
		}
	}
}

func (r *AnswerRepository) cardKey(id uint64) string {
	return fmt.Sprintf("askflow:answer:card:v1:%d", id)
}
func (r *AnswerRepository) statsKey(id uint64) string {
	return fmt.Sprintf("askflow:answer:stat:v1:%d", id)
}

func statsFromAnswer(answer *domainanswer.Answer) answerStats {
	return answerStats{LikeCount: answer.LikeCount, CommentCount: answer.CommentCount, FavoriteCount: answer.FavoriteCount}
}

func assembleAnswer(card answerCard, stats answerStats) *domainanswer.Answer {
	return &domainanswer.Answer{ID: card.ID, QuestionID: card.QuestionID, AuthorID: card.AuthorID, Content: card.Content, LikeCount: stats.LikeCount, CommentCount: stats.CommentCount, FavoriteCount: stats.FavoriteCount, CreatedAt: card.CreatedAt, UpdatedAt: card.UpdatedAt}
}

var _ domainanswer.Repository = (*AnswerRepository)(nil)
