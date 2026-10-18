package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"askflow/internal/infra/config"
)

func TestRedisStoreIntegration(t *testing.T) {
	addr := os.Getenv("ASKFLOW_REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("未设置 ASKFLOW_REDIS_TEST_ADDR，跳过 Redis 集成测试")
	}

	store := NewRedisStore(config.RedisConfig{Addr: addr})
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	key := fmt.Sprintf("askflow:test:cache:%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = store.Delete(ctx, key) })

	if err := store.Set(ctx, key, []byte("value"), time.Minute); err != nil {
		t.Fatalf("写入 Redis 失败: %v", err)
	}
	value, found, err := store.Get(ctx, key)
	if err != nil || !found || string(value) != "value" {
		t.Fatalf("读取 Redis 结果错误: value=%q found=%v err=%v", value, found, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("删除 Redis 键失败: %v", err)
	}
	if _, found, err := store.Get(ctx, key); err != nil || found {
		t.Fatalf("Redis 键删除后仍然存在: found=%v err=%v", found, err)
	}
}

func TestRedisHotWindowIsIdempotent(t *testing.T) {
	addr := os.Getenv("ASKFLOW_REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("未设置 ASKFLOW_REDIS_TEST_ADDR，跳过 Redis 集成测试")
	}
	store := NewRedisStore(config.RedisConfig{Addr: addr})
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	now := time.Now().UTC()
	questionID := uint64(now.UnixNano())
	eventID := fmt.Sprintf("hot-test-%d", now.UnixNano())
	keys := []string{hotHourKey(now), hotWindowKey(now), "askflow:event:processed:v1:" + eventID}
	t.Cleanup(func() { _ = store.client.Del(ctx, keys...).Err() })

	if err := store.ApplyHotDelta(ctx, eventID, questionID, 2, now); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyHotDelta(ctx, eventID, questionID, 2, now); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.FindHotWindow(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range candidates {
		if candidate.QuestionID == questionID {
			found = true
			if candidate.Score != 2 {
				t.Fatalf("重复事件不应重复加分，实际分数 %v", candidate.Score)
			}
		}
	}
	if !found {
		t.Fatalf("24 小时窗口未找到测试问题: %+v", candidates)
	}
	ttl, err := store.client.TTL(ctx, hotHourKey(now)).Result()
	if err != nil || ttl <= 47*time.Hour || ttl > hotBucketTTL {
		t.Fatalf("小时桶 TTL 错误: ttl=%v err=%v", ttl, err)
	}
}
