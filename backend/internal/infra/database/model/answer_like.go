package model

import (
	"time"

	domainanswerlike "askflow/internal/domain/answerlike"
)

// AnswerLike 对应 MySQL 中的 answer_likes 表。
type AnswerLike struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"not null;uniqueIndex:idx_answer_likes_pair,priority:1;index"`
	AnswerID  uint64    `gorm:"not null;uniqueIndex:idx_answer_likes_pair,priority:2;index"`
	CreatedAt time.Time `gorm:"not null"`
}

func (AnswerLike) TableName() string {
	return "answer_likes"
}

// AnswerLike_Domain2AnswerLike_Model 将点赞领域实体转换为数据库模型。
func AnswerLike_Domain2AnswerLike_Model(like *domainanswerlike.AnswerLike) *AnswerLike {
	return &AnswerLike{ID: like.ID, UserID: like.UserID, AnswerID: like.AnswerID, CreatedAt: like.CreatedAt}
}

// AnswerLike_Model2AnswerLike_Domain 将点赞数据库模型还原为领域实体。
func AnswerLike_Model2AnswerLike_Domain(record *AnswerLike) *domainanswerlike.AnswerLike {
	return &domainanswerlike.AnswerLike{ID: record.ID, UserID: record.UserID, AnswerID: record.AnswerID, CreatedAt: record.CreatedAt}
}
