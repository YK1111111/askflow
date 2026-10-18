package feed

import (
	"context"
	"testing"
	"time"

	domainfeed "askflow/internal/domain/feed"
)

type fakeHotRepository struct {
	items  []domainfeed.HotItem
	since  time.Time
	asOf   time.Time
	cursor *domainfeed.HotCursor
	limit  int
}

func (r *fakeHotRepository) Find_FollowingFeed(context.Context, uint64, *domainfeed.Cursor, int) ([]domainfeed.FollowingItem, error) {
	return nil, nil
}

func (r *fakeHotRepository) Find_HotFeed(_ context.Context, since, asOf time.Time, cursor *domainfeed.HotCursor, limit int) ([]domainfeed.HotItem, error) {
	r.since, r.asOf, r.cursor, r.limit = since, asOf, cursor, limit
	return r.items, nil
}

func (r *fakeHotRepository) Find_RecommendedFeed(context.Context, uint64, time.Time, *domainfeed.RecommendedCursor, int) ([]domainfeed.RecommendedItem, error) {
	return nil, nil
}

func TestGetHotFeedUsesWindowAndCursor(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	rankedAt := now.Add(-time.Second)
	repository := &fakeHotRepository{items: []domainfeed.HotItem{
		{QuestionID: 2, QuestionTitle: "热门问题", HotScore: 9.5, AnswerCount24H: 2, LikeCount24H: 3, RankedAt: rankedAt},
		{QuestionID: 1, HotScore: 4.2},
	}}
	service := NewHotService(repository)
	service.now = func() time.Time { return now }

	output, err := service.Get_HotFeed(context.Background(), "", 1)
	if err != nil {
		t.Fatalf("获取热门 Feed 失败: %v", err)
	}
	if !repository.since.Equal(now.Add(-24*time.Hour)) || !repository.asOf.Equal(now) || repository.limit != 2 {
		t.Fatalf("查询窗口或数量不符合预期: since=%v limit=%d", repository.since, repository.limit)
	}
	if len(output.Items) != 1 || output.Items[0].Stats.AnswerCount != 2 || output.NextCursor == "" {
		t.Fatalf("热门 Feed 输出不符合预期: %+v", output)
	}
	cursor, err := decodeHotCursor(output.NextCursor)
	if err != nil || cursor.QuestionID != 2 || cursor.Score != 9.5 || !cursor.AsOf.Equal(rankedAt) {
		t.Fatalf("热门 Feed 游标不符合预期: %+v, %v", cursor, err)
	}
}

func TestGetHotFeedRejectsInvalidCursor(t *testing.T) {
	_, err := NewHotService(&fakeHotRepository{}).Get_HotFeed(context.Background(), "bad", 20)
	if err != ErrInvalidCursor {
		t.Fatalf("无效游标应返回指定错误，实际为 %v", err)
	}
}
