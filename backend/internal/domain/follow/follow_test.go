package follow

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	created, err := New(1, 2)
	if err != nil {
		t.Fatalf("创建关注关系失败: %v", err)
	}
	if created.FollowerID != 1 || created.FolloweeID != 2 || created.CreatedAt.IsZero() {
		t.Fatalf("关注关系不符合预期: %+v", created)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		followerID uint64
		followeeID uint64
		wantErr    error
	}{
		{name: "关注者为空", followerID: 0, followeeID: 2, wantErr: ErrFollowerRequired},
		{name: "被关注用户为空", followerID: 1, followeeID: 0, wantErr: ErrFolloweeRequired},
		{name: "关注自己", followerID: 1, followeeID: 1, wantErr: ErrCannotFollowSelf},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.followerID, tt.followeeID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("期望错误为 %v，实际为 %v", tt.wantErr, err)
			}
		})
	}
}
