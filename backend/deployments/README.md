# 部署配置

当前 Compose 文件只负责启动本地开发依赖：MySQL、Redis 和 RabbitMQ。

```bash
docker compose -f backend/deployments/docker-compose.yml up -d
```

RabbitMQ 管理页面地址为 `http://localhost:15672`，默认账号和密码都是 `guest`。

服务会声明 Topic Exchange `askflow.events.v1` 和持久化队列
`askflow.cache-invalidation.v1`。RabbitMQ 暂时不可用时，业务事件保留在 MySQL
的 `outbox_events` 表中，由后台 Worker 按退避策略重试。
