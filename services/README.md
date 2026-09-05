# Service boundaries

This directory is the deployment boundary of the monorepo. Code may be built
together during the transition, but new business code should be added to its
own service instead of the repository-level `internal` directory.

| Service | Current role | Target deployment |
| --- | --- | --- |
| `gateway` | HTTP/WebSocket ingress, local connection hub, Redis subscription and gRPC ingress | Three or more identical gateway containers |
| `messages` | message API, chat file upload/download, Kafka producer/consumer, persistence cache and search indexing | API plus independently scalable processor/dispatcher/indexer workers |
| `admin` | admin HTTP API and runtime control plane | Independent admin container |
| `agent` | Python FastAPI management plane plus Kafka Agent workers | Independent agent runtime |
| `users` | migration target for users, friendships, rooms and memberships | Independent users service |

## Transitional boundaries

The repository root `main.go` remains the compatibility launcher. It starts
the same components as before so the current backend image and local startup
commands continue to work. Root packages `config`, `infrastructure`, `models`
and `repository` are temporary shared dependencies. They should be split only
after service APIs and repositories have been separated, rather than copied
between services.

Cross-service calls must use generated contracts from `contracts`/`proto`.
One service must not import another service's application implementation when
a network boundary is introduced.
