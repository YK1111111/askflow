// Package model 定义数据库表与领域对象之间的映射模型。
package model

import (
	"time"

	domainuser "askflow/internal/domain/user"
)

// User 对应 MySQL 中的 users 表。
type User struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement"`
	Username     string    `gorm:"size:50;not null;uniqueIndex"`
	PasswordHash string    `gorm:"size:255;not null"`
	Nickname     string    `gorm:"size:50;not null;default:''"`
	Bio          string    `gorm:"size:200;not null;default:''"`
	AvatarURL    string    `gorm:"size:500;not null;default:''"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (User) TableName() string {
	return "users"
}

// User_Domain2User_Model 将用户领域实体转换为用户数据库模型。
func User_Domain2User_Model(user *domainuser.User) *User {
	return &User{
		ID:           user.ID,
		Username:     user.Username,
		PasswordHash: user.PasswordHash,
		Nickname:     user.Nickname,
		Bio:          user.Bio,
		AvatarURL:    user.AvatarURL,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

// User_Model2User_Domain 将用户数据库模型还原为用户领域实体。
func User_Model2User_Domain(model *User) *domainuser.User {
	return &domainuser.User{
		ID:           model.ID,
		Username:     model.Username,
		PasswordHash: model.PasswordHash,
		Nickname:     model.Nickname,
		Bio:          model.Bio,
		AvatarURL:    model.AvatarURL,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}
