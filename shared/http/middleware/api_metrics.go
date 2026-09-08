package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"lan-im-go/shared/observability/metrics"
)

// APIMetrics measures the full Gin handler chain for application API requests.
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
