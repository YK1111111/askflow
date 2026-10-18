// Package follow 保存用户关注关系的领域模型和业务规则。
package follow

import (
	"errors"
	"time"
)

var (
	ErrFollowerRequired = errors.New("关注者不能为空")
	ErrFolloweeRequired = errors.New("被关注用户不能为空")
	ErrCannotFollowSelf = errors.New("不能关注自己")
)

// Follow 表示一个用户对另一个用户的关注关系。
type Follow struct {
	ID         uint64
	FollowerID uint64
	FolloweeID uint64
	CreatedAt  time.Time
}

// New 创建一个尚未持久化的用户关注关系。
func New(followerID, followeeID uint64) (*Follow, error) {
	if followerID == 0 {
		return nil, ErrFollowerRequired
	}
	if followeeID == 0 {
		return nil, ErrFolloweeRequired
	}
	if followerID == followeeID {
		return nil, ErrCannotFollowSelf
	}

	return &Follow{
		FollowerID: followerID,
		FolloweeID: followeeID,
		CreatedAt:  time.Now(),
	}, nil
}
