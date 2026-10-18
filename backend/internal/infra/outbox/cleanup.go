package outbox

import (
	"context"
	"log/slog"
	"time"
)

type CleanupStore interface {
	DeletePublishedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}

// CleanupWorker 分批清理超过保留期且已经发布成功的 Outbox 事件。
type CleanupWorker struct {
	store     CleanupStore
	retention time.Duration
	interval  time.Duration
	batchSize int
}

func NewCleanupWorker(store CleanupStore, retention, interval time.Duration, batchSize int) *CleanupWorker {
	return &CleanupWorker{store: store, retention: retention, interval: interval, batchSize: batchSize}
}

func (w *CleanupWorker) Run(ctx context.Context) {
	w.process(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *CleanupWorker) process(ctx context.Context) {
	cutoff := time.Now().Add(-w.retention)
	var total int64
	for {
		deleted, err := w.store.DeletePublishedBefore(ctx, cutoff, w.batchSize)
		if err != nil {
			slog.Error("清理已发布 Outbox 事件失败", "error", err)
			return
		}
		total += deleted
		if deleted < int64(w.batchSize) {
			break
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	if total > 0 {
		slog.Info("已清理过期 Outbox 事件", "count", total, "cutoff", cutoff)
	}
}
