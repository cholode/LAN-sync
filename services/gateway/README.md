# Gateway 服务

- `websocket`：维护单个 Gateway 进程内的连接、房间和分片状态。
- `handlers`：对外提供 HTTP 和 WebSocket 处理器。
- `http`：负责组装路由。
- `grpc`：为 Agent 运行时提供 IM gRPC 接入服务。
- `Dockerfile.monolith`：保留现有镜像文件名，启动根入口程序；消息 HTTP API 与归档已迁至独立消息容器。

Gateway 持有实时 Socket，负责认证、向待处理 Kafka 主题生产消息、订阅 Redis 并向本地连接扇出、Agent 操作和现有 gRPC 接口。消息先由 message-sequencer 统一编号，再由正式消息广播循环发布 Redis，Gateway 不直接发布未编号消息。历史、搜索和文件 HTTP API 由 message-service 提供，归档由 message-worker 提供。IMService 的消息读取暂时仍直连仓库，因此数据依赖尚未完全隔离。

每个 Hub 都拥有独立的消息扇出 goroutine 池。较大的本地房间扇出任务会被均衡拆分成至少两个子任务，`HUB_FANOUT_BATCH_SIZE` 用于限制单个任务可处理的连接数量，默认值为 `200`。`HUB_FANOUT_WORKERS` 用于控制协程池容量。分片必须等待一条消息的所有批次处理完成，才会继续处理下一条消息，以此保证同一房间内的消息投递顺序。
