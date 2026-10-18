package user

import (
	"context"
	"testing"

	domainuser "askflow/internal/domain/user"
)

type profileRepository struct {
	user *domainuser.User
}

func (r *profileRepository) Create_User(context.Context, *domainuser.User) error {
	return nil
}

func (r *profileRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return r.user, nil
}

func (r *profileRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return r.user, nil
}

func (r *profileRepository) Update_User_Profile(_ context.Context, user *domainuser.User) error {
	r.user = user
	return nil
}

func TestUpdateUserProfile(t *testing.T) {
	repository := &profileRepository{user: &domainuser.User{ID: 1, Username: "alice", Nickname: "alice"}}
	service := NewProfileService(repository)

	output, err := service.Update_UserProfile(context.Background(), 1, UpdateProfileInput{
		Nickname:  "Alice",
		Bio:       "Go 开发者",
		AvatarURL: "https://example.com/avatar.png",
	})
	if err != nil {
		t.Fatalf("修改用户资料失败: %v", err)
	}
	if output.Nickname != "Alice" || output.Bio != "Go 开发者" {
		t.Fatalf("用户资料不符合预期: %+v", output)
	}
}
