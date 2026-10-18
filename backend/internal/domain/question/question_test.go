package question

import "testing"

func TestNew(t *testing.T) {
	created, err := New(1, " Go 缓存怎么设计？ ", " Cache Aside 如何保证一致性？ ")
	if err != nil {
		t.Fatalf("创建问题失败: %v", err)
	}
	if created.Title != "Go 缓存怎么设计？" || created.AuthorID != 1 {
		t.Fatalf("问题内容不符合预期: %+v", created)
	}
}
