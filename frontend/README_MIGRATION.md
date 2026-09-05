# LAN IM Vue 前端 + Admin API 接入说明

## 前端

采用 Vue 3 + Vite + Pinia + Vue Router + ECharts。

```bash
cd frontend
npm install
npm run dev
```

开发环境默认把 `/api` 与 WebSocket 都代理到：

```env
VITE_DEV_BACKEND=http://127.0.0.1:8080
VITE_API_BASE=/api/v1
VITE_WS_AUTH_MODE=query
```

生产环境执行：

```bash
npm run build
```

生成 `frontend/dist/`，继续由现有 Go 服务托管。

## 已接入的普通 IM 接口

```text
POST   /api/v1/login
POST   /api/v1/register
GET    /api/v1/my_rooms
POST   /api/v1/rooms
GET    /api/v1/rooms/:id/messages
GET    /api/v1/rooms/:id/members
POST   /api/v1/rooms/:id/join
DELETE /api/v1/rooms/:id/members/:user_id
DELETE /api/v1/rooms/:id/disband
GET    /api/v1/ws
GET    /api/v1/upload/status
POST   /api/v1/upload/chunk
POST   /api/v1/upload/merge
DELETE /api/v1/upload/cancel
```

## 已接入的超级管理员接口

前端 `src/api/admin.js` 已全部改成真实请求，不再使用 demo fallback：

```text
GET    /api/v1/admin/users
GET    /api/v1/admin/users/:id
POST   /api/v1/admin/users/:id/action
GET    /api/v1/admin/rooms
GET    /api/v1/admin/rooms/:id
DELETE /api/v1/admin/rooms/:id
GET    /api/v1/admin/moderation
POST   /api/v1/admin/moderation/:id/action
GET    /api/v1/admin/agent-config
PUT    /api/v1/admin/agent-config
GET    /api/v1/admin/rag/queries
GET    /api/v1/admin/files
DELETE /api/v1/admin/files/:id
GET    /api/v1/admin/audit-logs
```

Go 接口实现位于交付包根目录的 `backend_patch/`。将它合并进完整后端仓库后即可使用。

## WebSocket 鉴权

浏览器无法自定义 WebSocket `Authorization` Header，因此当前默认：

```env
VITE_WS_AUTH_MODE=query
```

连接形式：

```text
/api/v1/ws?token=<JWT>
```

如果你的 JWT middleware 使用 `Sec-WebSocket-Protocol`，改为：

```env
VITE_WS_AUTH_MODE=protocol
```

## Admin 与 Grafana 的职责边界

- Admin：用户封禁与解封、权限、群聊、文件、内容治理、Agent/RAG 配置和审计日志。
- Grafana：WebSocket、Gateway、TaskPool、Kafka、Redis、数据库、Python Agent、Qdrant 和系统运行指标。

## Agent 配置热更新

当前公开后端中 RoomAgent 明确运行时读取 `SystemPrompt / Temperature / TopK`。管理员保存配置时，这三个字段会：

1. 写入控制台配置；
2. 同步更新现有 `agent_configs`；
3. 更新当前运行中的 RoomAgent 配置对象。

其他模型/Chunk 参数会正常保存为控制台策略，但只有后端对应组件真正支持这些字段后才应热更新，避免将不等价字段错误映射。
