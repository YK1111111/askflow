package model

import (
	"time"

	domainnotification "askflow/internal/domain/notification"
)

// Notification 对应用户站内通知表。
type Notification struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement"`
	EventID    string     `gorm:"type:char(32);not null;uniqueIndex"`
	UserID     uint64     `gorm:"not null;index:idx_notifications_user_id,priority:1"`
	ActorID    uint64     `gorm:"not null"`
	Type       string     `gorm:"size:64;not null"`
	AnswerID   uint64     `gorm:"not null;default:0"`
	QuestionID uint64     `gorm:"not null;default:0"`
	Content    string     `gorm:"size:255;not null"`
	ReadAt     *time.Time `gorm:"index"`
	CreatedAt  time.Time  `gorm:"not null;index:idx_notifications_user_id,priority:2,sort:desc"`
}

func (Notification) TableName() string { return "notifications" }

func Notification_Domain2Model(value *domainnotification.Notification) *Notification {
	return &Notification{ID: value.ID, EventID: value.EventID, UserID: value.UserID, ActorID: value.ActorID,
		Type: value.Type, AnswerID: value.AnswerID, QuestionID: value.QuestionID, Content: value.Content,
		ReadAt: value.ReadAt, CreatedAt: value.CreatedAt}
}

func Notification_Model2Domain(value *Notification) *domainnotification.Notification {
	return &domainnotification.Notification{ID: value.ID, EventID: value.EventID, UserID: value.UserID, ActorID: value.ActorID,
		Type: value.Type, AnswerID: value.AnswerID, QuestionID: value.QuestionID, Content: value.Content,
		ReadAt: value.ReadAt, CreatedAt: value.CreatedAt}
}
