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

// RequestID 为每个请求生成或传递 Request ID，并写回响应头。
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

// newRequestID 生成一个随机请求 ID。
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

// AdminRateLimit 为管理接口提供基于管理员身份的简单限流。
// limit 为每秒允许的请求数，burst 为短时突发容量。
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
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后重试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
