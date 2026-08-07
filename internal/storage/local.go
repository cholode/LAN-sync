package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalProvider 本地磁盘存储
type LocalProvider struct {
	uploadDir string
}

func NewLocalProvider(uploadDir string) *LocalProvider {
	_ = os.MkdirAll(uploadDir, 0755)
	return &LocalProvider{uploadDir: uploadDir}
}

func (p *LocalProvider) PreSignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "", fmt.Errorf("本地存储不支持预签名上传，请使用分片上传接口")
}

func (p *LocalProvider) Save(ctx context.Context, key string, reader io.Reader, size int64) (*UploadResult, error) {
	safeName := filepath.Base(filepath.Clean(key))
	if safeName == "." || safeName == "/" || safeName == "" {
		return nil, fmt.Errorf("非法文件名: %s", key)
	}

	filePath := filepath.Join(p.uploadDir, safeName)
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, reader)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	return &UploadResult{
		Key:       safeName,
		PublicURL: "/api/v1/download/" + safeName,
		Size:      written,
	}, nil
}

func (p *LocalProvider) GetDownloadURL(ctx context.Context, key string) (string, error) {
	safeName := filepath.Base(filepath.Clean(key))
	filePath := filepath.Join(p.uploadDir, safeName)
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("文件不存在: %s", safeName)
	}
	return filePath, nil
}

func (p *LocalProvider) Delete(ctx context.Context, key string) error {
	safeName := filepath.Base(filepath.Clean(key))
	return os.Remove(filepath.Join(p.uploadDir, safeName))
}

func (p *LocalProvider) BackendType() Backend {
	return BackendLocal
}