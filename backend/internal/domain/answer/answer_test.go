package answer

import (
	"errors"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	created, err := New(1, 2, " 这是一个回答。 ")
	if err != nil {
		t.Fatalf("创建回答失败: %v", err)
	}
	if created.QuestionID != 1 || created.AuthorID != 2 {
		t.Fatalf("回答关联信息不符合预期: %+v", created)
	}
	if created.Content != "这是一个回答。" {
		t.Fatalf("期望回答内容去除首尾空格，实际为 %q", created.Content)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("创建时间和更新时间不应为空")
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		questionID uint64
		authorID   uint64
		content    string
		wantErr    error
	}{
		{name: "问题为空", questionID: 0, authorID: 2, content: "回答", wantErr: ErrQuestionRequired},
		{name: "作者为空", questionID: 1, authorID: 0, content: "回答", wantErr: ErrAuthorRequired},
		{name: "内容为空", questionID: 1, authorID: 2, content: "  ", wantErr: ErrContentRequired},
		{name: "内容过长", questionID: 1, authorID: 2, content: strings.Repeat("答", maxContentLength+1), wantErr: ErrContentTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.questionID, tt.authorID, tt.content)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("期望错误为 %v，实际为 %v", tt.wantErr, err)
			}
		})
	}
}
