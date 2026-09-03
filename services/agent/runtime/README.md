# Agent 服务

基于 LangChain + LangGraph 构建的 Python 服务。它对外暴露 `AgentService` gRPC API，是当前项目实际运行的 Agent 主服务；Go backend 通过 `services/gateway/clients` 调用该服务，Go 侧编排代码位于 `services/agent/application`。

## 运行

```bash
cd services/agent/runtime
python -m pip install -r requirements.txt
python -m app.main
```

服务默认监听 `50051` 端口。

## 处理链路

```text
trigger -> RAG 检索 -> 构建提示词 -> LLM -> get_messages 工具 -> 回复
```

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
