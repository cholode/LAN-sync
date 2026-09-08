# LAN IM 接口测试手册

本文档按当前代码中的实际路由整理，供本地联调、接口回归和压测使用。接口发生变化时，应同步修改本文档。

## 1. 测试入口

| 服务 | 默认地址 | 用途 |
| --- | --- | --- |
| Nginx 统一入口 | `http://127.0.0.1` | 用户、群聊、消息和管理接口的推荐测试入口 |
| Gateway | `http://127.0.0.1:8080` | 绕过 Nginx 直接测试 Gateway |
| Admin Service | `http://127.0.0.1:8081` | 绕过 Nginx直接测试管理接口 |
| Room Service | `http://127.0.0.1:8082` | 绕过 Nginx 直接测试房间接口 |
| Message Service | 容器内 `http://message-service:8083` | 消息与文件 API；宿主机通过 Nginx 统一入口访问 |
| Message Worker | 容器内 `http://message-worker:6061` | 健康检查与指标，不提供业务 HTTP API |
| Message Sequencer | 容器内 `http://message-sequencer:6062` | 单实例内存编号与正式消息广播，暂不支持重启恢复 |
| Agent API | `http://127.0.0.1:8000` | Python Agent 管理面；当前没有鉴权，只允许内网测试 |
| Prometheus | `http://127.0.0.1:9090` | 指标、Target 和 PromQL |
| Grafana | `http://127.0.0.1:3000` | 监控与压测结果看板 |

以下示例以 PowerShell 为主：

```powershell
$base = "http://127.0.0.1"
$token = "用户 JWT"
$adminToken = "管理员 JWT"
$auth = @{ Authorization = "Bearer $token" }
$adminAuth = @{ Authorization = "Bearer $adminToken" }
```

除登录、注册、健康检查和监控端点外，HTTP 接口默认需要：

```http
Authorization: Bearer <JWT>
Content-Type: application/json
```

统一错误响应通常为：

```json
{"error":"错误说明"}
```

## 2. 用户认证

| 方法 | 路径 | 鉴权 | 请求体/说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/register` | 否 | `username` 长度 3～32，`password` 长度 6～32 |
| `POST` | `/api/v1/login` | 否 | 普通用户登录；管理员会收到 `403` |
| `POST` | `/api/v1/admin/login` | 否 | 管理员、审核员和运营人员登录 |

注册：

```powershell
Invoke-RestMethod -Method Post -Uri "$base/api/v1/register" -ContentType "application/json" -Body '{"username":"alice","password":"example-password"}'
```

普通用户登录并保存 Token：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "$base/api/v1/login" -ContentType "application/json" -Body '{"username":"alice","password":"example-password"}'
$token = $login.token
$auth = @{ Authorization = "Bearer $token" }
```

管理员登录：

```powershell
$login = Invoke-RestMethod -Method Post -Uri "$base/api/v1/admin/login" -ContentType "application/json" -Body '{"username":"admin","password":"example-password"}'
$adminToken = $login.token
$adminAuth = @{ Authorization = "Bearer $adminToken" }
```

登录接口可能返回 `400`、`401`、`403`、`429`、`503`、`504` 或 `500`。`429`、`503` 响应包含 `Retry-After`，压测脚本必须遵守该值，禁止持续重试密码接口。

## 3. 房间与成员

| 方法 | 路径 | 请求参数 | 预期结果 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/rooms` | JSON：`name` | 创建群聊并返回 `room_id` |
| `GET` | `/api/v1/rooms` | Query：`query`、`offset=0`、`limit=20` | 搜索群聊，`limit` 最大 100 |
| `GET` | `/api/v1/my_rooms` | 无 | 当前用户加入的群聊 |
| `POST` | `/api/v1/rooms/{room_id}/join` | 无 | 加入群聊 |
| `GET` | `/api/v1/rooms/{room_id}/members` | 无 | 群成员列表，仅成员可见 |
| `DELETE` | `/api/v1/rooms/{room_id}/members/{user_id}` | 无 | 群主、群管理员或平台管理员移除成员 |
| `DELETE` | `/api/v1/rooms/{room_id}/disband` | 无 | 群主解散群聊，破坏性操作 |

```powershell
$room = Invoke-RestMethod -Method Post -Uri "$base/api/v1/rooms" -Headers $auth -ContentType "application/json" -Body '{"name":"接口测试群"}'
$roomId = $room.room_id
Invoke-RestMethod -Method Get -Uri "$base/api/v1/rooms/$roomId/members" -Headers $auth
```

## 4. 消息查询与搜索

| 方法 | 路径 | 请求参数 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/rooms/{room_id}/messages` | `cursor=0`、`limit=50` | 游标分页历史消息，`limit` 最大 100 |
| `GET` | `/api/v1/rooms/{room_id}/messages/search` | `q` 必填；可选 `sender_id`、`start`、`end`、`from`、`size` | `start/end` 为 Unix 毫秒；`size` 最大 100 |

