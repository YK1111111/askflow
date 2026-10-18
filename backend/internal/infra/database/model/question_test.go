package model

import (
	"testing"

	domainquestion "askflow/internal/domain/question"
)

func TestQuestionMapping(t *testing.T) {
	domain, err := domainquestion.New(1, "问题标题", "问题内容")
	if err != nil {
		t.Fatalf("创建问题失败: %v", err)
	}
	domain.ID = 2

	restored := Question_Model2Question_Domain(Question_Domain2Question_Model(domain))
	if restored.ID != 2 || restored.AuthorID != 1 || restored.Title != "问题标题" {
		t.Fatalf("问题映射结果不符合预期: %+v", restored)
	}
}
