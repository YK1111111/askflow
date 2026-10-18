// Package answerlike 编排回答点赞相关的应用用例。
package answerlike

import (
	"context"

	domainanswer "askflow/internal/domain/answer"
	domainanswerlike "askflow/internal/domain/answerlike"
)

type StatusOutput struct {
	Liked bool `json:"liked"`
}

// Service 实现点赞、取消点赞和点赞状态查询用例。
type Service struct {
	likeRepository   domainanswerlike.Repository
	answerRepository domainanswer.Repository
}

func NewService(likeRepository domainanswerlike.Repository, answerRepository domainanswer.Repository) *Service {
	return &Service{likeRepository: likeRepository, answerRepository: answerRepository}
}

func (s *Service) Like_Answer(ctx context.Context, userID, answerID uint64) error {
	newLike, err := domainanswerlike.New(userID, answerID)
	if err != nil {
		return err
	}
	if _, err := s.answerRepository.Find_AnswerByID(ctx, answerID); err != nil {
		return err
	}
	return s.likeRepository.Create_AnswerLike(ctx, newLike)
}

func (s *Service) Unlike_Answer(ctx context.Context, userID, answerID uint64) error {
	if _, err := domainanswerlike.New(userID, answerID); err != nil {
		return err
	}
	return s.likeRepository.Delete_AnswerLike(ctx, userID, answerID)
}

func (s *Service) Get_AnswerLikeStatus(ctx context.Context, userID, answerID uint64) (StatusOutput, error) {
	if _, err := domainanswerlike.New(userID, answerID); err != nil {
		return StatusOutput{}, err
	}
	liked, err := s.likeRepository.Exists_AnswerLike(ctx, userID, answerID)
	if err != nil {
		return StatusOutput{}, err
	}
	return StatusOutput{Liked: liked}, nil
}
