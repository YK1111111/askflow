// Package answerfavorite 保存回答收藏关系的领域模型和业务规则。
package answerfavorite

import (
	"errors"
	"time"
)

var (
	ErrUserRequired   = errors.New("收藏用户不能为空")
	ErrAnswerRequired = errors.New("被收藏回答不能为空")
)

// AnswerFavorite 表示一个用户对一个回答的收藏关系。
type AnswerFavorite struct {
	ID        uint64
	UserID    uint64
	AnswerID  uint64
	CreatedAt time.Time
}

// New 创建一个尚未持久化的回答收藏关系。
func New(userID, answerID uint64) (*AnswerFavorite, error) {
	if userID == 0 {
		return nil, ErrUserRequired
	}
	if answerID == 0 {
		return nil, ErrAnswerRequired
	}
	return &AnswerFavorite{UserID: userID, AnswerID: answerID, CreatedAt: time.Now()}, nil
}
