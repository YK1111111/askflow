package comment

import (
	"context"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domaincomment "askflow/internal/domain/comment"
	domainuser "askflow/internal/domain/user"
)

type fakeCommentRepository struct{ comments []*domaincomment.Comment }

func (r *fakeCommentRepository) Create_Comment(_ context.Context, comment *domaincomment.Comment) error {
	comment.ID = 3
	r.comments = append(r.comments, comment)
	return nil
}
func (r *fakeCommentRepository) Find_CommentsByAnswerID(context.Context, uint64) ([]*domaincomment.Comment, error) {
	return r.comments, nil
}

type fakeAnswerRepository struct{}

func (fakeAnswerRepository) Create_Answer(context.Context, *domainanswer.Answer) error { return nil }
func (fakeAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	return &domainanswer.Answer{ID: 1}, nil
}
func (fakeAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return nil, nil
}

type fakeUserRepository struct{}

func (fakeUserRepository) Create_User(context.Context, *domainuser.User) error { return nil }
func (fakeUserRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return &domainuser.User{ID: 2, Username: "alice", Nickname: "Alice"}, nil
}
func (fakeUserRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return nil, nil
}
func (fakeUserRepository) Update_User_Profile(context.Context, *domainuser.User) error { return nil }

func TestPublishAndListComments(t *testing.T) {
	repository := &fakeCommentRepository{}
	service := NewService(repository, fakeAnswerRepository{}, fakeUserRepository{})
	published, err := service.Publish_Comment(context.Background(), PublishInput{AnswerID: 1, AuthorID: 2, Content: "评论"})
	if err != nil || published.ID != 3 {
		t.Fatalf("发布评论失败: %+v, %v", published, err)
	}
	comments, err := service.List_CommentsByAnswerID(context.Background(), 1)
	if err != nil || len(comments) != 1 || comments[0].Author.Nickname != "Alice" {
		t.Fatalf("评论列表不符合预期: %+v, %v", comments, err)
	}
}
