package files

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
	"lan-im-go/pkg"
)

// PreSignUploadRequest is the presigned upload request body.
type PreSignUploadRequest struct {
	FileName string `json:"filename" binding:"required"`
	FileType string `json:"file_type"` // e.g. png, jpg, pdf
	FileSize int64  `json:"file_size"`
}

// PreSignUploadHandler generates a presigned upload URL.
// POST /api/v1/files/presign
func (m *Module) PreSignUploadHandler(c *gin.Context) {
	var req PreSignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不合法"})
		return
	}

	now := time.Now()
	userID := c.GetInt64("user_id")

	// Object key: {date}/{userID}/{timestamp}_{safeFilename}
	// path.Base prevents path traversal and keeps S3/OSS slash semantics.
	safeName := path.Base(strings.TrimSpace(req.FileName))
	if safeName == "." || safeName == "/" || safeName == "" {
		safeName = "file"
	}
	key := fmt.Sprintf("%s/%d/%d_%s",
		now.Format("2006-01-02"),
		userID,
		now.UnixMilli(),
		safeName,
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	uploadURL, err := m.Storage.PreSignedUploadURL(ctx, key, 15*time.Minute)
	if err != nil {
		pkg.Errorf("[PreSign] generate presigned URL failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成上传链接失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url": uploadURL,
		"object_key": key,
		"expires_in": 900,
	})
}

// CompleteUploadHandler 记录客户端直传完成后的文件元数据，供超级管理员后台管理。
// POST /api/v1/files/complete
func (m *Module) CompleteUploadHandler(c *gin.Context) {
	var req struct {
		ObjectKey    string `json:"object_key" binding:"required"`
		OriginalName string `json:"original_name" binding:"required"`
		SHA256       string `json:"sha256"`
		FileSize     int64  `json:"file_size"`
		RoomID       int64  `json:"room_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不合法"})
		return
	}
	if m.AdminFiles == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	userID := c.GetInt64("user_id")
	record, err := m.AdminFiles.RecordUpload(c.Request.Context(), userID, adminservice.CompleteUploadRequest{
		ObjectKey:    req.ObjectKey,
		OriginalName: req.OriginalName,
		SHA256:       req.SHA256,
		FileSize:     req.FileSize,
		RoomID:       req.RoomID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": record.ID, "object_key": record.ObjectKey})
}

func downloadSegmentFromRequest(c *gin.Context) string {
	requestPath := c.Request.URL.Path
	marker := "/download/"
	idx := strings.LastIndex(requestPath, marker)
	var enc string
	if idx >= 0 {
		enc = requestPath[idx+len(marker):]
	}
	if enc == "" {
		enc = strings.TrimPrefix(strings.TrimSpace(c.Param("filepath")), "/")
	}
	if enc == "" {
		return ""
	}
	raw, err := url.PathUnescape(enc)
	if err != nil {
		raw = enc
	}
	return raw
}

// DownloadFile generates a presigned object-storage URL and redirects.
// GET /api/v1/download/{object_key}
func (m *Module) DownloadFile(c *gin.Context) {
	raw := downloadSegmentFromRequest(c)
	if raw == "" || raw == "." || raw == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件请求非法"})
		return
	}

	objectKey := strings.TrimPrefix(path.Clean(raw), "/")
	if objectKey == "." || objectKey == "/" || objectKey == "" || strings.HasPrefix(objectKey, "../") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件请求非法"})
		return
	}

	if m.Storage == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	downloadURL, err := m.Storage.GetDownloadURL(ctx, objectKey)
	if err != nil {
		pkg.Infof("[download] generate download URL failed key=%s: %v", objectKey, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, downloadURL)
}
