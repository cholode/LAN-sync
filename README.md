# Lan IM - 高性能 WebSocket 即时通讯系统

## 当前架构概览

- 接入层：Nginx 提供 HTTP/HTTPS 与 WebSocket 反向代理。
- 业务层：Go `backend` 提供 REST API、WebSocket Hub、`IMService` gRPC 与 `AdminControlService` gRPC。
- 管理端：`admin-service` 提供超级管理员后台 REST API。
- AI 层：Python `agent-service` 基于 LangChain + LangGraph，通过 gRPC 对外服务。
- 存储层：MySQL/MongoDB 存业务与消息，Redis 做在线状态/缓存/Pub-Sub，Elasticsearch 做消息检索，Qdrant 做向量检索，MinIO/OSS 做对象存储。

### 容器通信图
nginx -> backend：HTTP REST + WebSocket，聊天前端入口；浏览器 WebSocket 消息载荷为 JSON。
nginx -> admin-service：HTTP JSON，管理后台浏览器入口。
backend -> agent-service：gRPC/protobuf，调用 Python 智能体服务。
agent-service -> backend：gRPC/protobuf，调用 IMService 的 get_messages 等工具。
admin-service -> backend：gRPC/protobuf，调用 AdminControlService 执行控制指令。
backend -> MySQL：原生 MySQL 协议
backend -> Redis：原生 RESP；im:broadcast:* 消息载荷已使用 protobuf。
backend -> Kafka：原生 Kafka 协议；聊天消息 Value 已使用 protobuf。
backend -> MongoDB：原生 Mongo/BSON 协议
backend -> Elasticsearch：HTTP JSON，ES 原生 API
backend -> MinIO/OSS：S3 兼容 HTTP 协议
backend -> Qdrant：Qdrant gRPC/protobuf
agent-service -> Redis/Qdrant：原生协议
admin-service -> MySQL/Redis/Mongo/Qdrant/MinIO：基础设施原生协议

## 压测基线

最新结果来自 `perf3b/` 与 `perf4/`，结论均以实测数据为准。

### HTTP 只读接口（N+1 修复后，perf3b）

| 场景 | VU | RPS | P50 | P95 | P99 | 错误率 |
| --- | --- | --- | --- | --- | --- | --- |
| my_rooms/messages/members 慢爬坡 | 100 | 460.60 | 189.49ms | 421.95ms | 506.78ms | 0% |

### WebSocket（Hub 64 分片后，perf4）

建连爬坡：

| 目标 VU | 服务端峰值在线 | 建连 P50 | 建连 P95 | 建连最大 |
| --- | --- | --- | --- | --- |
| 100 | 100 | 80.72ms | 88.12ms | 113.03ms |
| 500 | 500 | 80.33ms | 88.46ms | 105.99ms |
| 1000 | 938 | 80.64ms | 1.04s | 21.06s |

群聊广播：

| 房间规模 | 服务端峰值在线 | E2E 平均 | E2E P95 | 备注 |
| --- | --- | --- | --- | --- |
| 500 人 | 500 | 70.15ms | 99ms | Kafka lag 0，任务池等待 0 |
| 1000 人 | 949 | 54.60ms | 71ms | 任务池运行峰值 256，等待 0 |

说明：压测使用预生成 JWT，避免登录 bcrypt 占满 CPU；完整数据见 `perf3b/HTTP慢爬坡报告.md`、`perf4/WebSocket分片压测报告.md`、`perf4/WebSocket分片压测分析.md`。

## 服务器部署

本文档以单台 Ubuntu 22.04 / 24.04 云服务器为例。项目已提供 `docker-compose.yml`，部署时只需要准备服务器环境、修改 `.env`、构建前端、启动容器。

### 1. 环境要求

- Docker Engine 20+ 与 Docker Compose v2
- Node.js 20+（仅用于构建前端 `dist`）
- Git
- 推荐配置：最低 `4C8G`，压测建议 `8C16G`

