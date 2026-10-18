package notificationhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	applicationnotification "askflow/internal/application/notification"
	"askflow/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type fakeService struct {
	afterID    uint64
	readID     uint64
	allRead    bool
	events     chan applicationnotification.Output
	replayItem []applicationnotification.Output
}

func (s *fakeService) List(_ context.Context, _ uint64, afterID uint64, _ int) (applicationnotification.ListOutput, error) {
	s.afterID = afterID
	return applicationnotification.ListOutput{Items: s.replayItem, UnreadCount: 2}, nil
}
func (s *fakeService) MarkRead(_ context.Context, _ uint64, notificationID uint64) error {
	s.readID = notificationID
	return nil
}
func (s *fakeService) MarkAllRead(context.Context, uint64) error { s.allRead = true; return nil }
func (s *fakeService) Subscribe(uint64) (<-chan applicationnotification.Output, func()) {
	return s.events, func() {}
}

func newNotificationRouter(service service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uint64(7))
		c.Next()
	})
	RegisterProtectedRoutes(router.Group("/api/v1"), service)
	return router
}

func TestNotificationListAndReadRoutes(t *testing.T) {
	service := &fakeService{events: make(chan applicationnotification.Output)}
	router := newNotificationRouter(service)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/notifications?after_id=8", nil))
	if response.Code != http.StatusOK || service.afterID != 8 || !strings.Contains(response.Body.String(), `"unread_count":2`) {
		t.Fatalf("通知列表响应错误: code=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/notifications/9/read", nil))
	if response.Code != http.StatusOK || service.readID != 9 {
		t.Fatalf("单条已读响应错误: code=%d id=%d", response.Code, service.readID)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/notifications/read-all", nil))
	if response.Code != http.StatusOK || !service.allRead {
		t.Fatalf("全部已读响应错误: code=%d", response.Code)
	}
}

func TestNotificationStreamReplaysFromLastEventID(t *testing.T) {
	service := &fakeService{events: make(chan applicationnotification.Output), replayItem: []applicationnotification.Output{
		{ID: 12, Type: "answer_liked", Content: "你的回答收到了点赞", CreatedAt: time.Now()},
	}}
	router := newNotificationRouter(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/stream", nil)
	request.Header.Set("Last-Event-ID", "11")
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		router.ServeHTTP(response, request)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE 连接未在客户端断开后退出")
	}
	if service.afterID != 11 || !strings.Contains(response.Body.String(), "id: 12\nevent: notification") {
		t.Fatalf("SSE 重放错误: after=%d body=%s", service.afterID, response.Body.String())
	}
}
