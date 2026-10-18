package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAccessToken(t *testing.T) {
	issuer, err := NewJWTIssuer("test-secret", "15m")
	if err != nil {
		t.Fatalf("创建 JWT 签发器失败: %v", err)
	}

	tokenString, expiresIn, err := issuer.Issue_AccessToken(1, "alice")
	if err != nil {
		t.Fatalf("签发 JWT 失败: %v", err)
	}

	claims := &accessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		t.Fatalf("JWT 无效: %v", err)
	}
	if claims.Subject != "1" || claims.Username != "alice" || expiresIn != 900 {
		t.Fatalf("JWT 声明不符合预期: %+v", claims)
	}

	identity, err := issuer.Parse_AccessToken(tokenString)
	if err != nil || identity.UserID != 1 || identity.Username != "alice" {
		t.Fatalf("解析访问令牌失败: identity=%+v, err=%v", identity, err)
	}
}

func TestParseAccessTokenRejectsExpiredToken(t *testing.T) {
	issuer, err := NewJWTIssuer("test-secret", "15m")
	if err != nil {
		t.Fatalf("创建 JWT 签发器失败: %v", err)
	}

	claims := accessTokenClaims{
		Username: "alice",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("创建过期令牌失败: %v", err)
	}

	_, err = issuer.Parse_AccessToken(expiredToken)
	if err == nil {
		t.Fatalf("期望拒绝过期令牌，实际为 %v", err)
	}
}
