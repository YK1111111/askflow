package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"askflow/internal/infra/database/model"
)

type fakeEventStore struct {
	events      []model.OutboxEvent
	publishedID string
	failedID    string
	failedCount uint32
	nextAttempt time.Time
	claimedBy   string
	releasedIDs []string
}

func (s *fakeEventStore) ClaimPending(_ context.Context, owner string, _, _ time.Time, _ int) ([]model.OutboxEvent, error) {
	s.claimedBy = owner
	return s.events, nil
}
func (s *fakeEventStore) MarkPublished(_ context.Context, _ string, id string, _ time.Time) error {
	s.publishedID = id
	return nil
}
func (s *fakeEventStore) MarkFailed(_ context.Context, _ string, id string, attempts uint32, next time.Time) error {
	s.failedID, s.failedCount, s.nextAttempt = id, attempts, next
	return nil
}
func (s *fakeEventStore) ReleaseClaims(_ context.Context, _ string, ids []string) error {
	s.releasedIDs = ids
	return nil
}

type fakePublisher struct{ err error }

func (p fakePublisher) Publish(context.Context, model.OutboxEvent) error { return p.err }

func TestWorkerMarksConfirmedEventPublished(t *testing.T) {
	store := &fakeEventStore{events: []model.OutboxEvent{{ID: "event-1"}}}
	NewWorker(store, fakePublisher{}).process(context.Background())
	if store.publishedID != "event-1" || store.failedID != "" {
		t.Fatalf("确认后的事件状态错误: published=%q failed=%q", store.publishedID, store.failedID)
	}
}

func TestWorkerSchedulesFailedEventRetry(t *testing.T) {
	store := &fakeEventStore{events: []model.OutboxEvent{{ID: "event-1", Attempts: 2}, {ID: "event-2"}}}
	before := time.Now()
	NewWorker(store, fakePublisher{err: errors.New("rabbit down")}).process(context.Background())
	if store.publishedID != "" || store.failedID != "event-1" || store.failedCount != 3 || !store.nextAttempt.After(before) {
		t.Fatalf("失败事件重试状态错误: %+v", store)
	}
	if len(store.releasedIDs) != 1 || store.releasedIDs[0] != "event-2" {
		t.Fatalf("批次中未处理的事件应立即释放: %+v", store.releasedIDs)
	}
}

func TestRetryDelayIsBounded(t *testing.T) {
	if retryDelay(1) != 2*time.Second || retryDelay(100) != 64*time.Second {
		t.Fatalf("重试退避不符合预期: first=%v max=%v", retryDelay(1), retryDelay(100))
	}
}
