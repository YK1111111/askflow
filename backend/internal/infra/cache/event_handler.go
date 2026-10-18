package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"askflow/internal/infra/messaging"
	"askflow/internal/infra/outbox"
)

type HotEventSink interface {
	ApplyHotDelta(ctx context.Context, eventID string, questionID uint64, delta float64, occurredAt time.Time) error
}

// EventInvalidationHandler 消费领域事件并执行幂等缓存失效。
type EventInvalidationHandler struct {
	stats       AnswerStatsInvalidator
	ranks       AnswerRankInvalidator
	following   followingFeedInvalidator
	recommended recommendedFeedInvalidator
	hot         HotEventSink
}

func NewEventInvalidationHandler(stats AnswerStatsInvalidator, ranks AnswerRankInvalidator, following followingFeedInvalidator, recommended recommendedFeedInvalidator, hot HotEventSink) *EventInvalidationHandler {
	return &EventInvalidationHandler{stats: stats, ranks: ranks, following: following, recommended: recommended, hot: hot}
}

func (h *EventInvalidationHandler) Handle(ctx context.Context, eventID, eventType string, content []byte) error {
	switch eventType {
	case outbox.AnswerPublished:
		var payload outbox.AnswerPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析回答发布事件: %w", err))
		}
		if err := h.hot.ApplyHotDelta(ctx, eventID, payload.QuestionID, 5, payload.OccurredAt); err != nil {
			return err
		}
		h.ranks.Invalidate_AnswerRank(ctx, payload.QuestionID)
	case outbox.AnswerLiked, outbox.AnswerUnliked, outbox.AnswerCommented, outbox.AnswerFavorited, outbox.AnswerUnfavorited:
		var payload outbox.AnswerPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析回答互动事件: %w", err))
		}
		delta := map[string]float64{outbox.AnswerLiked: 2, outbox.AnswerUnliked: -2, outbox.AnswerCommented: 3}[eventType]
		if delta != 0 {
			if err := h.hot.ApplyHotDelta(ctx, eventID, payload.QuestionID, delta, payload.OccurredAt); err != nil {
				return err
			}
		}
		h.stats.Invalidate_AnswerStats(ctx, payload.AnswerID)
		h.ranks.Invalidate_AnswerRank(ctx, payload.QuestionID)
	case outbox.UserFollowed, outbox.UserUnfollowed:
		var payload outbox.FollowPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析关注事件: %w", err))
		}
		h.following.Invalidate_FollowingFeed(ctx, payload.FollowerID)
		h.recommended.Invalidate_RecommendedFeed(ctx, payload.FollowerID)
	case outbox.QuestionViewed:
		var payload outbox.QuestionPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return messaging.Permanent(fmt.Errorf("解析问题浏览事件: %w", err))
		}
		return h.hot.ApplyHotDelta(ctx, eventID, payload.QuestionID, 0.1, payload.OccurredAt)
	}
	return nil
}
