// Package answer 保存回答领域模型和业务规则。
package answer

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const maxContentLength = 10000

var (
	ErrQuestionRequired = errors.New("回答所属问题不能为空")
	ErrAuthorRequired   = errors.New("回答作者不能为空")
	ErrContentRequired  = errors.New("回答内容不能为空")
	ErrContentTooLong   = errors.New("回答内容不能超过 10000 个字符")
)

// Answer 是回答领域实体。
type Answer struct {
	ID            uint64
	QuestionID    uint64
	AuthorID      uint64
	Content       string
	LikeCount     uint64
	CommentCount  uint64
	FavoriteCount uint64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// New 创建一个尚未持久化的回答。
func New(questionID, authorID uint64, content string) (*Answer, error) {
	content = strings.TrimSpace(content)

	if questionID == 0 {
		return nil, ErrQuestionRequired
	}
	if authorID == 0 {
		return nil, ErrAuthorRequired
	}
	if content == "" {
		return nil, ErrContentRequired
	}
	if utf8.RuneCountInString(content) > maxContentLength {
		return nil, ErrContentTooLong
	}

	now := time.Now()
	return &Answer{
		QuestionID: questionID,
		AuthorID:   authorID,
		Content:    content,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}
