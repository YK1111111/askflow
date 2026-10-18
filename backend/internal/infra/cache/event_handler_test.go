package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"askflow/internal/infra/outbox"
)

type countingFeedInvalidator struct {
	followingUser   uint64
	recommendedUser uint64
}

type countingHotSink struct {
	eventID    string
	questionID uint64
	delta      float64
}

func (s *countingHotSink) ApplyHotDelta(_ context.Context, eventID string, questionID uint64, delta float64, _ time.Time) error {
	s.eventID, s.questionID, s.delta = eventID, questionID, delta
	return nil
}

func (i *countingFeedInvalidator) Invalidate_FollowingFeed(_ context.Context, userID uint64) {
	i.followingUser = userID
}
func (i *countingFeedInvalidator) Invalidate_RecommendedFeed(_ context.Context, userID uint64) {
	i.recommendedUser = userID
}

func TestEventInvalidationHandler(t *testing.T) {
	answerInvalidator := &countingInvalidator{}
	feedInvalidator := &countingFeedInvalidator{}
	hotSink := &countingHotSink{}
	handler := NewEventInvalidationHandler(answerInvalidator, answerInvalidator, feedInvalidator, feedInvalidator, hotSink)

	answerPayload, _ := json.Marshal(outbox.AnswerPayload{AnswerID: 7, QuestionID: 3, OccurredAt: time.Now()})
	if err := handler.Handle(context.Background(), "event-1", outbox.AnswerLiked, answerPayload); err != nil || answerInvalidator.calls != 2 || hotSink.delta != 2 {
		t.Fatalf("回答互动应失效统计和排序缓存: calls=%d err=%v", answerInvalidator.calls, err)
	}
	followPayload, _ := json.Marshal(outbox.FollowPayload{FollowerID: 11, FolloweeID: 12})
	if err := handler.Handle(context.Background(), "event-2", outbox.UserFollowed, followPayload); err != nil || feedInvalidator.followingUser != 11 || feedInvalidator.recommendedUser != 11 {
		t.Fatalf("关注事件应失效两个 Feed: %+v err=%v", feedInvalidator, err)
	}
	if err := handler.Handle(context.Background(), "event-3", outbox.AnswerLiked, []byte("bad")); err == nil {
		t.Fatal("损坏事件载荷应返回错误")
	}
}
