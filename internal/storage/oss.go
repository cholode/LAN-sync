package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
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

func (p *OssProvider) Stat(ctx context.Context, key string) (ObjectStat, error) {
	headers, err := p.bucket.GetObjectMeta(key)
	if err != nil {
		return ObjectStat{Key: key}, nil
	}
	size, _ := strconv.ParseInt(headers.Get("Content-Length"), 10, 64)
	lastModified, _ := http.ParseTime(headers.Get("Last-Modified"))
	return ObjectStat{
		Key:          key,
		Size:         size,
		LastModified: lastModified,
		ETag:         headers.Get("ETag"),
		Exists:       true,
	}, nil
}

func (p *OssProvider) ListObjects(ctx context.Context, prefix string, limit int) ([]ObjectStat, error) {
	if limit <= 0 {
		limit = 1000
	}
	result, err := p.bucket.ListObjects(oss.Prefix(prefix), oss.MaxKeys(limit))
	if err != nil {
		return nil, err
	}
	out := make([]ObjectStat, 0, len(result.Objects))
	for _, obj := range result.Objects {
		out = append(out, ObjectStat{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Exists:       true,
		})
	}
	return out, nil
}

func (p *OssProvider) BackendType() Backend {
	return BackendOSS
}
