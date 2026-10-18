package feedhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationauth "askflow/internal/application/auth"
	applicationfeed "askflow/internal/application/feed"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeFollowingFeedService struct{}

func (fakeFollowingFeedService) Get_FollowingFeed(_ context.Context, _ uint64, cursor string, _ int) (applicationfeed.FollowingOutput, error) {
	if cursor == "bad" {
		return applicationfeed.FollowingOutput{}, applicationfeed.ErrInvalidCursor
	}
	return applicationfeed.FollowingOutput{Items: []applicationfeed.FollowingItemOutput{
		{Question: applicationfeed.QuestionOutput{ID: 1, Title: "问题"}, Answer: applicationfeed.AnswerOutput{ID: 2, Summary: "回答"}},
	}}, nil
}

type fakeHotFeedService struct{}

func (fakeHotFeedService) Get_HotFeed(_ context.Context, cursor string, _ int) (applicationfeed.HotOutput, error) {
	if cursor == "bad" {
		return applicationfeed.HotOutput{}, applicationfeed.ErrInvalidCursor
	}
	return applicationfeed.HotOutput{Items: []applicationfeed.HotItemOutput{
		{Question: applicationfeed.QuestionOutput{ID: 1, Title: "热门问题"}, HotScore: 8.5},
	}}, nil
}

type fakeRecommendedFeedService struct{ userID uint64 }

func (s *fakeRecommendedFeedService) Get_RecommendedFeed(_ context.Context, userID uint64, _ string, _ int) (applicationfeed.RecommendedOutput, error) {
	s.userID = userID
	return applicationfeed.RecommendedOutput{Items: []applicationfeed.RecommendedItemOutput{
		{Question: applicationfeed.QuestionOutput{ID: 1}, Answer: applicationfeed.AnswerOutput{ID: 2}},
	}}, nil
}

type fakeTokenParser struct{}

func (fakeTokenParser) Parse_AccessToken(string) (applicationauth.Identity, error) {
	return applicationauth.Identity{UserID: 1, Username: "alice"}, nil
}

func newFeedRouter() *gin.Engine {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(middleware.Authenticate(fakeTokenParser{}))
	RegisterProtectedRoutes(protected, fakeFollowingFeedService{})
	return router
}

func TestFollowingFeedRequiresAuthentication(t *testing.T) {
	response := httptest.NewRecorder()
	newFeedRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/feeds/following", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问应返回 401，实际为 %d", response.Code)
	}
}

func TestFollowingFeedHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/feeds/following?limit=10", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	newFeedRouter().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"summary":"回答"`) {
		t.Fatalf("响应不符合预期，状态码为 %d，响应为 %s", response.Code, response.Body.String())
	}
}

func TestFollowingFeedRejectsInvalidParameters(t *testing.T) {
	for _, path := range []string{
		"/api/v1/feeds/following?limit=zero",
		"/api/v1/feeds/following?cursor=bad",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer token")
		response := httptest.NewRecorder()
		newFeedRouter().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("无效参数应返回 400，路径为 %s，实际为 %d", path, response.Code)
		}
	}
}

func TestHotFeedIsPublic(t *testing.T) {
	router := gin.New()
	api := router.Group("/api/v1")
	RegisterPublicRoutes(api, fakeHotFeedService{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/feeds/hot", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"hot_score":8.5`) {
		t.Fatalf("热门 Feed 响应不符合预期，状态码为 %d，响应为 %s", response.Code, response.Body.String())
	}
}

func TestRecommendedFeedSupportsAnonymousAndAuthenticatedUsers(t *testing.T) {
	service := &fakeRecommendedFeedService{}
	router := gin.New()
	api := router.Group("/api/v1")
	recommended := api.Group("")
	recommended.Use(middleware.OptionalAuthenticate(fakeTokenParser{}))
	RegisterRecommendedRoutes(recommended, service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/feeds/recommended", nil))
	if response.Code != http.StatusOK || service.userID != 0 {
		t.Fatalf("匿名推荐响应不符合预期: code=%d user=%d", response.Code, service.userID)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/feeds/recommended", nil)
	request.Header.Set("Authorization", "Bearer token")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.userID != 1 {
		t.Fatalf("登录推荐响应不符合预期: code=%d user=%d", response.Code, service.userID)
	}
}
