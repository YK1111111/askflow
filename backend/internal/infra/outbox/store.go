package outbox

import (
	"context"
	"errors"
	"time"

	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLeaseLost = errors.New("Outbox 事件租约已失效")

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) ClaimPending(ctx context.Context, owner string, now, lockedUntil time.Time, limit int) ([]model.OutboxEvent, error) {
	var events []model.OutboxEvent
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("published_at IS NULL AND next_attempt_at <= ? AND (locked_until IS NULL OR locked_until < ?)", now, now).
			Order("created_at ASC").Limit(limit).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		ids := make([]string, 0, len(events))
		for i := range events {
			ids = append(ids, events[i].ID)
			events[i].LockOwner = &owner
			events[i].LockedUntil = &lockedUntil
		}
		return tx.Model(&model.OutboxEvent{}).Where("id IN ?", ids).
			Updates(map[string]any{"lock_owner": owner, "locked_until": lockedUntil}).Error
	})
	return events, err
}

func (s *Store) MarkPublished(ctx context.Context, owner, id string, publishedAt time.Time) error {
	result := s.db.WithContext(ctx).Model(&model.OutboxEvent{}).
		Where("id = ? AND published_at IS NULL AND lock_owner = ?", id, owner).
		Updates(map[string]any{"published_at": publishedAt, "lock_owner": nil, "locked_until": nil})
	return leaseResult(result)
}

func (s *Store) MarkFailed(ctx context.Context, owner, id string, attempts uint32, nextAttemptAt time.Time) error {
	result := s.db.WithContext(ctx).Model(&model.OutboxEvent{}).
		Where("id = ? AND published_at IS NULL AND lock_owner = ?", id, owner).
		Updates(map[string]any{"attempts": attempts, "next_attempt_at": nextAttemptAt, "lock_owner": nil, "locked_until": nil})
	return leaseResult(result)
}

func (s *Store) ReleaseClaims(ctx context.Context, owner string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.WithContext(ctx).Model(&model.OutboxEvent{}).
		Where("id IN ? AND published_at IS NULL AND lock_owner = ?", ids, owner).
		Updates(map[string]any{"lock_owner": nil, "locked_until": nil}).Error
}

func (s *Store) DeletePublishedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	result := s.db.WithContext(ctx).Exec(`
DELETE FROM outbox_events
WHERE published_at IS NOT NULL AND published_at < ?
ORDER BY published_at ASC
LIMIT ?`, cutoff, limit)
	return result.RowsAffected, result.Error
}

func leaseResult(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLeaseLost
	}
	return nil
}
