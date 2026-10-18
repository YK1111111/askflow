// Package feed 编排 Feed 查询用例。
package feed

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	domainfeed "askflow/internal/domain/feed"
)

const (
	defaultLimit        = 20
	maxLimit            = 50
	answerSummaryLength = 200
)

var ErrInvalidCursor = errors.New("Feed 游标不正确")

type AuthorOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type QuestionOutput struct {
	ID     uint64       `json:"id"`
	Title  string       `json:"title"`
	Author AuthorOutput `json:"author"`
}

type AnswerOutput struct {
	ID            uint64       `json:"id"`
	Summary       string       `json:"summary"`
	Author        AuthorOutput `json:"author"`
	LikeCount     uint64       `json:"like_count"`
	CommentCount  uint64       `json:"comment_count"`
	FavoriteCount uint64       `json:"favorite_count"`
	CreatedAt     time.Time    `json:"created_at"`
}

type FollowingItemOutput struct {
	Question QuestionOutput `json:"question"`
	Answer   AnswerOutput   `json:"answer"`
}

type FollowingOutput struct {
	Items      []FollowingItemOutput `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type cursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	AnswerID  uint64    `json:"answer_id"`
}

type FollowingService struct {
	repository domainfeed.Repository
}

func NewFollowingService(repository domainfeed.Repository) *FollowingService {
	return &FollowingService{repository: repository}
}

// Get_FollowingFeed 获取当前用户关注作者发布的回答，并按问题去重。
func (s *FollowingService) Get_FollowingFeed(ctx context.Context, userID uint64, cursorText string, limit int) (FollowingOutput, error) {
	cursor, err := decodeCursor(cursorText)
	if err != nil {
		return FollowingOutput{}, err
	}
	limit = normalizeLimit(limit)

	items, err := s.repository.Find_FollowingFeed(ctx, userID, cursor, limit+1)
	if err != nil {
		return FollowingOutput{}, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	output := FollowingOutput{Items: make([]FollowingItemOutput, 0, len(items))}
	for _, item := range items {
		output.Items = append(output.Items, FollowingItemOutput{
			Question: QuestionOutput{
				ID:    item.QuestionID,
				Title: item.QuestionTitle,
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
		})
	}
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		output.NextCursor = encodeCursor(domainfeed.Cursor{CreatedAt: last.AnswerCreatedAt, AnswerID: last.AnswerID})
	}
	return output, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func summarize(content string) string {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) <= answerSummaryLength {
		return content
	}
	runes := []rune(content)
	return string(runes[:answerSummaryLength]) + "…"
}

func encodeCursor(cursor domainfeed.Cursor) string {
	data, _ := json.Marshal(cursorPayload{CreatedAt: cursor.CreatedAt, AnswerID: cursor.AnswerID})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (*domainfeed.Cursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil || payload.CreatedAt.IsZero() || payload.AnswerID == 0 {
		return nil, ErrInvalidCursor
	}
	return &domainfeed.Cursor{CreatedAt: payload.CreatedAt, AnswerID: payload.AnswerID}, nil
}
