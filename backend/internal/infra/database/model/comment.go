package model

import (
	"time"

	domaincomment "askflow/internal/domain/comment"
)

// Comment 对应 MySQL 中的 answer_comments 表。
type Comment struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	AnswerID  uint64    `gorm:"not null;index:idx_comments_answer_created,priority:1"`
	AuthorID  uint64    `gorm:"not null;index"`
	Content   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null;index:idx_comments_answer_created,priority:2"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (Comment) TableName() string { return "answer_comments" }

func Comment_Domain2Comment_Model(comment *domaincomment.Comment) *Comment {
	return &Comment{ID: comment.ID, AnswerID: comment.AnswerID, AuthorID: comment.AuthorID, Content: comment.Content, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt}
}

func Comment_Model2Comment_Domain(record *Comment) *domaincomment.Comment {
	return &domaincomment.Comment{ID: record.ID, AnswerID: record.AnswerID, AuthorID: record.AuthorID, Content: record.Content, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
