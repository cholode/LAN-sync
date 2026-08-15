package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RequestID \u4e3a\u6bcf\u4e2a\u8bf7\u6c42\u751f\u6210\u6216\u4f20\u9012 Request ID\uff0c\u5e76\u5199\u56de\u54cd\u5e94\u5934\u3002
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

func newRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "req-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(raw[:])
}

type rateEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// AdminRateLimit \u4e3a\u7ba1\u7406\u63a5\u53e3\u63d0\u4f9b\u57fa\u4e8e\u7ba1\u7406\u5458\u8eab\u4efd\u7684\u7b80\u5355\u9650\u6d41\u3002
// limit \u4e3a\u6bcf\u79d2\u5141\u8bb8\u7684\u8bf7\u6c42\u6570\uff0cburst \u4e3a\u77ed\u65f6\u7a81\u53d1\u5bb9\u91cf\u3002
func AdminRateLimit(limit float64, burst int) gin.HandlerFunc {
	mu := &sync.Mutex{}
	entries := make(map[string]*rateEntry)

	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		ip := c.ClientIP()
		key := strconv.FormatInt(userID, 10) + "|" + ip
		if userID == 0 {
			key = ip
		}

		mu.Lock()
		entry, ok := entries[key]
		if !ok {
			entry = &rateEntry{limiter: rate.NewLimiter(rate.Limit(limit), burst)}
			entries[key] = entry
		}
		entry.lastSeen = time.Now()
		if len(entries) > 2000 {
			now := time.Now()
			for id, item := range entries {
				if now.Sub(item.lastSeen) > 30*time.Minute {
					delete(entries, id)
				}
			}
		}
		allowed := entry.limiter.Allow()
		mu.Unlock()

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "\u8bf7\u6c42\u8fc7\u4e8e\u9891\u7e41\uff0c\u8bf7\u7a0d\u540e\u91cd\u8bd5"})
			c.Abort()
			return
		}
		c.Next()
	}
}
