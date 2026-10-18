package userhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationuser "askflow/internal/application/user"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeUserRegister struct{}

func (fakeUserRegister) Register_User(context.Context, applicationuser.RegisterInput) (applicationuser.RegisterOutput, error) {
	return applicationuser.RegisterOutput{ID: 1, Username: "alice"}, nil
}

type fakeUserLogin struct{}

func (fakeUserLogin) Login_User(context.Context, applicationuser.LoginInput) (applicationuser.LoginOutput, error) {
	return applicationuser.LoginOutput{AccessToken: "access-token", ExpiresIn: 900}, nil
}

type fakeUserProfile struct{}

func (fakeUserProfile) Get_UserProfile(context.Context, uint64) (applicationuser.ProfileOutput, error) {
	return applicationuser.ProfileOutput{ID: 1, Username: "alice", Nickname: "Alice"}, nil
}

func (fakeUserProfile) Update_UserProfile(_ context.Context, userID uint64, input applicationuser.UpdateProfileInput) (applicationuser.ProfileOutput, error) {
	return applicationuser.ProfileOutput{
		ID:        userID,
		Username:  "alice",
		Nickname:  input.Nickname,
		Bio:       input.Bio,
		AvatarURL: input.AvatarURL,
	}, nil
}

func TestRegisterUserHandler(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeUserRegister{}, fakeUserLogin{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/users/register",
		strings.NewReader(`{"username":"alice","password":"password123"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("期望状态码为 201，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `{"id":1,"username":"alice"}` {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}

func TestLoginUserHandler(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeUserRegister{}, fakeUserLogin{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/users/login",
		strings.NewReader(`{"username":"alice","password":"password123"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("期望状态码为 200，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `{"access_token":"access-token","expires_in":900}` {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}

func TestUpdateCurrentUserProfileHandler(t *testing.T) {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(1))
		c.Next()
	})
	RegisterProtectedRoutes(protected, fakeUserProfile{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/users/me",
		strings.NewReader(`{"nickname":"Alice","bio":"Go 开发者","avatar_url":""}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("期望状态码为 200，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
}
