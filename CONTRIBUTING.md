# 参与贡献

感谢你抽出时间参与贡献！

## 技术栈

- **后端**: Go 1.26+, Gin, WebSocket (gorilla/websocket), GORM, gRPC/protobuf, Kafka, Redis
- **并发**: ANTS taskpool（`shared/concurrency/taskpool`）
- **存储**: MySQL 8.0, MongoDB, Redis, Elasticsearch, Qdrant, MinIO/OSS
- **AI 服务**: Python, LangChain, LangGraph, gRPC
- **前端**: 原生 JavaScript ES Module + Vite
- **基础设施**: Docker Compose, Kafka (KRaft)

## 开发环境搭建

```bash
git clone https://github.com/cholode/LAN-sync-communication-go-version.git
cd LAN-sync-communication-go-version

# 复制环境变量模板
cp .env.example .env

# 启动依赖服务
docker compose up -d db redis kafka qdrant mongo elasticsearch minio

# 运行后端
go run main.go

# 运行 Python Agent 服务（另一个终端）
cd services/agent/runtime
python -m pip install -r requirements.txt
python -m app.main

# 开发前端（另一个终端）
cd frontend && npm install && npm run dev
```

## 提交规范

- 使用语义化提交信息：`feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- 一个 commit 只做一件事
- PR 提交前确保 `go build ./...` 和 `go test ./...` 通过
- 修改 `.proto` 后需要重新生成 protobuf 代码，并在 PR 中说明协议变化

## 代码规范

- `gofmt` 格式化所有 Go 代码
- 遵循项目现有的目录结构和分层约定
- 新增功能需要对应的单元测试
- 不要在代码中硬编码密钥（使用环境变量）
- 文档使用中文，文件统一以 UTF-8 编码保存

## 目录结构

```
├── cache/           # Redis 缓存层
├── config/          # Redis/Kafka 等基础设施初始化
├── contracts/       # 跨服务 Kafka/protobuf 事件契约
├── deploy/          # Nginx 与部署配置
├── docs/            # 项目文档
├── infrastructure/  # 过渡期数据库初始化
├── proto/           # protobuf 定义
├── services/        # 各业务服务，数据模型位于所属服务的 models/
├── shared/          # 跨服务公共能力
│   ├── auth/        # JWT 签发、解析、角色权限及测试
│   ├── concurrency/ # 任务池
│   ├── http/        # HTTP 中间件
│   └── observability/ # logger 日志与 metrics 指标
└── frontend/        # 前端 SPA
```
