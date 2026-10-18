package cache

import (
	"context"

	domainanswer "askflow/internal/domain/answer"
	domainanswerfavorite "askflow/internal/domain/answerfavorite"
	domainanswerlike "askflow/internal/domain/answerlike"
	domaincomment "askflow/internal/domain/comment"
)

type AnswerLikeRepository struct {
	next        domainanswerlike.Repository
	answers     domainanswer.Repository
	invalidator AnswerStatsInvalidator
	ranks       AnswerRankInvalidator
}

func NewAnswerLikeRepository(next domainanswerlike.Repository, answers domainanswer.Repository, invalidator AnswerStatsInvalidator, ranks AnswerRankInvalidator) *AnswerLikeRepository {
	return &AnswerLikeRepository{next: next, answers: answers, invalidator: invalidator, ranks: ranks}
}

func (r *AnswerLikeRepository) Create_AnswerLike(ctx context.Context, like *domainanswerlike.AnswerLike) error {
	if err := r.next.Create_AnswerLike(ctx, like); err != nil {
		return err
	}
	r.invalidate(ctx, like.AnswerID)
	return nil
}

func (r *AnswerLikeRepository) Delete_AnswerLike(ctx context.Context, userID, answerID uint64) error {
	if err := r.next.Delete_AnswerLike(ctx, userID, answerID); err != nil {
		return err
	}
	r.invalidate(ctx, answerID)
	return nil
}

func (r *AnswerLikeRepository) invalidate(ctx context.Context, answerID uint64) {
	answer, err := r.answers.Find_AnswerByID(ctx, answerID)
	r.invalidator.Invalidate_AnswerStats(ctx, answerID)
	if err == nil {
		r.ranks.Invalidate_AnswerRank(ctx, answer.QuestionID)
	}
}

func (r *AnswerLikeRepository) Exists_AnswerLike(ctx context.Context, userID, answerID uint64) (bool, error) {
	return r.next.Exists_AnswerLike(ctx, userID, answerID)
}

type AnswerFavoriteRepository struct {
	next        domainanswerfavorite.Repository
	answers     domainanswer.Repository
	invalidator AnswerStatsInvalidator
	ranks       AnswerRankInvalidator
}

func NewAnswerFavoriteRepository(next domainanswerfavorite.Repository, answers domainanswer.Repository, invalidator AnswerStatsInvalidator, ranks AnswerRankInvalidator) *AnswerFavoriteRepository {
	return &AnswerFavoriteRepository{next: next, answers: answers, invalidator: invalidator, ranks: ranks}
}

func (r *AnswerFavoriteRepository) Create_AnswerFavorite(ctx context.Context, favorite *domainanswerfavorite.AnswerFavorite) error {
	if err := r.next.Create_AnswerFavorite(ctx, favorite); err != nil {
		return err
	}
	r.invalidate(ctx, favorite.AnswerID)
	return nil
}

func (r *AnswerFavoriteRepository) Delete_AnswerFavorite(ctx context.Context, userID, answerID uint64) error {
	if err := r.next.Delete_AnswerFavorite(ctx, userID, answerID); err != nil {
		return err
	}
	r.invalidate(ctx, answerID)
	return nil
}

func (r *AnswerFavoriteRepository) invalidate(ctx context.Context, answerID uint64) {
	answer, err := r.answers.Find_AnswerByID(ctx, answerID)
	r.invalidator.Invalidate_AnswerStats(ctx, answerID)
	if err == nil {
		r.ranks.Invalidate_AnswerRank(ctx, answer.QuestionID)
	}
}

func (r *AnswerFavoriteRepository) Exists_AnswerFavorite(ctx context.Context, userID, answerID uint64) (bool, error) {
	return r.next.Exists_AnswerFavorite(ctx, userID, answerID)
}

type CommentRepository struct {
	next        domaincomment.Repository
	answers     domainanswer.Repository
	invalidator AnswerStatsInvalidator
	ranks       AnswerRankInvalidator
}

func NewCommentRepository(next domaincomment.Repository, answers domainanswer.Repository, invalidator AnswerStatsInvalidator, ranks AnswerRankInvalidator) *CommentRepository {
	return &CommentRepository{next: next, answers: answers, invalidator: invalidator, ranks: ranks}
}

func (r *CommentRepository) Create_Comment(ctx context.Context, comment *domaincomment.Comment) error {
	if err := r.next.Create_Comment(ctx, comment); err != nil {
		return err
	}
	answer, err := r.answers.Find_AnswerByID(ctx, comment.AnswerID)
	r.invalidator.Invalidate_AnswerStats(ctx, comment.AnswerID)
	if err == nil {
		r.ranks.Invalidate_AnswerRank(ctx, answer.QuestionID)
	}
	return nil
}

func (r *CommentRepository) Find_CommentsByAnswerID(ctx context.Context, answerID uint64) ([]*domaincomment.Comment, error) {
	return r.next.Find_CommentsByAnswerID(ctx, answerID)
}

var _ domainanswerlike.Repository = (*AnswerLikeRepository)(nil)
var _ domainanswerfavorite.Repository = (*AnswerFavoriteRepository)(nil)
var _ domaincomment.Repository = (*CommentRepository)(nil)
