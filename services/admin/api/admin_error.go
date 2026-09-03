package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminErrorService *adminservice.ErrorCenterService

// InitAdminErrorService 初始化系统错误中心服务。
func InitAdminErrorService(svc *adminservice.ErrorCenterService) {
	adminErrorService = svc
}

// AdminErrorList 查询系统错误。\n// GET /api/v1/admin/errors
func AdminErrorList(c *gin.Context) {
	if adminErrorService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "错误中心服务未初始化"})
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
	var resolved *bool
	if raw := c.Query("resolved"); raw != "" {
		value := raw == "true" || raw == "1"
		resolved = &value
	}
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminErrorService.List(c.Request.Context(), adminservice.ErrorListQuery{
		Page:      page,
		PageSize:  pageSize,
		Module:    c.Query("module"),
		ErrorType: c.Query("error_type"),
		Resolved:  resolved,
		Start:     start,
		End:       end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询系统错误失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminErrorResolve 标记错误已处理。\n// POST /api/v1/admin/errors/:id/resolve
func AdminErrorResolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法错误 ID"})
		return
	}
	if adminErrorService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "错误中心服务未初始化"})
		return
	}
	adminID := c.GetInt64("user_id")
	if err := adminErrorService.Resolve(c.Request.Context(), id, adminID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "标记已处理失败"})
		return
	}
	if adminAuditServiceVar != nil {
		_ = adminAuditServiceVar.Log(c.Request.Context(), adminservice.AuditEntry{
			AdminUserID:   adminID,
			AdminUsername: c.GetString("admin_username"),
			Action:        "error.resolve",
			TargetType:    "system_error",
			TargetID:      strconv.FormatInt(id, 10),
			RequestID:     c.GetString("request_id"),
			RemoteIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Result:        "success",
		})
	}
	c.JSON(http.StatusOK, gin.H{"message": "已标记为已处理"})
}
