# Admin 服务接口文档

本文档描述 `admin-service` 提供的超级管理员后台接口。

## 管理端登录与端口

- 管理 API 默认端口：`8081`，通过 `ADMIN_SERVER_PORT` 配置。
- 独立管理前端默认端口：`5174`，通过 `ADMIN_FRONTEND_PORT` 配置。
- 普通用户 Backend 默认端口：`8080`。

### POST /api/v1/admin/login

该接口不需要 JWT，但只允许超级管理员、审核员或运营人员登录。

```json
{
  "username": "admin",
  "password": "example-password"
}
```

成功响应：

```json
{
  "msg": "管理员登录成功",
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 1,
    "username": "admin",
    "role": 1,
    "avatar": ""
  }
}
```

普通用户账号返回 `403`，密码错误返回 `401`，频率超限返回 `429`。

## 服务信息

- 服务地址：`http://<host>:8081`
- 接口前缀：`/api/v1/admin`
- 后台页面：`/admin/dashboard`
- 认证方式：`Authorization: Bearer <JWT>`
- 请求类型：`application/json`
- 控制通道：`admin-service` 通过 `AdminControlService` gRPC 调用 backend（默认 `backend:50053`，使用 `ADMIN_CONTROL_TOKEN` 做服务间认证）

所有管理接口都经过以下中间件：

1. `JWTAuth`：校验 JWT。
2. `RequireAdmin`：要求账号角色为 `super_admin`、`moderator` 或 `operator`。
3. `AdminRateLimit`：管理接口限流，默认限制约为 10 QPS、突发 30。
4. 部分接口额外要求对应权限点。

## 通用约定

### 分页

分页接口统一使用以下查询参数：

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 1 | 页码，从 1 开始 |
| `page_size` | int | 20 | 每页条数，范围为 1～100 |

分页接口统一返回：

```json
{
  "items": [],
  "total": 0,
  "page": 1,
  "page_size": 20
}
```

### 时间参数

时间参数使用 RFC3339 格式，例如：

```text
2026-08-01T00:00:00+08:00
```

### 布尔参数

布尔查询参数使用以下值表示 `true`：

```text
true
1
```

其他值视为 `false`。

### 错误响应

业务失败时统一返回：

```json
{
  "error": "错误描述"
}
```

常见状态码：

| 状态码 | 含义 |
| --- | --- |
| 400 | 参数非法 |
| 401 | JWT 无效或缺失 |
| 403 | 非管理员，或权限不足 |
| 404 | 目标资源不存在 |
| 429 | 触发管理端限流 |
| 500 | 服务内部错误 |
| 503 | 服务未初始化或下游不可用 |

---

## 接口总览