安装 Docker：

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
```

安装 Node.js：

```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
```

### 2. 拉取项目

```bash
mkdir -p ~/apps
cd ~/apps
git clone <你的仓库地址> lan-im
cd lan-im
```

### 3. 配置环境变量

```bash
cp .env.example .env
sed -i 's/\r$//' .env
vim .env
```

重点修改以下变量：

| 变量 | 说明 |
| --- | --- |
| `DB_PASSWORD` | MySQL root 密码，必须改成强密码 |
| `JWT_SECRET` | JWT 签名密钥，建议使用随机长字符串 |
| `ADMIN_CONTROL_TOKEN` | 管理端控制面 Token，必须修改 |
| `MINIO_ACCESS_KEY` | MinIO 访问账号 |
| `MINIO_SECRET_KEY` | MinIO 访问密钥，必须修改 |
| `MINIO_PUBLIC_ENDPOINT` | 浏览器可访问的 MinIO 地址，例如 `http://你的公网IP:9000` |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | LLM 对话模型配置 |
| `EMBED_BASE_URL` / `EMBED_API_KEY` / `EMBED_MODEL` | Embedding 模型配置 |
| `MESSAGE_STORE` | 消息存储后端，`mysql` 或 `mongo` |
| `MONGO_URI` | MongoDB 地址，仅 `MESSAGE_STORE=mongo` 时使用 |
| `ES_ADDR` | Elasticsearch 地址 |
| `NODE_ID` | 当前节点标识，单机可保持默认值 |
| `STORAGE_BACKEND` | 对象存储类型，`minio` 或 `oss` |
| `AGENT_GRPC_ADDR` | Python Agent 服务 gRPC 地址 |

注意：`MINIO_PUBLIC_ENDPOINT` 不能填 `localhost` 或 `127.0.0.1`，否则用户浏览器无法直接上传文件。

### 4. 构建前端

`docker-compose.yml` 中 Nginx 直接挂载宿主机 `frontend/dist`，因此启动前必须先构建前端。

```bash
cd frontend
npm ci
npm run build
cd ..
```

### 5. 启动服务

```bash
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

查看日志：

```bash
docker compose logs -f backend
docker compose logs -f admin-service
docker compose logs -f nginx
docker compose logs -f agent-service
```

### 6. 创建账号并设置超管

先访问 `http://你的公网IP/register` 注册一个普通账号，然后进入 MySQL 提升为超级管理员：

```bash
docker compose exec db mysql -uroot -p lan_im
```

```sql
UPDATE users SET role = 1 WHERE username = '你的用户名';
```

重新登录后访问 `http://你的公网IP/admin/dashboard` 即可进入管理后台。

### 7. 云服务器安全组

公网建议只开放：

- `80`：HTTP 入口
- `443`：HTTPS 入口
- `9000`：MinIO 文件直传，仅在使用预签名 URL 直传时需要

以下端口不要对公网开放：

```text
3306 6379 9092 9200 27017 6333 6334 8080 8081 6060 50051 50052 50053 9001
```

MinIO 控制台 `9001` 建议只允许办公 IP 或通过 SSH 隧道访问。

### 8. HTTPS 说明

当前 Nginx 默认只启用 HTTP，HTTPS 配置位于 `deploy/nginx/nginx.conf` 的注释块中。生产环境建议：

1. 将域名解析到服务器公网 IP。
2. 使用 Certbot 申请证书。
3. 将证书挂载到 Nginx 容器。
4. 开启 `443 ssl` 配置。

### 9. 升级部署

```bash
cd ~/apps/lan-im
git pull

cd frontend
npm ci
npm run build
cd ..

docker compose up -d --build
```

### 10. 数据备份

主要数据保存在 Docker named volume 中，可先查看：

```bash
docker volume ls
```

建议定期备份 MySQL、MongoDB、MinIO、Qdrant、Elasticsearch 对应的数据卷，并保存 `.env` 文件。
