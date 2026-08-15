package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
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
