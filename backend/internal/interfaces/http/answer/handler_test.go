package answerhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationanswer "askflow/internal/application/answer"
	applicationanswerranking "askflow/internal/application/answerranking"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeAnswerPublisher struct{}

func (fakeAnswerPublisher) Publish_Answer(_ context.Context, input applicationanswer.PublishInput) (applicationanswer.PublishOutput, error) {
	return applicationanswer.PublishOutput{ID: input.AuthorID}, nil
}

type fakeAnswerListGetter struct{}

func (fakeAnswerListGetter) List_RankedAnswers(context.Context, uint64, uint64, string, int) (applicationanswerranking.Output, error) {
	return applicationanswerranking.Output{Items: []applicationanswerranking.ItemOutput{{ID: 1, QuestionID: 2, Content: "回答内容"}}}, nil
}

func TestPublishAnswerHandler(t *testing.T) {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(1))
		c.Next()
	})
	RegisterProtectedRoutes(protected, fakeAnswerPublisher{})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions/2/answers", strings.NewReader(`{"content":"回答内容"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Body.String() != `{"id":1,"like_count":0,"comment_count":0,"favorite_count":0}` {
		t.Fatalf("响应不符合预期，状态码为 %d，响应为 %s", response.Code, response.Body.String())
	}
}

func TestListAnswersHandlerRejectsInvalidParameters(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeAnswerListGetter{})
	for _, path := range []string{
		"/api/v1/questions/2/answers?anchor_answer_id=bad",
		"/api/v1/questions/2/answers?limit=0",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("无效参数应返回 400，路径为 %s，实际为 %d", path, response.Code)
		}
	}
}

func TestListAnswersHandler(t *testing.T) {
	router := gin.New()
	apiV1 := router.Group("/api/v1")
	RegisterPublicRoutes(apiV1, fakeAnswerListGetter{})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/questions/2/answers", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"content":"回答内容"`) {
		t.Fatalf("响应不符合预期，状态码为 %d，响应为 %s", response.Code, response.Body.String())
	}
}
