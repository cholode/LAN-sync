package messages

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"lan-im-go/shared/http/middleware"
	"lan-im-go/shared/observability/metrics"
)

// NewRouter 为独立消息服务挂载原有接口，路径和 JWT 校验保持一致。
func NewRouter(module *Module, ready func(context.Context) error) *gin.Engine {
	router := gin.New()
	router.Use(middleware.APIMetrics("message"), middleware.RequestID(), gin.Recovery())
	router.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", gin.WrapH(metrics.Handler()))
	module.RegisterRoutes(router.Group("/api/v1", middleware.JWTAuth()))
	return router
}
