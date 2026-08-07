# 参与贡献

感谢你抽出时间参与贡献！

## 技术栈

- **后端**: Go 1.22+, Gin, WebSocket (gorilla/websocket), GORM, Kafka, Redis
- **前端**: 原生 JavaScript ES Module + Vite
- **基础设施**: Docker Compose, MySQL 8.0, Kafka (KRaft), Qdrant

## 开发环境搭建

```bash
git clone https://github.com/cholode/LAN-sync-communication-go-version.git
cd LAN-sync-communication-go-version

# 复制环境变量模板
cp .env.example .env

# 启动依赖服务
docker compose up -d db redis kafka qdrant

# 运行后端
go run main.go

# 开发前端（另一个终端）
cd frontend && npm install && npm run dev
```

## 提交规范

- 使用语义化提交信息：`feat:`, `fix:`, `docs:`, `test:`, `refactor:`
- 一个 commit 只做一件事
- PR 提交前确保 `go build ./...` 和 `go test ./...` 通过

## 代码规范

- `gofmt` 格式化所有 Go 代码
- 遵循项目现有的目录结构和分层约定
- 新增功能需要对应的单元测试
- 不要在代码中硬编码密钥（使用环境变量）

## 目录结构

```
├── api/            # HTTP 接口处理层
├── agent/          # LLM Agent 智能体
│   ├── llm/        # LLM 客户端
│   ├── rag/        # RAG 检索增强
│   └── tool/       # Agent 工具注册
├── cache/          # Redis 缓存层
├── config/         # 基础设施配置（Redis, Kafka）
├── core/           # WebSocket 核心引擎（Hub + Client）
├── infrastructure/ # 数据库初始化
├── internal/       # 内部组件（Kafka 生产者/消费者）
├── middleware/      # HTTP 中间件（JWT 鉴权）
├── models/         # 数据模型（GORM）
├── pkg/            # 公共工具（JWT, 日志）
├── repository/     # 数据访问层接口与实现
└── frontend/       # 前端 SPA
```