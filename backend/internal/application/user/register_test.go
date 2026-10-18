package user

import (
	"context"
	"testing"

	domainuser "askflow/internal/domain/user"
)

type fakeRepository struct {
	created *domainuser.User
}

func (r *fakeRepository) Create_User(_ context.Context, user *domainuser.User) error {
	user.ID = 1
	r.created = user
	return nil
}

func (r *fakeRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return nil, domainuser.ErrNotFound
}

func (r *fakeRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return nil, domainuser.ErrNotFound
}

func (r *fakeRepository) Update_User_Profile(context.Context, *domainuser.User) error {
	return nil
}

type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash_Password(string) (string, error) {
	return "hashed-password", nil
}

func TestRegisterUser(t *testing.T) {
	repository := &fakeRepository{}
	service := NewRegisterService(repository, fakePasswordHasher{})

	output, err := service.Register_User(context.Background(), RegisterInput{
		Username: "alice",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("注册用户失败: %v", err)
	}
	if output.ID != 1 || repository.created.PasswordHash != "hashed-password" {
		t.Fatalf("注册结果不符合预期: %+v", output)
	}
}
