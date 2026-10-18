package answerlike

import (
	"context"
	"errors"
)

var ErrAlreadyLiked = errors.New("已经点赞该回答")

// Repository 定义回答点赞关系持久化所需的能力。
type Repository interface {
	Create_AnswerLike(ctx context.Context, like *AnswerLike) error
	Delete_AnswerLike(ctx context.Context, userID, answerID uint64) error
	Exists_AnswerLike(ctx context.Context, userID, answerID uint64) (bool, error)
}
