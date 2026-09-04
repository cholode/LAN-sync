# LAN-IM HTTP API

## 用户登录

### POST /api/v1/login

使用用户名和密码登录，成功后返回 JWT。该接口不需要身份认证。

**Content-Type**：`application/json`

**请求体**：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `username` | string | 是 | 登录用户名 |
| `password` | string | 是 | 用户密码 |

```json
{
  "username": "alice",
  "password": "example-password"
}
```

**成功响应（200）**：

```json
{
  "msg": "登录成功",
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {
    "id": 10001,
    "username": "alice",
    "role": 0,
    "avatar": ""
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `msg` | string | 结果说明 |
| `token` | string | JWT Access Token，当前有效期为 24 小时 |
| `user.id` | integer | 用户唯一 ID |
| `user.username` | string | 用户名 |
| `user.role` | integer | `0` 普通用户，`1` 超级管理员 |
| `user.avatar` | string | 头像 URL，未设置时为空字符串 |

后续 HTTP 请求应通过标准请求头携带 Token：

```http
Authorization: Bearer <token>
```

**错误响应**：

所有错误响应都使用统一结构：

```json
{
  "error": "错误说明"
}
```

| HTTP 状态码 | 场景 | `error` 示例 | 前端建议 |
|---|---|---|---|
| `400` | 请求体不是合法 JSON，或缺少用户名、密码 | `请求参数不合法` | 提示用户检查输入 |
| `401` | 用户不存在或密码错误 | `账号或密码错误` | 保持统一提示，不区分账号是否存在 |
| `429` | IP 或 IP/用户名组合超过登录频率限制 | `登录尝试过于频繁，请稍后再试` | 读取 `Retry-After` 后再允许提交 |
| `503` | bcrypt 校验并发已满 | `登录请求繁忙，请稍后重试` | 短暂等待后重试 |
| `504` | 登录处理超过服务端超时时间 | `登录处理超时，请稍后重试` | 提示网络或服务繁忙，可重试 |
| `500` | JWT 签发失败 | `令牌生成失败` | 提示服务异常，不要自动连续重试 |

触发 `429` 时响应包含：

```http
Retry-After: 60
```

触发 `503` 时响应包含：

```http
Retry-After: 1
```

前端登录成功后应保存 `token` 和 `user`，然后根据角色跳转：普通用户进入 `/chat`，超级管理员进入 `/admin/dashboard`。遇到受保护接口返回 `401` 时，应清理本地登录状态并跳转到 `/login`。

## File Upload API

## Overview

LAN-IM uses object-storage direct upload, selected by `STORAGE_BACKEND`:

| Mode | Upload | Storage |
|------|--------|---------|
| `minio` | Presigned URL, client uploads directly | MinIO |
| `oss` | Presigned URL, client uploads directly | Aliyun OSS |

Local disk storage is no longer supported. The backend only issues presigned URLs and never proxies file bytes.

## 1. Presigned Upload

### POST /api/v1/files/presign

**Auth**: JWT Bearer Token

**Request body**:

```json
{ "filename": "report.pdf", "file_type": "pdf", "file_size": 1048576 }
```

**Success response (200)**:

```json
{
  "upload_url": "http://your-public-host:9000/lan-im-files/2026-08-17/1/1755400000000_report.pdf?...",
  "object_key": "2026-08-17/1/1755400000000_report.pdf",
  "expires_in": 900
}
```

`upload_url` 的实际域名由 `.env` 中的 `MINIO_PUBLIC_ENDPOINT` 决定，必须是浏览器可达的地址，不能填 `localhost` 或 `127.0.0.1`。

## 2. Complete Upload

### POST /api/v1/files/complete

**Auth**: JWT Bearer Token

直传成功后调用该接口，后端将对象 Key 转成可供客户端消息使用的下载路径。

**Request body**:

```json
{ "object_key": "2026-08-17/1/1755400000000_report.pdf" }
```

**Success response (200)**:

```json
{
  "download_url": "/api/v1/download/2026-08-17/1/1755400000000_report.pdf"
}
```

**Client flow**:

```text
1. POST /api/v1/files/presign  ->  receive upload_url
2. HTTP PUT upload_url         ->  upload directly to MinIO / OSS
3. POST /api/v1/files/complete ->  receive download_url
4. WS message carries download_url
```

## 3. Download

### GET /api/v1/download/{object_key}

No authentication required. The server generates a presigned download URL and responds with a `302` redirect to the object store.

## Switching Storage Mode

```bash
# .env
STORAGE_BACKEND=minio   # MinIO (default)
STORAGE_BACKEND=oss     # Aliyun OSS

MINIO_ENDPOINT=minio:9000
MINIO_PUBLIC_ENDPOINT=http://your-public-host:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=lan-im-files
```

MinIO console: `http://your-public-host:9001`，生产环境应仅对办公 IP 或 SSH 隧道开放。
