# LAN IM Vue 前端 + Admin API 接入说明

## 前端

采用 Vue 3 + Vite + Pinia + Vue Router + ECharts。

```bash
cd frontend
npm install
npm run dev
```

开发环境默认把 `/api` 与 WebSocket 代理到 Compose Nginx 统一入口，再由 Nginx 分发到各服务。先启动 Compose 服务；不要指向仅负责接入的 `8080`，该端口已不再提供房间和消息 HTTP 接口。已有 `frontend/.env` 也需要同步修改：

```env
VITE_DEV_BACKEND=http://127.0.0.1
VITE_API_BASE=/api/v1
VITE_WS_AUTH_MODE=query
```

生产环境执行：

```bash
npm run build
```

生成 `frontend/dist/`，由 Nginx 托管。

群聊内点击顶部“聊天记录”，输入关键词搜索当前群的已归档消息，支持加载更多结果及返回聊天。左侧“搜索会话”仅筛选群名称。搜索请求调用 `GET /api/v1/rooms/:id/messages/search?q=关键词&from=0&size=20`，后端负责 Elasticsearch 倒排索引查询；索引不可用时按现有逻辑回退到消息存储查询。

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
