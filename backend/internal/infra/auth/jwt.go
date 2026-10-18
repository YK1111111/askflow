// Package auth 提供身份认证相关的基础设施实现。
package auth

import (
	"errors"
	"strconv"
	"time"

	applicationauth "askflow/internal/application/auth"

	"github.com/golang-jwt/jwt/v5"
)

type accessTokenClaims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTIssuer 使用 HMAC SHA-256 签发访问令牌。
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTIssuer(secret, accessTTL string) (*JWTIssuer, error) {
	if secret == "" {
		return nil, errors.New("JWT 密钥不能为空")
	}

	ttl, err := time.ParseDuration(accessTTL)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		return nil, errors.New("JWT 有效期必须大于 0")
	}

	return &JWTIssuer{secret: []byte(secret), ttl: ttl}, nil
}

func (i *JWTIssuer) Issue_AccessToken(userID uint64, username string) (string, int64, error) {
	now := time.Now()
	claims := accessTokenClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", 0, err
	}
	return token, int64(i.ttl.Seconds()), nil
}

// Parse_AccessToken 验证并解析访问令牌中的用户身份。
func (i *JWTIssuer) Parse_AccessToken(tokenString string) (applicationauth.Identity, error) {
	claims := &accessTokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return i.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return applicationauth.Identity{}, errors.New("访问令牌无效")
	}

	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || userID == 0 || claims.Username == "" {
		return applicationauth.Identity{}, errors.New("访问令牌身份无效")
	}

	return applicationauth.Identity{UserID: userID, Username: claims.Username}, nil
}
