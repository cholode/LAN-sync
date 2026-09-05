package api

import (
	"github.com/gin-gonic/gin"

	"lan-im-go/models"
	"lan-im-go/shared/http/middleware"
)

// RegisterAdminRoutes 注册超级管理员后台路由。
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/rag/queries", middleware.RequirePermission(models.PermAgentRead), AdminRAGQueries)
	admin.GET("/dashboard/moderation", middleware.RequirePermission(models.PermModerationRead), AdminModerationDashboard)
	admin.GET("/moderation", middleware.RequirePermission(models.PermModerationRead), AdminModerationList)
	admin.GET("/moderation/:id", middleware.RequirePermission(models.PermModerationRead), AdminModerationDetail)
	admin.POST("/moderation/:id/action", middleware.RequirePermission(models.PermModerationReview), AdminModerationAction)
	admin.GET("/users", middleware.RequirePermission(models.PermUserRead), AdminUserList)
	admin.GET("/users/:id", middleware.RequirePermission(models.PermUserRead), AdminUserDetail)
	admin.POST("/users/:id/action", middleware.RequireAnyPermission(models.PermUserBan, models.PermUserRoleUpdate), AdminUserAction)
	admin.GET("/rooms", middleware.RequirePermission(models.PermRoomRead), AdminRoomList)
	admin.GET("/rooms/:id", middleware.RequirePermission(models.PermRoomRead), AdminRoomDetail)
	admin.POST("/rooms/:id/action", middleware.RequireAnyPermission(models.PermRoomFreeze, models.PermRoomDelete, models.PermAgentConfig), AdminRoomAction)
	admin.GET("/files", middleware.RequirePermission(models.PermFileRead), AdminFileList)
	admin.GET("/files/scan", middleware.RequirePermission(models.PermFileRead), AdminFileScan)
	admin.POST("/files/cleanup", middleware.RequirePermission(models.PermFileDelete), AdminFileCleanup)
	admin.GET("/files/:id", middleware.RequirePermission(models.PermFileRead), AdminFileDetail)
	admin.GET("/files/:id/download", middleware.RequirePermission(models.PermFileRead), AdminFileDownload)
	admin.DELETE("/files/:id", middleware.RequirePermission(models.PermFileDelete), AdminFileDelete)
	admin.GET("/agent-config", middleware.RequirePermission(models.PermAgentRead), AdminAgentConfigGet)
	admin.GET("/agent-config/history", middleware.RequirePermission(models.PermAgentRead), AdminAgentConfigHistory)
	admin.PUT("/agent-config", middleware.RequirePermission(models.PermAgentConfig), AdminAgentConfigUpdate)
	admin.POST("/agent-config/rollback", middleware.RequirePermission(models.PermAgentConfig), AdminAgentConfigRollback)
	admin.GET("/tool-calls", middleware.RequirePermission(models.PermAgentRead), AdminToolCallList)
	admin.GET("/audit-logs", middleware.RequirePermission(models.PermAuditRead), AdminAuditList)
	admin.DELETE("/rooms/:id", middleware.RequirePermission(models.PermRoomDelete), AdminDeleteRoom())
}
