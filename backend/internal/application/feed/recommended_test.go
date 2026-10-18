package feed

import (
	"context"
	"testing"
	"time"

	domainfeed "askflow/internal/domain/feed"
)

type fakeRecommendedRepository struct {
	items  []domainfeed.RecommendedItem
	userID uint64
	asOf   time.Time
	limit  int
}

func (r *fakeRecommendedRepository) Find_FollowingFeed(context.Context, uint64, *domainfeed.Cursor, int) ([]domainfeed.FollowingItem, error) {
	return nil, nil
}
func (r *fakeRecommendedRepository) Find_HotFeed(context.Context, time.Time, time.Time, *domainfeed.HotCursor, int) ([]domainfeed.HotItem, error) {
	return nil, nil
}
func (r *fakeRecommendedRepository) Find_RecommendedFeed(_ context.Context, userID uint64, asOf time.Time, _ *domainfeed.RecommendedCursor, limit int) ([]domainfeed.RecommendedItem, error) {
	r.userID, r.asOf, r.limit = userID, asOf, limit
	return r.items, nil
}

func TestGetRecommendedFeedBuildsPersonalizedPage(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	rankedAt := now.Add(-time.Second)
	repository := &fakeRecommendedRepository{items: []domainfeed.RecommendedItem{
		{QuestionID: 1, AnswerID: 10, AnswerContent: "回答", RecommendScore: 8.2, FollowingAuthor: true, RankedAt: rankedAt},
		{QuestionID: 2, AnswerID: 9, RecommendScore: 5},
	}}
	service := NewRecommendedService(repository)
	service.now = func() time.Time { return now }

	output, err := service.Get_RecommendedFeed(context.Background(), 7, "", 1)
	if err != nil {
		t.Fatalf("获取推荐 Feed 失败: %v", err)
	}
	if repository.userID != 7 || !repository.asOf.Equal(now) || repository.limit != 2 {
		t.Fatalf("推荐查询参数不符合预期: %+v", repository)
	}
	if len(output.Items) != 1 || !output.Items[0].FollowingAuthor || output.NextCursor == "" {
		t.Fatalf("推荐 Feed 输出不符合预期: %+v", output)
	}
	cursor, err := decodeRecommendedCursor(output.NextCursor)
	if err != nil || cursor.AnswerID != 10 || cursor.Score != 8.2 || !cursor.AsOf.Equal(rankedAt) {
		t.Fatalf("推荐游标不符合预期: %+v, %v", cursor, err)
	}
}
