package question

import (
	"context"
	"testing"

	domainquestion "askflow/internal/domain/question"
	domainuser "askflow/internal/domain/user"
)

type detailQuestionRepository struct {
	question *domainquestion.Question
}

func (r detailQuestionRepository) Create_Question(context.Context, *domainquestion.Question) error {
	return nil
}

func (r detailQuestionRepository) Find_QuestionByID(context.Context, uint64) (*domainquestion.Question, error) {
	return r.question, nil
}

type detailUserRepository struct {
	user *domainuser.User
}

type detailViewRepository struct{ recorded uint64 }

func (r *detailViewRepository) Record_QuestionView(_ context.Context, questionID uint64) error {
	r.recorded = questionID
	return nil
}

func (r detailUserRepository) Create_User(context.Context, *domainuser.User) error {
	return nil
}

func (r detailUserRepository) Find_UserByID(context.Context, uint64) (*domainuser.User, error) {
	return r.user, nil
}

func (r detailUserRepository) Find_UserByUsername(context.Context, string) (*domainuser.User, error) {
	return r.user, nil
}

func (r detailUserRepository) Update_User_Profile(context.Context, *domainuser.User) error {
	return nil
}

func TestGetQuestionDetail(t *testing.T) {
	views := &detailViewRepository{}
	service := NewDetailService(
		detailQuestionRepository{question: &domainquestion.Question{ID: 2, AuthorID: 1, Title: "问题标题", Content: "问题内容"}},
		detailUserRepository{user: &domainuser.User{ID: 1, Username: "alice", Nickname: "Alice"}},
		views,
	)

	output, err := service.Get_QuestionDetail(context.Background(), 2)
	if err != nil {
		t.Fatalf("查询问题详情失败: %v", err)
	}
	if output.ID != 2 || output.Author.Nickname != "Alice" || views.recorded != 2 {
		t.Fatalf("问题详情不符合预期: %+v", output)
	}
}
