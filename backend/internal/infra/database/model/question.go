package model

import (
	"time"

	domainquestion "askflow/internal/domain/question"
)

// Question 对应 MySQL 中的 questions 表。
type Question struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	AuthorID  uint64    `gorm:"not null;index"`
	Title     string    `gorm:"size:100;not null"`
	Content   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"not null;index"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (Question) TableName() string {
	return "questions"
}

// Question_Domain2Question_Model 将问题领域实体转换为问题数据库模型。
func Question_Domain2Question_Model(question *domainquestion.Question) *Question {
	return &Question{
		ID:        question.ID,
		AuthorID:  question.AuthorID,
		Title:     question.Title,
		Content:   question.Content,
		CreatedAt: question.CreatedAt,
		UpdatedAt: question.UpdatedAt,
	}
}

// Question_Model2Question_Domain 将问题数据库模型还原为问题领域实体。
func Question_Model2Question_Domain(model *Question) *domainquestion.Question {
	return &domainquestion.Question{
		ID:        model.ID,
		AuthorID:  model.AuthorID,
		Title:     model.Title,
		Content:   model.Content,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
