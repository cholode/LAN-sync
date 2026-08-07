# 安全策略

## 支持的版本

安全修复仅针对 **main 分支的最新提交**。请在报告前确认可在最新代码上复现。

## 报告漏洞

**请勿通过公开 Issue 报告安全漏洞。**

如果发现安全漏洞，请通过 GitHub 的 [Private Vulnerability Reporting](https://github.com/cholode/LAN-sync-communication-go-version/security/advisories/new) 私下报告。

我们会尽快确认并在修复后发布安全公告。

## 安全最佳实践

- **JWT 密钥**: 生产环境必须通过环境变量 `JWT_SECRET` 设置强随机密钥
- **密码存储**: 所有用户密码使用 bcrypt 哈希存储
- **API Key**: LLM / Embedding 服务的 API Key 仅通过环境变量注入，不硬编码
- **数据库密码**: 通过 Docker Compose 的 `${DB_PASSWORD}` 变量管理
- **容器安全**: 后端以非 root 用户 `imuser` 运行