package repository

import (
	"context"
	"time"

	domainnotification "askflow/internal/domain/notification"
	"askflow/internal/infra/database/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create_Notification(ctx context.Context, value *domainnotification.Notification) (bool, error) {
	record := model.Notification_Domain2Model(value)
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(record)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	value.ID, value.CreatedAt = record.ID, record.CreatedAt
	return true, nil
}

func (r *NotificationRepository) Find_NotificationsAfter(ctx context.Context, userID, afterID uint64, limit int) ([]*domainnotification.Notification, error) {
	var records []model.Notification
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if afterID > 0 {
		query = query.Where("id > ?", afterID).Order("id ASC")
	} else {
		query = query.Order("id DESC")
	}
	if err := query.Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]*domainnotification.Notification, 0, len(records))
	for i := range records {
		result = append(result, model.Notification_Model2Domain(&records[i]))
	}
	return result, nil
}

func (r *NotificationRepository) Count_UnreadNotifications(ctx context.Context, userID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ? AND read_at IS NULL", userID).Count(&count).Error
	return count, err
}

func (r *NotificationRepository) Mark_NotificationRead(ctx context.Context, userID, notificationID uint64) error {
	result := r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).Update("read_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *NotificationRepository) Mark_AllNotificationsRead(ctx context.Context, userID uint64) error {
	return r.db.WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read_at IS NULL", userID).Update("read_at", time.Now()).Error
}

var _ domainnotification.Repository = (*NotificationRepository)(nil)
