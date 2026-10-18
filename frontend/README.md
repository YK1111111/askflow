# AskFlow 前端

轻量 Vue 3 + Vite 单页前端。

```bash
npm install
npm run dev
```

开发服务器默认监听 `http://127.0.0.1:5173`，并将 `/api`、`/health` 代理到
`http://127.0.0.1:8080`。请先启动 AskFlow 后端及其 Docker 依赖。

`npm run build` 会将生产文件输出到 `dist/`。
