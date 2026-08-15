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

// Backend 标识对象存储的后端类型
type Backend string

const (
	BackendMinIO Backend = "minio"
	BackendOSS   Backend = "oss"
)

// UploadResult 保存对象后返回的结果
type UploadResult struct {
	Key       string
	PublicURL string
	Size      int64
}

// ObjectStat 描述对象存储中单个对象的元数据。
type ObjectStat struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	Exists       bool
}

// Provider 抽象了对象存储的操作接口
// Stat 和 ListObjects 主要服务于超级管理员后台的文件健康检查。
type Provider interface {
	PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (url string, err error)
	Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error)
	GetDownloadURL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Stat(ctx context.Context, key string) (ObjectStat, error)
	ListObjects(ctx context.Context, prefix string, limit int) ([]ObjectStat, error)
	BackendType() Backend
}

// New 根据环境变量创建对象存储提供者。
// 仅支持 MinIO 和阿里云 OSS，默认使用 MinIO。
// 若配置无效或不完整，会直接 panic 停止启动，不会降级到本地磁盘。
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
		panic(fmt.Sprintf("不支持的 STORAGE_BACKEND: %s（支持: minio, oss）", backend))
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

func storagePanic(message string, err error) {
	pkg.Errorf("%s: %v", message, err)
	panic(fmt.Sprintf("%s: %v", message, err))
}
