package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/services/admin/application"
)

// RecoveryWithErrorRecorder 替代 gin.Recovery，将 panic 和 5xx 响应写入系统错误中心。
func RecoveryWithErrorRecorder(errorService *adminservice.ErrorCenterService) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if errorService != nil {
					_ = errorService.Record(c.Request.Context(), adminservice.RecordErrorInput{
						Module:       "panic",
						ErrorType:    "panic",
						ErrorMessage: fmt.Sprint(recovered),
						RequestID:    c.GetString("request_id"),
						UserID:       c.GetInt64("user_id"),
						RoomID:       parseRoomIDFromContext(c),
						StackTrace:   string(debug.Stack()),
					})
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "系统内部错误"})
			}
		}()

		c.Next()

		if errorService != nil && c.Writer.Status() >= http.StatusInternalServerError {
			_ = errorService.Record(c.Request.Context(), adminservice.RecordErrorInput{
				Module:       "api",
				ErrorType:    "http_5xx",
				ErrorMessage: fmt.Sprintf("HTTP %d", c.Writer.Status()),
				RequestID:    c.GetString("request_id"),
				UserID:       c.GetInt64("user_id"),
				RoomID:       parseRoomIDFromContext(c),
			})
		}
	}
}

// parseRoomIDFromContext 从 Gin 上下文、路径参数或查询参数中解析房间 ID。
func parseRoomIDFromContext(c *gin.Context) int64 {
	if roomID, ok := c.Get("room_id"); ok {
		if value, ok := roomID.(int64); ok {
			return value
		}
	}
	if raw := c.Param("id"); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return value
		}
	}
	if raw := c.Query("room_id"); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return value
		}
	}
	return 0
}
