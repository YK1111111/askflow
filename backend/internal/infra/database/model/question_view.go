package model

import "time"

// QuestionView 记录一次问题详情访问，用于计算时间窗口内的热度。
type QuestionView struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement"`
	QuestionID uint64    `gorm:"not null;index:idx_question_views_question_created,priority:1"`
	CreatedAt  time.Time `gorm:"not null;index:idx_question_views_question_created,priority:2;index"`
}

func (QuestionView) TableName() string { return "question_views" }
