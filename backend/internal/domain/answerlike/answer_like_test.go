package answerlike

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	created, err := New(1, 2)
	if err != nil {
		t.Fatalf("创建点赞关系失败: %v", err)
	}
	if created.UserID != 1 || created.AnswerID != 2 || created.CreatedAt.IsZero() {
		t.Fatalf("点赞关系不符合预期: %+v", created)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	if _, err := New(0, 2); !errors.Is(err, ErrUserRequired) {
		t.Fatalf("期望点赞用户为空错误，实际为 %v", err)
	}
	if _, err := New(1, 0); !errors.Is(err, ErrAnswerRequired) {
		t.Fatalf("期望回答为空错误，实际为 %v", err)
	}
}
