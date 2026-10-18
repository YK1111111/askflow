package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainquestion "askflow/internal/domain/question"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

const (
	cardLocalTTL     = 30 * time.Second
	cardRedisTTL     = 10 * time.Minute
	cardTTLJitter    = time.Minute
	negativeLocalTTL = 10 * time.Second
	negativeRedisTTL = 45 * time.Second
)

type questionCacheEntry struct {
	Found bool                     `json:"found"`
	Value *domainquestion.Question `json:"value,omitempty"`
}

// QuestionRepository 为问题仓储增加 L1、Redis、空值缓存和并发回源合并。
type QuestionRepository struct {
	next  domainquestion.Repository
	store Store
	local *gocache.Cache
	group singleflight.Group
}

func NewQuestionRepository(next domainquestion.Repository, store Store) *QuestionRepository {
	return &QuestionRepository{next: next, store: store, local: gocache.New(cardLocalTTL, time.Minute)}
}

func (r *QuestionRepository) Create_Question(ctx context.Context, question *domainquestion.Question) error {
	if err := r.next.Create_Question(ctx, question); err != nil {
		return err
	}
	r.write(ctx, r.key(question.ID), questionCacheEntry{Found: true, Value: question})
	return nil
}

func (r *QuestionRepository) Find_QuestionByID(ctx context.Context, questionID uint64) (*domainquestion.Question, error) {
	key := r.key(questionID)
	if entry, ok := r.getLocal(key); ok {
		return questionFromEntry(entry)
	}
	if entry, ok := r.getShared(ctx, key); ok {
		r.setLocal(key, entry)
		return questionFromEntry(entry)
	}

	value, err, shared := r.group.Do(key, func() (any, error) {
		if entry, ok := r.getLocal(key); ok {
			return entry, nil
		}
		if entry, ok := r.getShared(ctx, key); ok {
			r.setLocal(key, entry)
			return entry, nil
		}
		DefaultMetrics.CacheFallback("question_card")
		startedAt := time.Now()
		question, err := r.next.Find_QuestionByID(ctx, questionID)
		DefaultMetrics.ObserveLoad("question_card", time.Since(startedAt))
		if errors.Is(err, domainquestion.ErrNotFound) {
			entry := questionCacheEntry{Found: false}
			r.write(ctx, key, entry)
			return entry, nil
		}
		if err != nil {
			return questionCacheEntry{}, err
		}
		entry := questionCacheEntry{Found: true, Value: question}
		r.write(ctx, key, entry)
		return entry, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("question_card")
	}
	if err != nil {
		return nil, err
	}
	return questionFromEntry(value.(questionCacheEntry))
}

func (r *QuestionRepository) key(id uint64) string {
	return fmt.Sprintf("askflow:question:card:v1:%d", id)
}

func (r *QuestionRepository) getLocal(key string) (questionCacheEntry, bool) {
	value, ok := r.local.Get(key)
	if !ok {
		return questionCacheEntry{}, false
	}
	entry, ok := value.(questionCacheEntry)
	if ok {
		DefaultMetrics.CacheRequest("question_card", "hit")
	}
	return entry, ok
}

func (r *QuestionRepository) getShared(ctx context.Context, key string) (questionCacheEntry, bool) {
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 问题缓存失败，降级到 MySQL", "key", key, "error", err)
		return questionCacheEntry{}, false
	}
	if !ok {
		return questionCacheEntry{}, false
	}
	var entry questionCacheEntry
	if err := json.Unmarshal(content, &entry); err != nil || (entry.Found && entry.Value == nil) {
		slog.Warn("问题缓存内容损坏，按未命中处理", "key", key, "error", err)
		_ = r.store.Delete(ctx, key)
		return questionCacheEntry{}, false
	}
	return entry, true
}

func (r *QuestionRepository) setLocal(key string, entry questionCacheEntry) {
	ttl := cardLocalTTL
	if !entry.Found {
		ttl = negativeLocalTTL
	}
	r.local.Set(key, entry, ttl)
}

func (r *QuestionRepository) write(ctx context.Context, key string, entry questionCacheEntry) {
	r.setLocal(key, entry)
	content, err := json.Marshal(entry)
	if err != nil {
		return
	}
	ttl := ttlWithJitter(key, cardRedisTTL, cardTTLJitter)
	if !entry.Found {
		ttl = negativeRedisTTL
	}
	if err := r.store.Set(ctx, key, content, ttl); err != nil {
		slog.Warn("回填 Redis 问题缓存失败", "key", key, "error", err)
	}
}

func questionFromEntry(entry questionCacheEntry) (*domainquestion.Question, error) {
	if !entry.Found || entry.Value == nil {
		return nil, domainquestion.ErrNotFound
	}
	copy := *entry.Value
	return &copy, nil
}

var _ domainquestion.Repository = (*QuestionRepository)(nil)
