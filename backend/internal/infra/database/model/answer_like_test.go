package model

import (
	"testing"

	domainanswerlike "askflow/internal/domain/answerlike"
)

func TestAnswerLikeMapping(t *testing.T) {
	domain, err := domainanswerlike.New(1, 2)
	if err != nil {
		t.Fatalf("创建点赞关系失败: %v", err)
	}
	domain.ID = 3
	restored := AnswerLike_Model2AnswerLike_Domain(AnswerLike_Domain2AnswerLike_Model(domain))
	if restored.ID != 3 || restored.UserID != 1 || restored.AnswerID != 2 {
		t.Fatalf("点赞映射结果不符合预期: %+v", restored)
	}
}
