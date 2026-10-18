package answerranking

import (
	"context"
	"testing"
	"time"

	domainanswerranking "askflow/internal/domain/answerranking"
	domainquestion "askflow/internal/domain/question"
)

type fakeRankingRepository struct {
	anchor        *domainanswerranking.Item
	ranked        []domainanswerranking.Item
	excludedID    uint64
	receivedLimit int
}

func (r *fakeRankingRepository) Find_AnswerForQuestion(context.Context, uint64, uint64) (*domainanswerranking.Item, error) {
	return r.anchor, nil
}
func (r *fakeRankingRepository) Find_RankedAnswers(_ context.Context, _ uint64, excludeID uint64, _ time.Time, _ *domainanswerranking.Cursor, limit int) ([]domainanswerranking.Item, error) {
	r.excludedID, r.receivedLimit = excludeID, limit
	return r.ranked, nil
}

type fakeQuestionRepository struct{}

func (fakeQuestionRepository) Create_Question(context.Context, *domainquestion.Question) error {
	return nil
}
func (fakeQuestionRepository) Find_QuestionByID(context.Context, uint64) (*domainquestion.Question, error) {
	return &domainquestion.Question{ID: 1}, nil
}

func TestListRankedAnswersPinsAnchorOnlyOnFirstPage(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	repository := &fakeRankingRepository{
		anchor: &domainanswerranking.Item{ID: 10, QuestionID: 1, Content: "Feed 回答"},
		ranked: []domainanswerranking.Item{
			{ID: 9, QuestionID: 1, QualityScore: 8},
			{ID: 8, QuestionID: 1, QualityScore: 7},
		},
	}
	service := NewService(repository, fakeQuestionRepository{})
	service.now = func() time.Time { return now }

	output, err := service.List_RankedAnswers(context.Background(), 1, 10, "", 2)
	if err != nil {
		t.Fatalf("查询回答排序失败: %v", err)
	}
	if len(output.Items) != 2 || !output.Items[0].Pinned || output.Items[0].ID != 10 || output.Items[1].ID != 9 {
		t.Fatalf("置顶及排序结果不符合预期: %+v", output)
	}
	if repository.excludedID != 10 || repository.receivedLimit != 2 || output.NextCursor == "" {
		t.Fatalf("分页查询参数不符合预期: excluded=%d limit=%d output=%+v", repository.excludedID, repository.receivedLimit, output)
	}

	cursor, err := decodeCursor(output.NextCursor)
	if err != nil || cursor.AnswerID != 9 || cursor.AnchorID != 10 || !cursor.AsOf.Equal(now) {
		t.Fatalf("回答游标不符合预期: %+v, %v", cursor, err)
	}
}

func TestListRankedAnswersSupportsPinnedOnlyPage(t *testing.T) {
	repository := &fakeRankingRepository{
		anchor: &domainanswerranking.Item{ID: 10, QuestionID: 1},
		ranked: []domainanswerranking.Item{{ID: 9, QuestionID: 1, QualityScore: 8}},
	}
	output, err := NewService(repository, fakeQuestionRepository{}).
		List_RankedAnswers(context.Background(), 1, 10, "", 1)
	if err != nil || len(output.Items) != 1 || !output.Items[0].Pinned || output.NextCursor == "" {
		t.Fatalf("仅返回置顶回答的第一页不符合预期: %+v, %v", output, err)
	}
	cursor, err := decodeCursor(output.NextCursor)
	if err != nil || cursor.Score != 0 || cursor.AnswerID != 0 {
		t.Fatalf("起始分页游标不符合预期: %+v, %v", cursor, err)
	}
}
