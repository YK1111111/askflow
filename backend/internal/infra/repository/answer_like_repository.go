package repository

import (
	"context"
	"time"

	domainanswerlike "askflow/internal/domain/answerlike"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AnswerLikeRepository 使用 GORM 持久化回答点赞关系。
type AnswerLikeRepository struct {
	db *gorm.DB
}

func NewAnswerLikeRepository(db *gorm.DB) *AnswerLikeRepository {
	return &AnswerLikeRepository{db: db}
}

func (r *AnswerLikeRepository) Create_AnswerLike(ctx context.Context, like *domainanswerlike.AnswerLike) error {
	record := model.AnswerLike_Domain2AnswerLike_Model(like)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domainanswerlike.ErrAlreadyLiked
		}
		result = tx.Model(&model.Answer{}).
			Where("id = ?", like.AnswerID).
			UpdateColumn("like_count", gorm.Expr("like_count + ?", 1))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var answer model.Answer
		if err := tx.Select("id", "question_id").First(&answer, like.AnswerID).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerLiked, "answer", like.AnswerID, outbox.AnswerPayload{AnswerID: like.AnswerID, QuestionID: answer.QuestionID, ActorID: like.UserID, OccurredAt: record.CreatedAt})
	})
	if err != nil {
		return err
	}
	like.ID = record.ID
	like.CreatedAt = record.CreatedAt
	return nil
}

func (r *AnswerLikeRepository) Delete_AnswerLike(ctx context.Context, userID, answerID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("user_id = ? AND answer_id = ?", userID, answerID).
			Delete(&model.AnswerLike{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if err := tx.Model(&model.Answer{}).
			Where("id = ?", answerID).
			UpdateColumn("like_count", gorm.Expr("CASE WHEN like_count > 0 THEN like_count - 1 ELSE 0 END")).Error; err != nil {
			return err
		}
		var answer model.Answer
		if err := tx.Select("id", "question_id").First(&answer, answerID).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerUnliked, "answer", answerID, outbox.AnswerPayload{AnswerID: answerID, QuestionID: answer.QuestionID, ActorID: userID, OccurredAt: time.Now()})
	})
}

func (r *AnswerLikeRepository) Exists_AnswerLike(ctx context.Context, userID, answerID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.AnswerLike{}).
		Where("user_id = ? AND answer_id = ?", userID, answerID).
		Count(&count).Error
	return count > 0, err
}

var _ domainanswerlike.Repository = (*AnswerLikeRepository)(nil)
