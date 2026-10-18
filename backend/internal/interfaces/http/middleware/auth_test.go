package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	applicationauth "askflow/internal/application/auth"

	"github.com/gin-gonic/gin"
)

type fakeTokenParser struct {
	identity applicationauth.Identity
	err      error
}

func (p fakeTokenParser) Parse_AccessToken(string) (applicationauth.Identity, error) {
	return p.identity, p.err
}

func TestAuthenticate(t *testing.T) {
	router := gin.New()
	router.Use(Authenticate(fakeTokenParser{
		identity: applicationauth.Identity{UserID: 1, Username: "alice"},
	}))
	router.GET("/protected", func(c *gin.Context) {
		userID, _ := c.Get(UserIDKey)
		username, _ := c.Get(UsernameKey)
		c.JSON(http.StatusOK, gin.H{"id": userID, "username": username})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("期望状态码为 200，实际为 %d", response.Code)
	}
	if response.Body.String() != `{"id":1,"username":"alice"}` {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}

func TestAuthenticateRejectsMissingToken(t *testing.T) {
	router := gin.New()
	router.Use(Authenticate(fakeTokenParser{}))
	router.GET("/protected", func(c *gin.Context) {})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("期望状态码为 401，实际为 %d", response.Code)
	}
}

func TestAuthenticateRejectsInvalidToken(t *testing.T) {
	router := gin.New()
	router.Use(Authenticate(fakeTokenParser{err: errors.New("invalid token")}))
	router.GET("/protected", func(c *gin.Context) {})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer invalid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("期望状态码为 401，实际为 %d", response.Code)
	}
}
