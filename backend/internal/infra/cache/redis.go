// Package cache 提供 Redis 和本地缓存实现。
package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"askflow/internal/infra/config"

	"github.com/redis/go-redis/v9"
)

// Store 定义缓存装饰器需要的最小共享缓存能力。
type Store interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type RedisStore struct{ client *redis.Client }

type HotCandidate struct {
	QuestionID uint64
	Score      float64
}

const hotBucketTTL = 48 * time.Hour

var applyHotDeltaScript = redis.NewScript(`
if redis.call('SET', KEYS[1], '1', 'NX', 'EX', ARGV[1]) then
  redis.call('ZINCRBY', KEYS[2], ARGV[2], ARGV[3])
  redis.call('EXPIRE', KEYS[2], ARGV[1])
  redis.call('DEL', KEYS[3])
  return 1
end
return 0`)

func NewRedisStore(cfg config.RedisConfig) *RedisStore {
	return &RedisStore{client: redis.NewClient(&redis.Options{
		Addr: cfg.Addr, Password: cfg.Password, DB: cfg.DB,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
	})}
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	startedAt := time.Now()
	value, err := s.client.Get(ctx, key).Bytes()
	DefaultMetrics.ObserveRedis("get", time.Since(startedAt))
	if err == redis.Nil {
		DefaultMetrics.CacheRequest(cacheArea(key), "miss")
		return nil, false, nil
	}
	if err != nil {
		DefaultMetrics.CacheRequest(cacheArea(key), "error")
	} else {
		DefaultMetrics.CacheRequest(cacheArea(key), "hit")
	}
	return value, err == nil, err
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	startedAt := time.Now()
	err := s.client.Set(ctx, key, value, ttl).Err()
	DefaultMetrics.ObserveRedis("set", time.Since(startedAt))
	result := "success"
	if err != nil {
		result = "error"
	}
	DefaultMetrics.CacheWrite(cacheArea(key), result)
	return err
}

func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	startedAt := time.Now()
	err := s.client.Del(ctx, keys...).Err()
	DefaultMetrics.ObserveRedis("delete", time.Since(startedAt))
	result := "success"
	if err != nil {
		result = "error"
	}
	areas := map[string]struct{}{}
	for _, key := range keys {
		areas[cacheArea(key)] = struct{}{}
	}
	for area := range areas {
		DefaultMetrics.CacheWrite(area, result)
	}
	return err
}

func (s *RedisStore) Close() error { return s.client.Close() }

func (s *RedisStore) ApplyHotDelta(ctx context.Context, eventID string, questionID uint64, delta float64, occurredAt time.Time) error {
	if eventID == "" || questionID == 0 || occurredAt.IsZero() {
		return fmt.Errorf("热门事件参数不完整")
	}
	bucket := hotHourKey(occurredAt)
	processed := "askflow:event:processed:v1:" + eventID
	window := hotWindowKey(time.Now())
	startedAt := time.Now()
	err := applyHotDeltaScript.Run(ctx, s.client, []string{processed, bucket, window}, int64(hotBucketTTL/time.Second), delta, questionID).Err()
	DefaultMetrics.ObserveRedis("eval_hot_delta", time.Since(startedAt))
	result := "success"
	if err != nil {
		result = "error"
	}
	DefaultMetrics.CacheWrite("hot_feed", result)
	return err
}

func (s *RedisStore) FindHotWindow(ctx context.Context, asOf time.Time, limit int) ([]HotCandidate, error) {
	window := hotWindowKey(asOf)
	startedAt := time.Now()
	exists, err := s.client.Exists(ctx, window).Result()
	DefaultMetrics.ObserveRedis("exists_hot_window", time.Since(startedAt))
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		keys := make([]string, 0, 24)
		hour := asOf.UTC().Truncate(time.Hour)
		for offset := 0; offset < 24; offset++ {
			keys = append(keys, hotHourKey(hour.Add(-time.Duration(offset)*time.Hour)))
		}
		startedAt = time.Now()
		if err := s.client.ZUnionStore(ctx, window, &redis.ZStore{Keys: keys}).Err(); err != nil {
			DefaultMetrics.ObserveRedis("zunionstore_hot_window", time.Since(startedAt))
			return nil, err
		}
		DefaultMetrics.ObserveRedis("zunionstore_hot_window", time.Since(startedAt))
		if err := s.client.Expire(ctx, window, 90*time.Second).Err(); err != nil {
			return nil, err
		}
	}
	startedAt = time.Now()
	values, err := s.client.ZRevRangeByScoreWithScores(ctx, window, &redis.ZRangeBy{
		Max: "+inf", Min: "(0", Offset: 0, Count: int64(limit),
	}).Result()
	DefaultMetrics.ObserveRedis("zrevrangebyscore_hot_window", time.Since(startedAt))
	if err != nil {
		return nil, err
	}
	result := make([]HotCandidate, 0, len(values))
	for _, value := range values {
		questionID, err := strconv.ParseUint(fmt.Sprint(value.Member), 10, 64)
		if err != nil {
			continue
		}
		result = append(result, HotCandidate{QuestionID: questionID, Score: value.Score})
	}
	return result, nil
}

func hotHourKey(at time.Time) string {
	return "askflow:feed:hot:hour:v1:" + at.UTC().Format("2006010215")
}

func hotWindowKey(at time.Time) string {
	return "askflow:feed:hot:window:v1:" + at.UTC().Truncate(time.Hour).Format("2006010215")
}
