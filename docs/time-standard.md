# 时间标准

LAN-IM 的微服务、数据库和消息链路内部统一使用 UTC：

- Go Backend 与 Admin 进程启动时将 `time.Local` 设为 UTC。
- Go MySQL DSN 使用 `loc=UTC`。
- Python Agent 使用 UTC 生成和解析内部时间。
- MySQL 会话默认时区为 `+00:00`。
- Kafka、JWT 和 gRPC 中的 Unix 时间戳表示 UTC 时间点，不做时区偏移。
- 前端或报表展示时再转换为用户时区；中国用户使用 `Asia/Shanghai`。

`DATETIME` 不携带时区信息。若数据库已有按北京时间写入的历史数据，切换到本标准前应先备份，
再在维护窗口将相关时间列转换成 UTC；不要对已经按 UTC 保存的数据重复转换。

示例：

```sql
-- 仅作为迁移示例，实际执行前需要确认列中的旧数据确实是北京时间。
UPDATE messages
SET created_at = CONVERT_TZ(created_at, '+08:00', '+00:00');
```

新部署或没有历史数据的开发环境不需要执行迁移。
