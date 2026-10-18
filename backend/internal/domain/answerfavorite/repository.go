package answerfavorite

import (
	"context"
	"errors"
)

var ErrAlreadyFavorited = errors.New("已经收藏该回答")

// Repository 定义回答收藏关系持久化所需的能力。
type Repository interface {
	Create_AnswerFavorite(ctx context.Context, favorite *AnswerFavorite) error
	Delete_AnswerFavorite(ctx context.Context, userID, answerID uint64) error
	Exists_AnswerFavorite(ctx context.Context, userID, answerID uint64) (bool, error)
}
