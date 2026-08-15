package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"lan-im-go/cache"
	adminservice "lan-im-go/internal/admin"
	"lan-im-go/repository"
)

// AdminDeleteUser 管理员删除用户（强制下线 + 数据库软删除）。
// 路由: DELETE /api/v1/admin/users/:id
func AdminDeleteUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		targetUserIDStr := c.Param("id")
		targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的用户 ID 参数"})
			return
		}
		if targetUserID == c.GetInt64("user_id") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录的管理员账号"})
			return
		}

		// 1. 数据库软删除用户。
		if err := repository.User.SoftDeleteUser(targetUserID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "用户删除失败，请查看日志"})
			return
		}

		// 2. 非阻塞地断开在线连接，并清理 Redis 在线状态。
		if adminRuntime != nil {
			_ = adminRuntime.KickUser(c.Request.Context(), targetUserID)
		}
		_ = cache.SetUserOffline(c.Request.Context(), targetUserID)

		if adminAuditServiceVar != nil {
			_ = adminAuditServiceVar.Log(c.Request.Context(), adminservice.AuditEntry{
				AdminUserID:   c.GetInt64("user_id"),
				AdminUsername: c.GetString("admin_username"),
				Action:        "user.delete",
				TargetType:    "user",
				TargetID:      strconv.FormatInt(targetUserID, 10),
				RequestID:     c.GetString("request_id"),
				RemoteIP:      c.ClientIP(),
				UserAgent:     c.Request.UserAgent(),
				Result:        "success",
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"msg": "用户已删除，在线连接已断开",
		})
	}
}

// AdminDeleteRoom 管理员解散群聊。
// 路由: DELETE /api/v1/admin/rooms/:id
func AdminDeleteRoom() gin.HandlerFunc {
	return func(c *gin.Context) {
		targetRoomIDStr := c.Param("id")
		targetRoomID, err := strconv.ParseInt(targetRoomIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "非法的群聊 ID 参数"})
			return
		}
		if adminRoomService == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "房间服务未初始化"})
			return
		}

		adminID := c.GetInt64("user_id")
		adminName := c.GetString("admin_username")
		if adminName == "" {
			adminName = strconv.FormatInt(adminID, 10)
		}
		err = adminRoomService.ApplyAction(c.Request.Context(), targetRoomID, adminservice.RoomAction{
			Action:      "disband",
			AdminUserID: adminID,
			AdminName:   adminName,
			RequestID:   c.GetString("request_id"),
			RemoteIP:    c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "解散群聊失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"msg": "群聊解散成功，已通知所有在线成员",
		})
	}
}
