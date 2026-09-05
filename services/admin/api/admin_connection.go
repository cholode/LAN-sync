package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

var adminConnectionService *adminservice.ConnectionService

// InitAdminConnectionService 初始化连接管理服务。
func InitAdminConnectionService(svc *adminservice.ConnectionService) {
	adminConnectionService = svc
}

// AdminConnectionList 查询 WebSocket 连接列表。
// 路由：GET /api/v1/admin/connections
func AdminConnectionList(c *gin.Context) {
	if adminConnectionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "连接管理服务未初始化"})
		return
	}
	items := adminConnectionService.ListConnections(c.Request.Context(), c.Query("keyword"))
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// AdminConnectionClose 关闭指定连接。
// 路由：POST /api/v1/admin/connections/close
func AdminConnectionClose(c *gin.Context) {
	var req struct {
		ConnectionID string `json:"connection_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少连接 ID"})
		return
	}
	if adminConnectionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "连接管理服务未初始化"})
		return
	}
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	err := adminConnectionService.CloseConnection(c.Request.Context(), req.ConnectionID, adminservice.ConnectionAction{
		AdminUserID: adminID,
		AdminName:   adminName,
		RequestID:   c.GetString("request_id"),
		RemoteIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "关闭连接失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "连接已关闭"})
}

// AdminUserForceOffline 强制用户下线。
// 路由：POST /api/v1/admin/connections/force-offline
func AdminUserForceOffline(c *gin.Context) {
	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少用户 ID"})
		return
	}
	if adminConnectionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "连接管理服务未初始化"})
		return
	}
	adminID := c.GetInt64("user_id")
	adminName := c.GetString("admin_username")
	if adminName == "" {
		adminName = strconv.FormatInt(adminID, 10)
	}
	err := adminConnectionService.ForceOffline(c.Request.Context(), req.UserID, adminservice.ConnectionAction{
		AdminUserID: adminID,
		AdminName:   adminName,
		RequestID:   c.GetString("request_id"),
		RemoteIP:    c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "强制下线失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "用户已强制下线"})
}
