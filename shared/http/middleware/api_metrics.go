package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"lan-im-go/shared/observability/metrics"
)

// APIMetrics 测试所有 gin 的 handler，记录请求的耗时和状态码
func APIMetrics(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		startedAt := time.Now()
		defer func() {
			metrics.ObserveServiceAPIRequest(service, c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(startedAt))
		}()
		c.Next()
	}
}
