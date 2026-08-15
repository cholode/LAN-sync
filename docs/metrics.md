# Metrics 指标说明

## 1. 技术方案

项目使用 Prometheus 标准指标协议，通过独立管理端口暴露：

- `/metrics`：Prometheus 抓取端点
- `/debug/pprof/`：Go pprof 性能分析端点

指标代码统一放在 `internal/metrics`，业务模块只调用埋点函数。

## 2. 环境变量

以下配置统一放在项目根目录 `.env`：

```env
METRICS_ENABLED=true
METRICS_ADDR=0.0.0.0:6060
METRICS_PATH=/metrics
NODE_ID=lan-im-node-1
```

迁移到云服务器时，只需要修改 `.env` 中的网络地址：

- `METRICS_ADDR` 建议保持 `0.0.0.0:6060`
- 如果使用内网监控，可改为 `10.0.0.2:6060`
- 多节点部署时，每个节点设置不同的 `NODE_ID`

Docker Compose 已自动将上述变量注入 `backend` 服务，并在宿主机映射 `6060` 端口。

## 3. 指标清单

### 基础指标

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_build_info` | Gauge | 服务构建信息 |
| `im_uptime_seconds` | Gauge | 进程运行时长 |

### WebSocket

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_ws_connections_active` | Gauge | 当前活跃连接数 |
| `im_ws_connections_total` | Counter | 累计连接数 |
| `im_ws_connection_duration_seconds` | Histogram | 连接存活时长 |
| `im_ws_read_messages_total` | Counter | 读取消息数 |
| `im_ws_write_messages_total` | Counter | 写出消息数 |
| `im_ws_read_errors_total` | Counter | 读取错误数 |
| `im_ws_write_errors_total` | Counter | 写出错误数 |

### Hub

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_hub_clients_total` | Gauge | 当前 Hub 客户端数 |
| `im_hub_rooms_total` | Gauge | 当前 Hub 房间数 |
| `im_hub_dispatched_messages_total` | Counter | 分发消息数 |
| `im_hub_dispatch_latency_seconds` | Histogram | 分发耗时 |
| `im_hub_queue_drops_total` | Counter | 队列满丢弃数 |
| `im_hub_task_pool_running` | Gauge | 任务池运行中任务数 |
| `im_hub_task_pool_waiting` | Gauge | 任务池等待任务数 |
| `im_hub_task_pool_capacity` | Gauge | 任务池容量 |

### Kafka

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_kafka_produce_total` | Counter | 生产消息数 |
| `im_kafka_produce_latency_seconds` | Histogram | 生产耗时 |
| `im_kafka_consume_total` | Counter | 消费消息数 |
| `im_kafka_consume_latency_seconds` | Histogram | 消费耗时 |
| `im_kafka_read_errors_total` | Counter | 读取错误数 |
| `im_kafka_consumer_lag` | Gauge | 消费滞后量 |

### Redis

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_redis_ops_total` | Counter | Redis 命令执行数 |
| `im_redis_errors_total` | Counter | Redis 命令错误数 |
| `im_redis_latency_seconds` | Histogram | Redis 命令耗时 |
| `im_redis_pubsub_total` | Counter | Pub/Sub 事件数 |
| `im_redis_up` | Gauge | Redis 可用状态 |
| `im_redis_pool_idle_connections` | Gauge | Redis 空闲连接数 |
| `im_redis_pool_total_connections` | Gauge | Redis 总连接数 |

### MySQL / MongoDB

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_db_query_total` | Counter | MySQL/GORM 查询数 |
| `im_db_query_errors_total` | Counter | MySQL/GORM 查询错误数 |
| `im_db_query_duration_seconds` | Histogram | MySQL/GORM 查询耗时 |
| `im_db_pool_open_connections` | Gauge | MySQL 打开连接数 |
| `im_db_pool_idle_connections` | Gauge | MySQL 空闲连接数 |
| `im_db_pool_in_use_connections` | Gauge | MySQL 使用中连接数 |
| `im_db_pool_wait_count_total` | Counter | MySQL 等待连接次数 |
| `im_db_pool_wait_duration_seconds_total` | Counter | MySQL 等待连接总时长 |
| `im_db_command_total` | Counter | MongoDB 命令数 |
| `im_db_command_errors_total` | Counter | MongoDB 命令错误数 |
| `im_db_command_duration_seconds` | Histogram | MongoDB 命令耗时 |

### Agent

| 指标 | 类型 | 说明 |
|---|---|---|
| `im_agent_rooms_enabled` | Gauge | 启用 Agent 的房间数 |
| `im_agent_inflight_requests` | Gauge | 处理中 Agent 请求数 |
| `im_agent_messages_received_total` | Counter | Agent 收到消息数 |
| `im_agent_messages_triggered_total` | Counter | Agent 触发回复数 |
| `im_agent_processed_total` | Counter | Agent 处理结果数 |
| `im_agent_errors_total` | Counter | Agent 错误数 |
| `im_agent_reply_latency_seconds` | Histogram | Agent 回复耗时 |

## 4. 本地验证

启动服务后，在 PowerShell 中执行：

```powershell
curl http://127.0.0.1:6060/metrics | Select-String "im_"
```

如果 Prometheus 已配置抓取，可将目标地址设置为：

```yaml
scrape_configs:
  - job_name: lan-im
    metrics_path: /metrics
    static_configs:
      - targets:
          - 'backend:6060'
```

## 5. 云服务器迁移

所有可调整的网络地址集中在 `.env`：

- `METRICS_ADDR`：指标和 pprof 管理地址
- `REDIS_ADDR`：Redis 地址
- `KAFKA_ADDR`：Kafka 地址
- `DB_DSN`：MySQL 地址
- `MONGO_URI`：MongoDB 地址
- `ES_ADDR`：Elasticsearch 地址
- `AGENT_GRPC_ADDR`：Agent gRPC 地址
- `MINIO_ENDPOINT`：MinIO 地址
- `MINIO_PUBLIC_ENDPOINT`：浏览器访问 MinIO 的地址

上传到云服务器后，只需修改 `.env` 中的对应地址，再重启 Docker Compose 即可。
