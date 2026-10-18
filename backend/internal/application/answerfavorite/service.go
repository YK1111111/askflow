// Package answerfavorite 编排回答收藏相关的应用用例。
package answerfavorite

import (
	"context"

	domainanswer "askflow/internal/domain/answer"
	domainanswerfavorite "askflow/internal/domain/answerfavorite"
)

type StatusOutput struct {
	Favorited bool `json:"favorited"`
}

type Service struct {
	favoriteRepository domainanswerfavorite.Repository
	answerRepository   domainanswer.Repository
}

func NewService(favoriteRepository domainanswerfavorite.Repository, answerRepository domainanswer.Repository) *Service {
	return &Service{favoriteRepository: favoriteRepository, answerRepository: answerRepository}
}

func (s *Service) Favorite_Answer(ctx context.Context, userID, answerID uint64) error {
	newFavorite, err := domainanswerfavorite.New(userID, answerID)
	if err != nil {
		return err
	}
	if _, err := s.answerRepository.Find_AnswerByID(ctx, answerID); err != nil {
		return err
	}
	return s.favoriteRepository.Create_AnswerFavorite(ctx, newFavorite)
}

func (s *Service) Unfavorite_Answer(ctx context.Context, userID, answerID uint64) error {
	if _, err := domainanswerfavorite.New(userID, answerID); err != nil {
		return err
	}
	return s.favoriteRepository.Delete_AnswerFavorite(ctx, userID, answerID)
}

func (s *Service) Get_AnswerFavoriteStatus(ctx context.Context, userID, answerID uint64) (StatusOutput, error) {
	if _, err := domainanswerfavorite.New(userID, answerID); err != nil {
		return StatusOutput{}, err
	}
	favorited, err := s.favoriteRepository.Exists_AnswerFavorite(ctx, userID, answerID)
	if err != nil {
		return StatusOutput{}, err
	}
	return StatusOutput{Favorited: favorited}, nil
}
