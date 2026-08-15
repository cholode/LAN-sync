package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
	"lan-im-go/internal/metrics"
)

var adminDashboardService *adminservice.DashboardService

// InitAdminDashboardService ?? Dashboard ?????
func InitAdminDashboardService(svc *adminservice.DashboardService) {
	adminDashboardService = svc
}

// AdminDashboardOverview ?????????????
// GET /api/v1/admin/dashboard/overview
func AdminDashboardOverview(c *gin.Context) {
	if adminDashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "???????????"})
		return
	}

	overview, err := adminDashboardService.CoreOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "?? Dashboard ????"})
		return
	}

	c.JSON(http.StatusOK, overview)
}

// AdminDashboardRuntime ???????????
// GET /api/v1/admin/dashboard/runtime
func AdminDashboardRuntime(c *gin.Context) {
	c.JSON(http.StatusOK, metrics.RuntimeSnapshotNow())
}

// AdminDashboardMessageTraffic ?????????
// GET /api/v1/admin/dashboard/message-traffic
func AdminDashboardMessageTraffic(c *gin.Context) {
	if adminDashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "???????????"})
		return
	}
	traffic, err := adminDashboardService.MessageTraffic(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "??????????"})
		return
	}
	c.JSON(http.StatusOK, traffic)
}
