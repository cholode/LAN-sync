package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
	"lan-im-go/shared/observability/metrics"
)

var adminDashboardService *adminservice.DashboardService

// InitAdminDashboardService 初始化 Dashboard 服务。
func InitAdminDashboardService(svc *adminservice.DashboardService) {
	adminDashboardService = svc
}

// AdminDashboardOverview 返回管理员首页聚合数据。
// GET /api/v1/admin/dashboard/overview
func AdminDashboardOverview(c *gin.Context) {
	if adminDashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dashboard 服务未初始化"})
		return
	}

	overview, err := adminDashboardService.CoreOverview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取 Dashboard 概览失败"})
		return
	}

	c.JSON(http.StatusOK, overview)
}

// AdminDashboardRuntime 返回 Go 运行时运行状态。
// GET /api/v1/admin/dashboard/runtime
func AdminDashboardRuntime(c *gin.Context) {
	if adminRuntime != nil {
		runtimeSnap, _, err := adminRuntime.RuntimeSnapshots(c.Request.Context())
		if err == nil {
			c.JSON(http.StatusOK, runtimeSnap)
			return
		}
	}
	c.JSON(http.StatusOK, metrics.RuntimeSnapshotNow())
}

// AdminDashboardMessageTraffic 返回消息流量图表数据。
// GET /api/v1/admin/dashboard/message-traffic
func AdminDashboardMessageTraffic(c *gin.Context) {
	if adminDashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dashboard 服务未初始化"})
		return
	}
	traffic, err := adminDashboardService.MessageTraffic(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取消息流量失败"})
		return
	}
	c.JSON(http.StatusOK, traffic)
}

// AdminAgentDashboard 返回 Agent 运行概览。
// GET /api/v1/admin/dashboard/agent
func AdminAgentDashboard(c *gin.Context) {
	if adminRuntime != nil {
		_, agentSnap, err := adminRuntime.RuntimeSnapshots(c.Request.Context())
		if err == nil {
			c.JSON(http.StatusOK, agentSnap)
			return
		}
	}
	c.JSON(http.StatusOK, metrics.AgentRuntimeSnapshotNow())
}

// AdminDashboardTimeSeries 返回首页图表需要的时间序列数据。
// GET /api/v1/admin/dashboard/timeseries?metric=messages&period=24h
func AdminDashboardTimeSeries(c *gin.Context) {
	if adminDashboardService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dashboard 服务未初始化"})
		return
	}

	metric := c.DefaultQuery("metric", "messages")
	period := c.DefaultQuery("period", "24h")
	data, err := adminDashboardService.TimeSeries(c.Request.Context(), metric, period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
