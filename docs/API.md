# File Upload API

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
