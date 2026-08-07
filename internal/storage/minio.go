package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioProvider MinIO 对象存储
type MinioProvider struct {
	client *minio.Client
	bucket string
}

func NewMinioProvider(endpoint, accessKey, secretKey, bucket string) (*MinioProvider, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("MinIO 客户端创建失败: %w", err)
	}

	// 确保 bucket 存在
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("MinIO bucket 检查失败: %w", err)
	}
	if !exists {
		err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("MinIO bucket 创建失败: %w", err)
		}
	}

	return &MinioProvider{
		client: client,
		bucket: bucket,
	}, nil
}

func (p *MinioProvider) PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	url, err := p.client.PresignedPutObject(ctx, p.bucket, key, ttl)
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败: %w", err)
	}
	return url.String(), nil
}

func (p *MinioProvider) Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error) {
	info, err := p.client.PutObject(ctx, p.bucket, key, reader, size, minio.PutObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("MinIO 上传失败: %w", err)
	}

	return &UploadResult{
		Key:       key,
		PublicURL: fmt.Sprintf("/api/v1/download/%s", key),
		Size:      info.Size,
	}, nil
}

func (p *MinioProvider) GetDownloadURL(ctx context.Context, key string) (string, error) {
	// 生成预签名下载 URL（1小时有效）
	url, err := p.client.PresignedGetObject(ctx, p.bucket, key, 1*time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("生成下载 URL 失败: %w", err)
	}
	return url.String(), nil
}

func (p *MinioProvider) Delete(ctx context.Context, key string) error {
	return p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
}

func (p *MinioProvider) BackendType() Backend {
	return BackendMinIO
}