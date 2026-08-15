package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminAuditServiceVar *adminservice.AuditService

// InitAdminAuditService 初始化审计日志服务。
func InitAdminAuditService(svc *adminservice.AuditService) {
	adminAuditServiceVar = svc
}

// AdminAuditList 分页查询审计日志。\n// GET /api/v1/admin/audit-logs
func AdminAuditList(c *gin.Context) {
	if adminAuditServiceVar == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "审计服务未初始化"})
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
	adminUserID, _ := strconv.ParseInt(c.DefaultQuery("admin_user_id", "0"), 10, 64)
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminAuditServiceVar.List(c.Request.Context(), adminservice.AuditListQuery{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     c.Query("keyword"),
		AdminUserID: adminUserID,
		Action:      c.Query("action"),
		TargetType:  c.Query("target_type"),
		TargetID:    c.Query("target_id"),
		Result:      c.Query("result"),
		Start:       start,
		End:         end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询审计日志失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}
