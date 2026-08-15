package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

var adminAgentConfigService *adminservice.AgentConfigService

// InitAdminAgentConfigService 初始化全局 Agent 配置服务。
func InitAdminAgentConfigService(svc *adminservice.AgentConfigService) {
	adminAgentConfigService = svc
}

// AdminAgentConfigGet 获取全局 Agent 配置。\n// GET /api/v1/admin/agent-config
func AdminAgentConfigGet(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置服务未初始化"})
		return
	}
	cfg, err := adminAgentConfigService.Get(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Agent 配置失败"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigUpdate 更新全局 Agent 配置。\n// PUT /api/v1/admin/agent-config
func AdminAgentConfigUpdate(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置服务未初始化"})
		return
	}
	var req adminservice.GlobalAgentConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	cfg, err := adminAgentConfigService.Update(c.Request.Context(), req, adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存 Agent 配置失败"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigRollback 回滚到上一版本。\n// POST /api/v1/admin/agent-config/rollback
func AdminAgentConfigRollback(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置服务未初始化"})
		return
	}
	cfg, err := adminAgentConfigService.Rollback(c.Request.Context(), adminAuditAction(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// AdminAgentConfigHistory 查询配置历史。\n// GET /api/v1/admin/agent-config/history
func AdminAgentConfigHistory(c *gin.Context) {
	if adminAgentConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置服务未初始化"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询配置历史失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}
