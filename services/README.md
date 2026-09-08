# Service boundaries

This directory is the deployment boundary of the monorepo. Code may be built
together during the transition, but new business code should be added to its
own service instead of the repository-level `internal` directory.

| Service | Current role | Target deployment |
| --- | --- | --- |
| `gateway` | HTTP/WebSocket ingress, local connection hub, Redis subscription and gRPC ingress | Three or more identical gateway containers |
| `messages` | message-service API、单实例内存 message-sequencer（含独立广播循环）、message-worker 归档 | 后续补齐编号状态恢复、消费分区和索引重试 |
| `admin` | admin HTTP API and runtime control plane | Independent admin container |
| `agent` | Python FastAPI management plane plus Kafka Agent workers | Independent agent runtime |
| `rooms` | room lifecycle, room search and memberships | Independent room service |
| `users` | migration target for users and friendships | Independent users service |

## Transitional boundaries

根目录 `main.go` 现在只装配 Gateway 接入、认证、Agent 操作和现有 gRPC 接口，
不再装配 Room/Message HTTP API，也不再启动 Kafka 归档。独立服务通过 Compose 启动。
Root packages `config`, `infrastructure`, `models`
and `repository` are temporary shared dependencies. They should be split only
after service APIs and repositories have been separated, rather than copied
between services.

Cross-service calls must use generated contracts from `contracts`/`proto`.
One service must not import another service's application implementation when
a network boundary is introduced.
