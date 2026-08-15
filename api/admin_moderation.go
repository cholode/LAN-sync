package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminModerationService *adminservice.ModerationService

// InitAdminModerationService ???????????
func InitAdminModerationService(svc *adminservice.ModerationService) {
	adminModerationService = svc
}

// AdminModerationDashboard ?????? Dashboard ???
// GET /api/v1/admin/dashboard/moderation
func AdminModerationDashboard(c *gin.Context) {
	if adminModerationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "?????????????"})
		return
	}
	data, err := adminModerationService.Dashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "??????????"})
		return
	}
	c.JSON(http.StatusOK, data)
}
