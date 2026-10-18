package repository

import (
	"context"
	"os"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domainanswerlike "askflow/internal/domain/answerlike"
	"askflow/internal/infra/database"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestBusinessWritesCreateOutboxEventsInTransaction(t *testing.T) {
	dsn := os.Getenv("ASKFLOW_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("未设置 ASKFLOW_MYSQL_TEST_DSN，跳过 MySQL 集成测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	t.Cleanup(func() { _ = tx.Rollback().Error })

	user := model.User{Username: uniqueName("outbox_user"), PasswordHash: "test", Nickname: "事件用户"}
	if err := tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	question := model.Question{AuthorID: user.ID, Title: "Outbox", Content: "事务事件"}
	if err := tx.Create(&question).Error; err != nil {
		t.Fatal(err)
	}

	answer := &domainanswer.Answer{QuestionID: question.ID, AuthorID: user.ID, Content: "回答"}
	if err := NewAnswerRepository(tx).Create_Answer(context.Background(), answer); err != nil {
		t.Fatalf("发布回答失败: %v", err)
	}
	like := &domainanswerlike.AnswerLike{UserID: user.ID, AnswerID: answer.ID}
	if err := NewAnswerLikeRepository(tx).Create_AnswerLike(context.Background(), like); err != nil {
		t.Fatalf("点赞回答失败: %v", err)
	}
	if err := NewQuestionViewRepository(tx).Record_QuestionView(context.Background(), question.ID); err != nil {
		t.Fatalf("记录问题浏览失败: %v", err)
	}

	var events []model.OutboxEvent
	if err := tx.Where("aggregate_type = ? AND aggregate_id = ?", "answer", answer.ID).Order("created_at ASC").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != outbox.AnswerPublished || events[1].EventType != outbox.AnswerLiked {
		t.Fatalf("业务事务应生成对应 Outbox 事件: %+v", events)
	}
	var viewEvents []model.OutboxEvent
	if err := tx.Where("aggregate_type = ? AND aggregate_id = ? AND event_type = ?", "question", question.ID, outbox.QuestionViewed).
		Find(&viewEvents).Error; err != nil {
		t.Fatal(err)
	}
	if len(viewEvents) != 1 {
		t.Fatalf("问题浏览与 Outbox 事件应在同一事务中生成: %+v", viewEvents)
	}
	var storedAnswer model.Answer
	if err := tx.First(&storedAnswer, answer.ID).Error; err != nil || storedAnswer.LikeCount != 1 {
		t.Fatalf("业务数据与事件应一同提交: answer=%+v err=%v", storedAnswer, err)
	}
}
