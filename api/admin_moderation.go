package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminModerationService *adminservice.ModerationService

// InitAdminModerationService ???????????
func InitAdminModerationService(svc *adminservice.ModerationService) {
	adminModerationService = svc
}

// AdminModerationDashboard ?????? Dashboard ???
// GET /api/v1/admin/dashboard/moderation
func AdminModerationDashboard(c *gin.Context) {
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "?????????????"})
		return
	}
	data, err := adminModerationService.Dashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "??????????"})
		return
	}
	c.JSON(http.StatusOK, data)
}

// AdminModerationList ?????????
// GET /api/v1/admin/moderation
func AdminModerationList(c *gin.Context) {
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "?????????????"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "????????"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminModerationDetail ?????????
// GET /api/v1/admin/moderation/:id
func AdminModerationDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "????? ID"})
		return
	}
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "?????????????"})
		return
	}
	item, err := adminModerationService.GetEvent(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "???????"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// AdminModerationAction ?????????/?????
// POST /api/v1/admin/moderation/:id/action
func AdminModerationAction(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "????? ID"})
		return
	}
	var req struct {
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "???????"})
		return
	}
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "?????????????"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "????????"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "????"})
}
