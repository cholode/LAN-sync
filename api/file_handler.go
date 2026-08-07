package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/unicode/norm"
	"lan-im-go/internal/storage"
	"lan-im-go/pkg"
)

// Storage 全局存储实例，由 main.go 初始化
var Storage storage.Provider

var (
	UploadBaseDir string
	TempChunkDir  string
)

// InitFileStorage 初始化文件存储（替代原 InitFileDirs）
func InitFileStorage() {
	Storage = storage.New()

	if Storage.BackendType() == storage.BackendLocal {
		// 本地存储需要初始化目录（与原来逻辑一致）
		if root := strings.TrimSpace(os.Getenv("LAN_IM_DATA_DIR")); root != "" {
			root = filepath.Clean(root)
			UploadBaseDir = filepath.Join(root, "uploads")
			TempChunkDir = filepath.Join(root, "temp_chunks")
			pkg.Infof("[目录] LAN_IM_DATA_DIR=%s -> uploads=%s temp_chunks=%s", root, UploadBaseDir, TempChunkDir)
		}
		if UploadBaseDir == "" {
			UploadBaseDir = filepath.Join(".", "data", "uploads")
		}
		if TempChunkDir == "" {
			TempChunkDir = filepath.Join(".", "data", "temp_chunks")
		}
		UploadBaseDir = ensureWritableDir(UploadBaseDir, "uploads")
		TempChunkDir = ensureWritableDir(TempChunkDir, "temp_chunks")
		if abs, err := filepath.Abs(UploadBaseDir); err == nil {
			UploadBaseDir = abs
		}
		if abs, err := filepath.Abs(TempChunkDir); err == nil {
			TempChunkDir = abs
		}
		pkg.Infof("[目录] 最终 UploadBaseDir=%s TempChunkDir=%s", UploadBaseDir, TempChunkDir)
	}
}

// ---------- 原 InitFileDirs 保留兼容 ----------
func InitFileDirs() {
	InitFileStorage()
}

// ---------- 目录工具函数 ----------

func isDirWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	f, err := os.CreateTemp(dir, ".lan-im-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func ensureWritableDir(primary string, fallbackParts ...string) string {
	_ = os.MkdirAll(primary, 0755)
	if isDirWritable(primary) {
		return primary
	}
	pkg.Infof("[目录] primary 不可写或创建失败，尝试回退: %s", primary)

	fallback := filepath.Join(append([]string{os.TempDir(), "lan-im-go"}, fallbackParts...)...)
	if err := os.MkdirAll(fallback, 0755); err == nil && isDirWritable(fallback) {
		pkg.Infof("[目录回退] 使用系统临时目录: %s", fallback)
		return fallback
	}

	if home, err := os.UserHomeDir(); err == nil {
		homeFallback := filepath.Join(append([]string{home, ".lan-im-go"}, fallbackParts...)...)
		if err := os.MkdirAll(homeFallback, 0755); err == nil && isDirWritable(homeFallback) {
			pkg.Infof("[目录回退] 使用用户主目录: %s", homeFallback)
			return homeFallback
		}
	}

	pkg.Infof("[目录错误] 所有候选路径均不可写，仍使用: %s", fallback)
	_ = os.MkdirAll(fallback, 0755)
	return fallback
}

