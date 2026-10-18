// Package messaging 提供 RabbitMQ 事件发布和消费能力。
package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"askflow/internal/infra/config"
	"askflow/internal/infra/database/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

const ExchangeName = "askflow.events.v1"

const (
	RetryExchangeName  = "askflow.retry.v1"
	DeadExchangeName   = "askflow.dlx.v1"
	consumerMaxRetries = int64(3)
	consumerRetryDelay = 5 * time.Second
	retryCountHeader   = "x-askflow-retry-count"
)

type RabbitPublisher struct {
	url      string
	mu       sync.Mutex
	conn     *amqp.Connection
	channel  *amqp.Channel
	confirms chan amqp.Confirmation
}

func NewRabbitPublisher(cfg config.RabbitMQConfig) *RabbitPublisher {
	return &RabbitPublisher{url: cfg.URL}
}

func (p *RabbitPublisher) Publish(ctx context.Context, event model.OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureConnected(); err != nil {
		return err
	}
	err := p.channel.PublishWithContext(ctx, ExchangeName, event.EventType, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: event.ID,
		Timestamp: event.CreatedAt, Body: event.Payload,
	})
	if err != nil {
		p.reset()
		return fmt.Errorf("发布 RabbitMQ 事件: %w", err)
	}
	select {
	case confirmation, ok := <-p.confirms:
		if !ok || !confirmation.Ack {
			p.reset()
			return errors.New("RabbitMQ 未确认事件")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *RabbitPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.reset()
}

func (p *RabbitPublisher) ensureConnected() error {
	if p.channel != nil && !p.channel.IsClosed() && p.conn != nil && !p.conn.IsClosed() {
		return nil
	}
	conn, err := amqp.DialConfig(p.url, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(3 * time.Second)})
	if err != nil {
		return fmt.Errorf("连接 RabbitMQ: %w", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("创建 RabbitMQ Channel: %w", err)
	}
	if err := channel.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return fmt.Errorf("声明 RabbitMQ Exchange: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return fmt.Errorf("启用 RabbitMQ Publisher Confirm: %w", err)
	}
	p.conn, p.channel = conn, channel
	p.confirms = channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	return nil
}

func (p *RabbitPublisher) reset() error {
	var result error
	if p.channel != nil && !p.channel.IsClosed() {
		result = p.channel.Close()
	}
	if p.conn != nil && !p.conn.IsClosed() {
		if err := p.conn.Close(); result == nil {
			result = err
		}
	}
	p.channel, p.conn, p.confirms = nil, nil, nil
	return result
}

type EventHandler interface {
	Handle(ctx context.Context, eventID, eventType string, payload []byte) error
}

type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }
func Permanent(err error) error         { return &PermanentError{Err: err} }

type RabbitConsumer struct {
	url     string
	queue   string
	handler EventHandler
}

func NewRabbitConsumer(cfg config.RabbitMQConfig, queue string, handler EventHandler) *RabbitConsumer {
	return &RabbitConsumer{url: cfg.URL, queue: queue, handler: handler}
}

func (c *RabbitConsumer) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := c.consume(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("RabbitMQ 消费连接中断，稍后重试", "error", err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
			}
		}
	}
}

func (c *RabbitConsumer) consume(ctx context.Context) error {
	conn, err := amqp.DialConfig(c.url, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(3 * time.Second)})
	if err != nil {
		return err
	}
	defer conn.Close()
	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	if err := channel.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	retryExchange := RetryExchangeName + "." + c.queue
	deadExchange := DeadExchangeName + "." + c.queue
	if err := channel.ExchangeDeclare(retryExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := channel.ExchangeDeclare(deadExchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	queue, err := channel.QueueDeclare(c.queue, true, false, false, false, nil)
	if err != nil {
		return err
	}
	retryQueue, err := channel.QueueDeclare(c.queue+".retry", true, false, false, false, amqp.Table{
		"x-message-ttl":          consumerRetryDelay.Milliseconds(),
		"x-dead-letter-exchange": ExchangeName,
	})
	if err != nil {
		return err
	}
	deadQueue, err := channel.QueueDeclare(c.queue+".dead", true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := channel.QueueBind(retryQueue.Name, "#", retryExchange, false, nil); err != nil {
		return err
	}
	if err := channel.QueueBind(deadQueue.Name, "#", deadExchange, false, nil); err != nil {
		return err
	}
	for _, pattern := range []string{"answer.#", "user.#", "question.#"} {
		if err := channel.QueueBind(queue.Name, pattern, ExchangeName, false, nil); err != nil {
			return err
		}
	}
	if err := channel.Qos(20, 0, false); err != nil {
		return err
	}
	if err := channel.Confirm(false); err != nil {
		return err
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	deliveries, err := channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RabbitMQ 消费通道已关闭")
			}
			if err := c.handler.Handle(ctx, delivery.MessageId, delivery.RoutingKey, delivery.Body); err != nil {
				var permanent *PermanentError
				retryCount := messageRetryCount(delivery.Headers)
				exchange, retryCount := failureDestination(retryExchange, deadExchange, retryCount, errors.As(err, &permanent))
				if err := republish(ctx, channel, confirms, exchange, delivery, retryCount); err != nil {
					_ = delivery.Nack(false, true)
					return fmt.Errorf("转移消费失败消息: %w", err)
				}
				_ = delivery.Ack(false)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func failureDestination(retryExchange, deadExchange string, retryCount int64, permanent bool) (string, int64) {
	if permanent || retryCount >= consumerMaxRetries {
		return deadExchange, retryCount
	}
	return retryExchange, retryCount + 1
}

func messageRetryCount(headers amqp.Table) int64 {
	if headers == nil {
		return 0
	}
	switch value := headers[retryCountHeader].(type) {
	case int64:
		return value
	case int32:
		return int64(value)
	case int:
		return int64(value)
	default:
		return 0
	}
}

func republish(ctx context.Context, channel *amqp.Channel, confirms <-chan amqp.Confirmation, exchange string, delivery amqp.Delivery, retryCount int64) error {
	headers := make(amqp.Table, len(delivery.Headers)+1)
	for key, value := range delivery.Headers {
		headers[key] = value
	}
	headers[retryCountHeader] = retryCount
	if err := channel.PublishWithContext(ctx, exchange, delivery.RoutingKey, false, false, amqp.Publishing{
		Headers: headers, ContentType: delivery.ContentType, ContentEncoding: delivery.ContentEncoding,
		DeliveryMode: amqp.Persistent, MessageId: delivery.MessageId, Timestamp: delivery.Timestamp,
		Type: delivery.Type, AppId: delivery.AppId, Body: delivery.Body,
	}); err != nil {
		return err
	}
	select {
	case confirmation, ok := <-confirms:
		if !ok || !confirmation.Ack {
			return errors.New("RabbitMQ 未确认失败消息转移")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
