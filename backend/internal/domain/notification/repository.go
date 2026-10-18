package notification

import "context"

// Repository 定义通知持久化和未读状态管理能力。
type Repository interface {
	Create_Notification(ctx context.Context, notification *Notification) (bool, error)
	Find_NotificationsAfter(ctx context.Context, userID, afterID uint64, limit int) ([]*Notification, error)
	Count_UnreadNotifications(ctx context.Context, userID uint64) (int64, error)
	Mark_NotificationRead(ctx context.Context, userID, notificationID uint64) error
	Mark_AllNotificationsRead(ctx context.Context, userID uint64) error
}
