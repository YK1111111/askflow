package feed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	domainfeed "askflow/internal/domain/feed"
)

const hotWindow = 24 * time.Hour

type HotStatsOutput struct {
	AnswerCount  uint64 `json:"answer_count_24h"`
	LikeCount    uint64 `json:"like_count_24h"`
	CommentCount uint64 `json:"comment_count_24h"`
	ViewCount    uint64 `json:"view_count_24h"`
}

type HotItemOutput struct {
	Question  QuestionOutput `json:"question"`
	Summary   string         `json:"summary"`
	Stats     HotStatsOutput `json:"stats"`
	HotScore  float64        `json:"hot_score"`
	CreatedAt time.Time      `json:"created_at"`
}

type HotOutput struct {
	Items      []HotItemOutput `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type hotCursorPayload struct {
	Score      float64   `json:"score"`
	QuestionID uint64    `json:"question_id"`
	AsOf       time.Time `json:"as_of"`
}

type HotService struct {
	repository domainfeed.Repository
	now        func() time.Time
}

func NewHotService(repository domainfeed.Repository) *HotService {
	return &HotService{repository: repository, now: time.Now}
}

func (s *HotService) Get_HotFeed(ctx context.Context, cursorText string, limit int) (HotOutput, error) {
	cursor, err := decodeHotCursor(cursorText)
	if err != nil {
		return HotOutput{}, err
	}
	limit = normalizeLimit(limit)
	asOf := s.now()
	if cursor != nil {
		asOf = cursor.AsOf
	}
	items, err := s.repository.Find_HotFeed(ctx, asOf.Add(-hotWindow), asOf, cursor, limit+1)
	if err != nil {
		return HotOutput{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	output := HotOutput{Items: make([]HotItemOutput, 0, len(items))}
	for _, item := range items {
		output.Items = append(output.Items, HotItemOutput{
			Question: QuestionOutput{
				ID: item.QuestionID, Title: item.QuestionTitle,
				Author: AuthorOutput{ID: item.QuestionAuthorID, Username: item.QuestionAuthorName,
					Nickname: item.QuestionAuthorNick, AvatarURL: item.QuestionAuthorAvatar},
			},
			Summary: summarize(item.QuestionContent),
			Stats: HotStatsOutput{
				AnswerCount: item.AnswerCount24H, LikeCount: item.LikeCount24H,
				CommentCount: item.CommentCount24H, ViewCount: item.ViewCount24H,
			},
			HotScore: item.HotScore, CreatedAt: item.QuestionCreatedAt,
		})
	}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		rankedAt := asOf
		if !last.RankedAt.IsZero() {
			rankedAt = last.RankedAt
		}
		output.NextCursor = encodeHotCursor(domainfeed.HotCursor{Score: last.HotScore, QuestionID: last.QuestionID, AsOf: rankedAt})
	}
	return output, nil
}

func encodeHotCursor(cursor domainfeed.HotCursor) string {
	data, _ := json.Marshal(hotCursorPayload{Score: cursor.Score, QuestionID: cursor.QuestionID, AsOf: cursor.AsOf})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeHotCursor(value string) (*domainfeed.HotCursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var payload hotCursorPayload
	if err := json.Unmarshal(data, &payload); err != nil || payload.Score <= 0 || payload.QuestionID == 0 || payload.AsOf.IsZero() {
		return nil, ErrInvalidCursor
	}
	return &domainfeed.HotCursor{Score: payload.Score, QuestionID: payload.QuestionID, AsOf: payload.AsOf}, nil
}
