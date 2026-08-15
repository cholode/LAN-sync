package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminHealthService *adminservice.HealthService

// InitAdminHealthService 初始化系统健康服务。
func InitAdminHealthService(svc *adminservice.HealthService) {
	adminHealthService = svc
}

// AdminHealthCheck 返回所有下游服务健康状态。\n// GET /api/v1/admin/health
func AdminHealthCheck(c *gin.Context) {
	if adminHealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "健康服务未初始化"})
		return
	}
	items := adminHealthService.CheckAll(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"items": items})
}