历史消息中的雪花 ID 使用 JSON 字符串返回，测试程序不能先转换成 JavaScript `Number`。

```powershell
Invoke-RestMethod -Method Get -Uri "$base/api/v1/rooms/$roomId/messages?cursor=0&limit=50" -Headers $auth
Invoke-RestMethod -Method Get -Uri "$base/api/v1/rooms/$roomId/messages/search?q=keyword&from=0&size=20" -Headers $auth
```

## 5. WebSocket 消息

连接地址：

```text
ws://127.0.0.1/api/v1/ws?token=<JWT>
```

生产环境优先使用 `Sec-WebSocket-Protocol` 传递 Token；查询参数模式适合本地工具和 k6。连接成功并完成鉴权、群成员加载、Hub 注册及 Redis 在线状态更新后才算可传输。

客户端发送格式：

```json
{
  "room_id": 4,
  "content": "hello",
  "client_msg_id": "test-20260906-0001"
}
```

`client_msg_id` 必填，长度不超过 64 字节，同一次发送重试必须复用；按发送者作用域去重。服务端经编号器和正式 Kafka 主题广播，包含字符串形式的 `id`、`room_seq`、`room_id`、`sender_id` 和 `client_msg_id`，以及内容、类型和创建时间。历史接口也返回 `room_seq` 和 `client_msg_id`。前端按消息身份合并、按群序号排序，不按到达时间排序。当前没有新增发送 ACK 或送达回执。设计与限制见 [消息幂等文档](message-idempotency.md)。旧 WebSocket 压测见 [perf7/README.md](../perf7/README.md)，不能将旧性能结果直接视为新链路数据。

## 6. 文件上传与下载

| 方法 | 路径 | 请求参数 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/files/presign` | `filename`、`file_type`、`file_size` | 获取对象存储上传地址 |
| `PUT` | 返回的 `upload_url` | 原始文件二进制 | 直接上传 MinIO/OSS，不携带 JWT |
| `POST` | `/api/v1/files/complete` | `object_key`、`original_name`、`sha256`、`file_size`、`room_id` | 登记文件并返回文件 ID |
| `GET` | `/api/v1/files/{file_id}/download` | 无 | 校验上传者或群成员后返回短期下载地址 |
| `GET` | `/api/v1/download/{object_key}` | URL 编码的对象键 | 兼容历史消息，同样执行权限校验 |

两段式流程：

```powershell
$presignBody = @{ filename = "report.pdf"; file_type = "pdf"; file_size = 1024 } | ConvertTo-Json
$presign = Invoke-RestMethod -Method Post -Uri "$base/api/v1/files/presign" -Headers $auth -ContentType "application/json" -Body $presignBody
Invoke-WebRequest -Method Put -Uri $presign.upload_url -InFile ".\report.pdf" -ContentType "application/pdf"
$completeBody = @{ object_key = $presign.object_key; original_name = "report.pdf"; sha256 = ""; file_size = 1024; room_id = $roomId } | ConvertTo-Json
$file = Invoke-RestMethod -Method Post -Uri "$base/api/v1/files/complete" -Headers $auth -ContentType "application/json" -Body $completeBody
$signed = Invoke-RestMethod -Method Get -Uri "$base$($file.download_url)" -Headers $auth
Invoke-WebRequest -Uri $signed.download_url -OutFile ".\downloaded-report.pdf"
```

## 7. 群聊 Agent 开关

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/rooms/{room_id}/agent/enable` | 启用已绑定的 Agent |
| `POST` | `/api/v1/rooms/{room_id}/agent/disable` | 暂停群聊 Agent |
| `DELETE` | `/api/v1/rooms/{room_id}/agent` | 软删除绑定和配置并移除 Bot 群成员，破坏性操作 |

只有群主、群管理员或平台管理员可以执行；尚未绑定 Agent 时启用会返回 `409`。

## 8. Python Agent 管理面

默认直连 `http://127.0.0.1:8000`。这些接口目前没有 JWT 中间件，不能直接暴露在公网。

