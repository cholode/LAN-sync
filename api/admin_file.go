package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

// InitAdminFileServiceVar \u521d\u59cb\u5316\u8d85\u7ba1\u6587\u4ef6\u7ba1\u7406\u670d\u52a1\u3002
func InitAdminFileServiceVar(svc *adminservice.FileService) {
	adminFileService = svc
}

func adminAuditAction(c *gin.Context) adminservice.AuditAction {
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	return adminservice.AuditAction{
		AdminUserID: adminID,
		AdminName:   adminName,
		RequestID:   c.GetString("request_id"),
		RemoteIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	}
}

// AdminFileList \u5206\u9875\u67e5\u8be2\u6587\u4ef6\u8bb0\u5f55\u3002\n// GET /api/v1/admin/files
func AdminFileList(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	uploaderID, _ := strconv.ParseInt(c.DefaultQuery("uploader_id", "0"), 10, 64)
	roomID, _ := strconv.ParseInt(c.DefaultQuery("room_id", "0"), 10, 64)
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminFileService.ListFiles(c.Request.Context(), adminservice.FileListQuery{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    c.Query("keyword"),
		UploaderID: uploaderID,
		RoomID:     roomID,
		FileType:   c.Query("file_type"),
		Status:     c.Query("status"),
		Start:      start,
		End:        end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u67e5\u8be2\u6587\u4ef6\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminFileDetail \u67e5\u770b\u5355\u4e2a\u6587\u4ef6\u8bb0\u5f55\u3002\n// GET /api/v1/admin/files/:id
func AdminFileDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u975e\u6cd5\u6587\u4ef6 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	item, err := adminFileService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "\u6587\u4ef6\u4e0d\u5b58\u5728"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// AdminFileDownload \u8fd4\u56de\u4e00\u4e2a\u53ef\u76f4\u63a5\u4e0b\u8f7d\u7684\u9884\u7b7e\u540d URL\u3002\n// GET /api/v1/admin/files/:id/download
func AdminFileDownload(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u975e\u6cd5\u6587\u4ef6 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	item, err := adminFileService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "\u6587\u4ef6\u4e0d\u5b58\u5728"})
		return
	}
	if Storage == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u5bf9\u8c61\u5b58\u50a8\u672a\u521d\u59cb\u5316"})
		return
	}
	downloadURL, err := Storage.GetDownloadURL(c.Request.Context(), item.ObjectKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "\u5bf9\u8c61\u6587\u4ef6\u4e0d\u5b58\u5728"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": downloadURL})
}

// AdminFileDelete \u5220\u9664\u5bf9\u8c61\u5b58\u50a8\u6587\u4ef6\u53ca\u6570\u636e\u5e93\u8bb0\u5f55\u3002\n// DELETE /api/v1/admin/files/:id
func AdminFileDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u975e\u6cd5\u6587\u4ef6 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	if err := adminFileService.DeleteFile(c.Request.Context(), id, adminAuditAction(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u5220\u9664\u6587\u4ef6\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "\u5220\u9664\u6210\u529f"})
}

// AdminFileScan \u68c0\u67e5\u5f02\u5e38\u6587\u4ef6\u3002\n// GET /api/v1/admin/files/scan
func AdminFileScan(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	result, err := adminFileService.ScanAnomalies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u6587\u4ef6\u5f02\u5e38\u68c0\u67e5\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminFileCleanup \u5b89\u5168\u6e05\u7406\u5b64\u7acb\u5bf9\u8c61\u3002\n// POST /api/v1/admin/files/cleanup
func AdminFileCleanup(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u6587\u4ef6\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	cleaned, err := adminFileService.CleanupOrphans(c.Request.Context(), adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u6e05\u7406\u5b64\u7acb\u6587\u4ef6\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleaned": cleaned})
}
