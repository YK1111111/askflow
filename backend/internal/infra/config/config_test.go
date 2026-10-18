package config

import "testing"

func TestLoad(t *testing.T) {
	cfg, err := Load("../../../configs/config.yaml")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("期望服务端口为 8080，实际为 %d", cfg.Server.Port)
	}
	if cfg.Database.Name != "askflow" {
		t.Fatalf("期望数据库名为 askflow，实际为 %s", cfg.Database.Name)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Fatalf("期望最大连接数为 50，实际为 %d", cfg.Database.MaxOpenConns)
	}
	if cfg.RabbitMQ.OutboxRetention != "168h" || cfg.RabbitMQ.OutboxCleanupInterval != "1h" || cfg.RabbitMQ.OutboxCleanupBatchSize != 500 {
		t.Fatalf("Outbox 清理配置不符合预期: %+v", cfg.RabbitMQ)
	}
}
