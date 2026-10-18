package question

import (
	"context"
	"testing"

	domainquestion "askflow/internal/domain/question"
)

type fakeQuestionRepository struct {
	created *domainquestion.Question
}

func (r *fakeQuestionRepository) Create_Question(_ context.Context, question *domainquestion.Question) error {
	question.ID = 1
	r.created = question
	return nil
}

func (r *fakeQuestionRepository) Find_QuestionByID(context.Context, uint64) (*domainquestion.Question, error) {
	return nil, domainquestion.ErrNotFound
}

func TestPublishQuestion(t *testing.T) {
	repository := &fakeQuestionRepository{}
	service := NewPublishService(repository)

	output, err := service.Publish_Question(context.Background(), PublishInput{
		AuthorID: 1,
		Title:    "Go 缓存怎么设计？",
		Content:  "Cache Aside 如何保证一致性？",
	})
	if err != nil {
		t.Fatalf("发布问题失败: %v", err)
	}
	if output.ID != 1 || repository.created.AuthorID != 1 {
		t.Fatalf("发布结果不符合预期: %+v", output)
	}
}