| 方法 | 路径 | 请求体/查询参数 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/health/live` | 无 | 进程存活检查 |
| `GET` | `/api/v1/health/ready` | 无 | 数据库就绪检查 |
| `GET` | `/docs` | 无 | FastAPI Swagger UI，仅用于开发测试 |
| `GET` | `/openapi.json` | 无 | FastAPI OpenAPI 描述 |
| `POST` | `/api/v1/bots` | `code`、`name`、可选 `description/avatar_url` | 创建独立 Bot 身份 |
| `GET` | `/api/v1/bots` | 无 | 查询未删除 Bot |
| `POST` | `/api/v1/room-agent-bindings` | `room_id`、`bot_id`、可选 `legacy_bot_user_id/enabled/priority` | 绑定 Bot 与房间 |
| `GET` | `/api/v1/rooms/{room_id}/agents` | 无 | 查询房间内全部 Agent |
| `PUT` | `/api/v1/room-agent-bindings/{binding_id}/config` | `system_prompt`、`trigger_words`、`rag_enabled`、`extra_config` | 新建或覆盖配置 |
| `GET` | `/api/v1/room-agent-bindings/{binding_id}/config` | 无 | 获取绑定配置 |
| `DELETE` | `/api/v1/room-agent-bindings/{binding_id}/config` | 无 | 软删除配置，返回 `204` |
| `GET` | `/api/v1/removal-requests` | 可选 `room_id`、`request_status=pending` | 查询审核 Agent 的移除申请，最多 200 条 |

```powershell
$agentBase = "http://127.0.0.1:8000"
$bot = Invoke-RestMethod -Method Post -Uri "$agentBase/api/v1/bots" -ContentType "application/json" -Body '{"code":"moderator-01","name":"审核助手"}'
$bindingBody = @{ room_id = $roomId; bot_id = $bot.id; enabled = $true; priority = 100 } | ConvertTo-Json
$binding = Invoke-RestMethod -Method Post -Uri "$agentBase/api/v1/room-agent-bindings" -ContentType "application/json" -Body $bindingBody
$configBody = @{ system_prompt = "遵守群规"; trigger_words = @("@助手"); rag_enabled = $true; extra_config = @{} } | ConvertTo-Json -Depth 5
Invoke-RestMethod -Method Put -Uri "$agentBase/api/v1/room-agent-bindings/$($binding.id)/config" -ContentType "application/json" -Body $configBody
```

## 9. Admin 治理接口

所有 `/api/v1/admin/*` 治理接口均需管理员 JWT，并按角色继续校验权限。分页接口统一使用 `page`、`page_size`，其中 `page_size` 最大 100；时间过滤使用 RFC3339。

### 9.1 用户与房间

| 方法 | 路径 | 查询参数/请求体 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/users` | `keyword`、`role`、`status`、`start`、`end`、分页 |
| `GET` | `/api/v1/admin/users/{id}` | 无 |
| `POST` | `/api/v1/admin/users/{id}/action` | `action`: `ban`、`unban`、`role_user`、`role_operator`、`role_moderator`、`role_super_admin` |
| `GET` | `/api/v1/admin/rooms` | `keyword`、`type`、`status`、`agent_enabled`、`start`、`end`、分页 |
| `GET` | `/api/v1/admin/rooms/{id}` | 无 |
| `POST` | `/api/v1/admin/rooms/{id}/action` | `action` 和可选 `target_user_id` |
| `DELETE` | `/api/v1/admin/rooms/{id}` | 解散群聊，破坏性操作 |

房间动作：`freeze`、`unfreeze`、`disband`、`agent_enable`、`agent_disable`、`moderation_enable`、`moderation_disable`、`remove_member`、`set_admin`、`transfer_owner`。后三项必须提供 `target_user_id`。

### 9.2 内容治理

| 方法 | 路径 | 查询参数/请求体 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/dashboard/moderation` | 审核统计，不是运行监控 |
| `GET` | `/api/v1/admin/moderation` | `username`、`user_id`、`room_id`、`category`、`risk_level`、`penalty_status`、`start`、`end`、分页 |
| `GET` | `/api/v1/admin/moderation/{id}` | 无 |
| `POST` | `/api/v1/admin/moderation/{id}/action` | `action`: `warn`、`mute`、`kick`、`ban`、`revoke`、`false_positive`、`confirmed` |

### 9.3 文件治理

| 方法 | 路径 | 查询参数/说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/files` | `keyword`、`uploader_id`、`room_id`、`file_type`、`status`、`start`、`end`、分页 |
| `GET` | `/api/v1/admin/files/scan` | 检查异常文件 |
| `POST` | `/api/v1/admin/files/cleanup` | 清理孤立对象，破坏性操作 |
| `GET` | `/api/v1/admin/files/{id}` | 文件详情 |
| `GET` | `/api/v1/admin/files/{id}/download` | 获取管理员预签名下载地址 |
| `DELETE` | `/api/v1/admin/files/{id}` | 删除对象及数据库记录，破坏性操作 |

### 9.4 Agent、RAG 与审计

| 方法 | 路径 | 查询参数/请求体 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/agent-config` | 获取全局配置 |
| `PUT` | `/api/v1/admin/agent-config` | 全局 Agent 配置 JSON |
| `GET` | `/api/v1/admin/agent-config/history` | 分页 |
| `POST` | `/api/v1/admin/agent-config/rollback` | 回滚上一版本，破坏性操作 |
| `GET` | `/api/v1/admin/rag/queries` | `room_id`、分页 |
| `GET` | `/api/v1/admin/tool-calls` | `tool_name`、`user_id`、`room_id`、`success`、`start`、`end`、分页 |
| `GET` | `/api/v1/admin/audit-logs` | `keyword`、`admin_user_id`、`action`、`target_type`、`target_id`、`result`、`start`、`end`、分页 |

全局 Agent 配置允许字段：`global_enabled`、`default_model`、`embedding_model`、`temperature`、`max_tokens`、`rag_top_k`、`rag_similarity_threshold`、`chunk_size`、`chunk_overlap`、`moderation_enabled`、`moderation_model`、`moderation_threshold`、`tool_calling_enabled`、`auto_kick_enabled`、`auto_ban_enabled`、`system_prompt`、`moderation_prompt`、`rag_prompt`、`tool_calling_prompt`。

Admin 不再提供 WebSocket、系统运行和告警查询；运行监控统一进入 Grafana。

## 10. 健康与监控接口

| 地址 | 说明 |
| --- | --- |
| `GET http://127.0.0.1:8082/health` | Room Service 健康检查 |
| `GET http://127.0.0.1:8000/api/v1/health/live` | Agent 存活检查 |
| `GET http://127.0.0.1:8000/api/v1/health/ready` | Agent 数据库就绪检查 |
| `GET http://127.0.0.1:6060/metrics` | Gateway/Backend 指标 |
| `GET http://127.0.0.1:8081/metrics` | Admin 指标 |
| `GET http://127.0.0.1:8082/metrics` | Room 指标 |
| `GET http://127.0.0.1:8000/metrics` | Agent API 指标 |
| `GET http://127.0.0.1:9090/targets` | Prometheus 抓取状态 |
| `GET http://127.0.0.1:9090/api/v1/query?query=up` | PromQL 即时查询 |
| `POST http://127.0.0.1:9090/api/v1/write` | k6 remote write，仅用于受控压测网络 |
| `GET http://127.0.0.1:3000` | Grafana 看板 |

快速检查：

```powershell
Invoke-RestMethod "http://127.0.0.1:9090/api/v1/query?query=up"
curl.exe http://127.0.0.1:6060/metrics | Select-String "im_"
```

Grafana 预置看板：

- `LAN IM 运行总览`
- `LAN IM WebSocket 与 Gateway`
- `LAN IM Agent 与 RAG`
- `LAN IM Gateway 压测`

## 11. 回归测试顺序

1. 检查 Room、Agent 健康接口和 Prometheus Targets。
2. 注册普通用户，登录并保存 JWT。
3. 创建群聊，另一个用户搜索并加入，检查成员列表。
4. 建立 WebSocket，发送带唯一 `client_msg_id` 的消息。
5. 查询历史消息并验证搜索结果。
6. 执行文件预签名、直传、完成登记和鉴权下载。
7. 创建 Bot、建立房间绑定、写入配置，再测试 Agent 开关。
8. 使用不同后台角色验证治理接口的 `200/403` 权限边界。
9. 检查 Grafana 是否出现 Gateway、Room、Admin、Agent 的 API QPS 和延迟。
10. 压测时使用唯一 `PERF_TEST_ID`，在 `LAN IM Gateway 压测` 看板核对客户端与服务端数据。

禁止在共享或生产数据上自动执行解散群聊、移除成员、封禁用户、删除文件、清理孤立对象、删除 Agent 配置和回滚配置等破坏性接口。
