package security

import "testing"

func TestBCryptHasher(t *testing.T) {
	hasher := BCryptHasher{}
	hash, err := hasher.Hash_Password("password123")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	if !hasher.Verify_Password("password123", hash) {
		t.Fatal("正确密码校验失败")
	}
	if hasher.Verify_Password("wrong-password", hash) {
		t.Fatal("错误密码不应通过校验")
	}
}
