package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	applicationnotification "askflow/internal/application/notification"
	"askflow/internal/infra/outbox"
)

type fakeCreator struct {
	input applicationnotification.CreateInput
}

func (c *fakeCreator) Create_FromEvent(_ context.Context, input applicationnotification.CreateInput) error {
	c.input = input
	return nil
}

func TestEventHandlerMapsAnswerAndFollowEvents(t *testing.T) {
	creator := &fakeCreator{}
	handler := NewEventHandler(creator)
	content, _ := json.Marshal(outbox.AnswerPayload{AnswerID: 2, QuestionID: 3, ActorID: 4, OccurredAt: time.Now()})
	if err := handler.Handle(context.Background(), "event-1", outbox.AnswerLiked, content); err != nil {
		t.Fatal(err)
	}
	if creator.input.EventID != "event-1" || creator.input.ActorID != 4 || creator.input.AnswerID != 2 {
		t.Fatalf("回答事件映射错误: %+v", creator.input)
	}

	content, _ = json.Marshal(outbox.FollowPayload{FollowerID: 5, FolloweeID: 6, OccurredAt: time.Now()})
	if err := handler.Handle(context.Background(), "event-2", outbox.UserFollowed, content); err != nil {
		t.Fatal(err)
	}
	if creator.input.ActorID != 5 || creator.input.RecipientID != 6 {
		t.Fatalf("关注事件映射错误: %+v", creator.input)
	}
}

func TestEventHandlerRejectsMalformedPayloadPermanently(t *testing.T) {
	if err := NewEventHandler(&fakeCreator{}).Handle(context.Background(), "event", outbox.AnswerLiked, []byte("bad")); err == nil {
		t.Fatal("损坏的事件载荷应返回永久错误")
	}
}
