// Package questionview 定义问题浏览记录能力。
package questionview

import "context"

// Repository 定义问题浏览事件的持久化能力。
type Repository interface {
	Record_QuestionView(ctx context.Context, questionID uint64) error
}
