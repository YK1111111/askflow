package answerfavorite

import (
	"context"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domainanswerfavorite "askflow/internal/domain/answerfavorite"
)

type fakeFavoriteRepository struct {
	created   *domainanswerfavorite.AnswerFavorite
	deleted   bool
	favorited bool
}

func (r *fakeFavoriteRepository) Create_AnswerFavorite(_ context.Context, favorite *domainanswerfavorite.AnswerFavorite) error {
	r.created = favorite
	return nil
}
func (r *fakeFavoriteRepository) Delete_AnswerFavorite(context.Context, uint64, uint64) error {
	r.deleted = true
	return nil
}
func (r *fakeFavoriteRepository) Exists_AnswerFavorite(context.Context, uint64, uint64) (bool, error) {
	return r.favorited, nil
}

type fakeAnswerRepository struct{}

func (fakeAnswerRepository) Create_Answer(context.Context, *domainanswer.Answer) error { return nil }
func (fakeAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	return &domainanswer.Answer{ID: 2}, nil
}
func (fakeAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return nil, nil
}

func TestFavoriteUnlikeAndStatus(t *testing.T) {
	repository := &fakeFavoriteRepository{favorited: true}
	service := NewService(repository, fakeAnswerRepository{})
	if err := service.Favorite_Answer(context.Background(), 1, 2); err != nil || repository.created == nil {
		t.Fatalf("收藏回答失败: %v", err)
	}
	if err := service.Unfavorite_Answer(context.Background(), 1, 2); err != nil || !repository.deleted {
		t.Fatalf("取消收藏失败: %v", err)
	}
	status, err := service.Get_AnswerFavoriteStatus(context.Background(), 1, 2)
	if err != nil || !status.Favorited {
		t.Fatalf("收藏状态不符合预期: %+v, %v", status, err)
	}
}
