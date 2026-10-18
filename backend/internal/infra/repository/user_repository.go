// Package repository 提供领域仓储接口的 GORM 实现。
package repository

import (
	"context"
	"errors"

	domainuser "askflow/internal/domain/user"
	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
)

// UserRepository 使用 GORM 持久化用户。
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create_User(ctx context.Context, user *domainuser.User) error {
	record := model.User_Domain2User_Model(user)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domainuser.ErrUsernameAlreadyExists
		}
		return err
	}

	user.ID = record.ID
	user.CreatedAt = record.CreatedAt
	user.UpdatedAt = record.UpdatedAt
	return nil
}

func (r *UserRepository) Find_UserByUsername(ctx context.Context, username string) (*domainuser.User, error) {
	var record model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainuser.ErrNotFound
		}
		return nil, err
	}

	return model.User_Model2User_Domain(&record), nil
}

func (r *UserRepository) Find_UserByID(ctx context.Context, userID uint64) (*domainuser.User, error) {
	var record model.User
	if err := r.db.WithContext(ctx).First(&record, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainuser.ErrNotFound
		}
		return nil, err
	}
	return model.User_Model2User_Domain(&record), nil
}

func (r *UserRepository) Update_User_Profile(ctx context.Context, user *domainuser.User) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"nickname":   user.Nickname,
		"bio":        user.Bio,
		"avatar_url": user.AvatarURL,
		"updated_at": user.UpdatedAt,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainuser.ErrNotFound
	}
	return nil
}

var _ domainuser.Repository = (*UserRepository)(nil)
