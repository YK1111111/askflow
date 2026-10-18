// Package feed 定义 Feed 读取模型及其持久化抽象。
package feed

import (
	"context"
	"time"
)

// Cursor 表示关注 Feed 的稳定分页位置。
type Cursor struct {
	CreatedAt time.Time
	AnswerID  uint64
}

// HotCursor 表示热门 Feed 的稳定分页位置。
type HotCursor struct {
	Score      float64
	QuestionID uint64
	AsOf       time.Time
}

// RecommendedCursor 表示推荐 Feed 的稳定分页位置。
type RecommendedCursor struct {
	Score    float64
	AnswerID uint64
	AsOf     time.Time
}

// FollowingItem 是关注 Feed 所需的数据库读取投影。
type FollowingItem struct {
	QuestionID           uint64
	QuestionTitle        string
	QuestionAuthorID     uint64
	QuestionAuthorName   string
	QuestionAuthorNick   string
	QuestionAuthorAvatar string
	AnswerID             uint64
	AnswerContent        string
	AnswerAuthorID       uint64
	AnswerAuthorName     string
	AnswerAuthorNick     string
	AnswerAuthorAvatar   string
	LikeCount            uint64
	CommentCount         uint64
	FavoriteCount        uint64
	AnswerCreatedAt      time.Time
}

// HotItem 是问题级热门 Feed 的聚合读取投影。
type HotItem struct {
	QuestionID           uint64
	QuestionTitle        string
	QuestionContent      string
	QuestionAuthorID     uint64
	QuestionAuthorName   string
	QuestionAuthorNick   string
	QuestionAuthorAvatar string
	AnswerCount24H       uint64 `gorm:"column:answer_count_24h"`
	LikeCount24H         uint64 `gorm:"column:like_count_24h"`
	CommentCount24H      uint64 `gorm:"column:comment_count_24h"`
	ViewCount24H         uint64 `gorm:"column:view_count_24h"`
	HotScore             float64
	QuestionCreatedAt    time.Time
	RankedAt             time.Time
}

// RecommendedItem 是推荐 Feed 的回答级读取投影。
type RecommendedItem struct {
	QuestionID           uint64
	QuestionTitle        string
	QuestionAuthorID     uint64
	QuestionAuthorName   string
	QuestionAuthorNick   string
	QuestionAuthorAvatar string
	AnswerID             uint64
	AnswerContent        string
	AnswerAuthorID       uint64
	AnswerAuthorName     string
	AnswerAuthorNick     string
	AnswerAuthorAvatar   string
	LikeCount            uint64
	CommentCount         uint64
	FavoriteCount        uint64
	AnswerCreatedAt      time.Time
	RecommendScore       float64
	FollowingAuthor      bool
	RankedAt             time.Time
}

// Repository 定义关注 Feed 的读取能力。
type Repository interface {
	Find_FollowingFeed(ctx context.Context, userID uint64, cursor *Cursor, limit int) ([]FollowingItem, error)
	Find_HotFeed(ctx context.Context, since, asOf time.Time, cursor *HotCursor, limit int) ([]HotItem, error)
	Find_RecommendedFeed(ctx context.Context, userID uint64, asOf time.Time, cursor *RecommendedCursor, limit int) ([]RecommendedItem, error)
}
