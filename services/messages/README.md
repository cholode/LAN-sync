# 消息模块

当前采用「广播前编号、正式消息允许重复投递、下游幂等」链路。完整讨论与设计边界见 [消息幂等与群序号](../../docs/message-idempotency.md)。

| 容器 | 入口 | 职责 |
| --- | --- | --- |
| `message-service` / `im-message` | `cmd/api`，8083 | 历史、搜索、文件接口 |
| `message-sequencer` / `im-message-sequencer` | `cmd/sequencer`，6062 | 单实例内存去重、群编号，以及独立正式消息广播循环 |
| `message-worker` / `im-message-worker` | `cmd/worker`，6061 | 批量归档、搜索索引、最近消息缓存 |

统一使用 `Dockerfile`，通过 `MESSAGE_PROCESS=api`、`worker` 或 `sequencer` 构建。编号器只依赖 Kafka、Redis，不同步等待数据库。

## 消息流

```text
多个 Gateway → Kafka 待处理主题 → 单实例编号器 → Kafka 正式主题
                                                    ├→ 广播循环 → Redis → 各 Gateway
                                                    ├→ message-worker → MySQL/MongoDB
                                                    └→ Agent Inbox
浏览器 → Nginx → message-service → 消息存储 / ES / MinIO
```

编号器以 `(sender_id, client_msg_id)` 去重，重复请求复用原 `message_id` 和 `room_seq`。输出失败重试，不修改编号，确认输出后才提交输入进度。正式消息的重投可能形成不同 Kafka offset，下游使用业务身份去重。

数据库使用消息 ID、发送者与客户端凭证联合键、群与序号联合键约束。正式消息冲突报错，归档不会重新编号。前端合并实时和历史，按群序号展示，并保留晚到的小序号消息。

## 当前限制

- 编号器仅一个实例，状态只在内存中，**不支持崩溃恢复、重启续接、容器重建续接或多实例接管**；Compose 不自动重启它。去重状态暂不清理。
- 当前两个主题均要求单分区，因为归档器仍直读分区 0；不能只增加分区而不改造消费。
- 归档器可以从 Redis 保存的进度恢复，数据库重复写入沿用原记录；这不代表编号器也能恢复。
- 当前 Redis Pub/Sub 没有离线送达保证；没有新增发送 ACK、送达回执或完整补拉机制。
- 数据库失败后归档器退出，由容器策略重试；不阻塞编号与广播。搜索写入失败仍只记录日志。
- Gateway 的部分 IM gRPC 查询仍直连仓库，数据所有权尚未完全隔离。

## 首次切换

1. 暂停写入，排空旧链路的 Kafka 归档积压。
2. 确认 `.env` 使用新的空主题：`KAFKA_INGRESS_TOPIC=im_chat_messages_ingress_v1`、`KAFKA_TOPIC=im_chat_messages_sequenced_v1`。旧主题不能直接当作正式主题。
3. 构建 `backend message-sequencer message-service message-worker agent-service agent-worker`；三 Gateway 部署还需更新其他 Gateway 实例。
4. 由 backend 完成消息字段和索引迁移后启动新链路；编号器、Agent、归档器必须使用同一正式主题。旧数据库历史保留，序号为空。
5. 构建前端，核对广播与历史包含相同 ID 和群序号。不要让旧生产者、旧归档器与新链路混跑。

本版本完成代码与隔离验证，不自动切换现有业务容器。编号器首次运行后若需重启并继续原群，应先实现状态持久化，不能清空业务数据来绕过冲突。

## 验证

- `go test ./...`、`go vet ./...`。
- `node --test frontend/src/utils/message-order.test.js`、前端构建。
- Agent：在生成 Python Protobuf 后运行 `python -m unittest discover -s tests -p test_message_idempotency.py`。
- 真实链路：`go test ./services/messages/integration -run TestSequencedPipeline -v`，必须提供 `MESSAGE_PIPELINE_ISOLATED=yes` 和独立的 `MESSAGE_TEST_BROKER`、`MESSAGE_TEST_MYSQL_DSN`、`MESSAGE_TEST_MONGO_URI`、`MESSAGE_TEST_REDIS_ADDR`。MySQL 数据库必须为空白测试库；测试会创建并清理独立 Kafka 主题、MongoDB 测试集合。
- 旧拆分冒烟脚本 `scripts/smoke-message-split.mjs` 仍可用于完整 HTTP/WebSocket 联调，但环境必须额外运行编号器，并统一新主题配置。

所有入口都有 `/health/live`、`/health/ready` 和 `/metrics`。健康检查不等于消息已送达或消费没有积压。
