// Package notification 定义站内通知领域对象。
package notification

import "time"

const (
	TypeAnswerLiked     = "answer_liked"
	TypeAnswerCommented = "answer_commented"
	TypeAnswerFavorited = "answer_favorited"
	TypeUserFollowed    = "user_followed"
)

// Notification 表示一条持久化的用户通知。
type Notification struct {
	ID         uint64
	EventID    string
	UserID     uint64
	ActorID    uint64
	Type       string
	AnswerID   uint64
	QuestionID uint64
	Content    string
	ReadAt     *time.Time
	CreatedAt  time.Time
}
