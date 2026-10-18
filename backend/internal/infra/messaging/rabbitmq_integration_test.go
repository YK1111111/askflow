package messaging

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"askflow/internal/infra/config"
	"askflow/internal/infra/database/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

type permanentFailureHandler struct{}

func (permanentFailureHandler) Handle(context.Context, string, string, []byte) error {
	return Permanent(errors.New("无法解析事件"))
}

func TestRabbitPublisherIntegration(t *testing.T) {
	url := os.Getenv("ASKFLOW_RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("未设置 ASKFLOW_RABBITMQ_TEST_URL，跳过 RabbitMQ 集成测试")
	}
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	channel, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	if err := channel.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	queue, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(queue.Name, "answer.#", ExchangeName, false, nil); err != nil {
		t.Fatal(err)
	}
	deliveries, err := channel.Consume(queue.Name, "", true, true, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	publisher := NewRabbitPublisher(config.RabbitMQConfig{URL: url})
	defer publisher.Close()
	event := model.OutboxEvent{ID: "integration-event", EventType: "answer.liked", Payload: []byte(`{"answer_id":1}`), CreatedAt: time.Now()}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("发布事件失败: %v", err)
	}
	select {
	case delivery := <-deliveries:
		if delivery.MessageId != event.ID || delivery.RoutingKey != event.EventType {
			t.Fatalf("收到的事件错误: id=%q key=%q", delivery.MessageId, delivery.RoutingKey)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("等待 RabbitMQ 事件超时")
	}
}

func TestRabbitConsumerSendsPermanentFailureToDeadLetterQueue(t *testing.T) {
	url := os.Getenv("ASKFLOW_RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("未设置 ASKFLOW_RABBITMQ_TEST_URL，跳过 RabbitMQ 集成测试")
	}
	queueName := "askflow.test-dead-letter"
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	channel, err := conn.Channel()
	if err != nil {
		t.Fatal(err)
	}
	defer channel.Close()
	defer func() {
		_, _ = channel.QueueDelete(queueName, false, false, false)
		_, _ = channel.QueueDelete(queueName+".retry", false, false, false)
		_, _ = channel.QueueDelete(queueName+".dead", false, false, false)
	}()

	retryExchange := RetryExchangeName + "." + queueName
	deadExchange := DeadExchangeName + "." + queueName
	for _, exchange := range []string{ExchangeName, retryExchange, deadExchange} {
		if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.QueueDeclare(queueName+".retry", true, false, false, false, amqp.Table{
		"x-message-ttl": consumerRetryDelay.Milliseconds(), "x-dead-letter-exchange": ExchangeName,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := channel.QueueDeclare(queueName+".dead", true, false, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(queueName, "answer.#", ExchangeName, false, nil); err != nil {
		t.Fatal(err)
	}
	if err := channel.QueueBind(queueName+".dead", "#", deadExchange, false, nil); err != nil {
		t.Fatal(err)
	}
	deadDeliveries, err := channel.Consume(queueName+".dead", "", true, false, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	consumer := NewRabbitConsumer(config.RabbitMQConfig{URL: url}, queueName, permanentFailureHandler{})
	go func() { done <- consumer.consume(ctx) }()
	time.Sleep(100 * time.Millisecond)

	publisher := NewRabbitPublisher(config.RabbitMQConfig{URL: url})
	defer publisher.Close()
	event := model.OutboxEvent{ID: "dead-letter-event", EventType: "answer.invalid", Payload: []byte(`{bad json`), CreatedAt: time.Now()}
	if err := publisher.Publish(context.Background(), event); err != nil {
		cancel()
		t.Fatal(err)
	}
	select {
	case delivery := <-deadDeliveries:
		if delivery.MessageId != event.ID || delivery.Exchange != deadExchange {
			t.Fatalf("死信消息错误: id=%q exchange=%q", delivery.MessageId, delivery.Exchange)
		}
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("等待死信消息超时")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("消费者未按时停止")
	}
}
