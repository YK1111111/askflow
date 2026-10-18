package answer

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("回答不存在")

// Repository 定义回答持久化所需的能力。
type Repository interface {
	Create_Answer(ctx context.Context, answer *Answer) error
	Find_AnswerByID(ctx context.Context, answerID uint64) (*Answer, error)
	Find_AnswersByQuestionID(ctx context.Context, questionID uint64) ([]*Answer, error)
}
