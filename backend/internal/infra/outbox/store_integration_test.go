package outbox

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"askflow/internal/infra/database"
	"askflow/internal/infra/database/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestStoreClaimsAreExclusive 使用真实 MySQL 验证多实例认领互斥和租约恢复。
func TestStoreClaimsAreExclusive(t *testing.T) {
	dsn := os.Getenv("ASKFLOW_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 ASKFLOW_MYSQL_TEST_DSN，跳过 MySQL 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	createdAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	events := make([]model.OutboxEvent, 6)
	ids := make([]string, 0, len(events))
	for i := range events {
		id, idErr := newID()
		if idErr != nil {
			t.Fatal(idErr)
		}
		ids = append(ids, id)
		events[i] = model.OutboxEvent{
			ID: id, AggregateType: "test", AggregateID: uint64(i + 1),
			EventType: "test.claim", Payload: []byte(`{}`),
			NextAttemptAt: createdAt, CreatedAt: createdAt.Add(time.Duration(i) * time.Millisecond),
		}
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Where("id IN ?", ids).Delete(&model.OutboxEvent{}).Error })

	type claimResult struct {
		events []model.OutboxEvent
		err    error
	}
	start := make(chan struct{})
	results := make(chan claimResult, 2)
	var workers sync.WaitGroup
	for _, owner := range []string{"worker-a", "worker-b"} {
		workers.Add(1)
		go func(owner string) {
			defer workers.Done()
			<-start
			claimed, claimErr := NewStore(db).ClaimPending(context.Background(), owner, time.Now(), time.Now().Add(time.Minute), 3)
			results <- claimResult{events: claimed, err: claimErr}
		}(owner)
	}
	close(start)
	workers.Wait()
	close(results)

	claimedIDs := map[string]struct{}{}
	for result := range results {
		if result.err != nil {
			t.Fatalf("并发认领失败: %v", result.err)
		}
		if len(result.events) > 3 {
			t.Fatalf("单个 Worker 不应超过批次上限，实际为 %d", len(result.events))
		}
		for _, event := range result.events {
			if _, duplicated := claimedIDs[event.ID]; duplicated {
				t.Fatalf("同一事件被多个 Worker 认领: %s", event.ID)
			}
			claimedIDs[event.ID] = struct{}{}
		}
	}
	if len(claimedIDs) < 3 || len(claimedIDs) > len(events) {
		t.Fatalf("并发首轮至少应成功认领一个批次且不能超出总数: %+v", claimedIDs)
	}

	if err := NewStore(db).MarkPublished(context.Background(), "wrong-worker", events[0].ID, time.Now()); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("非租约持有者不应完成事件，实际错误: %v", err)
	}
	owner := "worker-held"
	if err := db.Model(&model.OutboxEvent{}).Where("id IN ?", ids).
		Updates(map[string]any{"lock_owner": owner, "locked_until": time.Now().Add(time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.OutboxEvent{}).Where("id = ?", events[0].ID).
		Update("locked_until", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	reclaimed, err := NewStore(db).ClaimPending(context.Background(), "worker-c", time.Now(), time.Now().Add(time.Minute), 1)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].ID != events[0].ID {
		t.Fatalf("过期租约应被其他 Worker 接管: %+v, %v", reclaimed, err)
	}
}

func TestStoreDeletesOnlyExpiredPublishedEvents(t *testing.T) {
	dsn := os.Getenv("ASKFLOW_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 ASKFLOW_MYSQL_TEST_DSN，跳过 MySQL 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Date(2000, 2, 1, 0, 0, 0, 0, time.UTC)
	recentTime := time.Date(2099, 2, 1, 0, 0, 0, 0, time.UTC)
	events := make([]model.OutboxEvent, 3)
	ids := make([]string, 0, len(events))
	for i := range events {
		id, idErr := newID()
		if idErr != nil {
			t.Fatal(idErr)
		}
		ids = append(ids, id)
		events[i] = model.OutboxEvent{
			ID: id, AggregateType: "test", AggregateID: uint64(i + 1),
			EventType: "test.cleanup", Payload: []byte(`{}`),
			NextAttemptAt: oldTime, CreatedAt: oldTime,
		}
	}
	events[0].PublishedAt = &oldTime
	events[1].PublishedAt = &recentTime
	if err := db.Create(&events).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Where("id IN ?", ids).Delete(&model.OutboxEvent{}).Error })

	deleted, err := NewStore(db).DeletePublishedBefore(context.Background(), time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), 10)
	if err != nil || deleted != 1 {
		t.Fatalf("应只删除过期且已发布的事件: deleted=%d err=%v", deleted, err)
	}
	var remaining int64
	if err := db.Model(&model.OutboxEvent{}).Where("id IN ?", ids).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 2 {
		t.Fatalf("近期已发布事件和未发布事件必须保留，实际剩余 %d", remaining)
	}
}
