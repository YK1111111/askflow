package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	domainanswerranking "askflow/internal/domain/answerranking"
	domainfeed "askflow/internal/domain/feed"
	"askflow/internal/infra/database"
	"askflow/internal/infra/database/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestFeedRepositoryMySQL 使用真实 MySQL 验证窗口函数、去重及游标分页。
// 默认跳过，设置 ASKFLOW_MYSQL_TEST_DSN 后才执行，测试数据会在事务结束时回滚。
func TestFeedRepositoryMySQL(t *testing.T) {
	dsn := os.Getenv("ASKFLOW_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 ASKFLOW_MYSQL_TEST_DSN，跳过 MySQL 集成测试")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("连接 MySQL 失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取 MySQL 连接失败: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启测试事务失败: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })

	now := time.Now().UTC().Truncate(time.Millisecond)
	users := []model.User{
		{Username: uniqueName("viewer"), PasswordHash: "test", Nickname: "查看者"},
		{Username: uniqueName("followed_a"), PasswordHash: "test", Nickname: "关注作者 A"},
		{Username: uniqueName("followed_b"), PasswordHash: "test", Nickname: "关注作者 B"},
		{Username: uniqueName("outsider"), PasswordHash: "test", Nickname: "未关注作者"},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	viewer, followedA, followedB, outsider := users[0], users[1], users[2], users[3]
	if err := tx.Create(&[]model.Follow{
		{FollowerID: viewer.ID, FolloweeID: followedA.ID},
		{FollowerID: viewer.ID, FolloweeID: followedB.ID},
	}).Error; err != nil {
		t.Fatalf("创建关注关系失败: %v", err)
	}

	questions := []model.Question{
		{AuthorID: viewer.ID, Title: "同问题去重", Content: "内容"},
		{AuthorID: viewer.ID, Title: "第二个问题", Content: "内容"},
		{AuthorID: viewer.ID, Title: "未关注作者的问题", Content: "内容"},
	}
	if err := tx.Create(&questions).Error; err != nil {
		t.Fatalf("创建测试问题失败: %v", err)
	}
	answers := []model.Answer{
		{QuestionID: questions[0].ID, AuthorID: followedA.ID, Content: "同问题旧回答", CreatedAt: now.Add(-3 * time.Minute)},
		{QuestionID: questions[0].ID, AuthorID: followedB.ID, Content: "同问题最新回答", LikeCount: 7, CreatedAt: now.Add(-time.Minute)},
		{QuestionID: questions[1].ID, AuthorID: followedA.ID, Content: "第二个问题回答", CreatedAt: now.Add(-2 * time.Minute)},
		{QuestionID: questions[2].ID, AuthorID: outsider.ID, Content: "不应出现", CreatedAt: now},
	}
	if err := tx.Create(&answers).Error; err != nil {
		t.Fatalf("创建测试回答失败: %v", err)
	}

	repository := NewFeedRepository(tx)
	items, err := repository.Find_FollowingFeed(context.Background(), viewer.ID, nil, 10)
	if err != nil {
		t.Fatalf("查询关注 Feed 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("关注 Feed 应包含两个去重问题，实际为 %d: %+v", len(items), items)
	}
	if items[0].AnswerID != answers[1].ID || items[0].AnswerContent != "同问题最新回答" || items[0].LikeCount != 7 {
		t.Fatalf("同一问题应保留最新的关注作者回答: %+v", items[0])
	}
	if items[1].AnswerID != answers[2].ID {
		t.Fatalf("Feed 排序不符合预期: %+v", items)
	}
	hydrated, err := repository.Find_FollowingItemsByAnswerIDs(context.Background(), []uint64{answers[2].ID, answers[1].ID})
	if err != nil || len(hydrated) != 2 {
		t.Fatalf("批量补齐关注 Feed 项失败: %+v, %v", hydrated, err)
	}
	hydratedByID := map[uint64]domainfeed.FollowingItem{}
	for _, item := range hydrated {
		hydratedByID[item.AnswerID] = item
	}
	if hydratedByID[answers[1].ID].QuestionTitle != "同问题去重" || hydratedByID[answers[1].ID].AnswerAuthorID != followedB.ID {
		t.Fatalf("批量补齐的展示字段不正确: %+v", hydratedByID[answers[1].ID])
	}

	firstPage, err := repository.Find_FollowingFeed(context.Background(), viewer.ID, nil, 1)
	if err != nil || len(firstPage) != 1 {
		t.Fatalf("查询第一页失败: %+v, %v", firstPage, err)
	}
	cursor := &domainfeed.Cursor{CreatedAt: firstPage[0].AnswerCreatedAt, AnswerID: firstPage[0].AnswerID}
	secondPage, err := repository.Find_FollowingFeed(context.Background(), viewer.ID, cursor, 1)
	if err != nil || len(secondPage) != 1 || secondPage[0].AnswerID != answers[2].ID {
		t.Fatalf("游标翻页结果不符合预期: %+v, %v", secondPage, err)
	}

	if err := tx.Create(&[]model.AnswerLike{
		{UserID: viewer.ID, AnswerID: answers[1].ID, CreatedAt: now},
		{UserID: outsider.ID, AnswerID: answers[1].ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("创建点赞事件失败: %v", err)
	}
	if err := tx.Create(&model.Comment{
		AnswerID: answers[1].ID, AuthorID: viewer.ID, Content: "热门评论", CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("创建评论事件失败: %v", err)
	}
	if err := tx.Create(&[]model.QuestionView{
		{QuestionID: questions[0].ID, CreatedAt: now},
		{QuestionID: questions[0].ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("创建浏览事件失败: %v", err)
	}

	hotItems, err := repository.Find_HotFeed(context.Background(), now.Add(-24*time.Hour), now, nil, 10)
	if err != nil {
		t.Fatalf("查询热门 Feed 失败: %v", err)
	}
	if len(hotItems) != 3 {
		t.Fatalf("热门 Feed 应包含三个有近期活动的问题，实际为 %d: %+v", len(hotItems), hotItems)
	}
	if hotItems[0].QuestionID != questions[0].ID || hotItems[0].AnswerCount24H != 2 ||
		hotItems[0].LikeCount24H != 2 || hotItems[0].CommentCount24H != 1 || hotItems[0].ViewCount24H != 2 {
		t.Fatalf("热门 Feed 聚合或排序不符合预期: %+v", hotItems[0])
	}
	hotHydrated, err := repository.Find_HotQuestionsByIDs(context.Background(), []uint64{questions[1].ID, questions[0].ID})
	if err != nil || len(hotHydrated) != 2 {
		t.Fatalf("批量补齐热门问题失败: %+v, %v", hotHydrated, err)
	}
	hotByID := map[uint64]domainfeed.HotItem{}
	for _, item := range hotHydrated {
		hotByID[item.QuestionID] = item
	}
	if hotByID[questions[0].ID].QuestionTitle != "同问题去重" || hotByID[questions[0].ID].QuestionAuthorID != viewer.ID {
		t.Fatalf("批量补齐热门问题字段错误: %+v", hotByID[questions[0].ID])
	}
	hotWindowHydrated, err := repository.Find_HotItemsByQuestionIDs(
		context.Background(), []uint64{questions[1].ID, questions[0].ID}, now.Add(-24*time.Hour), now,
	)
	if err != nil || len(hotWindowHydrated) != 2 {
		t.Fatalf("按 Redis 候选批量聚合热门问题失败: %+v, %v", hotWindowHydrated, err)
	}
	hotWindowByID := map[uint64]domainfeed.HotItem{}
	for _, item := range hotWindowHydrated {
		hotWindowByID[item.QuestionID] = item
	}
	if hotWindowByID[questions[0].ID].AnswerCount24H != 2 ||
		hotWindowByID[questions[0].ID].LikeCount24H != 2 ||
		hotWindowByID[questions[0].ID].CommentCount24H != 1 ||
		hotWindowByID[questions[0].ID].ViewCount24H != 2 {
		t.Fatalf("Redis 候选的 MySQL 聚合字段错误: %+v", hotWindowByID[questions[0].ID])
	}

	recommended, err := repository.Find_RecommendedFeed(context.Background(), viewer.ID, now, nil, 10)
	if err != nil {
		t.Fatalf("查询登录用户推荐 Feed 失败: %v", err)
	}
	if len(recommended) != 3 {
		t.Fatalf("推荐 Feed 应按问题去重为三项，实际为 %d: %+v", len(recommended), recommended)
	}
	if recommended[0].AnswerID != answers[1].ID || !recommended[0].FollowingAuthor {
		t.Fatalf("同问题应保留高分回答，且识别为关注作者: %+v", recommended[0])
	}
	if recommended[1].QuestionID != questions[1].ID || !recommended[1].FollowingAuthor {
		t.Fatalf("关注作者加分应使第二个问题排在未关注作者之前: %+v", recommended)
	}
	recommendedHydrated, err := repository.Find_RecommendedItemsByAnswerIDs(context.Background(), viewer.ID, []uint64{answers[3].ID, answers[1].ID})
	if err != nil || len(recommendedHydrated) != 2 {
		t.Fatalf("批量补齐推荐 Feed 项失败: %+v, %v", recommendedHydrated, err)
	}
	recommendedByID := map[uint64]domainfeed.RecommendedItem{}
	for _, item := range recommendedHydrated {
		recommendedByID[item.AnswerID] = item
	}
	if !recommendedByID[answers[1].ID].FollowingAuthor || recommendedByID[answers[3].ID].FollowingAuthor {
		t.Fatalf("批量补齐应正确识别关注关系: %+v", recommendedByID)
	}

	anonymous, err := repository.Find_RecommendedFeed(context.Background(), 0, now, nil, 10)
	if err != nil {
		t.Fatalf("查询匿名推荐 Feed 失败: %v", err)
	}
	if len(anonymous) != 3 || anonymous[1].QuestionID != questions[2].ID || anonymous[1].FollowingAuthor {
		t.Fatalf("匿名推荐不应应用关注加分: %+v", anonymous)
	}

	highQuality := model.Answer{
		QuestionID: questions[0].ID, AuthorID: followedA.ID, Content: "高质量回答",
		CommentCount: 10, CreatedAt: now.Add(-10 * time.Minute),
	}
	if err := tx.Create(&highQuality).Error; err != nil {
		t.Fatalf("创建回答排序测试数据失败: %v", err)
	}
	rankingRepository := NewAnswerRankingRepository(tx)
	anchor, err := rankingRepository.Find_AnswerForQuestion(context.Background(), questions[0].ID, answers[0].ID)
	if err != nil || anchor.ID != answers[0].ID {
		t.Fatalf("查询指定回答失败: %+v, %v", anchor, err)
	}
	if _, err := rankingRepository.Find_AnswerForQuestion(context.Background(), questions[1].ID, answers[0].ID); !errors.Is(err, domainanswerranking.ErrAnchorWrongQuestion) {
		t.Fatalf("跨问题置顶应被拒绝，实际为 %v", err)
	}
	ranked, err := rankingRepository.Find_RankedAnswers(context.Background(), questions[0].ID, answers[0].ID, now, nil, 10)
	if err != nil {
		t.Fatalf("查询问题回答排序失败: %v", err)
	}
	if len(ranked) != 2 || ranked[0].ID != highQuality.ID || ranked[1].ID != answers[1].ID {
		t.Fatalf("回答质量排序或置顶排除不符合预期: %+v", ranked)
	}
}

func uniqueName(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405.000000000")
}
