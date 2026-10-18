package user

import (
	"context"
	"errors"
)

var (
	ErrNotFound              = errors.New("用户不存在")
	ErrUsernameAlreadyExists = errors.New("用户名已存在")
)

// Repository 定义用户持久化所需的能力。
type Repository interface {
	Create_User(ctx context.Context, user *User) error
	Find_UserByID(ctx context.Context, userID uint64) (*User, error)
	Find_UserByUsername(ctx context.Context, username string) (*User, error)
	Update_User_Profile(ctx context.Context, user *User) error
}
