// Package comment 保存回答评论的领域模型和业务规则。
package comment

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const maxContentLength = 500

var (
	ErrAnswerRequired  = errors.New("评论所属回答不能为空")
	ErrAuthorRequired  = errors.New("评论作者不能为空")
	ErrContentRequired = errors.New("评论内容不能为空")
	ErrContentTooLong  = errors.New("评论内容不能超过 500 个字符")
)

// Comment 是回答下的一条一级评论。
type Comment struct {
	ID        uint64
	AnswerID  uint64
	AuthorID  uint64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New 创建一条尚未持久化的评论。
func New(answerID, authorID uint64, content string) (*Comment, error) {
	content = strings.TrimSpace(content)
	if answerID == 0 {
		return nil, ErrAnswerRequired
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
	return &Comment{AnswerID: answerID, AuthorID: authorID, Content: content, CreatedAt: now, UpdatedAt: now}, nil
}
