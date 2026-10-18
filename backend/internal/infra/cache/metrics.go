package cache

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type metricCounter struct{ value atomic.Uint64 }

type durationMetric struct {
	count atomic.Uint64
	nanos atomic.Uint64
}

// Metrics 保存缓存与 Redis 的进程内累计指标，并输出 Prometheus 文本格式。
type Metrics struct {
	counters  sync.Map
	durations sync.Map
}

var DefaultMetrics = NewMetrics()

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) CacheRequest(area, result string) {
	m.increment("cache_requests_total", area, result)
}

func (m *Metrics) CacheWrite(area, result string) {
	m.increment("cache_write_total", area, result)
}

func (m *Metrics) CacheFallback(area string) {
	m.increment("cache_fallback_total", area, "")
}

func (m *Metrics) SingleflightShared(area string) {
	m.increment("cache_singleflight_shared_total", area, "")
}

func (m *Metrics) ObserveLoad(area string, elapsed time.Duration) {
	m.observe("cache_load_duration_seconds", area, elapsed)
}

func (m *Metrics) ObserveRedis(command string, elapsed time.Duration) {
	m.observe("redis_command_duration_seconds", strings.ToLower(command), elapsed)
}

func (m *Metrics) increment(name, label, result string) {
	key := name + "\x00" + label + "\x00" + result
	value, _ := m.counters.LoadOrStore(key, &metricCounter{})
	value.(*metricCounter).value.Add(1)
}

func (m *Metrics) observe(name, label string, elapsed time.Duration) {
	key := name + "\x00" + label
	value, _ := m.durations.LoadOrStore(key, &durationMetric{})
	metric := value.(*durationMetric)
	metric.count.Add(1)
	metric.nanos.Add(uint64(elapsed))
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = io.WriteString(w, m.Render())
}

func (m *Metrics) Render() string {
	lines := make([]string, 0)
	m.counters.Range(func(rawKey, rawValue any) bool {
		parts := strings.Split(rawKey.(string), "\x00")
		name, label, result := parts[0], parts[1], parts[2]
		labels := fmt.Sprintf("area=%q", label)
		if result != "" {
			labels += fmt.Sprintf(",result=%q", result)
		}
		lines = append(lines, fmt.Sprintf("askflow_%s{%s} %d", name, labels, rawValue.(*metricCounter).value.Load()))
		return true
	})
	m.durations.Range(func(rawKey, rawValue any) bool {
		parts := strings.Split(rawKey.(string), "\x00")
		metric := rawValue.(*durationMetric)
		labelName := "area"
		if parts[0] == "redis_command_duration_seconds" {
			labelName = "command"
		}
		labels := fmt.Sprintf("%s=%q", labelName, parts[1])
		seconds := float64(metric.nanos.Load()) / float64(time.Second)
		lines = append(lines,
			fmt.Sprintf("askflow_%s_count{%s} %d", parts[0], labels, metric.count.Load()),
			fmt.Sprintf("askflow_%s_sum{%s} %.9f", parts[0], labels, seconds),
		)
		return true
	})
	sort.Strings(lines)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func cacheArea(key string) string {
	switch {
	case strings.HasPrefix(key, "askflow:question:card:"):
		return "question_card"
	case strings.HasPrefix(key, "askflow:answer:card:"):
		return "answer_card"
	case strings.HasPrefix(key, "askflow:answer:stat:"):
		return "answer_stat"
	case strings.HasPrefix(key, "askflow:answer:rank:"):
		return "answer_rank"
	case strings.HasPrefix(key, "askflow:user:card:"):
		return "user_card"
	case strings.HasPrefix(key, "askflow:feed:following:"):
		return "following_feed"
	case strings.HasPrefix(key, "askflow:feed:recommended:"):
		return "recommended_feed"
	case strings.HasPrefix(key, "askflow:feed:hot:"):
		return "hot_feed"
	default:
		return "unknown"
	}
}
