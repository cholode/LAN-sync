# 编号器批处理

生产入口 `services/messages/cmd/sequencer/main.go` 使用 `RunBatched`：

- 每批最多 4000 条，输入 key/value 总计最多 1 MiB，最多收集 5ms。
- 单批在途，顺序分配编号。同步写入 Kafka 并获得 RequireAll 确认后，批量提交输入偏移。
- Kafka Writer 同样限制批次条数与字节数，BatchTimeout 为 5ms；不足一批时收集与写入等待可能叠加。
- 重试复用相同业务编号。写入结果未知或部分成功时可能重发，消费者仍需幂等；这不是 exactly-once 保证。

4000 是本机短消息阶梯测试中吞吐最高的候选值，不是固定最优值。真实流量未必能在 5ms 内凑满一批。
广播 Relay 使用相同的批次上限，与编号循环独立。通过 Redis Pipeline 按输入顺序发布一批消息，
整批成功后统一提交 Kafka 偏移；提交失败仅重试提交，不重新发布。发布结果不确定时会重试整批，
可能产生重复投递，订阅端仍需按业务消息 ID 去重。Redis 发布成功不代表所有客户端已经收到消息。
4000 条对 Relay 尚未经过阶梯压测，不代表它的最优值。

## 验证

`go test ./services/messages/...` 运行普通测试。Kafka 吞吐集成测试默认跳过，需显式设置
`KAFKA_SEQUENCER_TEST_BROKERS`。`TestKafkaSequencerBatchSweep` 测试 0（逐条）、100、500，
以及 1000 到 10000（步长 1000），每档两轮。`SEQUENCER_PERF_MESSAGES` 默认为每轮 10000 条。

## 压测隔离

编号状态和请求去重表仍驻留内存，尚无重启恢复；去重表会随唯一消息数量增长。
新编号器必须使用独立 Kafka 主题。复用已有房间压测时，归档 Worker 也必须使用独立
`MONGO_DB`，否则重置后的群序号会与旧数据库的 `(room_id, room_seq)` 唯一索引冲突。
正式压测前应同时验证握手、广播、归档记录和归档偏移，不能只检查 HTTP 存活。
