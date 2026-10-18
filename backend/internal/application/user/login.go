package user

import (
	"context"
	"errors"
	"strings"

	domainuser "askflow/internal/domain/user"
)

var ErrInvalidCredentials = errors.New("用户名或密码错误")

type PasswordVerifier interface {
	Verify_Password(password, passwordHash string) bool
}

type AccessTokenIssuer interface {
	Issue_AccessToken(userID uint64, username string) (token string, expiresIn int64, err error)
}

type LoginInput struct {
	Username string
	Password string
}

type LoginOutput struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// LoginService 实现用户登录用例。
type LoginService struct {
	repository       domainuser.Repository
	passwordVerifier PasswordVerifier
	tokenIssuer      AccessTokenIssuer
}

func NewLoginService(
	repository domainuser.Repository,
	passwordVerifier PasswordVerifier,
	tokenIssuer AccessTokenIssuer,
) *LoginService {
	return &LoginService{
		repository:       repository,
		passwordVerifier: passwordVerifier,
		tokenIssuer:      tokenIssuer,
	}
}

func (s *LoginService) Login_User(ctx context.Context, input LoginInput) (LoginOutput, error) {
	username := strings.TrimSpace(input.Username)
	foundUser, err := s.repository.Find_UserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domainuser.ErrNotFound) {
			return LoginOutput{}, ErrInvalidCredentials
		}
		return LoginOutput{}, err
	}

	if !s.passwordVerifier.Verify_Password(input.Password, foundUser.PasswordHash) {
		return LoginOutput{}, ErrInvalidCredentials
	}

	token, expiresIn, err := s.tokenIssuer.Issue_AccessToken(foundUser.ID, foundUser.Username)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{AccessToken: token, ExpiresIn: expiresIn}, nil
}
