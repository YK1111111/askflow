package repository

import (
	"context"
	"errors"

	domainanswer "askflow/internal/domain/answer"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
)

// AnswerRepository 使用 GORM 持久化回答。
type AnswerRepository struct {
	db *gorm.DB
}

func (r *AnswerRepository) Find_AnswerByID(ctx context.Context, answerID uint64) (*domainanswer.Answer, error) {
	var record model.Answer
	if err := r.db.WithContext(ctx).First(&record, answerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainanswer.ErrNotFound
		}
		return nil, err
	}
	return model.Answer_Model2Answer_Domain(&record), nil
}

func NewAnswerRepository(db *gorm.DB) *AnswerRepository {
	return &AnswerRepository{db: db}
}

func (r *AnswerRepository) Create_Answer(ctx context.Context, answer *domainanswer.Answer) error {
	record := model.Answer_Domain2Answer_Model(answer)
	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerPublished, "answer", record.ID, outbox.AnswerPayload{
			AnswerID: record.ID, QuestionID: record.QuestionID, AuthorID: record.AuthorID, OccurredAt: record.CreatedAt,
		})
	}); err != nil {
		return err
	}

	answer.ID = record.ID
	answer.CreatedAt = record.CreatedAt
	answer.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *AnswerRepository) Find_AnswersByQuestionID(ctx context.Context, questionID uint64) ([]*domainanswer.Answer, error) {
	var records []model.Answer
	if err := r.db.WithContext(ctx).
		Where("question_id = ?", questionID).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	answers := make([]*domainanswer.Answer, 0, len(records))
	for i := range records {
		answers = append(answers, model.Answer_Model2Answer_Domain(&records[i]))
	}
	return answers, nil
}

var _ domainanswer.Repository = (*AnswerRepository)(nil)
