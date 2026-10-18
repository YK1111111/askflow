package repository

import (
	"context"

	domainquestionview "askflow/internal/domain/questionview"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
)

type QuestionViewRepository struct{ db *gorm.DB }

func NewQuestionViewRepository(db *gorm.DB) *QuestionViewRepository {
	return &QuestionViewRepository{db: db}
}

func (r *QuestionViewRepository) Record_QuestionView(ctx context.Context, questionID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		view := &model.QuestionView{QuestionID: questionID}
		if err := tx.Create(view).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.QuestionViewed, "question", questionID, outbox.QuestionPayload{QuestionID: questionID, OccurredAt: view.CreatedAt})
	})
}

var _ domainquestionview.Repository = (*QuestionViewRepository)(nil)
