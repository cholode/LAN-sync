package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"lan-im-go/pkg"
)

// Backend identifies the object storage backend.
type Backend string

const (
	BackendMinIO Backend = "minio"
	BackendOSS   Backend = "oss"
)

// UploadResult is returned after an object is saved.
type UploadResult struct {
	Key       string
	PublicURL string
	Size      int64
}

// Provider abstracts object storage operations.
type Provider interface {
	PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (url string, err error)
	Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error)
	GetDownloadURL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	BackendType() Backend
}

// New creates an object storage provider from environment variables.
// Only MinIO and Aliyun OSS are supported; MinIO is the default.
// Invalid or incomplete configuration stops startup instead of falling back to local disk.
func New() Provider {
	backend := Backend(strings.TrimSpace(os.Getenv("STORAGE_BACKEND")))
	if backend == "" {
		backend = BackendMinIO
	}

	switch backend {
	case BackendOSS:
		return newOSSProviderFromEnv()
	case BackendMinIO:
		return newMinioProviderFromEnv()
	default:
		panic(fmt.Sprintf("unsupported STORAGE_BACKEND: %s (supported: minio, oss)", backend))
	}
}

func newOSSProviderFromEnv() Provider {
	endpoint := os.Getenv("OSS_ENDPOINT")
	accessKey := os.Getenv("OSS_ACCESS_KEY")
	secretKey := os.Getenv("OSS_SECRET_KEY")
	bucket := os.Getenv("OSS_BUCKET")
	if bucket == "" {
		bucket = "lan-im-files"
	}
	if endpoint == "" || accessKey == "" {
		panic("[Storage] OSS 配置不完整")
	}

	provider, err := NewOssProvider(endpoint, accessKey, secretKey, bucket)
	if err != nil {
		storagePanic("[Storage] OSS 初始化失败", err)
	}
	pkg.Infof("[Storage] 使用 OSS 对象存储, endpoint=%s, bucket=%s", endpoint, bucket)
	return provider
}

func newMinioProviderFromEnv() Provider {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")
	if bucket == "" {
		bucket = "lan-im-files"
	}
	if endpoint == "" || accessKey == "" {
		panic("[Storage] MinIO 配置不完整")
	}

	provider, err := NewMinioProvider(endpoint, accessKey, secretKey, bucket)
	if err != nil {
		storagePanic("[Storage] MinIO 初始化失败", err)
	}
	pkg.Infof("[Storage] 使用 MinIO 对象存储, endpoint=%s, bucket=%s", endpoint, bucket)
	return provider
}

func storagePanic(format string, args ...interface{}) {
	pkg.Errorf(format, args...)
	panic(fmt.Sprintf(format, args...))
}
