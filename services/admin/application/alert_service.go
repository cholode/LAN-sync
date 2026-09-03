package admin

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
	"lan-im-go/shared/observability/metrics"
)

// AlertService 负责生成、查询和处理 Dashboard 告警。
type AlertService struct {
	db      *gorm.DB
	health  *HealthService
	runtime RuntimeController
}

func NewAlertService(db *gorm.DB, health *HealthService, runtime RuntimeController) *AlertService {
	return &AlertService{db: db, health: health, runtime: runtime}
}

const (
	AlertLevelInfo     = "info"
	AlertLevelWarning  = "warning"
	AlertLevelCritical = "critical"
)

// AlertListQuery 告警查询条件。
type AlertListQuery struct {
	Page     int
	PageSize int
	Level    string
	Status   string
}

// List 分页查询告警。
func (s *AlertService) List(ctx context.Context, q AlertListQuery) ([]models.AlertEvent, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.AlertEvent{})
	if q.Level != "" {
		query = query.Where("level = ?", q.Level)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.AlertEvent
	if err := query.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// UnresolvedCount 统计未处理告警数量。
func (s *AlertService) UnresolvedCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.AlertEvent{}).Where("status = ?", "unresolved").Count(&count).Error
	return count, err
}

// Resolve 标记告警已处理。
func (s *AlertService) Resolve(ctx context.Context, id int64) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.AlertEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      "resolved",
		"resolved_at": now,
	}).Error
}

// Evaluate 基于实时 metrics 和健康检查结果刷新告警。
func (s *AlertService) Evaluate(ctx context.Context) ([]models.AlertEvent, error) {
	candidates := s.buildCandidates(ctx)
	for _, candidate := range candidates {
		if err := s.upsert(ctx, candidate); err != nil {
			return nil, err
		}
	}
	var alerts []models.AlertEvent
	if err := s.db.WithContext(ctx).Where("status = ?", "unresolved").Order("id DESC").Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

func (s *AlertService) buildCandidates(ctx context.Context) []models.AlertEvent {
	var alerts []models.AlertEvent
	runtimeSnap := metrics.RuntimeSnapshotNow()
	agentSnap := metrics.AgentRuntimeSnapshotNow()
	if s.runtime != nil {
		if remoteRuntime, remoteAgent, err := s.runtime.RuntimeSnapshots(ctx); err == nil {
			runtimeSnap = remoteRuntime
			agentSnap = remoteAgent
		}
	}

	if runtimeSnap.API.ErrorRate >= 0.10 {
		alerts = append(alerts, newAlert("api_5xx_rate", AlertLevelCritical, "api", fmt.Sprintf("API 错误率 %.2f%% 超过 10%%", runtimeSnap.API.ErrorRate*100)))
	} else if runtimeSnap.API.ErrorRate >= 0.05 {
		alerts = append(alerts, newAlert("api_5xx_rate", AlertLevelWarning, "api", fmt.Sprintf("API 错误率 %.2f%% 超过 5%%", runtimeSnap.API.ErrorRate*100)))
	}

	if runtimeSnap.WebSocket.AbnormalClosedTotal > 100 {
		alerts = append(alerts, newAlert("ws_abnormal_close", AlertLevelWarning, "websocket", fmt.Sprintf("WebSocket 异常断开次数 %d 次", runtimeSnap.WebSocket.AbnormalClosedTotal)))
	}
	if runtimeSnap.WebSocket.SendQueueBacklog > 10000 {
		alerts = append(alerts, newAlert("ws_send_backlog", AlertLevelWarning, "websocket", fmt.Sprintf("WebSocket 发送积压数 %d", runtimeSnap.WebSocket.SendQueueBacklog)))
	}

	if runtimeSnap.Golang.Goroutines > 50000 {
		alerts = append(alerts, newAlert("goroutines_high", AlertLevelCritical, "runtime", fmt.Sprintf("Goroutine 数量过高 %d", runtimeSnap.Golang.Goroutines)))
	} else if runtimeSnap.Golang.Goroutines > 10000 {
		alerts = append(alerts, newAlert("goroutines_high", AlertLevelWarning, "runtime", fmt.Sprintf("Goroutine 数量偏高 %d", runtimeSnap.Golang.Goroutines)))
	}

	if runtimeSnap.Golang.HeapAlloc > 2*1024*1024*1024 {
		alerts = append(alerts, newAlert("heap_high", AlertLevelCritical, "runtime", fmt.Sprintf("Heap Alloc %.2f GB 超过 2GB", float64(runtimeSnap.Golang.HeapAlloc)/(1024*1024*1024))))
	} else if runtimeSnap.Golang.HeapAlloc > 1024*1024*1024 {
		alerts = append(alerts, newAlert("heap_high", AlertLevelWarning, "runtime", fmt.Sprintf("Heap Alloc %.2f GB 超过 1GB", float64(runtimeSnap.Golang.HeapAlloc)/(1024*1024*1024))))
	}

	if agentSnap.FailureRate >= 0.20 {
		alerts = append(alerts, newAlert("agent_failure_rate", AlertLevelCritical, "agent", fmt.Sprintf("Agent API 失败率 %.2f%% 超过 20%%", agentSnap.FailureRate*100)))
	} else if agentSnap.FailureRate >= 0.10 {
		alerts = append(alerts, newAlert("agent_failure_rate", AlertLevelWarning, "agent", fmt.Sprintf("Agent API 失败率 %.2f%% 超过 10%%", agentSnap.FailureRate*100)))
	}
	if agentSnap.EmbeddingFailures > 50 {
		alerts = append(alerts, newAlert("embedding_failures", AlertLevelWarning, "embedding", fmt.Sprintf("Embedding 失败次数 %d 次", agentSnap.EmbeddingFailures)))
	}

	if s.health != nil {
		for _, health := range s.health.CheckAll(ctx) {
			if health.Status == HealthDown {
				alerts = append(alerts, newAlert(health.Name+"_down", AlertLevelCritical, health.Name, health.Name+" 不可用"+health.Error))
			} else if health.Status == HealthDegraded {
				alerts = append(alerts, newAlert(health.Name+"_degraded", AlertLevelWarning, health.Name, health.Name+" 已降级"+health.Error))
			}
		}
	}
	return alerts
}

func (s *AlertService) upsert(ctx context.Context, candidate models.AlertEvent) error {
	var existing models.AlertEvent
	err := s.db.WithContext(ctx).Where("name = ?", candidate.Name).First(&existing).Error
	if err == nil {
		return s.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"level":       candidate.Level,
			"source":      candidate.Source,
			"message":     candidate.Message,
			"status":      "unresolved",
			"resolved_at": nil,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return s.db.WithContext(ctx).Create(&candidate).Error
}

func newAlert(name, level, source, message string) models.AlertEvent {
	return models.AlertEvent{
		Name:    name,
		Level:   level,
		Source:  source,
		Message: message,
		Status:  "unresolved",
	}
}
