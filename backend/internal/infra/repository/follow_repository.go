package repository

import (
	"context"
	"time"

	domainfollow "askflow/internal/domain/follow"
	"askflow/internal/infra/database/model"
	"askflow/internal/infra/outbox"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FollowRepository 使用 GORM 持久化用户关注关系。
type FollowRepository struct {
	db *gorm.DB
}

func NewFollowRepository(db *gorm.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Create_Follow(ctx context.Context, follow *domainfollow.Follow) error {
	record := model.Follow_Domain2Follow_Model(follow)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domainfollow.ErrAlreadyFollowing
		}
		return outbox.Append(ctx, tx, outbox.UserFollowed, "follow", record.ID, outbox.FollowPayload{FollowerID: follow.FollowerID, FolloweeID: follow.FolloweeID, OccurredAt: record.CreatedAt})
	})
	if err != nil {
		return err
	}

	follow.ID = record.ID
	follow.CreatedAt = record.CreatedAt
	return nil
}

func (r *FollowRepository) Delete_Follow(ctx context.Context, followerID, followeeID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&model.Follow{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return outbox.Append(ctx, tx, outbox.UserUnfollowed, "follow", followerID, outbox.FollowPayload{FollowerID: followerID, FolloweeID: followeeID, OccurredAt: time.Now()})
	})
}

func (r *FollowRepository) Exists_Follow(ctx context.Context, followerID, followeeID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Follow{}).
		Where("follower_id = ? AND followee_id = ?", followerID, followeeID).
		Count(&count).Error
	return count > 0, err
}

func (r *FollowRepository) Find_FolloweeIDsByFollowerID(ctx context.Context, followerID uint64) ([]uint64, error) {
	ids := make([]uint64, 0)
	err := r.db.WithContext(ctx).
		Model(&model.Follow{}).
		Where("follower_id = ?", followerID).
		Order("created_at DESC").
		Pluck("followee_id", &ids).Error
	return ids, err
}

var _ domainfollow.Repository = (*FollowRepository)(nil)
