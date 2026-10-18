package user

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	created, err := New(" alice ", "hashed-password")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	if created.Username != "alice" {
		t.Fatalf("期望用户名为 alice，实际为 %s", created.Username)
	}
	if created.Nickname != "alice" {
		t.Fatalf("期望默认昵称为 alice，实际为 %s", created.Nickname)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("创建时间和更新时间不应为空")
	}
}

func TestUpdateProfile(t *testing.T) {
	created, err := New("alice", "hashed-password")
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	err = created.Update_Profile("Alice", "Go 开发者", "https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("修改资料失败: %v", err)
	}
	if created.Nickname != "Alice" || created.Bio != "Go 开发者" {
		t.Fatalf("用户资料不符合预期: %+v", created)
	}
}

func TestNewRejectsEmptyUsername(t *testing.T) {
	_, err := New("  ", "hashed-password")
	if !errors.Is(err, ErrUsernameRequired) {
		t.Fatalf("期望用户名为空错误，实际为 %v", err)
	}
}
