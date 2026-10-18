package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	NewRouter().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("期望状态码为 200，实际为 %d", response.Code)
	}
	if response.Body.String() != `{"status":"ok"}` {
		t.Fatalf("响应内容不符合预期: %s", response.Body.String())
	}
}

func TestMetricsRoute(t *testing.T) {
	router := NewRouter()
	RegisterMetricsRoute(router, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("askflow_test_metric 1\n"))
	}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "askflow_test_metric 1") {
		t.Fatalf("指标路由响应错误: code=%d body=%q", response.Code, response.Body.String())
	}
}
