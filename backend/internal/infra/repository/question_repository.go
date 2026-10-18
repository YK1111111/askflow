package repository

import (
	"context"
	"errors"

	domainquestion "askflow/internal/domain/question"
	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
)

// QuestionRepository 使用 GORM 持久化问题。
type QuestionRepository struct {
	db *gorm.DB
}

func NewQuestionRepository(db *gorm.DB) *QuestionRepository {
	return &QuestionRepository{db: db}
}

func (r *QuestionRepository) Create_Question(ctx context.Context, question *domainquestion.Question) error {
	record := model.Question_Domain2Question_Model(question)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return err
	}

	question.ID = record.ID
	question.CreatedAt = record.CreatedAt
	question.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *QuestionRepository) Find_QuestionByID(ctx context.Context, questionID uint64) (*domainquestion.Question, error) {
	var record model.Question
	if err := r.db.WithContext(ctx).First(&record, questionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainquestion.ErrNotFound
		}
		return nil, err
	}
	return model.Question_Model2Question_Domain(&record), nil
}

var _ domainquestion.Repository = (*QuestionRepository)(nil)
