package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"

	adminservice "lan-im-go/internal/admin"
)

// RecoveryWithErrorRecorder \u66ff\u4ee3 gin.Recovery\uff0c\u5c06 panic \u548c 5xx \u54cd\u5e94\u5199\u5165\u7cfb\u7edf\u9519\u8bef\u4e2d\u5fc3\u3002
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
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "\u7cfb\u7edf\u5185\u90e8\u9519\u8bef"})
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
