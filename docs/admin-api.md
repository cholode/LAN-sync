# Admin 治理接口

Admin 接口已经合并进统一的 [接口测试手册](./API.md#9-admin-治理接口)，以该文档和当前路由源码为准。

当前 Admin 只负责：

- 用户封禁、解封和后台角色调整
- 群聊管理、解散和移除成员
- 文件审核、异常扫描与删除
- 内容治理和违规记录
- Agent 配置与 RAG 查询记录
- Tool Call、审计日志和权限控制

WebSocket 连接、系统运行、健康聚合和告警接口已从 Admin 删除，统一通过 Prometheus 和 Grafana 查看。不要再调用旧的 `/api/v1/admin/dashboard/runtime`、`/health`、`/connections`、`/errors` 或 `/alerts`。
