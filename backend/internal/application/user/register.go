// Package user 编排用户相关的应用用例。
package user

import (
	"context"
	"errors"
	"strings"

	domainuser "askflow/internal/domain/user"
)

var ErrPasswordTooShort = errors.New("密码长度不能少于 8 位")

// PasswordHasher 定义应用层需要的密码哈希能力。
type PasswordHasher interface {
	Hash_Password(password string) (string, error)
}

type RegisterInput struct {
	Username string
	Password string
}

type RegisterOutput struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
}

// RegisterService 实现用户注册用例。
type RegisterService struct {
	repository     domainuser.Repository
	passwordHasher PasswordHasher
}

func NewRegisterService(repository domainuser.Repository, passwordHasher PasswordHasher) *RegisterService {
	return &RegisterService{repository: repository, passwordHasher: passwordHasher}
}

func (s *RegisterService) Register_User(ctx context.Context, input RegisterInput) (RegisterOutput, error) {
	username := strings.TrimSpace(input.Username)
	if username == "" {
		return RegisterOutput{}, domainuser.ErrUsernameRequired
	}
	if len(input.Password) < 8 {
		return RegisterOutput{}, ErrPasswordTooShort
	}

	_, err := s.repository.Find_UserByUsername(ctx, username)
	if err == nil {
		return RegisterOutput{}, domainuser.ErrUsernameAlreadyExists
	}
	if !errors.Is(err, domainuser.ErrNotFound) {
		return RegisterOutput{}, err
	}

	passwordHash, err := s.passwordHasher.Hash_Password(input.Password)
	if err != nil {
		return RegisterOutput{}, err
	}

	newUser, err := domainuser.New(username, passwordHash)
	if err != nil {
		return RegisterOutput{}, err
	}
	if err := s.repository.Create_User(ctx, newUser); err != nil {
		return RegisterOutput{}, err
	}

	return RegisterOutput{ID: newUser.ID, Username: newUser.Username}, nil
}
