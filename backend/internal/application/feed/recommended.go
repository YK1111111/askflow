package feed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	domainfeed "askflow/internal/domain/feed"
)

type RecommendedItemOutput struct {
	Question        QuestionOutput `json:"question"`
	Answer          AnswerOutput   `json:"answer"`
	RecommendScore  float64        `json:"recommend_score"`
	FollowingAuthor bool           `json:"following_author"`
}

type RecommendedOutput struct {
	Items      []RecommendedItemOutput `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}

type recommendedCursorPayload struct {
	Score    float64   `json:"score"`
	AnswerID uint64    `json:"answer_id"`
	AsOf     time.Time `json:"as_of"`
}

type RecommendedService struct {
	repository domainfeed.Repository
	now        func() time.Time
}

func NewRecommendedService(repository domainfeed.Repository) *RecommendedService {
	return &RecommendedService{repository: repository, now: time.Now}
}

func (s *RecommendedService) Get_RecommendedFeed(ctx context.Context, userID uint64, cursorText string, limit int) (RecommendedOutput, error) {
	cursor, err := decodeRecommendedCursor(cursorText)
	if err != nil {
		return RecommendedOutput{}, err
	}
	asOf := s.now()
	if cursor != nil {
		asOf = cursor.AsOf
	}
	limit = normalizeLimit(limit)
	items, err := s.repository.Find_RecommendedFeed(ctx, userID, asOf, cursor, limit+1)
	if err != nil {
		return RecommendedOutput{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	output := RecommendedOutput{Items: make([]RecommendedItemOutput, 0, len(items))}
	for _, item := range items {
		output.Items = append(output.Items, RecommendedItemOutput{
			Question: QuestionOutput{
				ID: item.QuestionID, Title: item.QuestionTitle,
				Author: AuthorOutput{ID: item.QuestionAuthorID, Username: item.QuestionAuthorName,
					Nickname: item.QuestionAuthorNick, AvatarURL: item.QuestionAuthorAvatar},
			},
			Answer: AnswerOutput{
				ID: item.AnswerID, Summary: summarize(item.AnswerContent),
				Author: AuthorOutput{ID: item.AnswerAuthorID, Username: item.AnswerAuthorName,
					Nickname: item.AnswerAuthorNick, AvatarURL: item.AnswerAuthorAvatar},
				LikeCount: item.LikeCount, CommentCount: item.CommentCount,
				FavoriteCount: item.FavoriteCount, CreatedAt: item.AnswerCreatedAt,
			},
			RecommendScore:  item.RecommendScore,
			FollowingAuthor: item.FollowingAuthor,
		})
	}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		rankedAt := asOf
		if !last.RankedAt.IsZero() {
			rankedAt = last.RankedAt
		}
		output.NextCursor = encodeRecommendedCursor(domainfeed.RecommendedCursor{
			Score: last.RecommendScore, AnswerID: last.AnswerID, AsOf: rankedAt,
		})
	}
	return output, nil
}

func encodeRecommendedCursor(cursor domainfeed.RecommendedCursor) string {
	data, _ := json.Marshal(recommendedCursorPayload{Score: cursor.Score, AnswerID: cursor.AnswerID, AsOf: cursor.AsOf})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeRecommendedCursor(value string) (*domainfeed.RecommendedCursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var payload recommendedCursorPayload
	if err := json.Unmarshal(data, &payload); err != nil || payload.Score <= 0 || payload.AnswerID == 0 || payload.AsOf.IsZero() {
		return nil, ErrInvalidCursor
	}
	return &domainfeed.RecommendedCursor{Score: payload.Score, AnswerID: payload.AnswerID, AsOf: payload.AsOf}, nil
}
