package admin

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/qdrant/go-client/qdrant"
	"gorm.io/gorm"

	"lan-im-go/internal/metrics"
	"lan-im-go/internal/storage"
)

// HealthService 提供带超时的下游服务健康检测，避免 Dashboard 被单个服务拖死。
type HealthService struct {
	db      *gorm.DB
	redis   *redis.Client
	storage storage.Provider
	runtime RuntimeController
	qdrant  *qdrant.Client
}

func NewHealthService(db *gorm.DB, redisClient *redis.Client, provider storage.Provider, runtime RuntimeController, qdrantClients ...*qdrant.Client) *HealthService {
	var qdrantClient *qdrant.Client
	if len(qdrantClients) > 0 {
		qdrantClient = qdrantClients[0]
	}
	return &HealthService{db: db, redis: redisClient, storage: provider, runtime: runtime, qdrant: qdrantClient}
}

// HealthItem 单个下游服务的健康结果。
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

// CheckAll 并行检查所有下游服务。
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
		item.Error = "MySQL 未配置"
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
		item.Error = "Redis 未配置"
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
	if s.qdrant == nil {
		item.Error = "Qdrant 客户端未配置"
		return item
	}
	err := func() error {
		startRequest := time.Now()
		_, err := s.qdrant.ListCollections(ctx)
		metrics.ObserveQdrantRequest("list_collections", startRequest, err)
		return err
	}()
	if err != nil {
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
		item.Error = "对象存储未配置"
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
	if s.runtime == nil {
		item.Error = "WebSocket Hub 未配置"
		return item
	}
	stats, err := s.runtime.HubStats(ctx)
	if err != nil {
		item.Error = err.Error()
		return item
	}
	_ = stats
	item.Status = HealthHealthy
	return item
}

func checkHTTPProvider(ctx context.Context, name, baseURL, apiKey string) HealthItem {
	start := time.Now()
	item := HealthItem{Name: name, CheckedAt: time.Now(), Status: HealthDown}
	if baseURL == "" {
		item.Error = "Provider URL 未配置"
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