| 模块 | 方法 | 路径 | 权限点 |
| --- | --- | --- | --- |
| Dashboard | GET | `/api/v1/admin/dashboard/overview` | `dashboard.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/runtime` | `dashboard.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/message-traffic` | `dashboard.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/timeseries` | `dashboard.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/agent` | `agent.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/rag` | `dashboard.read` 或 `agent.read` |
| Dashboard | GET | `/api/v1/admin/dashboard/moderation` | `moderation.read` |
| RAG | GET | `/api/v1/admin/rag/queries` | `agent.read` |
| 审核 | GET | `/api/v1/admin/moderation` | `moderation.read` |
| 审核 | GET | `/api/v1/admin/moderation/:id` | `moderation.read` |
| 审核 | POST | `/api/v1/admin/moderation/:id/action` | `moderation.review` |
| 用户 | GET | `/api/v1/admin/users` | `user.read` |
| 用户 | GET | `/api/v1/admin/users/:id` | `user.read` |
| 用户 | POST | `/api/v1/admin/users/:id/action` | 按动作要求 |
| 用户 | DELETE | `/api/v1/admin/users/:id` | `user.delete` |
| 群聊 | GET | `/api/v1/admin/rooms` | `room.read` |
| 群聊 | GET | `/api/v1/admin/rooms/:id` | `room.read` |
| 群聊 | POST | `/api/v1/admin/rooms/:id/action` | 按动作要求 |
| 群聊 | DELETE | `/api/v1/admin/rooms/:id` | `room.delete` |
| 连接 | GET | `/api/v1/admin/connections` | `connection.read` |
| 连接 | POST | `/api/v1/admin/connections/close` | `connection.close` |
| 连接 | POST | `/api/v1/admin/connections/force-offline` | `connection.close` |
| 文件 | GET | `/api/v1/admin/files` | `file.read` |
| 文件 | GET | `/api/v1/admin/files/scan` | `file.read` |
| 文件 | POST | `/api/v1/admin/files/cleanup` | `file.delete` |
| 文件 | GET | `/api/v1/admin/files/:id` | `file.read` |
| 文件 | GET | `/api/v1/admin/files/:id/download` | `file.read` |
| 文件 | DELETE | `/api/v1/admin/files/:id` | `file.delete` |
| Agent | GET | `/api/v1/admin/agent-config` | `agent.read` |
| Agent | PUT | `/api/v1/admin/agent-config` | `agent.config` |
| Agent | POST | `/api/v1/admin/agent-config/rollback` | `agent.config` |
| Agent | GET | `/api/v1/admin/agent-config/history` | `agent.read` |
| Tool Call | GET | `/api/v1/admin/tool-calls` | `agent.read` |
| 系统 | GET | `/api/v1/admin/errors` | `system.read` |
| 系统 | POST | `/api/v1/admin/errors/:id/resolve` | `system.read` |
| 审计 | GET | `/api/v1/admin/audit-logs` | `audit.read` |
| 健康 | GET | `/api/v1/admin/health` | `system.read` |
| 告警 | GET | `/api/v1/admin/alerts` | `system.read` |
| 告警 | GET | `/api/v1/admin/alerts/unresolved-count` | `dashboard.read` 或 `system.read` |
| 告警 | POST | `/api/v1/admin/alerts/evaluate` | `system.read` |
| 告警 | POST | `/api/v1/admin/alerts/:id/resolve` | `system.read` |

---

## Dashboard 接口

### 1. 获取首页概览

```http
GET /api/v1/admin/dashboard/overview
```

返回用户、群聊、消息、WebSocket、Agent、RAG、审核和系统健康状态等聚合数据。

```json
{
  "generated_at": "2026-08-15T12:00:00+08:00",
  "sections": {
    "users": {
      "total": 100,
      "new_today": 3,
      "online": 12
    },
    "rooms": {
      "total": 20,
      "new_today": 1
    },
    "messages": {
      "today": 500
    },
    "realtime": {
      "websocket_connections": 12
    }
  },
  "websocket": {},
  "moderation": {},
  "agent": {},
  "rag": {},
  "system": []
}
```

### 2. 获取运行指标

```http
GET /api/v1/admin/dashboard/runtime
```

返回 Go 运行时、WebSocket、API 指标。

主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `websocket.current_connections` | int64 | 当前连接数 |
| `websocket.abnormal_closed_total` | int64 | 异常断开总数 |
| `golang.goroutines` | int | Goroutine 数量 |
| `golang.heap_alloc` | uint64 | 堆内存分配量 |
| `api.qps_1m` | float64 | 最近 1 分钟 QPS |
| `api.error_rate` | float64 | API 错误率 |

### 3. 获取消息流量

```http
GET /api/v1/admin/dashboard/message-traffic
```

返回最近 24 小时与最近 7 天的消息流量统计。

```json
{
  "hourly": [],
  "daily": [],
  "private_group": {
    "private": 0,
    "group": 0
  },
  "type_distribution": {},
  "peak_hour": "",
  "top_rooms": [],
  "top_users": [],
  "generated_at": "2026-08-15T12:00:00+08:00"
}
```

### 4. 获取时间序列

```http
GET /api/v1/admin/dashboard/timeseries?metric=messages&period=24h
```

查询参数：

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `metric` | string | `messages` | 当前仅支持 `messages` |
| `period` | string | `24h` | 支持 `1h`、`24h`、`7d`、`30d` |

响应：

```json
{
  "metric": "messages",
  "period": "24h",
  "points": [
    {
      "time": "2026-08-15 12",
      "count": 42
    }
  ],
  "generated_at": "2026-08-15T12:00:00+08:00"
}
```

### 5. 获取 Agent 运行概览

```http
GET /api/v1/admin/dashboard/agent
```

返回 Agent 调用量、成功率、Token 用量、Tool Call 和 Embedding 等指标。

