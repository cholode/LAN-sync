# Lan IM - 高性能单机 WebSocket 消息分发网关

### 容器通信图
nginx -> backend：HTTP REST + WebSocket，浏览器入口；消息载荷当前为 JSON。
nginx -> admin-service：HTTP JSON，管理后台浏览器入口。
backend -> agent-service：gRPC/protobuf，已有。
agent-service -> backend：gRPC/protobuf IMService，已有。
admin-service -> backend：gRPC/protobuf。
backend -> MySQL：原生 MySQL 协议
backend -> Redis：原生 RESP；im:broadcast:* 消息载荷已使用 protobuf。
backend -> Kafka：原生 Kafka 协议；聊天消息 Value 已使用 protobuf。
backend -> MongoDB：原生 Mongo/BSON 协议
backend -> Elasticsearch：HTTP JSON，ES 原生 API
backend -> MinIO：S3 HTTP 协议
backend -> Qdrant：Qdrant gRPC/protobuf
agent-service -> Redis/Qdrant：原生协议
admin-service -> MySQL/Redis/Mongo/Qdrant/MinIO：基础设施原生协议