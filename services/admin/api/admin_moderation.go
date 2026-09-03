package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminModerationService *adminservice.ModerationService

// InitAdminModerationService 初始化内容审核服务。
func InitAdminModerationService(svc *adminservice.ModerationService) {
	adminModerationService = svc
}

// AdminModerationDashboard 获取审核看板数据。
// GET /api/v1/admin/dashboard/moderation
func AdminModerationDashboard(c *gin.Context) {
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "审核服务未初始化"})
		return
	}
	data, err := adminModerationService.Dashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取审核看板失败"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// AdminModerationList 分页查询审核事件。
// GET /api/v1/admin/moderation
func AdminModerationList(c *gin.Context) {
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "审核服务未初始化"})
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
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	roomID, _ := strconv.ParseInt(c.Query("room_id"), 10, 64)
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminModerationService.ListEvents(c.Request.Context(), adminservice.ModerationListQuery{
		Page:          page,
		PageSize:      pageSize,
		Username:      c.Query("username"),
		UserID:        userID,
		RoomID:        roomID,
		Category:      c.Query("category"),
		RiskLevel:     c.Query("risk_level"),
		PenaltyStatus: c.Query("penalty_status"),
		Start:         start,
		End:           end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询审核事件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminModerationDetail 查询单条审核事件。
// GET /api/v1/admin/moderation/:id
func AdminModerationDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的审核记录 ID"})
		return
	}
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "审核服务未初始化"})
		return
	}
	item, err := adminModerationService.GetEvent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审核记录不存在"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// AdminModerationAction 审核通过或驳回。
// POST /api/v1/admin/moderation/:id/action
func AdminModerationAction(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的审核记录 ID"})
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少审核动作"})
		return
	}
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "审核服务未初始化"})
		return
	}
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	err = adminModerationService.ApplyAction(c.Request.Context(), id, adminservice.ModerationAction{
		Action:      req.Action,
		AdminUserID: adminID,
		AdminName:   adminName,
		RequestID:   c.GetString("request_id"),
		RemoteIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "审核处理失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "审核处理成功"})
}