### 6. 获取 RAG 看板

```http
GET /api/v1/admin/dashboard/rag
```

主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `qdrant_online` | bool | Qdrant 是否在线 |
| `collection_count` | int | 集合数量 |
| `vector_count` | uint64 | 向量数量 |
| `today_new_vectors` | int64 | 今日新增向量数 |
| `embedding_queue` | int64 | Embedding 队列长度 |
| `embedding_avg_ms` | float64 | Embedding 平均耗时 |
| `qdrant_query_avg_ms` | float64 | Qdrant 查询平均耗时 |
| `rag_avg_recall` | float64 | RAG 平均召回率 |
| `top_k` | int | 当前 TopK |
| `vector_dimension` | int | 向量维度 |

### 7. 获取审核看板

```http
GET /api/v1/admin/dashboard/moderation
```

主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `today_reviewed` | int64 | 今日审核量 |
| `today_violations` | int64 | 今日违规量 |
| `violation_rate` | float64 | 违规率 |
| `auto_kick_count` | int64 | 自动踢出次数 |
| `auto_ban_count` | int64 | 自动封禁次数 |
| `manual_review_count` | int64 | 人工复核数 |
| `revoked_count` | int64 | 已撤销数 |
| `category_stats` | array | 分类统计 |
| `recent_violations` | array | 最近违规事件 |

---

## RAG 接口

### 查询 RAG 记录

```http
GET /api/v1/admin/rag/queries?page=1&page_size=20&room_id=10
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `room_id` | int64 | 可选，按群聊过滤 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 记录 ID |
| `room_id` | int64 | 群聊 ID |
| `user_id` | int64 | 用户 ID |
| `question` | string | 查询问题 |
| `query_time` | time | 查询时间 |
| `retrieved_count` | int | 召回数量 |
| `query_latency_ms` | float64 | 查询耗时 |
| `used_time_tool` | bool | 是否使用时间工具 |
| `context_summary` | string | 上下文摘要 |

---

## 审核接口

### 查询审核事件

```http
GET /api/v1/admin/moderation
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `username` | string | 用户名 |
| `user_id` | int64 | 用户 ID |
| `room_id` | int64 | 群聊 ID |
| `category` | string | 分类 |
| `risk_level` | string | 风险等级 |
| `penalty_status` | string | 处罚状态 |
| `start` | string | 开始时间，RFC3339 |
| `end` | string | 结束时间，RFC3339 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 审核事件 ID |
| `message_id` | int64 | 消息 ID |
| `room_id` | int64 | 群聊 ID |
| `user_id` | int64 | 用户 ID |
| `username` | string | 用户名 |
| `original_msg` | string | 原始消息 |
| `category` | string | 违规分类 |
| `risk_level` | string | 风险等级 |
| `risk_score` | float64 | 风险分数 |
| `model_result` | string | 模型结果 |
| `penalty_status` | string | 处罚状态 |
| `review_status` | string | 复核状态 |
| `created_at` | time | 创建时间 |

### 查询单条审核事件

```http
GET /api/v1/admin/moderation/:id
```

### 处理审核事件

```http
POST /api/v1/admin/moderation/:id/action
```

请求体：

```json
{
  "action": "kick"
}
```

支持的 `action`：

| 值 | 说明 |
| --- | --- |
| `warn` | 警告 |
| `mute` | 禁言 |
| `kick` | 踢出 |
| `ban` | 封禁 |
| `revoke` | 撤销处罚 |
| `false_positive` | 标记为误报 |
| `confirmed` | 确认违规 |

---

## 用户接口

### 查询用户列表

