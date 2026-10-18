package answerlike

import (
	"context"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domainanswerlike "askflow/internal/domain/answerlike"
)

type fakeLikeRepository struct {
	created *domainanswerlike.AnswerLike
	deleted bool
	liked   bool
}

func (r *fakeLikeRepository) Create_AnswerLike(_ context.Context, like *domainanswerlike.AnswerLike) error {
	r.created = like
	return nil
}
func (r *fakeLikeRepository) Delete_AnswerLike(context.Context, uint64, uint64) error {
	r.deleted = true
	return nil
}
func (r *fakeLikeRepository) Exists_AnswerLike(context.Context, uint64, uint64) (bool, error) {
	return r.liked, nil
}

type fakeAnswerRepository struct{}

func (fakeAnswerRepository) Create_Answer(context.Context, *domainanswer.Answer) error { return nil }
func (fakeAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	return &domainanswer.Answer{ID: 2}, nil
}
func (fakeAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return nil, nil
}

func TestLikeAnswer(t *testing.T) {
	repository := &fakeLikeRepository{}
	service := NewService(repository, fakeAnswerRepository{})
	if err := service.Like_Answer(context.Background(), 1, 2); err != nil {
		t.Fatalf("点赞回答失败: %v", err)
	}
	if repository.created.UserID != 1 || repository.created.AnswerID != 2 {
		t.Fatalf("点赞关系不符合预期: %+v", repository.created)
	}
}

func TestUnlikeAndStatus(t *testing.T) {
	repository := &fakeLikeRepository{liked: true}
	service := NewService(repository, fakeAnswerRepository{})
	if err := service.Unlike_Answer(context.Background(), 1, 2); err != nil || !repository.deleted {
		t.Fatalf("取消点赞失败: %v", err)
	}
	status, err := service.Get_AnswerLikeStatus(context.Background(), 1, 2)
	if err != nil || !status.Liked {
		t.Fatalf("点赞状态不符合预期: %+v, %v", status, err)
	}
}
