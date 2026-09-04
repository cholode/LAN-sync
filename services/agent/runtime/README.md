# Agent 服务

基于 FastAPI、LangChain 和 LangGraph 构建的 Python Agent 微服务。FastAPI 提供 Bot、群绑定、配置和移除申请管理接口；独立 Worker 从 Kafka 可靠写入 Inbox，然后执行审核、关键词对话、话题分块和 Qdrant 向量化。

## 运行

```bash
cd services/agent/runtime
python -m pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8000
python -m app.workers.main
```

FastAPI 默认监听 `8000`。迁移期间 API 进程还会监听旧 `50051` gRPC 端口，保证 Go backend 可以继续调用；完成 Go 编排移除后可删除该兼容入口。

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
- `app/prompt.py`：与 Go 语言 `services/agent/application/prompt.go` 对应的提示词模板
- `app/rag.py`：Qdrant 向量检索
- `app/embeddings.py`：Doubao 多模态 embedding HTTP 客户端
- `app/tools.py`：基于 Go `IMService` 的 `get_messages` 工具
- `app/im_client.py`：用于调用 Go `IMService` 的 gRPC 客户端

## 配置

服务默认从当前目录的 `config.yaml` 读取配置。可以通过环境变量 `AGENT_CONFIG_PATH` 覆盖路径。

```yaml
grpc:
  host: "[::]"
  port: 50051
  max_workers: 10

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
