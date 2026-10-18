// Package answerranking 编排问题内回答排序和分页。
package answerranking

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	domainanswerranking "askflow/internal/domain/answerranking"
	domainquestion "askflow/internal/domain/question"
)

const (
	defaultLimit = 20
	maxLimit     = 50
)

type AuthorOutput struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type ItemOutput struct {
	ID            uint64       `json:"id"`
	QuestionID    uint64       `json:"question_id"`
	Content       string       `json:"content"`
	Author        AuthorOutput `json:"author"`
	LikeCount     uint64       `json:"like_count"`
	CommentCount  uint64       `json:"comment_count"`
	FavoriteCount uint64       `json:"favorite_count"`
	QualityScore  float64      `json:"quality_score"`
	Pinned        bool         `json:"pinned"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Output struct {
	Items      []ItemOutput `json:"items"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

type cursorPayload struct {
	Score      float64   `json:"score"`
	AnswerID   uint64    `json:"answer_id"`
	QuestionID uint64    `json:"question_id"`
	AnchorID   uint64    `json:"anchor_answer_id"`
	AsOf       time.Time `json:"as_of"`
}

type Service struct {
	repository         domainanswerranking.Repository
	questionRepository domainquestion.Repository
	now                func() time.Time
}

func NewService(repository domainanswerranking.Repository, questionRepository domainquestion.Repository) *Service {
	return &Service{repository: repository, questionRepository: questionRepository, now: time.Now}
}

func (s *Service) List_RankedAnswers(ctx context.Context, questionID, anchorID uint64, cursorText string, limit int) (Output, error) {
	if _, err := s.questionRepository.Find_QuestionByID(ctx, questionID); err != nil {
		return Output{}, err
	}
	cursor, err := decodeCursor(cursorText)
	if err != nil {
		return Output{}, err
	}
	if cursor != nil {
		if cursor.QuestionID != questionID || (anchorID != 0 && anchorID != cursor.AnchorID) {
			return Output{}, domainanswerranking.ErrInvalidCursor
		}
		anchorID = cursor.AnchorID
	}
	limit = normalizeLimit(limit)
	asOf := s.now()
	if cursor != nil {
		asOf = cursor.AsOf
	}

	output := Output{Items: make([]ItemOutput, 0, limit)}
	if cursor == nil && anchorID != 0 {
		anchor, err := s.repository.Find_AnswerForQuestion(ctx, questionID, anchorID)
		if err != nil {
			return Output{}, err
		}
		output.Items = append(output.Items, toOutput(*anchor, true))
	}

	capacity := limit - len(output.Items)
	queryLimit := capacity + 1
	if queryLimit < 1 {
		queryLimit = 1
	}
	ranked, err := s.repository.Find_RankedAnswers(ctx, questionID, anchorID, asOf, cursor, queryLimit)
	if err != nil {
		return Output{}, err
	}
	hasMore := len(ranked) > capacity
	if len(ranked) > capacity {
		ranked = ranked[:capacity]
	}
	for _, item := range ranked {
		output.Items = append(output.Items, toOutput(item, false))
	}
	if hasMore {
		rankedAt := asOf
		if len(ranked) > 0 && !ranked[0].RankedAt.IsZero() {
			rankedAt = ranked[0].RankedAt
		}
		next := domainanswerranking.Cursor{QuestionID: questionID, AnchorID: anchorID, AsOf: rankedAt}
		if len(ranked) > 0 {
			last := ranked[len(ranked)-1]
			next.Score, next.AnswerID = last.QualityScore, last.ID
		}
		output.NextCursor = encodeCursor(next)
	}
	return output, nil
}

func toOutput(item domainanswerranking.Item, pinned bool) ItemOutput {
	return ItemOutput{
		ID: item.ID, QuestionID: item.QuestionID, Content: item.Content,
		Author:    AuthorOutput{ID: item.AuthorID, Username: item.AuthorName, Nickname: item.AuthorNick, AvatarURL: item.AuthorAvatar},
		LikeCount: item.LikeCount, CommentCount: item.CommentCount, FavoriteCount: item.FavoriteCount,
		QualityScore: item.QualityScore, Pinned: pinned, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
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

func encodeCursor(cursor domainanswerranking.Cursor) string {
	data, _ := json.Marshal(cursorPayload{
		Score: cursor.Score, AnswerID: cursor.AnswerID, QuestionID: cursor.QuestionID,
		AnchorID: cursor.AnchorID, AsOf: cursor.AsOf,
	})
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeCursor(value string) (*domainanswerranking.Cursor, error) {
	if value == "" {
		return nil, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, domainanswerranking.ErrInvalidCursor
	}
	var payload cursorPayload
	if err := json.Unmarshal(data, &payload); err != nil || payload.QuestionID == 0 || payload.AsOf.IsZero() ||
		(payload.Score < 0 || (payload.Score > 0 && payload.AnswerID == 0) || (payload.Score == 0 && payload.AnchorID == 0)) {
		return nil, domainanswerranking.ErrInvalidCursor
	}
	return &domainanswerranking.Cursor{
		Score: payload.Score, AnswerID: payload.AnswerID, QuestionID: payload.QuestionID,
		AnchorID: payload.AnchorID, AsOf: payload.AsOf,
	}, nil
}
