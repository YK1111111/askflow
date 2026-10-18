package outbox

import (
	"context"
	"testing"
	"time"
)

type fakeCleanupStore struct {
	results []int64
	calls   int
	limits  []int
	cutoffs []time.Time
}

func (s *fakeCleanupStore) DeletePublishedBefore(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	s.limits = append(s.limits, limit)
	s.cutoffs = append(s.cutoffs, cutoff)
	result := s.results[s.calls]
	s.calls++
	return result, nil
}

func TestCleanupWorkerDeletesInBatches(t *testing.T) {
	store := &fakeCleanupStore{results: []int64{2, 2, 1}}
	worker := NewCleanupWorker(store, 7*24*time.Hour, time.Hour, 2)
	before := time.Now().Add(-7 * 24 * time.Hour)
	worker.process(context.Background())
	after := time.Now().Add(-7 * 24 * time.Hour)

	if store.calls != 3 {
		t.Fatalf("应持续分批清理到不足一个批次，实际调用 %d 次", store.calls)
	}
	for i := range store.limits {
		if store.limits[i] != 2 || store.cutoffs[i].Before(before) || store.cutoffs[i].After(after) {
			t.Fatalf("清理参数不符合预期: limits=%+v cutoffs=%+v", store.limits, store.cutoffs)
		}
	}
}
