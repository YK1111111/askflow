// Package answerranking 定义问题内回答排序的读取模型。
package answerranking

import (
	"context"
	"errors"
	"time"
)

var (
	ErrAnchorNotFound      = errors.New("置顶回答不存在")
	ErrAnchorWrongQuestion = errors.New("置顶回答不属于当前问题")
	ErrInvalidCursor       = errors.New("回答列表游标不正确")
)

type Cursor struct {
	Score      float64
	AnswerID   uint64
	QuestionID uint64
	AnchorID   uint64
	AsOf       time.Time
}

type Item struct {
	ID            uint64
	QuestionID    uint64
	Content       string
	AuthorID      uint64
	AuthorName    string
	AuthorNick    string
	AuthorAvatar  string
	LikeCount     uint64
	CommentCount  uint64
	FavoriteCount uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	QualityScore  float64
	RankedAt      time.Time
}

type Repository interface {
	Find_AnswerForQuestion(ctx context.Context, questionID, answerID uint64) (*Item, error)
	Find_RankedAnswers(ctx context.Context, questionID, excludeAnswerID uint64, asOf time.Time, cursor *Cursor, limit int) ([]Item, error)
}
