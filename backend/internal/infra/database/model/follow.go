package model

import (
	"time"

	domainfollow "askflow/internal/domain/follow"
)

// Follow 对应 MySQL 中的 user_follows 表。
type Follow struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	FollowerID uint64    `gorm:"not null;uniqueIndex:idx_user_follows_pair,priority:1;index"`
	FolloweeID uint64    `gorm:"not null;uniqueIndex:idx_user_follows_pair,priority:2;index"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (Follow) TableName() string {
	return "user_follows"
}

// Follow_Domain2Follow_Model 将关注领域实体转换为数据库模型。
func Follow_Domain2Follow_Model(follow *domainfollow.Follow) *Follow {
	return &Follow{
		ID:         follow.ID,
		FollowerID: follow.FollowerID,
		FolloweeID: follow.FolloweeID,
		CreatedAt:  follow.CreatedAt,
	}
}

// Follow_Model2Follow_Domain 将关注数据库模型还原为领域实体。
func Follow_Model2Follow_Domain(record *Follow) *domainfollow.Follow {
	return &domainfollow.Follow{
		ID:         record.ID,
		FollowerID: record.FollowerID,
		FolloweeID: record.FolloweeID,
		CreatedAt:  record.CreatedAt,
	}
}
