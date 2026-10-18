package model

import (
	"testing"

	domainfollow "askflow/internal/domain/follow"
)

func TestFollowMapping(t *testing.T) {
	domain, err := domainfollow.New(1, 2)
	if err != nil {
		t.Fatalf("创建关注关系失败: %v", err)
	}
	domain.ID = 3

	restored := Follow_Model2Follow_Domain(Follow_Domain2Follow_Model(domain))
	if restored.ID != 3 || restored.FollowerID != 1 || restored.FolloweeID != 2 {
		t.Fatalf("关注映射结果不符合预期: %+v", restored)
	}
}
