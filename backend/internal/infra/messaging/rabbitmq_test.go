package messaging

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestMessageRetryCount(t *testing.T) {
	tests := []struct {
		name    string
		headers amqp.Table
		want    int64
	}{
		{name: "无消息头", want: 0},
		{name: "AMQP 长整数", headers: amqp.Table{retryCountHeader: int64(2)}, want: 2},
		{name: "AMQP 整数", headers: amqp.Table{retryCountHeader: int32(3)}, want: 3},
		{name: "非法类型", headers: amqp.Table{retryCountHeader: "3"}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := messageRetryCount(test.headers); got != test.want {
				t.Fatalf("重试次数错误: got=%d want=%d", got, test.want)
			}
		})
	}
}

func TestFailureDestination(t *testing.T) {
	tests := []struct {
		name           string
		retryCount     int64
		permanent      bool
		wantExchange   string
		wantRetryCount int64
	}{
		{name: "首次临时失败", wantExchange: RetryExchangeName, wantRetryCount: 1},
		{name: "第三次临时失败", retryCount: 2, wantExchange: RetryExchangeName, wantRetryCount: 3},
		{name: "超过重试上限", retryCount: 3, wantExchange: DeadExchangeName, wantRetryCount: 3},
		{name: "永久错误", permanent: true, wantExchange: DeadExchangeName, wantRetryCount: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exchange, retryCount := failureDestination(RetryExchangeName, DeadExchangeName, test.retryCount, test.permanent)
			if exchange != test.wantExchange || retryCount != test.wantRetryCount {
				t.Fatalf("失败消息去向错误: exchange=%q retry_count=%d", exchange, retryCount)
			}
		})
	}
}
