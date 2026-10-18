package model

import (
	"testing"

	domainuser "askflow/internal/domain/user"
)

func TestUserMapping(t *testing.T) {
	domain, err := domainuser.New("alice", "hashed-password")
	if err != nil {
		t.Fatalf("创建领域用户失败: %v", err)
	}
	domain.ID = 1

	restored := User_Model2User_Domain(User_Domain2User_Model(domain))
	if restored.ID != 1 || restored.Username != "alice" {
		t.Fatalf("用户映射结果不符合预期: %+v", restored)
	}
	if restored.PasswordHash != "hashed-password" {
		t.Fatal("密码哈希在映射后发生变化")
	}
}
