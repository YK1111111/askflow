// Package database 提供数据库连接的基础设施实现。
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"askflow/internal/infra/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQL 保存 GORM 与底层数据库连接。
type MySQL struct {
	DB    *gorm.DB
	sqlDB *sql.DB
}

// NewMySQL 建立 MySQL 连接，并验证数据库是否可用。
func NewMySQL(cfg config.DatabaseConfig) (*MySQL, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("建立 MySQL 连接: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 MySQL 连接: %w", err)
	}
	connMaxLifetime, err := time.ParseDuration(cfg.ConnMaxLifetime)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("解析 MySQL 连接最大生命周期: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	pingContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingContext); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("检查 MySQL 连接: %w", err)
	}

	return &MySQL{DB: db, sqlDB: sqlDB}, nil
}

// Close 关闭底层数据库连接池。
func (m *MySQL) Close() error {
	return m.sqlDB.Close()
}