func sanitizeHash(raw string) (string, error) {
	safeHash := filepath.Clean(raw)
	if safeHash == "." || safeHash == "/" || safeHash == "" || strings.Contains(safeHash, `\`) {
		return "", fmt.Errorf("参数非法")
	}
	return safeHash, nil
}

// ---------- 预签名上传（MinIO 场景） ----------

// PreSignUploadRequest 预签名上传请求
type PreSignUploadRequest struct {
	FileName string `json:"filename" binding:"required"`
	FileType string `json:"file_type"` // 如 "png", "jpg", "pdf"
	FileSize int64  `json:"file_size"`
}

// PreSignUploadHandler 生成预签名上传 URL
// POST /api/v1/files/presign
// 客户端拿到 URL 直传 MinIO，不经过服务器中转
func PreSignUploadHandler(c *gin.Context) {
	var req PreSignUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数不合法"})
		return
	}

	now := time.Now()
	userID := c.GetInt64("user_id")

	// 构造对象键：{date}/{userID}/{timestamp}_{filename}
	key := fmt.Sprintf("%s/%d/%d_%s",
		now.Format("2006-01-02"),
		userID,
		now.UnixMilli(),
		req.FileName,
	)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	uploadURL, err := Storage.PreSignedUploadURL(ctx, key, 15*time.Minute)
	if err != nil {
		pkg.Errorf("[PreSign] 生成预签名 URL 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成上传链接失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_url": uploadURL,
		"object_key": key,
		"expires_in": 900,
	})
}

// ---------- 下载（兼容本地 + MinIO） ----------

func downloadSegmentFromRequest(c *gin.Context) string {
	path := c.Request.URL.Path
	marker := "/download/"
	idx := strings.LastIndex(path, marker)
	var enc string
	if idx >= 0 {
		enc = path[idx+len(marker):]
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

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func findUploadedObject(uploadDir, logicalName string) (string, bool) {
	p := filepath.Join(uploadDir, logicalName)
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p, true
	}
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return "", false
	}
	wantLow := strings.ToLower(logicalName)
	wantNFC := norm.NFC.String(logicalName)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.ToLower(n) == wantLow || norm.NFC.String(n) == wantNFC {
			return filepath.Join(uploadDir, n), true
		}
	}
	if len(logicalName) >= 65 && logicalName[64] == '_' {
		hash := logicalName[:64]
		if isHex64(hash) {
			prefix := hash + "_"
			var hits []string
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if strings.HasPrefix(e.Name(), prefix) {
					hits = append(hits, e.Name())
				}
			}
			if len(hits) == 1 {
				return filepath.Join(uploadDir, hits[0]), true
			}
		}
	}
	return "", false
}

func clientDownloadName(raw string) string {
	clean := filepath.Base(filepath.Clean(raw))
	if idx := strings.Index(clean, "_"); idx >= 0 && isHex64(clean[:idx]) {
		return clean[idx+1:]
	}
	return clean
}

// DownloadFile 文件下载接口
func DownloadFile(c *gin.Context) {
	raw := downloadSegmentFromRequest(c)
	safeFileName := filepath.Base(filepath.Clean(raw))

	if safeFileName == "." || safeFileName == "/" || safeFileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件请求非法"})
		return
	}

	// MinIO 场景：生成预签名下载 URL 并重定向
	if Storage != nil && Storage.BackendType() == storage.BackendMinIO {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		downloadURL, err := Storage.GetDownloadURL(ctx, safeFileName)
		if err != nil {
			pkg.Infof("[download] MinIO 下载 URL 生成失败: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, downloadURL)
		return
	}

	// 本地存储场景：直接返回文件
	if UploadBaseDir != "" {
		if filePath, ok := findUploadedObject(UploadBaseDir, safeFileName); ok {
			c.FileAttachment(filePath, clientDownloadName(safeFileName))
			return
		}
	}

	pkg.Infof("[download] 未找到文件 want=%s uploadDir=%s", safeFileName, UploadBaseDir)
	c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
}

// ---------- 以下为存量分片上传代码（保持兼容） ----------

func getUserChunkDir(c *gin.Context) string {
	userID := c.GetInt64("user_id")
	return filepath.Join(TempChunkDir, strconv.FormatInt(userID, 10))
}

func chunkFileName(hash string, idx int) string {
	return fmt.Sprintf("%s_%d", hash, idx)
}

func removeHashChunkFiles(chunkDirPath string, safeHash string) error {
	entries, err := os.ReadDir(chunkDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	prefix := safeHash + "_"
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if err := os.Remove(filepath.Join(chunkDirPath, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// CheckUploadStatus 断点续传-文件状态校验
func CheckUploadStatus(c *gin.Context) {
	fileHash := c.Query("hash")
	fileName := c.Query("filename")
	if fileHash == "" || fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}
	safeHash, err := sanitizeHash(fileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数非法"})
		return
	}

	safeFileName := filepath.Base(fileName)
	finalFilePath := filepath.Join(UploadBaseDir, fmt.Sprintf("%s_%s", safeHash, safeFileName))

	if _, err := os.Stat(finalFilePath); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"msg":          "文件已存在，无需重复上传",
			"download_url": fmt.Sprintf("/api/v1/download/%s_%s", safeHash, safeFileName),
		})
		return
	}

	chunkDirPath := getUserChunkDir(c)
	if err := os.MkdirAll(chunkDirPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "临时目录创建失败"})
		return
	}

	entries, err := os.ReadDir(chunkDirPath)
	if err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分片索引读取失败"})
		return
	}
	prefix := safeHash + "_"
	existing := make([]int, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) {
			if idx, ok := parseChunkIndex(entry.Name(), prefix); ok {
				existing = append(existing, idx)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"msg":              "续传检测完成",
		"uploaded_chunks":  existing,
		"uploaded_count":   len(existing),
	})
}

func parseChunkIndex(fileName string, prefix string) (int, bool) {
	part := strings.TrimPrefix(fileName, prefix)
	idx, err := strconv.Atoi(part)
	if err != nil {
		return 0, false
	}
	return idx, true
}

// UploadChunk 断点续传-分片上传
func UploadChunk(c *gin.Context) {
	fileHash := c.PostForm("hash")
	chunkIdxStr := c.PostForm("chunk_index")
	chunkTotalStr := c.PostForm("total_chunks")

	chunkIdx, err := strconv.Atoi(chunkIdxStr)
	chunkTotal, err2 := strconv.Atoi(chunkTotalStr)
	if err != nil || err2 != nil || fileHash == "" || chunkIdx < 0 || chunkTotal <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数非法"})
		return
	}

	safeHash, err := sanitizeHash(fileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数非法"})
		return
	}

	chunkDirPath := getUserChunkDir(c)
	if err := os.MkdirAll(chunkDirPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "临时目录创建失败"})
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分片文件缺失"})
		return
	}
	defer file.Close()

	chunkFilePath := filepath.Join(chunkDirPath, chunkFileName(safeHash, chunkIdx))
	dst, err := os.Create(chunkFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分片创建失败"})
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		_ = os.Remove(chunkFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分片写入失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": fmt.Sprintf("分片 %d 上传成功", chunkIdx)})
}

// MergeChunks 断点续传-分片合并
func MergeChunks(c *gin.Context) {
	fileHash := c.PostForm("hash")
	fileName := c.PostForm("filename")
	totalChunksStr := c.PostForm("total_chunks")
	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil || fileHash == "" || fileName == "" || totalChunks <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "合并参数缺失或非法"})
		return
	}
	safeHash, err := sanitizeHash(fileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "合并参数缺失或非法"})
		return
	}

	safeFileName := filepath.Base(fileName)
	finalFilePath := filepath.Join(UploadBaseDir, fmt.Sprintf("%s_%s", safeHash, safeFileName))

	if _, err := os.Stat(finalFilePath); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"msg":          "文件已存在，无需重复合并",
			"download_url": fmt.Sprintf("/api/v1/download/%s_%s", safeHash, safeFileName),
		})
		return
	}

	finalFile, err := os.OpenFile(finalFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "目标文件创建失败"})
		return
	}
	defer finalFile.Close()

	chunkDirPath := getUserChunkDir(c)

	for i := 0; i < totalChunks; i++ {
		chunkFilePath := filepath.Join(chunkDirPath, chunkFileName(safeHash, i))
		chunkFile, err := os.Open(chunkFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("分片 %d 缺失，合并终止", i)})
			os.Remove(finalFilePath)
			return
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "文件合并IO异常"})
			os.Remove(finalFilePath)
			return
		}
	}

	if err := removeHashChunkFiles(chunkDirPath, safeHash); err != nil {
		pkg.Infof("[上传清理错误] 清理分片失败 path=%s hash=%s err=%v", chunkDirPath, safeHash, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"msg":          "文件合并成功",
		"download_url": fmt.Sprintf("/api/v1/download/%s_%s", safeHash, safeFileName),
	})
}

// CancelUpload 取消文件上传
func CancelUpload(c *gin.Context) {
	fileHash := c.Query("hash")
	if fileHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数缺失"})
		return
	}

	safeHash, err := sanitizeHash(fileHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数非法"})
		return
	}

	chunkDirPath := getUserChunkDir(c)

	if err := removeHashChunkFiles(chunkDirPath, safeHash); err != nil {
		pkg.Infof("[上传清理错误] 临时目录删除失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "临时文件清理失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "上传已终止，临时文件已清理"})
}