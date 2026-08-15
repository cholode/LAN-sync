package admin

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"

	"lan-im-go/core"
	"lan-im-go/internal/storage"
)

// HealthService \u63d0\u4f9b\u5e26\u8d85\u65f6\u7684\u4e0b\u6e38\u670d\u52a1\u5065\u5eb7\u68c0\u6d4b\uff0c\u907f\u514d Dashboard \u88ab\u5355\u4e2a\u670d\u52a1\u62d6\u6b7b\u3002
type HealthService struct {
	db      *gorm.DB
	redis   *redis.Client
	storage storage.Provider
	hub     *core.Hub
}

func NewHealthService(db *gorm.DB, redisClient *redis.Client, provider storage.Provider, hub *core.Hub) *HealthService {
	return &HealthService{db: db, redis: redisClient, storage: provider, hub: hub}
}

// HealthItem \u5355\u4e2a\u4e0b\u6e38\u670d\u52a1\u7684\u5065\u5eb7\u7ed3\u679c\u3002
type HealthItem struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	LatencyMS float64   `json:"latency_ms"`
	Error     string    `json:"error"`
	CheckedAt time.Time `json:"checked_at"`
}

const (
	HealthHealthy  = "healthy"
	HealthDegraded = "degraded"
	HealthDown     = "down"
)

// CheckAll \u5e76\u884c\u68c0\u67e5\u6240\u6709\u4e0b\u6e38\u670d\u52a1\u3002
func (s *HealthService) CheckAll(ctx context.Context) []HealthItem {
	items := make([]HealthItem, 0, 7)
	ch := make(chan HealthItem, 7)
	checks := []func(context.Context) HealthItem{
		s.checkMySQL,
		s.checkRedis,
		s.checkQdrant,
		s.checkMinIO,
		s.checkLLM,
		s.checkEmbedding,
		s.checkWebSocketHub,
	}
	for _, check := range checks {
		go func(check func(context.Context) HealthItem) {
			checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			ch <- check(checkCtx)
		}(check)
	}
	for range checks {
		items = append(items, <-ch)
	}
	return items
}

func (s *HealthService) checkMySQL(ctx context.Context) HealthItem {
	start := time.Now()
	item := HealthItem{Name: "mysql", CheckedAt: time.Now(), Status: HealthDown}
	if s.db == nil {
		item.Error = "???????"
		return item
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		item.Error = err.Error()
		return item
	}
	item.Status = HealthHealthy
	item.LatencyMS = latencyMS(start)
	return item
}

func (s *HealthService) checkRedis(ctx context.Context) HealthItem {
	start := time.Now()
	item := HealthItem{Name: "redis", CheckedAt: time.Now(), Status: HealthDown}
	if s.redis == nil {
		item.Error = "Redis ????"
		return item
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		item.Error = err.Error()
		return item
	}
	item.Status = HealthHealthy
	item.LatencyMS = latencyMS(start)
	return item
}

func (s *HealthService) checkQdrant(ctx context.Context) HealthItem {
	start := time.Now()
	item := HealthItem{Name: "qdrant", CheckedAt: time.Now(), Status: HealthDown}
	host := os.Getenv("QDRANT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 6334
	if raw := os.Getenv("QDRANT_PORT"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			port = value
		}
	}
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if _, err := client.ListCollections(ctx); err != nil {
		item.Error = err.Error()
		return item
	}
	item.Status = HealthHealthy
	item.LatencyMS = latencyMS(start)
	return item
}

func (s *HealthService) checkMinIO(ctx context.Context) HealthItem {
	start := time.Now()
	item := HealthItem{Name: "minio", CheckedAt: time.Now(), Status: HealthDown}
	if s.storage == nil {
		item.Error = "????????"
		return item
	}
	if _, err := s.storage.ListObjects(ctx, "", 1); err != nil {
		item.Error = err.Error()
		return item
	}
	item.Status = HealthHealthy
	item.LatencyMS = latencyMS(start)
	return item
}

func (s *HealthService) checkLLM(ctx context.Context) HealthItem {
	return checkHTTPProvider(ctx, "llm", os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_API_KEY"))
}

func (s *HealthService) checkEmbedding(ctx context.Context) HealthItem {
	return checkHTTPProvider(ctx, "embedding", os.Getenv("EMBED_BASE_URL"), os.Getenv("EMBED_API_KEY"))
}

func (s *HealthService) checkWebSocketHub(ctx context.Context) HealthItem {
	item := HealthItem{Name: "websocket_hub", CheckedAt: time.Now(), Status: HealthDown}
	if s.hub == nil {
		item.Error = "WebSocket Hub ????"
		return item
	}
	stats := s.hub.Stats()
	_ = stats
	item.Status = HealthHealthy
	return item
}

func checkHTTPProvider(ctx context.Context, name, baseURL, apiKey string) HealthItem {
	start := time.Now()
	item := HealthItem{Name: name, CheckedAt: time.Now(), Status: HealthDown}
	if baseURL == "" {
		item.Error = "Provider URL ???"
		return item
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	defer resp.Body.Close()
	item.LatencyMS = latencyMS(start)
	switch {
	case resp.StatusCode < 400:
		item.Status = HealthHealthy
	case resp.StatusCode < 500:
		item.Status = HealthDegraded
		item.Error = "HTTP " + resp.Status
	default:
		item.Status = HealthDown
		item.Error = "HTTP " + resp.Status
	}
	return item
}

func latencyMS(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}
