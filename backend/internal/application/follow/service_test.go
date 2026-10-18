package follow

import (
	"context"
	"testing"

	domainfollow "askflow/internal/domain/follow"
	domainuser "askflow/internal/domain/user"
)

type fakeFollowRepository struct {
	created   *domainfollow.Follow
	deleted   bool
	following bool
	ids       []uint64
}

func (r *fakeFollowRepository) Create_Follow(_ context.Context, follow *domainfollow.Follow) error {
	r.created = follow
	return nil
}
func (r *fakeFollowRepository) Delete_Follow(context.Context, uint64, uint64) error {
	r.deleted = true
	return nil
}
func (r *fakeFollowRepository) Exists_Follow(context.Context, uint64, uint64) (bool, error) {
	return r.following, nil
}
func (r *fakeFollowRepository) Find_FolloweeIDsByFollowerID(context.Context, uint64) ([]uint64, error) {
	return r.ids, nil
}

type fakeUserRepository struct {
	users map[uint64]*domainuser.User
}

func (r fakeUserRepository) Create_User(context.Context, *domainuser.User) error { return nil }
func (r fakeUserRepository) Find_UserByID(_ context.Context, id uint64) (*domainuser.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domainuser.ErrNotFound
	}
	return user, nil
}
func (r fakeUserRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return nil, domainuser.ErrNotFound
}
func (r fakeUserRepository) Update_User_Profile(context.Context, *domainuser.User) error { return nil }

func TestFollowUser(t *testing.T) {
	followRepository := &fakeFollowRepository{}
	service := NewService(followRepository, fakeUserRepository{users: map[uint64]*domainuser.User{2: {ID: 2}}})

	if err := service.Follow_User(context.Background(), 1, 2); err != nil {
		t.Fatalf("关注用户失败: %v", err)
	}
	if followRepository.created.FollowerID != 1 || followRepository.created.FolloweeID != 2 {
		t.Fatalf("关注关系不符合预期: %+v", followRepository.created)
	}
}

func TestUnfollowAndStatusAndList(t *testing.T) {
	followRepository := &fakeFollowRepository{following: true, ids: []uint64{2}}
	users := fakeUserRepository{users: map[uint64]*domainuser.User{
		1: {ID: 1, Username: "bob"},
		2: {ID: 2, Username: "alice", Nickname: "Alice"},
	}}
	service := NewService(followRepository, users)

	if err := service.Unfollow_User(context.Background(), 1, 2); err != nil || !followRepository.deleted {
		t.Fatalf("取消关注失败: %v", err)
	}
	status, err := service.Get_FollowStatus(context.Background(), 1, 2)
	if err != nil || !status.Following {
		t.Fatalf("关注状态不符合预期: %+v, %v", status, err)
	}
	following, err := service.List_Following(context.Background(), 1)
	if err != nil || len(following) != 1 || following[0].Nickname != "Alice" {
		t.Fatalf("关注列表不符合预期: %+v, %v", following, err)
	}
}
