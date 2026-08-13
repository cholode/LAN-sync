# File Upload API

## Overview

LAN-IM uses object-storage direct upload, selected by `STORAGE_BACKEND`:

| Mode | Upload | Storage |
|------|--------|---------|
| `minio` | Presigned URL, client uploads directly | MinIO |
| `oss` | Presigned URL, client uploads directly | Aliyun OSS |

Local disk storage is no longer supported. The backend only issues credentials and never proxies file bytes.

---

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
  "upload_url": "http://localhost:9000/lan-im-files/2026-08-13/1/1723000000000_report.pdf?...",
  "object_key": "2026-08-13/1/1723000000000_report.pdf",
  "expires_in": 900
}
```

**Client flow**:

```
1. POST /api/v1/files/presign  ->  receive upload_url
2. HTTP PUT upload_url         ->  upload directly to MinIO / OSS
3. WS message carries download URL -> /api/v1/download/{object_key}
```

---

## 2. Download

### GET /api/v1/download/{object_key}

No authentication required. The server generates a presigned download URL and responds with a 302 redirect to the object store.

---

## Switching Storage Mode

```bash
# .env
STORAGE_BACKEND=minio   # MinIO (default)
STORAGE_BACKEND=oss     # Aliyun OSS

MINIO_ENDPOINT=localhost:9000
MINIO_PUBLIC_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_BUCKET=lan-im-files
```

MinIO console: `http://localhost:9001`
