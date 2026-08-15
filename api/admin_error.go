package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminErrorService *adminservice.ErrorCenterService

// InitAdminErrorService \u521d\u59cb\u5316\u7cfb\u7edf\u9519\u8bef\u4e2d\u5fc3\u670d\u52a1\u3002
func InitAdminErrorService(svc *adminservice.ErrorCenterService) {
	adminErrorService = svc
}

// AdminErrorList \u67e5\u8be2\u7cfb\u7edf\u9519\u8bef\u3002\n// GET /api/v1/admin/errors
func AdminErrorList(c *gin.Context) {
	if adminErrorService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u9519\u8bef\u4e2d\u5fc3\u670d\u52a1\u672a\u521d\u59cb\u5316"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u67e5\u8be2\u7cfb\u7edf\u9519\u8bef\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminErrorResolve \u6807\u8bb0\u9519\u8bef\u5df2\u5904\u7406\u3002\n// POST /api/v1/admin/errors/:id/resolve
func AdminErrorResolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u975e\u6cd5\u9519\u8bef ID"})
		return
	}
	if adminErrorService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u9519\u8bef\u4e2d\u5fc3\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	adminID := c.GetInt64("user_id")
	if err := adminErrorService.Resolve(c.Request.Context(), id, adminID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u6807\u8bb0\u5df2\u5904\u7406\u5931\u8d25"})
		return
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
	}
	c.JSON(http.StatusOK, gin.H{"message": "\u5df2\u6807\u8bb0\u4e3a\u5df2\u5904\u7406"})
}
