package followhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationfollow "askflow/internal/application/follow"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeFollowService struct{}

func (fakeFollowService) Follow_User(context.Context, uint64, uint64) error   { return nil }
func (fakeFollowService) Unfollow_User(context.Context, uint64, uint64) error { return nil }
func (fakeFollowService) Get_FollowStatus(context.Context, uint64, uint64) (applicationfollow.StatusOutput, error) {
	return applicationfollow.StatusOutput{Following: true}, nil
}
func (fakeFollowService) List_Following(context.Context, uint64) ([]applicationfollow.UserOutput, error) {
	return []applicationfollow.UserOutput{{ID: 2, Username: "alice", Nickname: "Alice"}}, nil
}

func protectedFollowRouter() *gin.Engine {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(1))
		c.Next()
	})
	RegisterProtectedRoutes(protected, fakeFollowService{})
	return router
}

func TestFollowAndUnfollowHandlers(t *testing.T) {
	for _, path := range []string{"/api/v1/users/2/follow", "/api/v1/users/2/unfollow"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		protectedFollowRouter().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("接口 %s 期望状态码为 200，实际为 %d", path, response.Code)
		}
	}
}

func TestFollowStatusHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/2/follow-status", nil)
	response := httptest.NewRecorder()
	protectedFollowRouter().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"following":true}` {
		t.Fatalf("关注状态响应不符合预期: %s", response.Body.String())
	}
}

func TestFollowingListHandler(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeFollowService{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users/1/following", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"username":"alice"`) {
		t.Fatalf("关注列表响应不符合预期: %s", response.Body.String())
	}
}
