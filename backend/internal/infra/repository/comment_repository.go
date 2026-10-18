package repository

import (
	"context"

	domaincomment "askflow/internal/domain/comment"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
)

type CommentRepository struct{ db *gorm.DB }

func NewCommentRepository(db *gorm.DB) *CommentRepository { return &CommentRepository{db: db} }

func (r *CommentRepository) Create_Comment(ctx context.Context, comment *domaincomment.Comment) error {
	record := model.Comment_Domain2Comment_Model(comment)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		result := tx.Model(&model.Answer{}).
			Where("id = ?", comment.AnswerID).
			UpdateColumn("comment_count", gorm.Expr("comment_count + ?", 1))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var answer model.Answer
		if err := tx.Select("id", "question_id").First(&answer, comment.AnswerID).Error; err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.AnswerCommented, "answer", comment.AnswerID, outbox.AnswerPayload{AnswerID: comment.AnswerID, QuestionID: answer.QuestionID, ActorID: comment.AuthorID, OccurredAt: record.CreatedAt})
	})
	if err != nil {
		return err
	}
	comment.ID, comment.CreatedAt, comment.UpdatedAt = record.ID, record.CreatedAt, record.UpdatedAt
	return nil
}

func (r *CommentRepository) Find_CommentsByAnswerID(ctx context.Context, answerID uint64) ([]*domaincomment.Comment, error) {
	var records []model.Comment
	if err := r.db.WithContext(ctx).Where("answer_id = ?", answerID).Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	comments := make([]*domaincomment.Comment, 0, len(records))
	for i := range records {
		comments = append(comments, model.Comment_Model2Comment_Domain(&records[i]))
	}
	return comments, nil
}

var _ domaincomment.Repository = (*CommentRepository)(nil)
