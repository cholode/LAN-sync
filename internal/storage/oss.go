package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OssProvider 阿里云 OSS 对象存储
type OssProvider struct {
	client *oss.Client
	bucket *oss.Bucket
	name   string
}

func NewOssProvider(endpoint, accessKey, secretKey, bucketName string) (*OssProvider, error) {
	client, err := oss.New(endpoint, accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("OSS 客户端创建失败: %w", err)
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("OSS bucket 获取失败: %w", err)
	}

	return &OssProvider{
		client: client,
		bucket: bucket,
		name:   bucketName,
	}, nil
}

func (p *OssProvider) PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	url, err := p.bucket.SignURL(key, oss.HTTPPut, int64(ttl.Seconds()))
	if err != nil {
		return "", fmt.Errorf("OSS 生成预签名 URL 失败: %w", err)
	}
	return url, nil
}

func (p *OssProvider) Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error) {
	_ = size
	err := p.bucket.PutObject(key, reader)
	if err != nil {
		return nil, fmt.Errorf("OSS 上传失败: %w", err)
	}

	return &UploadResult{
		Key:       key,
		PublicURL: fmt.Sprintf("/api/v1/download/%s", key),
		Size:      size,
	}, nil
}

func (p *OssProvider) GetDownloadURL(ctx context.Context, key string) (string, error) {
	url, err := p.bucket.SignURL(key, oss.HTTPGet, 3600)
	if err != nil {
		return "", fmt.Errorf("OSS 生成下载 URL 失败: %w", err)
	}
	return url, nil
}

func (p *OssProvider) Delete(ctx context.Context, key string) error {
	return p.bucket.DeleteObject(key)
}

func (p *OssProvider) BackendType() Backend {
	return BackendOSS
}
