package model

import "time"

// OutboxEvent 保存与业务事务一同提交、等待投递到 RabbitMQ 的领域事件。
type OutboxEvent struct {
	ID            string     `gorm:"type:char(32);primaryKey"`
	AggregateType string     `gorm:"size:64;not null"`
	AggregateID   uint64     `gorm:"not null"`
	EventType     string     `gorm:"size:128;not null"`
	Payload       []byte     `gorm:"type:json;not null"`
	Attempts      uint32     `gorm:"not null;default:0"`
	NextAttemptAt time.Time  `gorm:"not null;index:idx_outbox_pending,priority:2"`
	PublishedAt   *time.Time `gorm:"index:idx_outbox_pending,priority:1"`
	LockOwner     *string    `gorm:"size:32"`
	LockedUntil   *time.Time `gorm:"index:idx_outbox_pending,priority:3"`
	CreatedAt     time.Time  `gorm:"not null;index:idx_outbox_pending,priority:4"`
}

func (OutboxEvent) TableName() string { return "outbox_events" }
