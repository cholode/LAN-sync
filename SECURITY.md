# 安全策略

## 支持的版本

安全修复仅针对 **main 分支的最新提交**。请在报告前确认可在最新代码上复现。

## 报告漏洞

**请勿通过公开 Issue 报告安全漏洞。**

如果发现安全漏洞，请通过 GitHub 的 [Private Vulnerability Reporting](https://github.com/cholode/LAN-sync-communication-go-version/security/advisories/new) 私下报告。

我们会尽快确认并在修复后发布安全公告。

## 当前架构中的安全边界

- **公网只暴露 Nginx**：`80`/`443` 是浏览器入口，MinIO 直传仅在确实需要公网预签名上传时开放 `9000`。
- **内部端口禁止公网**：`3306`、`6379`、`9092`、`9200`、`27017`、`6333`、`6334`、`8000`、`8080`、`8081`、`6060`、`50052`、`50053`、`9001` 应由安全组限制为内网或办公 IP。
- **管理端二次鉴权**：`admin-service` 的 JWT 校验之外，还经过 `RequireAdmin`、权限点和 `AdminRateLimit` 限流。
- **控制面 Token**：backend 的 `AdminControlService` gRPC 使用 `ADMIN_CONTROL_TOKEN` 做服务间认证。
- **对象存储**：文件上传/下载使用后端签发的预签名 URL；`MINIO_SECRET_KEY` 和 OSS 密钥只能保存在 `.env`，不得进入前端或 Git。
- **Agent 服务**：Python `agent-service` 与 backend 之间通过 gRPC 通信，部署时应保持该端口只在内部网络可达。

## 安全最佳实践

- **JWT 密钥**: 生产环境必须通过环境变量 `JWT_SECRET` 设置强随机密钥
- **密码存储**: 所有用户密码使用 bcrypt 哈希存储
- **API Key**: LLM / Embedding 服务的 API Key 仅通过环境变量注入，不硬编码
- **数据库密码**: 通过 Docker Compose 的 `${DB_PASSWORD}` 变量管理
- **MinIO/OSS 密钥**: 使用 `MINIO_ACCESS_KEY`、`MINIO_SECRET_KEY` 或 OSS 对应变量注入
- **容器安全**: 后端以非 root 用户 `imuser` 运行
- **限流**: 管理接口默认约 10 QPS、突发 30；WebSocket 侧还应对新建连接和广播速率设置保护
