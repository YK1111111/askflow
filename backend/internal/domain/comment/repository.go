package comment

import "context"

// Repository 定义评论持久化所需的能力。
type Repository interface {
	Create_Comment(ctx context.Context, comment *Comment) error
	Find_CommentsByAnswerID(ctx context.Context, answerID uint64) ([]*Comment, error)
}
