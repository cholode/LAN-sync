package messages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"lan-im-go/pkg"
	messageapp "lan-im-go/services/messages/application"
)

type preSignUploadRequest struct {
	FileName string `json:"filename" binding:"required"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
}

func (m *Module) PreSignUpload(c *gin.Context) {
	var req preSignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不合法"})
		return
	}
	now := time.Now().UTC()
	userID := c.GetInt64("user_id")
	safeName := path.Base(strings.TrimSpace(req.FileName))
	if safeName == "" || safeName == "." || safeName == "/" {
		safeName = "file"
	}
	objectKey := fmt.Sprintf("%s/%d/%d_%s", now.Format("2006-01-02"), userID, now.UnixMilli(), safeName)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	uploadURL, err := m.Storage.PreSignedUploadURL(ctx, objectKey, 15*time.Minute)
	if err != nil {
		pkg.Errorf("[消息文件] 生成上传链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成上传链接失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"upload_url": uploadURL, "object_key": objectKey, "expires_in": 900})
}

func (m *Module) CompleteUpload(c *gin.Context) {
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
	record, err := m.Files.CompleteUpload(c.Request.Context(), c.GetInt64("user_id"), messageapp.CompleteUploadRequest{
		ObjectKey: req.ObjectKey, OriginalName: req.OriginalName, SHA256: req.SHA256, FileSize: req.FileSize, RoomID: req.RoomID,
	})
	if err != nil {
		writeFileError(c, err, "保存文件记录失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": record.ID, "object_key": record.ObjectKey,
		"download_url": fmt.Sprintf("/api/v1/files/%d/download", record.ID),
	})
}

func (m *Module) DownloadFileByID(c *gin.Context) {
	fileID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || fileID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件编号非法"})
		return
	}
	downloadURL, err := m.Files.DownloadURLByID(c.Request.Context(), c.GetInt64("user_id"), fileID)
	if err != nil {
		writeFileError(c, err, "生成下载链接失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": downloadURL})
}

// DownloadFileByObjectKey 为历史消息保留旧地址，但现在同样必须登录并通过群成员校验。
func (m *Module) DownloadFileByObjectKey(c *gin.Context) {
	encoded := strings.TrimPrefix(c.Param("filepath"), "/")
	raw, err := url.PathUnescape(encoded)
	if err != nil {
		raw = encoded
	}
	downloadURL, err := m.Files.DownloadURLByObjectKey(c.Request.Context(), c.GetInt64("user_id"), raw)
	if err != nil {
		writeFileError(c, err, "生成下载链接失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": downloadURL})
}

func writeFileError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, messageapp.ErrInvalidFileKey):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, messageapp.ErrFileForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, messageapp.ErrFileNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		pkg.Errorf("[消息文件] %s: %v", fallback, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}
