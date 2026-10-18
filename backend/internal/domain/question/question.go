// Package question 保存问题领域模型和业务规则。
package question

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrAuthorRequired  = errors.New("问题作者不能为空")
	ErrTitleRequired   = errors.New("问题标题不能为空")
	ErrTitleTooLong    = errors.New("问题标题不能超过 100 个字符")
	ErrContentRequired = errors.New("问题内容不能为空")
	ErrContentTooLong  = errors.New("问题内容不能超过 5000 个字符")
)

// Question 是问题领域实体。
type Question struct {
	ID        uint64
	AuthorID  uint64
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New 创建一个尚未持久化的问题。
func New(authorID uint64, title, content string) (*Question, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)

	if authorID == 0 {
		return nil, ErrAuthorRequired
	}
	if title == "" {
		return nil, ErrTitleRequired
	}
	if utf8.RuneCountInString(title) > 100 {
		return nil, ErrTitleTooLong
	}
	if content == "" {
		return nil, ErrContentRequired
	}
	if utf8.RuneCountInString(content) > 5000 {
		return nil, ErrContentTooLong
	}

	now := time.Now()
	return &Question{
		AuthorID:  authorID,
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
