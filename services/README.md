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
Root packages `config` and `infrastructure` remain temporary shared
dependencies. Repository interfaces and implementations now belong to their
services: `users/repository`, `rooms/repository`, and `messages/repository`.
Message MySQL/MongoDB implementations are no longer part of the HTTP API package.

The root `models` package has also been removed. Models belong to
`users/models`, `rooms/models`, `messages/models`, `agent/models`, and
`admin/models`. Roles and permissions live in `shared/auth`; audit records
remain in `admin/models`. The message document helper remains a separate file
in `messages/models`. Existing model fields, table names, and migration
registrations are preserved, including the currently migration-only RAG chunk
and alert models. Central database migration still lives in `infrastructure`.
Some consumers still import other domains' model types during this transition;
this directory split does not itself establish independent database ownership.

The root `repository` compatibility facade and its global instances have been
removed. Gateway HTTP/gRPC and Admin receive their user/member/message
dependencies explicitly through consumer-owned interfaces. Messages depends on
a membership-check interface rather than the complete room-member repository.

Composition roots (`main.go`, service `cmd`, and Messages `runtime`) currently
wire these interfaces to repositories backed by the existing shared database.
This preserves current deployment behavior; it is not full data isolation.
Future service clients can replace these adapters without changing handlers.

Cross-service calls must use generated contracts from `contracts`/`proto`.
One service must not import another service's application implementation when
a network boundary is introduced.
