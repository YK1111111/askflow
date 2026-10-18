// Package auth 定义认证相关的应用层数据和接口。
package auth

// Identity 表示访问令牌中的登录用户身份。
type Identity struct {
	UserID   uint64
	Username string
}

// AccessTokenParser 定义解析访问令牌所需的能力。
type AccessTokenParser interface {
	Parse_AccessToken(token string) (Identity, error)
}
