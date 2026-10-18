// Package notification 将 RabbitMQ 领域事件转换为持久化站内通知。
package notification

import (
	"context"
	"encoding/json"
	"fmt"

	applicationnotification "askflow/internal/application/notification"
	domainnotification "askflow/internal/domain/notification"
	"askflow/internal/infra/messaging"
	"askflow/internal/infra/outbox"
)

type creator interface {
	Create_FromEvent(ctx context.Context, input applicationnotification.CreateInput) error
}

type EventHandler struct{ creator creator }

func NewEventHandler(creator creator) *EventHandler { return &EventHandler{creator: creator} }

func (h *EventHandler) Handle(ctx context.Context, eventID, eventType string, content []byte) error {
	switch eventType {
	case outbox.AnswerLiked, outbox.AnswerCommented, outbox.AnswerFavorited:
		var payload outbox.AnswerPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析回答通知事件: %w", err))
		}
		notificationType := map[string]string{
			outbox.AnswerLiked: domainnotification.TypeAnswerLiked, outbox.AnswerCommented: domainnotification.TypeAnswerCommented,
			outbox.AnswerFavorited: domainnotification.TypeAnswerFavorited,
		}[eventType]
		return h.creator.Create_FromEvent(ctx, applicationnotification.CreateInput{EventID: eventID, EventType: notificationType,
			ActorID: payload.ActorID, AnswerID: payload.AnswerID, QuestionID: payload.QuestionID, OccurredAt: payload.OccurredAt})
	case outbox.UserFollowed:
		var payload outbox.FollowPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析关注通知事件: %w", err))
		}
		return h.creator.Create_FromEvent(ctx, applicationnotification.CreateInput{EventID: eventID,
			EventType: domainnotification.TypeUserFollowed, ActorID: payload.FollowerID,
			RecipientID: payload.FolloweeID, OccurredAt: payload.OccurredAt})
	default:
		return nil
	}
}
