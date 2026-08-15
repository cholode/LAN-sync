package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminAuditServiceVar *adminservice.AuditService

// InitAdminAuditService \u521d\u59cb\u5316\u5ba1\u8ba1\u65e5\u5fd7\u670d\u52a1\u3002
func InitAdminAuditService(svc *adminservice.AuditService) {
	adminAuditServiceVar = svc
}

// AdminAuditList \u5206\u9875\u67e5\u8be2\u5ba1\u8ba1\u65e5\u5fd7\u3002\n// GET /api/v1/admin/audit-logs
func AdminAuditList(c *gin.Context) {
	if adminAuditServiceVar == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u5ba1\u8ba1\u670d\u52a1\u672a\u521d\u59cb\u5316"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u67e5\u8be2\u5ba1\u8ba1\u65e5\u5fd7\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}
