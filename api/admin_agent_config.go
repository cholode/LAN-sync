package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminAgentConfigService *adminservice.AgentConfigService

// InitAdminAgentConfigService \u521d\u59cb\u5316\u5168\u5c40 Agent \u914d\u7f6e\u670d\u52a1\u3002
func InitAdminAgentConfigService(svc *adminservice.AgentConfigService) {
	adminAgentConfigService = svc
}

// AdminAgentConfigGet \u83b7\u53d6\u5168\u5c40 Agent \u914d\u7f6e\u3002\n// GET /api/v1/admin/agent-config
func AdminAgentConfigGet(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u914d\u7f6e\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	cfg, err := adminAgentConfigService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u83b7\u53d6 Agent \u914d\u7f6e\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigUpdate \u66f4\u65b0\u5168\u5c40 Agent \u914d\u7f6e\u3002\n// PUT /api/v1/admin/agent-config
func AdminAgentConfigUpdate(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u914d\u7f6e\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	var req adminservice.GlobalAgentConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "\u53c2\u6570\u683c\u5f0f\u9519\u8bef"})
		return
	}
	cfg, err := adminAgentConfigService.Update(c.Request.Context(), req, adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u4fdd\u5b58 Agent \u914d\u7f6e\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigRollback \u56de\u6eda\u5230\u4e0a\u4e00\u7248\u672c\u3002\n// POST /api/v1/admin/agent-config/rollback
func AdminAgentConfigRollback(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u914d\u7f6e\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	cfg, err := adminAgentConfigService.Rollback(c.Request.Context(), adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigHistory \u67e5\u8be2\u914d\u7f6e\u5386\u53f2\u3002\n// GET /api/v1/admin/agent-config/history
func AdminAgentConfigHistory(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "\u914d\u7f6e\u670d\u52a1\u672a\u521d\u59cb\u5316"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := adminAgentConfigService.History(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "\u67e5\u8be2\u914d\u7f6e\u5386\u53f2\u5931\u8d25"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}
