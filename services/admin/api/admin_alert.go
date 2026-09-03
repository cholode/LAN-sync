package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminAlertService *adminservice.AlertService

// InitAdminAlertService 初始化告警服务。
func InitAdminAlertService(svc *adminservice.AlertService) {
	adminAlertService = svc
}

// AdminAlertList 查询告警列表。\n// GET /api/v1/admin/alerts
func AdminAlertList(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "告警服务未初始化"})
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
	items, total, err := adminAlertService.List(c.Request.Context(), adminservice.AlertListQuery{
		Page:     page,
		PageSize: pageSize,
		Level:    c.Query("level"),
		Status:   c.Query("status"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询告警失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminAlertEvaluate 手动触发告警评估。\n// POST /api/v1/admin/alerts/evaluate
func AdminAlertEvaluate(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "告警服务未初始化"})
		return
	}
	items, err := adminAlertService.Evaluate(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "告警评估失败"})
		return
	}
	if adminAuditServiceVar != nil {
		_ = adminAuditServiceVar.Log(c.Request.Context(), adminservice.AuditEntry{
			AdminUserID:   c.GetInt64("user_id"),
			AdminUsername: c.GetString("admin_username"),
			Action:        "alert.evaluate",
			TargetType:    "alert",
			TargetID:      "all",
			RequestID:     c.GetString("request_id"),
			RemoteIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Result:        "success",
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// AdminAlertResolve 标记告警已处理。\n// POST /api/v1/admin/alerts/:id/resolve
func AdminAlertResolve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法告警 ID"})
		return
	}
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "告警服务未初始化"})
		return
	}
	if err := adminAlertService.Resolve(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理告警失败"})
		return
	}
	if adminAuditServiceVar != nil {
		_ = adminAuditServiceVar.Log(c.Request.Context(), adminservice.AuditEntry{
			AdminUserID:   c.GetInt64("user_id"),
			AdminUsername: c.GetString("admin_username"),
			Action:        "alert.resolve",
			TargetType:    "alert",
			TargetID:      strconv.FormatInt(id, 10),
			RequestID:     c.GetString("request_id"),
			RemoteIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Result:        "success",
		})
	}
	c.JSON(http.StatusOK, gin.H{"message": "已标记为已处理"})
}

// AdminAlertUnresolvedCount 返回未处理告警数量。\n// GET /api/v1/admin/alerts/unresolved-count
func AdminAlertUnresolvedCount(c *gin.Context) {
	if adminAlertService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "告警服务未初始化"})
		return
	}
	count, err := adminAlertService.UnresolvedCount(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "统计告警失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}
