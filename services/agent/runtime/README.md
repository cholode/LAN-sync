# Agent 服务

基于 FastAPI、LangChain 和 LangGraph 构建的 Python Agent 微服务。FastAPI 提供 Bot、群绑定、配置和移除申请管理接口；独立 Worker 从 Kafka 可靠写入 Inbox，然后执行审核、关键词对话、话题分块和 Qdrant 向量化。

## 运行

```bash
cd services/agent/runtime
python -m pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8000
python -m app.workers.main
```

FastAPI 默认监听 `8000`。后台 Worker 通过 Kafka 接收群聊消息；Python 调用 Go IMService 的 `50052` gRPC 内部端口查询原始消息和发送回复。

## 处理链路

```text
Kafka -> MySQL Inbox -> 审核 Agent
  -> rejected/needs_review -> 禁止向量化，必要时创建移除申请
  -> approved -> 关键词对话 Agent -> 回复
              -> Chunk Agent -> Embedding -> Qdrant
```

Kafka offset 在 Inbox 事务提交后才提交。一个 Partition 可以包含多个群聊，后续处理按 `room_id` 从 Inbox 独立聚合。

## 管理接口

```text
GET    /api/v1/health/live
GET    /api/v1/health/ready
POST   /api/v1/bots
GET    /api/v1/bots
POST   /api/v1/room-agent-bindings
GET    /api/v1/rooms/{room_id}/agents
GET    /api/v1/room-agent-bindings/{binding_id}/config
PUT    /api/v1/room-agent-bindings/{binding_id}/config
DELETE /api/v1/room-agent-bindings/{binding_id}/config
GET    /api/v1/removal-requests
```

数据库表会在 API 或 Worker 启动时创建；正式发布仍可按 `migrations/` 中的 SQL 纳入部署迁移流程。

- `app/graph.py`：LangGraph 状态图
- `app/prompt.py`：群聊对话提示词模板
- `app/rag.py`：Qdrant 向量检索
- `app/embeddings.py`：Doubao 多模态 embedding HTTP 客户端
- `app/tools.py`：`get_messages` 与群聊数据库查询工具
- `app/storage/room_query.py`：群聊 SQL Schema、SELECT 校验、群范围绑定与结果限制
- `app/im_client.py`：用于调用 Go `IMService` 的 gRPC 客户端

## 配置

服务默认从当前目录的 `config.yaml` 读取配置。可以通过环境变量 `AGENT_CONFIG_PATH` 覆盖路径。

```yaml
llm:
  base_url: "https://api.deepseek.com/v1"
  api_key: "${LLM_API_KEY}"
  model: "deepseek-chat"
  timeout_seconds: 60

embedding:
  base_url: "https://ark.cn-beijing.volces.com/api/v3"
  api_key: "${EMBED_API_KEY}"
  model: "doubao-embedding-vision-251215"

qdrant:
  host: "qdrant"
  grpc_port: 6334
  vector_size: 1024

redis:
  addr: "redis:6379"

im_service:
  grpc_addr: "backend:50052"
```

诸如 `${LLM_API_KEY}` 这样的值会在启动时从环境变量中解析。运行时智能体默认配置也定义在 `config.yaml` 的 `agent` 部分中。

## 群聊数据库查询工具

对话 Agent 可以调用 `query_room_database` 查询当前群聊的数据。Schema 由服务端白名单生成，未列出的表和字段（例如用户密码）不可访问。SQL 必须是单条 `SELECT`，每个群范围表都必须在 `WHERE` 的强制 `AND` 条件中使用 `__ROOM_ID__`；实际群号由服务端绑定，模型不能指定。子查询、注释、通配字段、非白名单函数和跨群 JOIN 会被拒绝。

默认最多返回 100 行、执行 3 秒、输出 20000 字符；查询出错时 Agent 最多根据错误自动修正 3 次。Docker Compose 使用 `AGENT_QUERY_DB_PASSWORD` 创建并连接只授予 `SELECT` 权限的 `agent_reader`；在宿主机直接运行 Agent 时，可通过 `AGENT_QUERY_DATABASE_URL` 指向同一账号。
