package comment

import (
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	created, err := New(1, 2, " 评论内容 ")
	if err != nil || created.Content != "评论内容" || created.CreatedAt.IsZero() {
		t.Fatalf("创建评论结果不符合预期: %+v, %v", created, err)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		answerID uint64
		authorID uint64
		content  string
		wantErr  error
	}{
		{0, 2, "评论", ErrAnswerRequired},
		{1, 0, "评论", ErrAuthorRequired},
		{1, 2, " ", ErrContentRequired},
		{1, 2, strings.Repeat("评", maxContentLength+1), ErrContentTooLong},
	}
	for _, tt := range tests {
		if _, err := New(tt.answerID, tt.authorID, tt.content); !errors.Is(err, tt.wantErr) {
			t.Fatalf("期望错误为 %v，实际为 %v", tt.wantErr, err)
		}
	}
}
