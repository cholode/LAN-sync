package api

import (
	"github.com/gin-gonic/gin"

	auth "lan-im-go/shared/auth"
	"lan-im-go/shared/http/middleware"
)

// RegisterAdminRoutes 注册超级管理员后台路由。
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/rag/queries", middleware.RequirePermission(auth.PermAgentRead), AdminRAGQueries)
	admin.GET("/dashboard/moderation", middleware.RequirePermission(auth.PermModerationRead), AdminModerationDashboard)
	admin.GET("/moderation", middleware.RequirePermission(auth.PermModerationRead), AdminModerationList)
	admin.GET("/moderation/:id", middleware.RequirePermission(auth.PermModerationRead), AdminModerationDetail)
	admin.POST("/moderation/:id/action", middleware.RequirePermission(auth.PermModerationReview), AdminModerationAction)
	admin.GET("/users", middleware.RequirePermission(auth.PermUserRead), AdminUserList)
	admin.GET("/users/:id", middleware.RequirePermission(auth.PermUserRead), AdminUserDetail)
	admin.POST("/users/:id/action", middleware.RequireAnyPermission(auth.PermUserBan, auth.PermUserRoleUpdate), AdminUserAction)
	admin.GET("/rooms", middleware.RequirePermission(auth.PermRoomRead), AdminRoomList)
	admin.GET("/rooms/:id", middleware.RequirePermission(auth.PermRoomRead), AdminRoomDetail)
	admin.POST("/rooms/:id/action", middleware.RequireAnyPermission(auth.PermRoomFreeze, auth.PermRoomDelete, auth.PermAgentConfig), AdminRoomAction)
	admin.GET("/files", middleware.RequirePermission(auth.PermFileRead), AdminFileList)
	admin.GET("/files/scan", middleware.RequirePermission(auth.PermFileRead), AdminFileScan)
	admin.POST("/files/cleanup", middleware.RequirePermission(auth.PermFileDelete), AdminFileCleanup)
	admin.GET("/files/:id", middleware.RequirePermission(auth.PermFileRead), AdminFileDetail)
	admin.GET("/files/:id/download", middleware.RequirePermission(auth.PermFileRead), AdminFileDownload)
	admin.DELETE("/files/:id", middleware.RequirePermission(auth.PermFileDelete), AdminFileDelete)
	admin.GET("/agent-config", middleware.RequirePermission(auth.PermAgentRead), AdminAgentConfigGet)
	admin.GET("/agent-config/history", middleware.RequirePermission(auth.PermAgentRead), AdminAgentConfigHistory)
	admin.PUT("/agent-config", middleware.RequirePermission(auth.PermAgentConfig), AdminAgentConfigUpdate)
	admin.POST("/agent-config/rollback", middleware.RequirePermission(auth.PermAgentConfig), AdminAgentConfigRollback)
	admin.GET("/tool-calls", middleware.RequirePermission(auth.PermAgentRead), AdminToolCallList)
	admin.GET("/audit-logs", middleware.RequirePermission(auth.PermAuditRead), AdminAuditList)
	admin.DELETE("/rooms/:id", middleware.RequirePermission(auth.PermRoomDelete), AdminDeleteRoom())
}