```http
GET /api/v1/admin/users
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `keyword` | string | 用户名或用户 ID 关键字 |
| `role` | int | 角色值 |
| `status` | int | 状态值 |
| `start` | string | 创建时间起始值，RFC3339 |
| `end` | string | 创建时间结束值，RFC3339 |

列表项字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 用户 ID |
| `username` | string | 用户名 |
| `role` | int8 | 角色值 |
| `role_name` | string | 角色名 |
| `online` | bool | 是否在线 |
| `status` | int8 | 账号状态 |
| `room_count` | int64 | 房间数 |
| `message_count` | int64 | 消息数 |
| `violation_count` | int64 | 违规数 |
| `created_at` | time | 注册时间 |
| `last_active_at` | time | 最后活跃时间 |

### 查询用户详情

```http
GET /api/v1/admin/users/:id
```

返回用户详情、房间、最近消息、违规记录、文件上传和 Agent 调用记录。

### 执行用户操作

```http
POST /api/v1/admin/users/:id/action
```

请求体：

```json
{
  "action": "ban"
}
```

支持的 `action`：

| 值 | 说明 | 权限点 |
| --- | --- | --- |
| `ban` | 封禁 | `user.ban` |
| `unban` | 解封 | `user.ban` |
| `force_offline` | 强制下线 | `user.kick` |
| `role_super_admin` | 设为超级管理员 | `user.role.update` |
| `role_moderator` | 设为版主 | `user.role.update` |
| `role_operator` | 设为运营 | `user.role.update` |
| `role_user` | 设为普通用户 | `user.role.update` |

### 删除用户

```http
DELETE /api/v1/admin/users/:id
```

执行软删除，并断开该用户在线连接。

---

## 群聊接口

### 查询群聊列表

```http
GET /api/v1/admin/rooms
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `keyword` | string | 群名或群 ID 关键字 |
| `type` | int | 房间类型 |
| `status` | int | 房间状态 |
| `agent_enabled` | bool | 是否启用 Agent |
| `start` | string | 创建时间起始值，RFC3339 |
| `end` | string | 创建时间结束值，RFC3339 |

列表项字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 群聊 ID |
| `name` | string | 群名 |
| `type` | int8 | 房间类型 |
| `owner_id` | int64 | 群主 ID |
| `member_count` | int64 | 成员数 |
| `online_member_count` | int64 | 在线成员数 |
| `today_message_count` | int64 | 今日消息数 |
| `total_message_count` | int64 | 总消息数 |
| `agent_enabled` | bool | 是否启用 Agent |
| `moderation_enabled` | bool | 是否启用审核 |
| `status` | int8 | 状态 |
| `created_at` | time | 创建时间 |

### 查询群聊详情

```http
GET /api/v1/admin/rooms/:id
```

返回群聊详情、成员、最近消息、Agent 配置和违规记录。

### 执行群聊操作

```http
POST /api/v1/admin/rooms/:id/action
```

请求体：

```json
{
  "action": "remove_member",
  "target_user_id": 1001
}
```

支持的 `action`：

| 值 | 说明 | 是否需要 `target_user_id` |
| --- | --- | --- |
| `freeze` | 冻结群聊 | 否 |
| `unfreeze` | 解冻群聊 | 否 |
| `disband` | 解散群聊 | 否 |
| `agent_enable` | 启用 Agent | 否 |
| `agent_disable` | 停用 Agent | 否 |
| `moderation_enable` | 启用内容审核 | 否 |
| `moderation_disable` | 停用内容审核 | 否 |
| `remove_member` | 移除成员 | 是 |
| `set_admin` | 设为管理员 | 是 |
| `transfer_owner` | 转让群主 | 是 |

### 解散群聊

```http
DELETE /api/v1/admin/rooms/:id
```

---

## 连接接口

### 查询 WebSocket 连接

```http
GET /api/v1/admin/connections?keyword=alice
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `keyword` | string | 按用户 ID 或用户名过滤 |

响应项字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `user_id` | int64 | 用户 ID |
| `username` | string | 用户名 |
| `connection_id` | string | 连接 ID |
| `remote_ip` | string | 客户端 IP |
| `connected_at` | time | 连接时间 |
| `last_read_at` | time | 最近读取时间 |
| `last_write_at` | time | 最近写入时间 |
| `send_queue_len` | int | 发送队列长度 |
| `room_ids` | int64[] | 所在房间 ID |

### 关闭连接

```http
POST /api/v1/admin/connections/close
```

请求体：

```json
{
  "connection_id": "conn-uuid"
}
```

### 强制用户下线

```http
POST /api/v1/admin/connections/force-offline
```

请求体：

```json
{
  "user_id": 1001
}
```

---

## 文件接口

### 查询文件列表

```http
GET /api/v1/admin/files
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `keyword` | string | 文件名或对象键关键字 |
| `uploader_id` | int64 | 上传者 ID |
| `room_id` | int64 | 群聊 ID |
| `file_type` | string | 文件类型 |
| `status` | string | 文件状态 |
| `start` | string | 创建时间起始值，RFC3339 |
| `end` | string | 创建时间结束值，RFC3339 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 文件记录 ID |
| `object_key` | string | 对象存储键 |
| `original_name` | string | 原始文件名 |
| `sha256` | string | SHA-256 |
| `size` | int64 | 文件大小 |
| `uploader_id` | int64 | 上传者 ID |
| `room_id` | int64 | 群聊 ID |
| `backend` | string | 对象存储后端 |
| `status` | string | 状态 |
| `username` | string | 上传者用户名 |
| `room_name` | string | 群聊名 |
| `exists` | bool | 对象是否存在于存储 |
| `has_message` | bool | 是否关联消息 |

