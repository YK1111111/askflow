// Package security 提供密码等安全能力的基础设施实现。
package security

import "golang.org/x/crypto/bcrypt"

// BCryptHasher 使用 bcrypt 生成密码哈希。
type BCryptHasher struct{}

func (BCryptHasher) Hash_Password(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// Verify_Password 校验明文密码是否与哈希匹配。
func (BCryptHasher) Verify_Password(password, passwordHash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}
