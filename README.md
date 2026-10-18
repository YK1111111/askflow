# AskFlow

AskFlow 是一个使用 Go、Vue 3、MySQL、Redis 和 RabbitMQ 实现的轻量问答社区，包含
推荐 Feed、关注 Feed、热门问题、回答互动和实时通知。

## 主要功能

- 用户注册、登录和个人资料
- 发布问题与回答
- 点赞、评论、收藏和关注用户
- 推荐、关注、热门三类 Feed
- Redis 与本地缓存
- Transactional Outbox、消息重试和死信队列
- SSE 实时通知、未读状态和断线重连

## 环境要求

- Go 1.22 或更高版本
- Node.js 18 或更高版本
- Docker 和 Docker Compose

## 快速启动

### 1. 启动基础服务

在项目根目录执行：

```bash
cd backend/deployments
docker-compose up -d
```

这会启动：

- MySQL：`127.0.0.1:3306`
- Redis：`127.0.0.1:6379`
- RabbitMQ：`127.0.0.1:5672`
- RabbitMQ 管理页面：`http://127.0.0.1:15672`

RabbitMQ 管理页面默认用户名和密码均为 `guest`。

### 2. 启动后端

打开一个新终端：

```bash
cd backend
go run ./cmd/server
```

后端默认运行在 `http://127.0.0.1:8080`。可以用下面的命令检查：

```bash
curl http://127.0.0.1:8080/health
```

预期响应：

```json
{"status":"ok"}
```

### 3. 启动前端

再打开一个新终端：

```bash
cd frontend
npm install
npm run dev
```

浏览器访问：

```text
http://127.0.0.1:5173
```

开发模式下，Vite 会自动把 `/api` 请求代理到 `127.0.0.1:8080`。

## 停止服务

```bash
cd backend/deployments
docker-compose stop
```

使用 `stop` 会保留 MySQL、Redis 和 RabbitMQ 数据卷。需要再次使用时执行
`docker-compose up -d` 即可。

## 运行测试

普通测试不要求外部服务：

```bash
cd backend
go test ./...
```

前端生产构建：

```bash
cd frontend
npm run build
```
