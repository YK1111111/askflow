package answer

import (
	"context"
	"testing"

	domainanswer "askflow/internal/domain/answer"
	domainquestion "askflow/internal/domain/question"
)

type fakeAnswerRepository struct {
	created *domainanswer.Answer
	answers []*domainanswer.Answer
}

func (r *fakeAnswerRepository) Create_Answer(_ context.Context, answer *domainanswer.Answer) error {
	answer.ID = 3
	r.created = answer
	return nil
}

func (r *fakeAnswerRepository) Find_AnswersByQuestionID(context.Context, uint64) ([]*domainanswer.Answer, error) {
	return r.answers, nil
}

func (r *fakeAnswerRepository) Find_AnswerByID(context.Context, uint64) (*domainanswer.Answer, error) {
	return nil, domainanswer.ErrNotFound
}

type fakeQuestionRepository struct {
	question *domainquestion.Question
}

func (r fakeQuestionRepository) Create_Question(context.Context, *domainquestion.Question) error {
	return nil
}

func (r fakeQuestionRepository) Find_QuestionByID(context.Context, uint64) (*domainquestion.Question, error) {
	return r.question, nil
}

func TestPublishAnswer(t *testing.T) {
	answerRepository := &fakeAnswerRepository{}
	service := NewPublishService(
		answerRepository,
		fakeQuestionRepository{question: &domainquestion.Question{ID: 1}},
	)

	output, err := service.Publish_Answer(context.Background(), PublishInput{
		QuestionID: 1,
		AuthorID:   2,
		Content:    "回答内容",
	})
	if err != nil {
		t.Fatalf("发布回答失败: %v", err)
	}
	if output.ID != 3 || output.LikeCount != 0 || output.CommentCount != 0 || output.FavoriteCount != 0 ||
		answerRepository.created.QuestionID != 1 || answerRepository.created.AuthorID != 2 {
		t.Fatalf("发布结果不符合预期: %+v", output)
	}
}