### 文件异常扫描

```http
GET /api/v1/admin/files/scan
```

检查数据库记录与对象存储是否一致。

响应：

```json
{
  "total_records": 100,
  "total_objects": 98,
  "missing_records": [],
  "orphan_objects": [],
  "stale_records": []
}
```

### 清理孤儿对象

```http
POST /api/v1/admin/files/cleanup
```

响应：

```json
{
  "cleaned": 2
}
```

### 查看文件详情

```http
GET /api/v1/admin/files/:id
```

### 获取下载链接

```http
GET /api/v1/admin/files/:id/download
```

响应：

```json
{
  "download_url": "https://minio.example.com/bucket/object?..."
}
```

### 删除文件

```http
DELETE /api/v1/admin/files/:id
```

删除对象存储文件与数据库记录。

---

## Agent 配置接口

### 获取全局配置

```http
GET /api/v1/admin/agent-config
```

### 更新全局配置

```http
PUT /api/v1/admin/agent-config
```

请求体示例：

```json
{
  "global_enabled": true,
  "default_model": "deepseek-chat",
  "embedding_model": "text-embedding-v3",
  "temperature": 0.7,
  "max_tokens": 4096,
  "rag_top_k": 5,
  "rag_similarity_threshold": 0.7,
  "chunk_size": 800,
  "chunk_overlap": 120,
  "moderation_enabled": true,
  "moderation_model": "deepseek-chat",
  "moderation_threshold": 0.75,
  "tool_calling_enabled": true,
  "auto_kick_enabled": false,
  "auto_ban_enabled": false,
  "system_prompt": "",
  "moderation_prompt": "",
  "rag_prompt": "",
  "tool_calling_prompt": ""
}
```

字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `global_enabled` | bool | 是否全局启用 Agent |
| `default_model` | string | 默认模型 |
| `embedding_model` | string | Embedding 模型 |
| `temperature` | float64 | 采样温度 |
| `max_tokens` | int | 最大 Token 数 |
| `rag_top_k` | int | RAG 召回数量 |
| `rag_similarity_threshold` | float64 | RAG 相似度阈值 |
| `chunk_size` | int | 分块大小 |
| `chunk_overlap` | int | 分块重叠长度 |
| `moderation_enabled` | bool | 是否启用审核 |
| `moderation_model` | string | 审核模型 |
| `moderation_threshold` | float64 | 审核阈值 |
| `tool_calling_enabled` | bool | 是否启用工具调用 |
| `auto_kick_enabled` | bool | 是否自动踢出 |
| `auto_ban_enabled` | bool | 是否自动封禁 |
| `system_prompt` | string | 系统提示词 |
| `moderation_prompt` | string | 审核提示词 |
| `rag_prompt` | string | RAG 提示词 |
| `tool_calling_prompt` | string | 工具调用提示词 |

### 回滚到上一版本

```http
POST /api/v1/admin/agent-config/rollback
```

无请求体。

### 查询配置历史

```http
GET /api/v1/admin/agent-config/history?page=1&page_size=20
```

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 历史记录 ID |
| `config_id` | int64 | 配置 ID |
| `version` | int64 | 版本号 |
| `before_data` | string | 修改前 JSON |
| `after_data` | string | 修改后 JSON |
| `admin_user_id` | int64 | 操作管理员 ID |
| `admin_username` | string | 操作管理员用户名 |
| `created_at` | time | 操作时间 |

---

## Tool Call 接口

