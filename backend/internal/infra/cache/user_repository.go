package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	domainuser "askflow/internal/domain/user"

	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type userCacheEntry struct {
	Found bool             `json:"found"`
	Value *domainuser.User `json:"value,omitempty"`
}

// UserRepository 为按 ID 查询的用户公开资料增加三层缓存，用户名登录查询仍直接读取 MySQL。
type UserRepository struct {
	next  domainuser.Repository
	store Store
	local *gocache.Cache
	group singleflight.Group
}

func NewUserRepository(next domainuser.Repository, store Store) *UserRepository {
	return &UserRepository{next: next, store: store, local: gocache.New(cardLocalTTL, time.Minute)}
}

func (r *UserRepository) Create_User(ctx context.Context, user *domainuser.User) error {
	if err := r.next.Create_User(ctx, user); err != nil {
		return err
	}
	r.write(ctx, r.key(user.ID), userCacheEntry{Found: true, Value: user})
	return nil
}

func (r *UserRepository) Find_UserByID(ctx context.Context, userID uint64) (*domainuser.User, error) {
	key := r.key(userID)
	if entry, ok := r.getLocal(key); ok {
		return userFromEntry(entry)
	}
	if entry, ok := r.getShared(ctx, key); ok {
		r.setLocal(key, entry)
		return userFromEntry(entry)
	}
	value, err, shared := r.group.Do(key, func() (any, error) {
		if entry, ok := r.getLocal(key); ok {
			return entry, nil
		}
		if entry, ok := r.getShared(ctx, key); ok {
			r.setLocal(key, entry)
			return entry, nil
		}
		DefaultMetrics.CacheFallback("user_card")
		startedAt := time.Now()
		user, err := r.next.Find_UserByID(ctx, userID)
		DefaultMetrics.ObserveLoad("user_card", time.Since(startedAt))
		if errors.Is(err, domainuser.ErrNotFound) {
			entry := userCacheEntry{Found: false}
			r.write(ctx, key, entry)
			return entry, nil
		}
		if err != nil {
			return userCacheEntry{}, err
		}
		entry := userCacheEntry{Found: true, Value: user}
		r.write(ctx, key, entry)
		return entry, nil
	})
	if shared {
		DefaultMetrics.SingleflightShared("user_card")
	}
	if err != nil {
		return nil, err
	}
	return userFromEntry(value.(userCacheEntry))
}

func (r *UserRepository) Find_UserByUsername(ctx context.Context, username string) (*domainuser.User, error) {
	return r.next.Find_UserByUsername(ctx, username)
}

func (r *UserRepository) Update_User_Profile(ctx context.Context, user *domainuser.User) error {
	if err := r.next.Update_User_Profile(ctx, user); err != nil {
		return err
	}
	key := r.key(user.ID)
	r.local.Delete(key)
	if err := r.store.Delete(ctx, key); err != nil {
		slog.Warn("删除 Redis 用户缓存失败", "key", key, "error", err)
	}
	return nil
}

func (r *UserRepository) key(id uint64) string {
	return fmt.Sprintf("askflow:user:card:v1:%d", id)
}

func (r *UserRepository) getLocal(key string) (userCacheEntry, bool) {
	value, ok := r.local.Get(key)
	if !ok {
		return userCacheEntry{}, false
	}
	entry, ok := value.(userCacheEntry)
	if ok {
		DefaultMetrics.CacheRequest("user_card", "hit")
	}
	return entry, ok
}

func (r *UserRepository) getShared(ctx context.Context, key string) (userCacheEntry, bool) {
	content, ok, err := r.store.Get(ctx, key)
	if err != nil {
		slog.Warn("读取 Redis 用户缓存失败，降级到 MySQL", "key", key, "error", err)
		return userCacheEntry{}, false
	}
	if !ok {
		return userCacheEntry{}, false
	}
	var entry userCacheEntry
	if err := json.Unmarshal(content, &entry); err != nil || (entry.Found && entry.Value == nil) {
		slog.Warn("用户缓存内容损坏，按未命中处理", "key", key, "error", err)
		_ = r.store.Delete(ctx, key)
		return userCacheEntry{}, false
	}
	return entry, true
}

func (r *UserRepository) setLocal(key string, entry userCacheEntry) {
	ttl := cardLocalTTL
	if !entry.Found {
		ttl = negativeLocalTTL
	}
	r.local.Set(key, entry, ttl)
}

func (r *UserRepository) write(ctx context.Context, key string, entry userCacheEntry) {
	if entry.Found && entry.Value != nil {
		publicUser := *entry.Value
		publicUser.PasswordHash = ""
		entry.Value = &publicUser
	}
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
		slog.Warn("回填 Redis 用户缓存失败", "key", key, "error", err)
	}
}

func userFromEntry(entry userCacheEntry) (*domainuser.User, error) {
	if !entry.Found || entry.Value == nil {
		return nil, domainuser.ErrNotFound
	}
	copy := *entry.Value
	copy.PasswordHash = ""
	return &copy, nil
}

var _ domainuser.Repository = (*UserRepository)(nil)
