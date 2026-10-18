package database

import (
	"context"
	"fmt"

	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
)

// Migrate 创建或更新当前项目需要的数据库表。
func Migrate(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接池: %w", err)
	}
	lockConnection, err := sqlDB.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("获取数据库迁移专用连接: %w", err)
	}
	defer lockConnection.Close()

	var acquired int
	if err := lockConnection.QueryRowContext(context.Background(), "SELECT GET_LOCK(?, ?)", "askflow_schema_migrate", 30).Scan(&acquired); err != nil {
		return fmt.Errorf("获取数据库迁移锁: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("获取数据库迁移锁超时")
	}
	defer func() {
		var released int
		_ = lockConnection.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", "askflow_schema_migrate").Scan(&released)
	}()

	if err := db.AutoMigrate(&model.User{}, &model.Question{}, &model.Answer{}, &model.Follow{}, &model.AnswerLike{}, &model.Comment{}, &model.AnswerFavorite{}, &model.QuestionView{}, &model.OutboxEvent{}, &model.Notification{}); err != nil {
		return fmt.Errorf("迁移数据库表: %w", err)
	}
	if err := db.Model(&model.User{}).
		Where("nickname = '' OR nickname IS NULL").
		Update("nickname", gorm.Expr("username")).Error; err != nil {
		return fmt.Errorf("初始化用户昵称: %w", err)
	}
	return nil
}
