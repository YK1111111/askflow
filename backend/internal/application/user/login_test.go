package user

import (
	"context"
	"errors"
	"testing"

	domainuser "askflow/internal/domain/user"
)

type loginRepository struct {
	user *domainuser.User
}

func (r loginRepository) Create_User(context.Context, *domainuser.User) error {
	return nil
}

func (r loginRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	if r.user == nil {
		return nil, domainuser.ErrNotFound
	}
	return r.user, nil
}

func (r loginRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return r.user, nil
}

func (r loginRepository) Update_User_Profile(context.Context, *domainuser.User) error {
	return nil
}

type fakePasswordVerifier struct {
	valid bool
}

func (v fakePasswordVerifier) Verify_Password(string, string) bool {
	return v.valid
}

type fakeTokenIssuer struct{}

func (fakeTokenIssuer) Issue_AccessToken(uint64, string) (string, int64, error) {
	return "access-token", 900, nil
}

func TestLoginUser(t *testing.T) {
	service := NewLoginService(
		loginRepository{user: &domainuser.User{ID: 1, Username: "alice", PasswordHash: "hash"}},
		fakePasswordVerifier{valid: true},
		fakeTokenIssuer{},
	)

	output, err := service.Login_User(context.Background(), LoginInput{Username: "alice", Password: "password123"})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if output.AccessToken != "access-token" || output.ExpiresIn != 900 {
		t.Fatalf("登录结果不符合预期: %+v", output)
	}
}

func TestLoginUserRejectsWrongPassword(t *testing.T) {
	service := NewLoginService(
		loginRepository{user: &domainuser.User{ID: 1, Username: "alice", PasswordHash: "hash"}},
		fakePasswordVerifier{valid: false},
		fakeTokenIssuer{},
	)

	_, err := service.Login_User(context.Background(), LoginInput{Username: "alice", Password: "wrong-password"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("期望凭证错误，实际为 %v", err)
	}
}
