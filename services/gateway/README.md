# Gateway 服务

- `websocket`：维护单个 Gateway 进程内的连接、房间和分片状态。
- `handlers`：对外提供 HTTP 和 WebSocket 处理器。
- `http`：负责组装路由。
- `grpc`：为 Agent 运行时提供 IM gRPC 接入服务。
- `Dockerfile.monolith`：兼容性镜像；在消息处理被拆分为独立可执行程序之前，暂时用于启动根入口程序。

Gateway 只负责持有实时 Socket 对象。持久化消息历史、消息搜索和文件存储由各自对应的服务负责。

每个 Hub 都拥有独立的消息扇出 goroutine 池。较大的本地房间扇出任务会被均衡拆分成至少两个子任务，`HUB_FANOUT_BATCH_SIZE` 用于限制单个任务可处理的连接数量，默认值为 `200`。`HUB_FANOUT_WORKERS` 用于控制协程池容量。分片必须等待一条消息的所有批次处理完成，才会继续处理下一条消息，以此保证同一房间内的消息投递顺序。
