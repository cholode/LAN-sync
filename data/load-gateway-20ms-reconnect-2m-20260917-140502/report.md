# 10000 连接、100 群压测

- Run ID: mu54llt6
- 起点 UTC：2026-09-17T06:05:56.280Z（北京时间 +08:00）
- 120 秒平缓建连；120 秒发送；60 秒静默；随后断连。
- 每群 100 人，其中 10 人各以 1 条/秒发送单字符 #。
- 同一台 Windows 主机运行 k6 与 Docker/WSL 服务端；客户端资源与服务端互相竞争。
- 临时独立 Kafka 主题见 environment.json；本轮关闭链路容器日志输出。

| 项目 | 结果 |
|---|---:|
| 连接成功 | 10000 |
| 握手失败 | 0 |
| 建连期指数退避重试 | 0 |
| 建连期连接错误（不计正式失败） | 0 |
| 服务端连接峰值 | 10000 |
| 首次采样到全部连接（秒） | 121.474 |
| 最后服务端连接数 | 0 |
| 计划发送消息 | 120000 |
| 客户端实际发送 | 119999 |
| 实际平均发送速率（条/秒） | 999.99 |
| 客户端错过的发送时隙 | 0 |
| 客户端收到的广播次数 | 11999900 |
| 客户端收到的WebSocket帧 | 1895153 |
| 全部发送均广播到 100 人时应有次数 | 11999900 |
| 服务端读取消息增量 | 119999 |
| 服务端写入消息增量 | 11999900 |
| 服务端实际写出帧数 | 1895153 |
| 平均每帧业务消息数 | 6.33 |
| 服务端读取超时 | 0 |
| 服务端写错误 | 0 |
| 服务端队列丢弃增量 | 0 |
| 提前断连 | 0 |
| Socket 错误 | 9 |
| 重复或乱序广播 | 0 |
| 非法或错群广播 | 0 |
| Gateway内部消息平均延迟（毫秒） | 91.34 |
| Gateway出站帧最老消息平均延迟（毫秒） | 99.92 |
| Gateway出站帧最老消息 p95（毫秒） | 257.08 |
| Gateway出站帧最老消息 p99（毫秒） | 392.79 |
| k6 物理内存峰值（MiB） | 3976.2 |
| k6 私有提交内存峰值（MiB） | 4026.6 |
| WSL 物理内存峰值（MiB） | 8114.4 |
| Windows 最小可用物理内存（MiB） | 3330.3 |

Gateway链路延迟从入站WebSocket读取完成开始，到对应出站WebSocket帧在Gateway成功写完为止；不包含客户端到Gateway的上行网络、Gateway到客户端的下行网络、客户端事件循环和JSON解析。
客户端发送成功仅表示写入客户端连接，不代表服务端已接收、编号、入库或交付。
不能仅凭广播差额断言永久丢失，还需结合 Kafka backlog 判断积压。

## 容器资源峰值（10 秒采样，CPU 100% 相当于一个逻辑核）

| 容器 | 内存 MiB | CPU % |
|---|---:|---:|
| im-message-sequencer-load-gateway-20ms-reconnect-2m-20260917-140502 | 80.2 | 34.26 |
| im-agent | 58.5 | 1.55 |
| im-agent-worker | 150.5 | 74.75 |
| im-message-worker | 22.9 | 15.52 |
| im-backend | 1267.7 | 304.63 |
| im-message-sequencer-load-gateway-latency-20ms-2m-20260917-134241 | 75.4 | 0.00 |
| im-message-sequencer-load-gateway-latency-20ms-5m-20260917-134144 | 7.9 | 0.00 |
| im-message-sequencer-load-gateway-latency-5m-20260917-123921 | 146.1 | 0.00 |
| im-message-sequencer-ws-parse-1rps-20260917-121004 | 79.9 | 0.00 |
| im-message-sequencer-ws-receive-only-20260917-115915 | 299.6 | 0.00 |
| im-message-sequencer-ws-coalesce-20260917-113854 | 74.0 | 0.00 |
| im-message-sequencer-full-batch | 57.9 | 0.00 |
| im-message-sequencer-relay-batch | 57.6 | 0.00 |
| im-message-sequencer-batch | 1327.1 | 0.00 |
| im-nginx | 797.9 | 148.46 |
| im-message | 35.8 | 1.19 |
| im-message-sequencer | 48.6 | 0.00 |
| im-grafana | 186.8 | 3.02 |
| im-prometheus | 151.8 | 3.75 |
| im-admin | 26.0 | 1.54 |
| im-room | 26.2 | 0.96 |
| im-admin-frontend | 14.7 | 0.00 |
| im-mysql | 471.6 | 22.10 |
| kafka | 1621.0 | 276.17 |
| im-mongo | 918.1 | 158.64 |
| im-redis | 26.3 | 46.47 |
| im-qdrant | 32.0 | 0.11 |
| im-elasticsearch | 1366.0 | 45.55 |
| im-minio | 140.4 | 6.18 |

## 原始数据

- Gateway 原始 /metrics：164 份，每 2 秒采样，metrics/gateway-*.prom.gz。
- Prometheus 服务端与本轮 k6 指标：66 份，每 5 秒采样，metrics/prometheus-*.json.gz。
- containers.jsonl、host-client.jsonl：每 10 秒容器及主机/客户端资源。
- k6-summary.json：客户端汇总；run.json：时间线与进程退出状态。
- capture-errors.jsonl（若存在）：采集错误。
