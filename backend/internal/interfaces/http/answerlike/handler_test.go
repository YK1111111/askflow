package answerlikehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	applicationanswerlike "askflow/internal/application/answerlike"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeAnswerLikeService struct{}

func (fakeAnswerLikeService) Like_Answer(context.Context, uint64, uint64) error   { return nil }
func (fakeAnswerLikeService) Unlike_Answer(context.Context, uint64, uint64) error { return nil }
func (fakeAnswerLikeService) Get_AnswerLikeStatus(context.Context, uint64, uint64) (applicationanswerlike.StatusOutput, error) {
	return applicationanswerlike.StatusOutput{Liked: true}, nil
}

func answerLikeRouter() *gin.Engine {
	router := gin.New()
	protected := router.Group("/api/v1")
	protected.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(1))
		c.Next()
	})
	RegisterProtectedRoutes(protected, fakeAnswerLikeService{})
	return router
}

func TestLikeAndUnlikeHandlers(t *testing.T) {
	for _, path := range []string{"/api/v1/answers/2/like", "/api/v1/answers/2/unlike"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		answerLikeRouter().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("接口 %s 期望状态码为 200，实际为 %d", path, response.Code)
		}
	}
}

func TestAnswerLikeStatusHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/answers/2/like-status", nil)
	response := httptest.NewRecorder()
	answerLikeRouter().ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"liked":true}` {
		t.Fatalf("点赞状态响应不符合预期: %s", response.Body.String())
	}
}
