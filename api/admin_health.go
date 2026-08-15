package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminHealthService *adminservice.HealthService

// InitAdminHealthService \u521d\u59cb\u5316\u7cfb\u7edf\u5065\u5eb7\u670d\u52a1\u3002
func InitAdminHealthService(svc *adminservice.HealthService) {
	adminHealthService = svc
}

// AdminHealthCheck \u8fd4\u56de\u6240\u6709\u4e0b\u6e38\u670d\u52a1\u5065\u5eb7\u72b6\u6001\u3002\n// GET /api/v1/admin/health
func AdminHealthCheck(c *gin.Context) {
	if adminHealthService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u5065\u5eb7\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	items := adminHealthService.CheckAll(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"items": items})
}
