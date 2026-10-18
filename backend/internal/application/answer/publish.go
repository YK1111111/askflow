// Package answer 编排回答相关的应用用例。
package answer

import (
	"context"

	domainanswer "askflow/internal/domain/answer"
	domainquestion "askflow/internal/domain/question"
)

type PublishInput struct {
	QuestionID uint64
	AuthorID   uint64
	Content    string
}

type PublishOutput struct {
	ID            uint64 `json:"id"`
	LikeCount     uint64 `json:"like_count"`
	CommentCount  uint64 `json:"comment_count"`
	FavoriteCount uint64 `json:"favorite_count"`
}

// PublishService 实现发布回答用例。
type PublishService struct {
	answerRepository   domainanswer.Repository
	questionRepository domainquestion.Repository
}

func NewPublishService(
	answerRepository domainanswer.Repository,
	questionRepository domainquestion.Repository,
) *PublishService {
	return &PublishService{
		answerRepository:   answerRepository,
		questionRepository: questionRepository,
	}
}

func (s *PublishService) Publish_Answer(ctx context.Context, input PublishInput) (PublishOutput, error) {
	if _, err := s.questionRepository.Find_QuestionByID(ctx, input.QuestionID); err != nil {
		return PublishOutput{}, err
	}

	newAnswer, err := domainanswer.New(input.QuestionID, input.AuthorID, input.Content)
	if err != nil {
		return PublishOutput{}, err
	}
	if err := s.answerRepository.Create_Answer(ctx, newAnswer); err != nil {
		return PublishOutput{}, err
	}
	return PublishOutput{
		ID:            newAnswer.ID,
		LikeCount:     newAnswer.LikeCount,
		CommentCount:  newAnswer.CommentCount,
		FavoriteCount: newAnswer.FavoriteCount,
	}, nil
}
