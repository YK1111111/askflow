// Package question 编排问题相关的应用用例。
package question

import (
	"context"

	domainquestion "askflow/internal/domain/question"
)

type PublishInput struct {
	AuthorID uint64
	Title    string
	Content  string
}

type PublishOutput struct {
	ID uint64 `json:"id"`
}

// PublishService 实现发布问题用例。
type PublishService struct {
	repository domainquestion.Repository
}

func NewPublishService(repository domainquestion.Repository) *PublishService {
	return &PublishService{repository: repository}
}

func (s *PublishService) Publish_Question(ctx context.Context, input PublishInput) (PublishOutput, error) {
	newQuestion, err := domainquestion.New(input.AuthorID, input.Title, input.Content)
	if err != nil {
		return PublishOutput{}, err
	}
	if err := s.repository.Create_Question(ctx, newQuestion); err != nil {
		return PublishOutput{}, err
	}
	return PublishOutput{ID: newQuestion.ID}, nil
}
