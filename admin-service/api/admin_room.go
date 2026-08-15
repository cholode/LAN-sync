package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminRoomService *adminservice.RoomService

// InitAdminRoomService 初始化房间管理服务。
func InitAdminRoomService(svc *adminservice.RoomService) {
	adminRoomService = svc
}

// AdminRoomList 分页查询群聊。
// GET /api/v1/admin/rooms
func AdminRoomList(c *gin.Context) {
	if adminRoomService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "房间服务未初始化"})
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
	roomType := int8(0)
	if raw := c.Query("type"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			roomType = int8(v)
		}
	}
	status := int8(-1)
	if raw := c.Query("status"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			status = int8(v)
		}
	}
	var agentEnabled *bool
	if raw := c.Query("agent_enabled"); raw != "" {
		value := raw == "true" || raw == "1"
		agentEnabled = &value
	}
	start, _ := time.Parse(time.RFC3339, c.Query("start"))
	end, _ := time.Parse(time.RFC3339, c.Query("end"))

	items, total, err := adminRoomService.ListRooms(c.Request.Context(), adminservice.RoomListQuery{
		Page:         page,
		PageSize:     pageSize,
		Keyword:      c.Query("keyword"),
		RoomType:     roomType,
		AgentEnabled: agentEnabled,
		Status:       status,
		Start:        start,
		End:          end,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询房间失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// AdminRoomDetail 查询房间详情。
// GET /api/v1/admin/rooms/:id
func AdminRoomDetail(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间 ID"})
		return
	}
	if adminRoomService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "房间服务未初始化"})
		return
	}
	detail, err := adminRoomService.GetRoomDetail(c.Request.Context(), roomID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// AdminRoomAction 执行房间管理动作。
// POST /api/v1/admin/rooms/:id/action
func AdminRoomAction(c *gin.Context) {
	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法的房间 ID"})
		return
	}
	var req struct {
		Action       string `json:"action" binding:"required"`
		TargetUserID int64  `json:"target_user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少操作参数"})
		return
	}
	if adminRoomService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "房间服务未初始化"})
		return
	}
	permission := roomActionPermission(req.Action)
	if permission == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的群聊操作"})
		return
	}
	if !requireActionPermission(c, permission) {
		return
	}
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	err = adminRoomService.ApplyAction(c.Request.Context(), roomID, adminservice.RoomAction{
		Action:       req.Action,
		AdminUserID:  adminID,
		AdminName:    adminName,
		RequestID:    c.GetString("request_id"),
		RemoteIP:     c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		TargetUserID: req.TargetUserID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "房间操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "操作成功"})
}
