package cache

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsRenderPrometheusText(t *testing.T) {
	metrics := NewMetrics()
	metrics.CacheRequest("question_card", "hit")
	metrics.CacheRequest("question_card", "miss")
	metrics.CacheWrite("question_card", "success")
	metrics.CacheFallback("question_card")
	metrics.SingleflightShared("question_card")
	metrics.ObserveLoad("question_card", 250*time.Millisecond)
	metrics.ObserveRedis("get", 10*time.Millisecond)

	output := metrics.Render()
	wanted := []string{
		`askflow_cache_requests_total{area="question_card",result="hit"} 1`,
		`askflow_cache_requests_total{area="question_card",result="miss"} 1`,
		`askflow_cache_write_total{area="question_card",result="success"} 1`,
		`askflow_cache_fallback_total{area="question_card"} 1`,
		`askflow_cache_singleflight_shared_total{area="question_card"} 1`,
		`askflow_cache_load_duration_seconds_count{area="question_card"} 1`,
		`askflow_redis_command_duration_seconds_count{command="get"} 1`,
	}
	for _, line := range wanted {
		if !strings.Contains(output, line) {
			t.Fatalf("指标输出缺少 %q:\n%s", line, output)
		}
	}

	response := httptest.NewRecorder()
	metrics.ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil))
	if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("指标 HTTP 响应错误: code=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestCacheAreaFromKey(t *testing.T) {
	cases := map[string]string{
		"askflow:question:card:v1:1":               "question_card",
		"askflow:answer:stat:v1:1":                 "answer_stat",
		"askflow:answer:rank:v1:1":                 "answer_rank",
		"askflow:feed:following:v1:1:first:21":     "following_feed",
		"askflow:feed:recommended:v1:user:1:first": "recommended_feed",
		"askflow:feed:hot:v1:snapshot:1":           "hot_feed",
	}
	for key, expected := range cases {
		if actual := cacheArea(key); actual != expected {
			t.Fatalf("Key %q 区域错误: got=%q want=%q", key, actual, expected)
		}
	}
}
