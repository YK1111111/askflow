package repository

import (
	"context"
	"os"
	"testing"
	"time"

	domainnotification "askflow/internal/domain/notification"
	"askflow/internal/infra/database"
	"askflow/internal/infra/database/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestNotificationRepositoryIsIdempotentAndUserScoped(t *testing.T) {
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
	repository := NewNotificationRepository(db)
	eventID := "notification-test-event-00000001"
	t.Cleanup(func() { _ = db.Where("event_id = ?", eventID).Delete(&model.Notification{}).Error })
	value := &domainnotification.Notification{EventID: eventID, UserID: 101, ActorID: 202,
		Type: domainnotification.TypeUserFollowed, Content: "有用户关注了你", CreatedAt: time.Now()}
	created, err := repository.Create_Notification(context.Background(), value)
	if err != nil || !created {
		t.Fatalf("创建通知失败: created=%v err=%v", created, err)
	}
	created, err = repository.Create_Notification(context.Background(), value)
	if err != nil || created {
		t.Fatalf("重复事件不应创建通知: created=%v err=%v", created, err)
	}
	if err := repository.Mark_NotificationRead(context.Background(), 999, value.ID); err == nil {
		t.Fatal("其他用户不能修改通知状态")
	}
	if err := repository.Mark_NotificationRead(context.Background(), 101, value.ID); err != nil {
		t.Fatal(err)
	}
	count, err := repository.Count_UnreadNotifications(context.Background(), 101)
	if err != nil || count != 0 {
		t.Fatalf("未读计数错误: count=%d err=%v", count, err)
	}
}
