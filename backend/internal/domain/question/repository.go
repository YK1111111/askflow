package question

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("问题不存在")

// Repository 定义问题持久化所需的能力。
type Repository interface {
	Create_Question(ctx context.Context, question *Question) error
	Find_QuestionByID(ctx context.Context, questionID uint64) (*Question, error)
}
