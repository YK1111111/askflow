package feed

import (
	"context"
	"strings"
	"testing"
	"time"

	domainfeed "askflow/internal/domain/feed"
)

type fakeFeedRepository struct {
	items          []domainfeed.FollowingItem
	receivedCursor *domainfeed.Cursor
	receivedLimit  int
}

func (r *fakeFeedRepository) Find_FollowingFeed(_ context.Context, _ uint64, cursor *domainfeed.Cursor, limit int) ([]domainfeed.FollowingItem, error) {
	r.receivedCursor = cursor
	r.receivedLimit = limit
	return r.items, nil
}

func (r *fakeFeedRepository) Find_HotFeed(context.Context, time.Time, time.Time, *domainfeed.HotCursor, int) ([]domainfeed.HotItem, error) {
	return nil, nil
}

func (r *fakeFeedRepository) Find_RecommendedFeed(context.Context, uint64, time.Time, *domainfeed.RecommendedCursor, int) ([]domainfeed.RecommendedItem, error) {
	return nil, nil
}

func TestGetFollowingFeedReturnsPageAndCursor(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	repository := &fakeFeedRepository{items: []domainfeed.FollowingItem{
		{
			QuestionID: 1, QuestionTitle: "问题一", QuestionAuthorID: 10, QuestionAuthorName: "questioner",
			AnswerID: 101, AnswerContent: strings.Repeat("答", 201), AnswerAuthorID: 20,
			AnswerAuthorName: "alice", LikeCount: 3, CommentCount: 4, FavoriteCount: 5, AnswerCreatedAt: now,
		},
		{QuestionID: 2, AnswerID: 100, AnswerCreatedAt: now.Add(-time.Minute)},
	}}
	service := NewFollowingService(repository)

	output, err := service.Get_FollowingFeed(context.Background(), 1, "", 1)
	if err != nil {
		t.Fatalf("获取关注 Feed 失败: %v", err)
	}
	if repository.receivedLimit != 2 || len(output.Items) != 1 || output.NextCursor == "" {
		t.Fatalf("分页结果不符合预期: %+v", output)
	}
	item := output.Items[0]
	if item.Question.ID != 1 || item.Answer.ID != 101 || item.Answer.LikeCount != 3 ||
		!strings.HasSuffix(item.Answer.Summary, "…") {
		t.Fatalf("Feed 内容不符合预期: %+v", item)
	}

	cursor, err := decodeCursor(output.NextCursor)
	if err != nil || cursor.AnswerID != 101 || !cursor.CreatedAt.Equal(now) {
		t.Fatalf("下一页游标不符合预期: %+v, %v", cursor, err)
	}
}

func TestGetFollowingFeedAcceptsAndRejectsCursor(t *testing.T) {
	now := time.Now().UTC()
	repository := &fakeFeedRepository{}
	service := NewFollowingService(repository)
	cursorText := encodeCursor(domainfeed.Cursor{CreatedAt: now, AnswerID: 9})

	if _, err := service.Get_FollowingFeed(context.Background(), 1, cursorText, 0); err != nil {
		t.Fatalf("有效游标不应失败: %v", err)
	}
	if repository.receivedCursor == nil || repository.receivedCursor.AnswerID != 9 || repository.receivedLimit != defaultLimit+1 {
		t.Fatalf("仓储参数不符合预期: %+v, limit=%d", repository.receivedCursor, repository.receivedLimit)
	}
	if _, err := service.Get_FollowingFeed(context.Background(), 1, "not-a-cursor", 20); err != ErrInvalidCursor {
		t.Fatalf("无效游标应返回指定错误，实际为 %v", err)
	}
}

func TestGetFollowingFeedReturnsEmptyItems(t *testing.T) {
	output, err := NewFollowingService(&fakeFeedRepository{}).Get_FollowingFeed(context.Background(), 1, "", 20)
	if err != nil {
		t.Fatalf("获取空 Feed 失败: %v", err)
	}
	if output.Items == nil || len(output.Items) != 0 || output.NextCursor != "" {
		t.Fatalf("空 Feed 应返回空数组: %+v", output)
	}
}
