package storage

import (
	"context"
	"io"
	"os"
	"lan-im-go/pkg"
	"time"
)

// Backend 存储后端类型
type Backend string

const (
	BackendLocal Backend = "local"
	BackendMinIO Backend = "minio"
)

// UploadResult 上传结果
type UploadResult struct {
	Key       string // 对象键（MinIO objectKey 或本地文件名）
	PublicURL string // 可公开访问的 URL
	Size      int64  // 文件大小（字节）
}

// Provider 存储抽象接口
type Provider interface {
	// PreSignedUploadURL 生成预签名上传 URL（MinIO 场景）
	// 客户端拿到 URL 后直传，不经过 IM 服务器
	PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (url string, err error)

	// Save 保存文件内容（本地存储场景）
	Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error)

	// GetDownloadURL 获取下载 URL 或文件路径
	GetDownloadURL(ctx context.Context, key string) (string, error)

	// Delete 删除文件
	Delete(ctx context.Context, key string) error

	// Backend 返回当前后端类型
	BackendType() Backend
}

// New 根据配置创建存储实例
func New() Provider {
	backend := Backend(os.Getenv("STORAGE_BACKEND"))
	if backend == "" {
		backend = BackendLocal
	}

	switch backend {
	case BackendMinIO:
		endpoint := os.Getenv("MINIO_ENDPOINT")
		accessKey := os.Getenv("MINIO_ACCESS_KEY")
		secretKey := os.Getenv("MINIO_SECRET_KEY")
		bucket := os.Getenv("MINIO_BUCKET")
		if bucket == "" {
			bucket = "lan-im-files"
		}

		if endpoint != "" && accessKey != "" {
			provider, err := NewMinioProvider(endpoint, accessKey, secretKey, bucket)
			if err != nil {
				pkg.Errorf("[Storage] MinIO 初始化失败，降级为本地存储: %v", err)
			} else {
				pkg.Infof("[Storage] 使用 MinIO 对象存储, endpoint=%s, bucket=%s", endpoint, bucket)
				return provider
			}
		}
		pkg.Infoln("[Storage] MinIO 配置不完整，降级为本地存储")

	case BackendLocal:
		fallthrough
	default:
		dir := os.Getenv("LAN_IM_UPLOAD_DIR")
		if dir == "" {
			dir = "./data/uploads"
		}
		pkg.Infof("[Storage] 使用本地磁盘存储, dir=%s", dir)
		return NewLocalProvider(dir)
	}

	// 兜底
	dir := os.Getenv("LAN_IM_UPLOAD_DIR")
	if dir == "" {
		dir = "./data/uploads"
	}
	return NewLocalProvider(dir)
}