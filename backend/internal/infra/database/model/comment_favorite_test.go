package model

import (
	"testing"

	domainanswerfavorite "askflow/internal/domain/answerfavorite"
	domaincomment "askflow/internal/domain/comment"
)

func TestCommentMapping(t *testing.T) {
	domain, _ := domaincomment.New(1, 2, "评论")
	domain.ID = 3
	restored := Comment_Model2Comment_Domain(Comment_Domain2Comment_Model(domain))
	if restored.ID != 3 || restored.AnswerID != 1 || restored.AuthorID != 2 {
		t.Fatalf("评论映射结果不符合预期: %+v", restored)
	}
}

func TestAnswerFavoriteMapping(t *testing.T) {
	domain, _ := domainanswerfavorite.New(1, 2)
	domain.ID = 3
	restored := AnswerFavorite_Model2AnswerFavorite_Domain(AnswerFavorite_Domain2AnswerFavorite_Model(domain))
	if restored.ID != 3 || restored.UserID != 1 || restored.AnswerID != 2 {
		t.Fatalf("收藏映射结果不符合预期: %+v", restored)
	}
}
