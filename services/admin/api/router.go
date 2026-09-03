package api

import (
	"github.com/gin-gonic/gin"

	"lan-im-go/models"
	"lan-im-go/shared/http/middleware"
)

// RegisterAdminRoutes 注册超级管理员后台路由。
func RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/dashboard/runtime", middleware.RequirePermission(models.PermDashboardRead), AdminDashboardRuntime)
	admin.GET("/dashboard/message-traffic", middleware.RequirePermission(models.PermDashboardRead), AdminDashboardMessageTraffic)
	admin.GET("/dashboard/timeseries", middleware.RequirePermission(models.PermDashboardRead), AdminDashboardTimeSeries)
	admin.GET("/dashboard/agent", middleware.RequirePermission(models.PermAgentRead), AdminAgentDashboard)
	admin.GET("/dashboard/rag", middleware.RequireAnyPermission(models.PermDashboardRead, models.PermAgentRead), AdminRAGDashboard)
	admin.GET("/rag/queries", middleware.RequirePermission(models.PermAgentRead), AdminRAGQueries)
	admin.GET("/dashboard/moderation", middleware.RequirePermission(models.PermModerationRead), AdminModerationDashboard)
	admin.GET("/moderation", middleware.RequirePermission(models.PermModerationRead), AdminModerationList)
	admin.GET("/moderation/:id", middleware.RequirePermission(models.PermModerationRead), AdminModerationDetail)
	admin.POST("/moderation/:id/action", middleware.RequirePermission(models.PermModerationReview), AdminModerationAction)
	admin.GET("/dashboard/overview", middleware.RequirePermission(models.PermDashboardRead), AdminDashboardOverview)
	admin.GET("/users", middleware.RequirePermission(models.PermUserRead), AdminUserList)
	admin.GET("/users/:id", middleware.RequirePermission(models.PermUserRead), AdminUserDetail)
	admin.POST("/users/:id/action", middleware.RequireAnyPermission(models.PermUserBan, models.PermUserKick, models.PermUserRoleUpdate), AdminUserAction)
	admin.DELETE("/users/:id", middleware.RequirePermission(models.PermUserDelete), AdminDeleteUser())
	admin.GET("/rooms", middleware.RequirePermission(models.PermRoomRead), AdminRoomList)
	admin.GET("/rooms/:id", middleware.RequirePermission(models.PermRoomRead), AdminRoomDetail)
	admin.POST("/rooms/:id/action", middleware.RequireAnyPermission(models.PermRoomFreeze, models.PermRoomDelete, models.PermAgentConfig), AdminRoomAction)
	admin.GET("/connections", middleware.RequirePermission(models.PermConnectionRead), AdminConnectionList)
	admin.POST("/connections/close", middleware.RequirePermission(models.PermConnectionClose), AdminConnectionClose)
	admin.POST("/connections/force-offline", middleware.RequirePermission(models.PermConnectionClose), AdminUserForceOffline)
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
	admin.GET("/errors", middleware.RequirePermission(models.PermSystemRead), AdminErrorList)
	admin.POST("/errors/:id/resolve", middleware.RequirePermission(models.PermSystemRead), AdminErrorResolve)
	admin.GET("/audit-logs", middleware.RequirePermission(models.PermAuditRead), AdminAuditList)
	admin.GET("/health", middleware.RequirePermission(models.PermSystemRead), AdminHealthCheck)
	admin.GET("/alerts", middleware.RequirePermission(models.PermSystemRead), AdminAlertList)
	admin.GET("/alerts/unresolved-count", middleware.RequireAnyPermission(models.PermDashboardRead, models.PermSystemRead), AdminAlertUnresolvedCount)
	admin.POST("/alerts/evaluate", middleware.RequirePermission(models.PermSystemRead), AdminAlertEvaluate)
	admin.POST("/alerts/:id/resolve", middleware.RequirePermission(models.PermSystemRead), AdminAlertResolve)
	admin.DELETE("/rooms/:id", middleware.RequirePermission(models.PermRoomDelete), AdminDeleteRoom())
}
