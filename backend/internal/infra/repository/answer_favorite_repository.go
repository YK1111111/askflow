package repository

import (
	"context"
	"time"

	domainanswerfavorite "askflow/internal/domain/answerfavorite"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AnswerFavoriteRepository struct{ db *gorm.DB }

func NewAnswerFavoriteRepository(db *gorm.DB) *AnswerFavoriteRepository {
	return &AnswerFavoriteRepository{db: db}
}

func (r *AnswerFavoriteRepository) Create_AnswerFavorite(ctx context.Context, favorite *domainanswerfavorite.AnswerFavorite) error {
	record := model.AnswerFavorite_Domain2AnswerFavorite_Model(favorite)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domainanswerfavorite.ErrAlreadyFavorited
		}
		result = tx.Model(&model.Answer{}).
			Where("id = ?", favorite.AnswerID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count + ?", 1))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var answer model.Answer
		if err := tx.Select("id", "question_id").First(&answer, favorite.AnswerID).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerFavorited, "answer", favorite.AnswerID, outbox.AnswerPayload{AnswerID: favorite.AnswerID, QuestionID: answer.QuestionID, ActorID: favorite.UserID, OccurredAt: record.CreatedAt})
	})
	if err != nil {
		return err
	}
	favorite.ID, favorite.CreatedAt = record.ID, record.CreatedAt
	return nil
}

func (r *AnswerFavoriteRepository) Delete_AnswerFavorite(ctx context.Context, userID, answerID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("user_id = ? AND answer_id = ?", userID, answerID).Delete(&model.AnswerFavorite{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if err := tx.Model(&model.Answer{}).
			Where("id = ?", answerID).
			UpdateColumn("favorite_count", gorm.Expr("CASE WHEN favorite_count > 0 THEN favorite_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		var answer model.Answer
		if err := tx.Select("id", "question_id").First(&answer, answerID).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerUnfavorited, "answer", answerID, outbox.AnswerPayload{AnswerID: answerID, QuestionID: answer.QuestionID, ActorID: userID, OccurredAt: time.Now()})
	})
}

func (r *AnswerFavoriteRepository) Exists_AnswerFavorite(ctx context.Context, userID, answerID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AnswerFavorite{}).Where("user_id = ? AND answer_id = ?", userID, answerID).Count(&count).Error
	return count > 0, err
}

var _ domainanswerfavorite.Repository = (*AnswerFavoriteRepository)(nil)