```http
GET /api/v1/admin/tool-calls
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `tool_name` | string | 工具名 |
| `user_id` | int64 | 用户 ID |
| `room_id` | int64 | 群聊 ID |
| `success` | bool | 是否成功 |
| `start` | string | 开始时间，RFC3339 |
| `end` | string | 结束时间，RFC3339 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 记录 ID |
| `tool_call_id` | string | Tool Call ID |
| `agent_request_id` | string | Agent 请求 ID |
| `user_id` | int64 | 用户 ID |
| `room_id` | int64 | 群聊 ID |
| `tool_name` | string | 工具名 |
| `arguments` | string | 工具参数 JSON |
| `started_at` | time | 开始时间 |
| `finished_at` | time | 结束时间 |
| `latency_ms` | float64 | 耗时 |
| `success` | bool | 是否成功 |
| `error` | string | 错误信息 |

---

## 系统错误接口

### 查询系统错误

```http
GET /api/v1/admin/errors
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `module` | string | 模块 |
| `error_type` | string | 错误类型 |
| `resolved` | bool | 是否已处理 |
| `start` | string | 开始时间，RFC3339 |
| `end` | string | 结束时间，RFC3339 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 错误 ID |
| `timestamp` | time | 发生时间 |
| `module` | string | 模块 |
| `error_type` | string | 错误类型 |
| `error_message` | string | 错误信息 |
| `request_id` | string | 请求 ID |
| `user_id` | int64 | 用户 ID |
| `room_id` | int64 | 群聊 ID |
| `resolved` | bool | 是否已处理 |

### 标记错误已处理

```http
POST /api/v1/admin/errors/:id/resolve
```

无请求体。

---

## 审计日志接口

```http
GET /api/v1/admin/audit-logs
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `keyword` | string | 关键字 |
| `admin_user_id` | int64 | 管理员 ID |
| `action` | string | 动作 |
| `target_type` | string | 目标类型 |
| `target_id` | string | 目标 ID |
| `result` | string | 结果 |
| `start` | string | 开始时间，RFC3339 |
| `end` | string | 结束时间，RFC3339 |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 审计日志 ID |
| `admin_user_id` | int64 | 管理员 ID |
| `admin_username` | string | 管理员用户名 |
| `action` | string | 动作 |
| `target_type` | string | 目标类型 |
| `target_id` | string | 目标 ID |
| `before_data` | string | 操作前数据 |
| `after_data` | string | 操作后数据 |
| `request_id` | string | 请求 ID |
| `remote_ip` | string | 来源 IP |
| `result` | string | 结果 |
| `error_message` | string | 错误信息 |
| `created_at` | time | 操作时间 |

---

## 健康检查接口

```http
GET /api/v1/admin/health
```

返回 MySQL、Redis、Qdrant、MinIO、LLM、Embedding、WebSocket Hub 的健康状态。当前健康检查暂不覆盖 MongoDB、Elasticsearch 和 Kafka。

```json
{
  "items": [
    {
      "name": "mysql",
      "status": "healthy",
      "latency_ms": 1.2,
      "error": "",
      "checked_at": "2026-08-15T12:00:00+08:00"
    }
  ]
}
```

`status` 可选值：

| 值 | 说明 |
| --- | --- |
| `healthy` | 健康 |
| `degraded` | 降级 |
| `down` | 不可用 |

---

## 告警接口

### 查询告警列表

```http
GET /api/v1/admin/alerts?page=1&page_size=20&level=warning&status=unresolved
```

查询参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `page` | int | 页码 |
| `page_size` | int | 每页条数 |
| `level` | string | `info`、`warning`、`critical` |
| `status` | string | `unresolved`、`resolved` |

响应项主要字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | int64 | 告警 ID |
| `name` | string | 告警名称 |
| `level` | string | 告警级别 |
| `source` | string | 来源 |
| `message` | string | 告警内容 |
| `status` | string | 状态 |
| `resolved_at` | time/null | 处理时间 |
| `created_at` | time | 创建时间 |

### 未处理告警数量

```http
GET /api/v1/admin/alerts/unresolved-count
```

响应：

```json
{
  "count": 3
}
```

### 触发告警评估

```http
POST /api/v1/admin/alerts/evaluate
```

无请求体，会立即基于当前运行指标生成或更新告警。

### 处理告警

```http
POST /api/v1/admin/alerts/:id/resolve
```

无请求体。
