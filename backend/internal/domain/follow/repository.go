package follow

import (
	"context"
	"errors"
)

var ErrAlreadyFollowing = errors.New("已经关注该用户")

// Repository 定义用户关注关系持久化所需的能力。
type Repository interface {
	Create_Follow(ctx context.Context, follow *Follow) error
	Delete_Follow(ctx context.Context, followerID, followeeID uint64) error
	Exists_Follow(ctx context.Context, followerID, followeeID uint64) (bool, error)
	Find_FolloweeIDsByFollowerID(ctx context.Context, followerID uint64) ([]uint64, error)
}
