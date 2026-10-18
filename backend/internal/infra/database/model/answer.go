package model

import (
	"time"

	domainanswer "askflow/internal/domain/answer"
)

// Answer 对应 MySQL 中的 answers 表。
type Answer struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement"`
	QuestionID    uint64    `gorm:"not null;index:idx_answers_question_created,priority:1"`
	AuthorID      uint64    `gorm:"not null;index:idx_answers_author_created,priority:1"`
	Content       string    `gorm:"type:text;not null"`
	LikeCount     uint64    `gorm:"not null;default:0"`
	CommentCount  uint64    `gorm:"not null;default:0"`
	FavoriteCount uint64    `gorm:"not null;default:0"`
	CreatedAt     time.Time `gorm:"not null;index:idx_answers_question_created,priority:2,sort:desc;index:idx_answers_author_created,priority:2,sort:desc"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (Answer) TableName() string {
	return "answers"
}

// Answer_Domain2Answer_Model 将回答领域实体转换为回答数据库模型。
func Answer_Domain2Answer_Model(answer *domainanswer.Answer) *Answer {
	return &Answer{
		ID:            answer.ID,
		QuestionID:    answer.QuestionID,
		AuthorID:      answer.AuthorID,
		Content:       answer.Content,
		LikeCount:     answer.LikeCount,
		CommentCount:  answer.CommentCount,
		FavoriteCount: answer.FavoriteCount,
		CreatedAt:     answer.CreatedAt,
		UpdatedAt:     answer.UpdatedAt,
	}
}

// Answer_Model2Answer_Domain 将回答数据库模型还原为回答领域实体。
func Answer_Model2Answer_Domain(record *Answer) *domainanswer.Answer {
	return &domainanswer.Answer{
		ID:            record.ID,
		QuestionID:    record.QuestionID,
		AuthorID:      record.AuthorID,
		Content:       record.Content,
		LikeCount:     record.LikeCount,
		CommentCount:  record.CommentCount,
		FavoriteCount: record.FavoriteCount,
		CreatedAt:     record.CreatedAt,
		UpdatedAt:     record.UpdatedAt,
	}
}
