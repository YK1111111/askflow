// Package middleware 提供 HTTP 请求中间件。
package middleware

import (
	"net/http"
	"strings"

	applicationauth "askflow/internal/application/auth"

	"github.com/gin-gonic/gin"
)

const (
	UserIDKey   = "user_id"
	UsernameKey = "username"
)

// Authenticate 验证 Bearer Token，并将用户身份写入请求上下文。
func Authenticate(parser applicationauth.AccessTokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, found := strings.Cut(c.GetHeader("Authorization"), " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少访问令牌"})
			return
		}

		identity, err := parser.Parse_AccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问令牌无效"})
			return
		}

		c.Set(UserIDKey, identity.UserID)
		c.Set(UsernameKey, identity.Username)
		c.Next()
	}
}

// OptionalAuthenticate 在请求携带令牌时解析身份，未携带令牌时按匿名请求继续。
func OptionalAuthenticate(parser applicationauth.AccessTokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}
		scheme, token, found := strings.Cut(header, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问令牌无效"})
			return
		}
		identity, err := parser.Parse_AccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "访问令牌无效"})
			return
		}
		c.Set(UserIDKey, identity.UserID)
		c.Set(UsernameKey, identity.Username)
		c.Next()
	}
}
