# Room Service

Room Service 负责群聊及成员关系的持久化业务，包括创建、搜索、加入、退出、移除成员、查询成员和解散群聊。

Gateway 只维护 WebSocket 连接和本机房间订阅。独立部署时，Room Service 将成员变化发布到 Redis 的 `im:room:events` 频道，每个 Gateway 实例消费事件后更新自己的 Hub。用户重新连接时仍会以数据库中的成员关系为准恢复订阅。

默认监听 `8082`，可通过 `ROOM_SERVER_PORT` 修改。服务使用与主系统相同的 `JWT_SECRET`、`DB_DSN` 和 Redis 配置。

接口均位于 `/api/v1`：

- `POST /rooms`：创建群聊
- `GET /rooms?query=关键词&offset=0&limit=20`：搜索群聊
- `GET /my_rooms`：查询当前用户加入的群聊
- `POST /rooms/:id/join`：加入群聊
- `GET /rooms/:id/members`：查询群成员
- `DELETE /rooms/:id/members/:user_id`：退出或移除成员
- `DELETE /rooms/:id/disband`：解散群聊
