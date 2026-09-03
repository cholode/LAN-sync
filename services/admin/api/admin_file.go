package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

// InitAdminFileServiceVar 初始化超管文件管理服务。
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

// AdminFileList 分页查询文件记录。\n// GET /api/v1/admin/files
func AdminFileList(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminFileDetail 查看单个文件记录。\n// GET /api/v1/admin/files/:id
func AdminFileDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	item, err := adminFileService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// AdminFileDownload 返回一个可直接下载的预签名 URL。\n// GET /api/v1/admin/files/:id/download
func AdminFileDownload(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	item, err := adminFileService.GetFile(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	if Storage == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "对象存储未初始化"})
		return
	}
	downloadURL, err := Storage.GetDownloadURL(c.Request.Context(), item.ObjectKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "对象文件不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"download_url": downloadURL})
}

// AdminFileDelete 删除对象存储文件及数据库记录。\n// DELETE /api/v1/admin/files/:id
func AdminFileDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件 ID"})
		return
	}
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	if err := adminFileService.DeleteFile(c.Request.Context(), id, adminAuditAction(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// AdminFileScan 检查异常文件。\n// GET /api/v1/admin/files/scan
func AdminFileScan(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	result, err := adminFileService.ScanAnomalies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件异常检查失败"})
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminFileCleanup 安全清理孤立对象。\n// POST /api/v1/admin/files/cleanup
func AdminFileCleanup(c *gin.Context) {
	if adminFileService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "文件服务未初始化"})
		return
	}
	cleaned, err := adminFileService.CleanupOrphans(c.Request.Context(), adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理孤立文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"cleaned": cleaned})
}
