package outbox

import (
	"context"
	"log/slog"
	"time"

	"askflow/internal/infra/database/model"
)

type EventStore interface {
	ClaimPending(ctx context.Context, owner string, now, lockedUntil time.Time, limit int) ([]model.OutboxEvent, error)
	MarkPublished(ctx context.Context, owner, id string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, owner, id string, attempts uint32, nextAttemptAt time.Time) error
	ReleaseClaims(ctx context.Context, owner string, ids []string) error
}

type Publisher interface {
	Publish(ctx context.Context, event model.OutboxEvent) error
}

type Worker struct {
	store          EventStore
	publisher      Publisher
	interval       time.Duration
	batchSize      int
	owner          string
	lease          time.Duration
	publishTimeout time.Duration
}

func NewWorker(store EventStore, publisher Publisher) *Worker {
	owner, err := newID()
	if err != nil {
		panic("生成 Outbox Worker ID 失败: " + err.Error())
	}
	return &Worker{store: store, publisher: publisher, interval: time.Second, batchSize: 20,
		owner: owner, lease: 30 * time.Second, publishTimeout: 10 * time.Second}
}

func (w *Worker) Run(ctx context.Context) {
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

func (w *Worker) process(ctx context.Context) {
	now := time.Now()
	events, err := w.store.ClaimPending(ctx, w.owner, now, now.Add(w.lease), w.batchSize)
	if err != nil {
		slog.Error("查询 Outbox 待投递事件失败", "error", err)
		return
	}
	for _, event := range events {
		publishContext, cancel := context.WithTimeout(ctx, w.publishTimeout)
		err := w.publisher.Publish(publishContext, event)
		cancel()
		if err != nil {
			attempts := event.Attempts + 1
			next := time.Now().Add(retryDelay(attempts))
			if markErr := w.store.MarkFailed(ctx, w.owner, event.ID, attempts, next); markErr != nil {
				slog.Error("更新 Outbox 重试时间失败", "event_id", event.ID, "error", markErr)
			}
			slog.Warn("投递 Outbox 事件失败", "event_id", event.ID, "event_type", event.EventType, "attempts", attempts, "error", err)
			w.releaseRemaining(ctx, events, event.ID)
			return
		}
		if err := w.store.MarkPublished(ctx, w.owner, event.ID, time.Now()); err != nil {
			slog.Error("标记 Outbox 事件已发布失败", "event_id", event.ID, "error", err)
			w.releaseRemaining(ctx, events, event.ID)
			return
		}
	}
}

func (w *Worker) releaseRemaining(ctx context.Context, events []model.OutboxEvent, currentID string) {
	ids := make([]string, 0, len(events))
	foundCurrent := false
	for _, event := range events {
		if event.ID == currentID {
			foundCurrent = true
			continue
		}
		if foundCurrent {
			ids = append(ids, event.ID)
		}
	}
	if err := w.store.ReleaseClaims(ctx, w.owner, ids); err != nil {
		slog.Error("释放未处理 Outbox 事件失败", "error", err)
	}
}

func retryDelay(attempt uint32) time.Duration {
	if attempt > 6 {
		attempt = 6
	}
	return time.Second * time.Duration(1<<attempt)
}
