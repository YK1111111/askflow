package model

import (
	"testing"

	domainanswer "askflow/internal/domain/answer"
)

func TestAnswerMapping(t *testing.T) {
	domain, err := domainanswer.New(1, 2, "回答内容")
	if err != nil {
		t.Fatalf("创建回答失败: %v", err)
	}
	domain.ID = 3
	domain.LikeCount = 4
	domain.CommentCount = 5
	domain.FavoriteCount = 6

	restored := Answer_Model2Answer_Domain(Answer_Domain2Answer_Model(domain))
	if restored.ID != 3 || restored.QuestionID != 1 || restored.AuthorID != 2 || restored.Content != "回答内容" ||
		restored.LikeCount != 4 || restored.CommentCount != 5 || restored.FavoriteCount != 6 {
		t.Fatalf("回答映射结果不符合预期: %+v", restored)
	}
}
