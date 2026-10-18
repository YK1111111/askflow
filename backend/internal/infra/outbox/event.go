// Package outbox 实现与业务事务一致提交的事件表。
package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
)

const (
	AnswerPublished   = "answer.published"
	AnswerLiked       = "answer.liked"
	AnswerUnliked     = "answer.unliked"
	AnswerCommented   = "answer.commented"
	AnswerFavorited   = "answer.favorited"
	AnswerUnfavorited = "answer.unfavorited"
	UserFollowed      = "user.followed"
	UserUnfollowed    = "user.unfollowed"
	QuestionViewed    = "question.viewed"
)

type AnswerPayload struct {
	AnswerID   uint64    `json:"answer_id"`
	QuestionID uint64    `json:"question_id"`
	AuthorID   uint64    `json:"author_id,omitempty"`
	ActorID    uint64    `json:"actor_id,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type FollowPayload struct {
	FollowerID uint64    `json:"follower_id"`
	FolloweeID uint64    `json:"followee_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

type QuestionPayload struct {
	QuestionID uint64    `json:"question_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func Append(ctx context.Context, tx *gorm.DB, eventType, aggregateType string, aggregateID uint64, payload any) error {
	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化 Outbox 事件: %w", err)
	}
	id, err := newID()
	if err != nil {
		return fmt.Errorf("生成 Outbox 事件 ID: %w", err)
	}
	now := time.Now()
	event := model.OutboxEvent{ID: id, AggregateType: aggregateType, AggregateID: aggregateID, EventType: eventType, Payload: content, NextAttemptAt: now, CreatedAt: now}
	return tx.WithContext(ctx).Create(&event).Error
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
