// Package notification 编排通知创建、查询和实时推送用例。
package notification

import (
	"context"
	"time"

	domainanswer "askflow/internal/domain/answer"
	domainnotification "askflow/internal/domain/notification"
)

type Output struct {
	ID         uint64     `json:"id"`
	ActorID    uint64     `json:"actor_id"`
	Type       string     `json:"type"`
	AnswerID   uint64     `json:"answer_id,omitempty"`
	QuestionID uint64     `json:"question_id,omitempty"`
	Content    string     `json:"content"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ListOutput struct {
	Items       []Output `json:"items"`
	UnreadCount int64    `json:"unread_count"`
}

type CreateInput struct {
	EventID     string
	EventType   string
	ActorID     uint64
	RecipientID uint64
	AnswerID    uint64
	QuestionID  uint64
	OccurredAt  time.Time
}

type Service struct {
	repository       domainnotification.Repository
	answerRepository domainanswer.Repository
	hub              *Hub
}

func NewService(repository domainnotification.Repository, answerRepository domainanswer.Repository, hub *Hub) *Service {
	return &Service{repository: repository, answerRepository: answerRepository, hub: hub}
}

func (s *Service) Create_FromEvent(ctx context.Context, input CreateInput) error {
	userID, notificationType, content, err := s.resolveRecipient(ctx, input)
	if err != nil || userID == 0 || userID == input.ActorID {
		return err
	}
	value := &domainnotification.Notification{EventID: input.EventID, UserID: userID, ActorID: input.ActorID,
		Type: notificationType, AnswerID: input.AnswerID, QuestionID: input.QuestionID,
		Content: content, CreatedAt: input.OccurredAt}
	created, err := s.repository.Create_Notification(ctx, value)
	if err != nil {
		return err
	}
	if created {
		s.hub.Publish(userID, toOutput(value))
	}
	return nil
}

func (s *Service) resolveRecipient(ctx context.Context, input CreateInput) (uint64, string, string, error) {
	if input.EventType == domainnotification.TypeUserFollowed {
		return input.RecipientID, domainnotification.TypeUserFollowed, "有用户关注了你", nil
	}
	answer, err := s.answerRepository.Find_AnswerByID(ctx, input.AnswerID)
	if err != nil {
		return 0, "", "", err
	}
	switch input.EventType {
	case domainnotification.TypeAnswerLiked:
		return answer.AuthorID, input.EventType, "你的回答收到了点赞", nil
	case domainnotification.TypeAnswerCommented:
		return answer.AuthorID, input.EventType, "你的回答收到了评论", nil
	case domainnotification.TypeAnswerFavorited:
		return answer.AuthorID, input.EventType, "你的回答被收藏了", nil
	default:
		return 0, "", "", nil
	}
}

func (s *Service) List(ctx context.Context, userID, afterID uint64, limit int) (ListOutput, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	values, err := s.repository.Find_NotificationsAfter(ctx, userID, afterID, limit)
	if err != nil {
		return ListOutput{}, err
	}
	count, err := s.repository.Count_UnreadNotifications(ctx, userID)
	if err != nil {
		return ListOutput{}, err
	}
	items := make([]Output, 0, len(values))
	for _, value := range values {
		items = append(items, toOutput(value))
	}
	return ListOutput{Items: items, UnreadCount: count}, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID uint64) error {
	return s.repository.Mark_NotificationRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID uint64) error {
	return s.repository.Mark_AllNotificationsRead(ctx, userID)
}

func (s *Service) Subscribe(userID uint64) (<-chan Output, func()) {
	return s.hub.Subscribe(userID)
}

func toOutput(value *domainnotification.Notification) Output {
	return Output{ID: value.ID, ActorID: value.ActorID, Type: value.Type, AnswerID: value.AnswerID,
		QuestionID: value.QuestionID, Content: value.Content, ReadAt: value.ReadAt, CreatedAt: value.CreatedAt}
}
