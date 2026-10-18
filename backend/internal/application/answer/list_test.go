package answer

import (
	"context"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domainquestion "askflow/internal/domain/question"
	domainuser "askflow/internal/domain/user"
)

type fakeUserRepository struct {
	user *domainuser.User
}

func (r fakeUserRepository) Create_User(context.Context, *domainuser.User) error { return nil }
func (r fakeUserRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return r.user, nil
}
func (r fakeUserRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return r.user, nil
}
func (r fakeUserRepository) Update_User_Profile(context.Context, *domainuser.User) error { return nil }

func TestListAnswersByQuestionID(t *testing.T) {
	service := NewListService(
		&fakeAnswerRepository{answers: []*domainanswer.Answer{{
			ID: 3, QuestionID: 1, AuthorID: 2, Content: "回答内容",
			LikeCount: 4, CommentCount: 5, FavoriteCount: 6,
		}}},
		fakeQuestionRepository{question: &domainquestion.Question{ID: 1}},
		fakeUserRepository{user: &domainuser.User{ID: 2, Username: "alice", Nickname: "Alice"}},
	)

	output, err := service.List_AnswersByQuestionID(context.Background(), 1)
	if err != nil {
		t.Fatalf("查询回答列表失败: %v", err)
	}
	if len(output) != 1 || output[0].ID != 3 || output[0].Author.Nickname != "Alice" ||
		output[0].LikeCount != 4 || output[0].CommentCount != 5 || output[0].FavoriteCount != 6 {
		t.Fatalf("回答列表不符合预期: %+v", output)
	}
}
