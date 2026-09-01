package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioProvider MinIO 对象存储
//
// 维护两个客户端：
//   - client   : 连内部地址（Docker DNS），用于 Save / Delete 等数据操作
//   - urlClient: 连公网地址，用于生成浏览器可访问的预签名 URL
//     urlClient 使用自定义 Transport 将对公网地址的网络请求转发到内部地址，
//     但签名中的 Host 仍为公网地址。
type MinioProvider struct {
	client    *minio.Client
	urlClient *minio.Client
	bucket    string
}

func NewMinioProvider(endpoint, accessKey, secretKey, bucket string) (*MinioProvider, error) {
	// 内部客户端
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

	// 预签名 URL 客户端：使用宿主机地址，但网络请求转发到内部地址
	publicEndpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT")
	var urlClient *minio.Client
	if publicEndpoint != "" && publicEndpoint != endpoint {
		// 自定义 Transport：签名按 publicEndpoint 计算，但实际 TCP 连接走内部地址
		transport := &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				// 将对 publicEndpoint 的连接重定向到内部 MinIO 地址
				if addr == publicEndpoint {
					addr = endpoint
				}
				dialer := &net.Dialer{Timeout: 10 * time.Second}
				return dialer.DialContext(dialCtx, network, addr)
			},
		}
		urlClient, err = minio.New(publicEndpoint, &minio.Options{
			Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure:    false,
			Transport: transport,
		})
		if err != nil {
			urlClient = client // 降级
		}
	} else {
		urlClient = client
	}

	return &MinioProvider{
		client:    client,
		urlClient: urlClient,
		bucket:    bucket,
	}, nil
}

func (p *MinioProvider) PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	url, err := p.urlClient.PresignedPutObject(ctx, p.bucket, key, ttl)
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
	url, err := p.urlClient.PresignedGetObject(ctx, p.bucket, key, 1*time.Hour, nil)
	if err != nil {
		return "", fmt.Errorf("生成下载 URL 失败: %w", err)
	}
	return url.String(), nil
}

func (p *MinioProvider) Delete(ctx context.Context, key string) error {
	return p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
}

func (p *MinioProvider) Stat(ctx context.Context, key string) (ObjectStat, error) {
	info, err := p.client.StatObject(ctx, p.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return ObjectStat{Key: key}, nil
		}
		return ObjectStat{Key: key}, err
	}
	return ObjectStat{
		Key:          key,
		Size:         info.Size,
		LastModified: info.LastModified,
		ETag:         info.ETag,
		Exists:       true,
	}, nil
}

func (p *MinioProvider) ListObjects(ctx context.Context, prefix string, limit int) ([]ObjectStat, error) {
	if limit <= 0 {
		limit = 1000
	}
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := p.client.ListObjects(listCtx, p.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	out := make([]ObjectStat, 0)
	var listErr error
	for obj := range ch {
		if obj.Err != nil {
			listErr = obj.Err
			cancel()
			break
		}
		out = append(out, ObjectStat{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
			Exists:       true,
		})
		if len(out) >= limit {
			// Stop the producer as soon as the requested page is full. This is
			// important for health checks, which intentionally request one item.
			cancel()
			break
		}
	}
	// minio-go requires the channel to be drained after cancellation. Otherwise
	// its listObjectsV2 producer can remain blocked and leak a goroutine.
	if listErr != nil || len(out) >= limit {
		for range ch {
		}
	}
	return out, listErr
}

func (p *MinioProvider) BackendType() Backend {
	return BackendMinIO
}
