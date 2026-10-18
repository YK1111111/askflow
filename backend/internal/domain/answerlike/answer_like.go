// Package answerlike 保存回答点赞关系的领域模型和业务规则。
package answerlike

import (
	"errors"
	"time"
)

var (
	ErrUserRequired   = errors.New("点赞用户不能为空")
	ErrAnswerRequired = errors.New("被点赞回答不能为空")
)

// AnswerLike 表示一个用户对一个回答的点赞关系。
type AnswerLike struct {
	ID        uint64
	UserID    uint64
	AnswerID  uint64
	CreatedAt time.Time
}

// New 创建一个尚未持久化的回答点赞关系。
func New(userID, answerID uint64) (*AnswerLike, error) {
	if userID == 0 {
		return nil, ErrUserRequired
	}
	if answerID == 0 {
		return nil, ErrAnswerRequired
	}
	return &AnswerLike{UserID: userID, AnswerID: answerID, CreatedAt: time.Now()}, nil
}
