package commenthttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	applicationcomment "askflow/internal/application/comment"
	"askflow/internal/interfaces/http/middleware"
	"github.com/gin-gonic/gin"
)

type fakeCommentService struct{}

func (fakeCommentService) Publish_Comment(context.Context, applicationcomment.PublishInput) (applicationcomment.PublishOutput, error) {
	return applicationcomment.PublishOutput{ID: 1}, nil
}
func (fakeCommentService) List_CommentsByAnswerID(context.Context, uint64) ([]applicationcomment.CommentOutput, error) {
	return []applicationcomment.CommentOutput{{ID: 1, Content: "评论"}}, nil
}

func TestCommentHandlers(t *testing.T) {
	router := gin.New()
	api := router.Group("/api/v1")
	RegisterPublicRoutes(api, fakeCommentService{})
	protected := api.Group("")
	protected.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, uint64(1)); c.Next() })
	RegisterProtectedRoutes(protected, fakeCommentService{})

	post := httptest.NewRequest(http.MethodPost, "/api/v1/answers/2/comments", strings.NewReader(`{"content":"评论"}`))
	post.Header.Set("Content-Type", "application/json")
	postResponse := httptest.NewRecorder()
	router.ServeHTTP(postResponse, post)
	if postResponse.Code != http.StatusCreated {
		t.Fatalf("发布评论状态码为 %d", postResponse.Code)
	}
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/v1/answers/2/comments", nil))
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"content":"评论"`) {
		t.Fatalf("评论列表响应为 %s", getResponse.Body.String())
	}
}
