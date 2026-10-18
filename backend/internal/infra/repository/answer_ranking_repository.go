package repository

import (
	"context"
	"errors"
	"time"

	domainanswerranking "askflow/internal/domain/answerranking"

	"gorm.io/gorm"
)

type AnswerRankingRepository struct{ db *gorm.DB }

func NewAnswerRankingRepository(db *gorm.DB) *AnswerRankingRepository {
	return &AnswerRankingRepository{db: db}
}

func (r *AnswerRankingRepository) Find_AnswerForQuestion(ctx context.Context, questionID, answerID uint64) (*domainanswerranking.Item, error) {
	var item domainanswerranking.Item
	err := r.db.WithContext(ctx).Table("answers AS a").
		Select(`a.id, a.question_id, a.content, a.author_id,
			u.username AS author_name, u.nickname AS author_nick, u.avatar_url AS author_avatar,
			a.like_count, a.comment_count, a.favorite_count, a.created_at, a.updated_at`).
		Joins("JOIN users u ON u.id = a.author_id").
		Where("a.id = ?", answerID).
		Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainanswerranking.ErrAnchorNotFound
	}
	if err != nil {
		return nil, err
	}
	if item.QuestionID != questionID {
		return nil, domainanswerranking.ErrAnchorWrongQuestion
	}
	return &item, nil
}

func (r *AnswerRankingRepository) Find_RankedAnswers(ctx context.Context, questionID, excludeAnswerID uint64, asOf time.Time, cursor *domainanswerranking.Cursor, limit int) ([]domainanswerranking.Item, error) {
	query := `
WITH scored_answers AS (
    SELECT
        a.id, a.question_id, a.content, a.author_id,
        u.username AS author_name, u.nickname AS author_nick, u.avatar_url AS author_avatar,
        a.like_count, a.comment_count, a.favorite_count, a.created_at, a.updated_at,
        (1 + a.like_count * 2 + a.comment_count * 3 + a.favorite_count * 2) /
        POW(2 + TIMESTAMPDIFF(SECOND, a.created_at, ?) / 86400, 1.2) AS quality_score
    FROM answers a
    JOIN users u ON u.id = a.author_id
    WHERE a.question_id = ? AND (? = 0 OR a.id <> ?)
)
SELECT * FROM scored_answers WHERE 1 = 1`
	args := []any{asOf, questionID, excludeAnswerID, excludeAnswerID}
	if cursor != nil && cursor.Score > 0 {
		query += " AND (quality_score < ? OR (quality_score = ? AND id < ?))"
		args = append(args, cursor.Score, cursor.Score, cursor.AnswerID)
	}
	query += " ORDER BY quality_score DESC, id DESC LIMIT ?"
	args = append(args, limit)

	var items []domainanswerranking.Item
	err := r.db.WithContext(ctx).Raw(query, args...).Scan(&items).Error
	for i := range items {
		items[i].RankedAt = asOf
	}
	return items, err
}

var _ domainanswerranking.Repository = (*AnswerRankingRepository)(nil)
