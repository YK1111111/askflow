package model

import (
	"time"

	domainanswerfavorite "askflow/internal/domain/answerfavorite"
)

// AnswerFavorite 对应 MySQL 中的 answer_favorites 表。
type AnswerFavorite struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	UserID    uint64    `gorm:"not null;uniqueIndex:idx_answer_favorites_pair,priority:1;index"`
	AnswerID  uint64    `gorm:"not null;uniqueIndex:idx_answer_favorites_pair,priority:2;index"`
	CreatedAt time.Time `gorm:"not null"`
}

func (AnswerFavorite) TableName() string { return "answer_favorites" }

func AnswerFavorite_Domain2AnswerFavorite_Model(favorite *domainanswerfavorite.AnswerFavorite) *AnswerFavorite {
	return &AnswerFavorite{ID: favorite.ID, UserID: favorite.UserID, AnswerID: favorite.AnswerID, CreatedAt: favorite.CreatedAt}
}

func AnswerFavorite_Model2AnswerFavorite_Domain(record *AnswerFavorite) *domainanswerfavorite.AnswerFavorite {
	return &domainanswerfavorite.AnswerFavorite{ID: record.ID, UserID: record.UserID, AnswerID: record.AnswerID, CreatedAt: record.CreatedAt}
}
