package questionhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationquestion "askflow/internal/application/question"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeQuestionPublisher struct{}

func (fakeQuestionPublisher) Publish_Question(_ context.Context, input applicationquestion.PublishInput) (applicationquestion.PublishOutput, error) {
	return applicationquestion.PublishOutput{ID: input.AuthorID}, nil
}

type fakeQuestionDetailGetter struct{}

func (fakeQuestionDetailGetter) Get_QuestionDetail(context.Context, uint64) (applicationquestion.DetailOutput, error) {
	return applicationquestion.DetailOutput{
		ID:      1,
		Title:   "问题标题",
		Content: "问题内容",
		Author:  applicationquestion.AuthorOutput{ID: 1, Username: "alice", Nickname: "Alice"},
	}, nil
}

func TestPublishQuestionHandler(t *testing.T) {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(1))
		c.Next()
	})
	RegisterProtectedRoutes(protected, fakeQuestionPublisher{})

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/questions",
		strings.NewReader(`{"title":"Go 缓存怎么设计？","content":"Cache Aside 如何保证一致性？"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("期望状态码为 201，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
	if response.Body.String() != `{"id":1}` {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}

func TestGetQuestionDetailHandler(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeQuestionDetailGetter{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions/1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("期望状态码为 200，实际为 %d，响应为 %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"title":"问题标题"`) {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}
