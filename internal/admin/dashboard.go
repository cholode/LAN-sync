package admin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/internal/metrics"
	"lan-im-go/models"
)

const dashboardCacheTTL = 30 * time.Second

// DashboardService 聚合管理员首页所需的用户、房间、消息、实时运行状态等数据。
// 所有数据库访问都集中在 service 层，避免 Handler 直接接触 SQL。
type DashboardService struct {
	db           *gorm.DB
	messageStore MessageStatsStore
	hub          *core.Hub
	moderation   *ModerationService
	rag          *RAGService
	health       *HealthService

	mu            sync.Mutex
	overviewCache *dashboardOverviewCache
	trafficCache  *messageTrafficCache
}

type dashboardOverviewCache struct {
	data      *DashboardOverview
	expiresAt time.Time
}

type messageTrafficCache struct {
	data      *MessageTraffic
	expiresAt time.Time
}

// NewDashboardService 创建管理员首页聚合服务。
// messageStore 用于区分 MySQL 与 MongoDB 两种消息存储实现。
func NewDashboardService(
	db *gorm.DB,
	messageCollection *mongo.Collection,
	messageStore string,
	hub *core.Hub,
	moderation *ModerationService,
	rag *RAGService,
	health *HealthService,
) *DashboardService {
	var store MessageStatsStore
	if messageStore == "mongo" && messageCollection != nil {
		store = newMongoMessageStats(messageCollection, db)
	} else {
		store = newMySQLMessageStats(db)
	}
	return &DashboardService{
		db:           db,
		messageStore: store,
		hub:          hub,
		moderation:   moderation,
		rag:          rag,
		health:       health,
	}
}

// MetricPoint 表示一个统计指标及其趋势。
type MetricPoint struct {
	Value          int64   `json:"value"`
	YesterdayValue int64   `json:"yesterday_value"`
	GrowthRate     float64 `json:"growth_rate"`
	Trend          []int64 `json:"trend"`
}

// OverviewSection 管理员首页核心业务数据。
type OverviewSection struct {
	Users    UserOverview     `json:"users"`
	Rooms    RoomOverview     `json:"rooms"`
	Messages MessageOverview  `json:"messages"`
	Realtime RealtimeOverview `json:"realtime"`
}

type UserOverview struct {
	Total            int64 `json:"total"`
	NewToday         int64 `json:"new_today"`
	NewLast7Days     int64 `json:"new_last_7_days"`
	NewLast30Days    int64 `json:"new_last_30_days"`
	Online           int64 `json:"online"`
	ActiveToday      int64 `json:"active_today"`
	ActiveLast7Days  int64 `json:"active_last_7_days"`
	ActiveLast30Days int64 `json:"active_last_30_days"`
}

type RoomOverview struct {
	Total    int64 `json:"total"`
	NewToday int64 `json:"new_today"`
}

type MessageOverview struct {
	Today        int64 `json:"today"`
	PrivateToday int64 `json:"private_today"`
	GroupToday   int64 `json:"group_today"`
	FileToday    int64 `json:"file_today"`
	AgentToday   int64 `json:"agent_today"`
}

type RealtimeOverview struct {
	WebSocketConnections int64 `json:"websocket_connections"`
}

// DashboardOverview 管理员首页聚合返回体。
type DashboardOverview struct {
	GeneratedAt time.Time                    `json:"generated_at"`
	Sections    OverviewSection              `json:"sections"`
	Websocket   metrics.WebSocketRuntime     `json:"websocket"`
	Moderation  ModerationOverview           `json:"moderation"`
	Agent       metrics.AgentRuntimeSnapshot `json:"agent"`
	RAG         *RAGDashboard                `json:"rag"`
	System      []HealthItem                 `json:"system"`
}

// CoreOverview 聚合首页核心数据，并通过内存缓存减少频繁打开首页的数据库压力。
func (s *DashboardService) CoreOverview(ctx context.Context) (*DashboardOverview, error) {
	if cached, ok := s.getOverviewCache(); ok {
		return cached, nil
	}

	data, err := s.buildCoreOverview(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.overviewCache = &dashboardOverviewCache{data: data, expiresAt: time.Now().Add(dashboardCacheTTL)}
	s.mu.Unlock()
	return data, nil
}

func (s *DashboardService) getOverviewCache() (*DashboardOverview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overviewCache == nil || time.Now().After(s.overviewCache.expiresAt) {
		return nil, false
	}
	return s.overviewCache.data, true
}

func (s *DashboardService) buildCoreOverview(ctx context.Context) (*DashboardOverview, error) {
	now := time.Now()
	todayStart := startOfDay(now)
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	weekStart := todayStart.AddDate(0, 0, -6)
	monthStart := todayStart.AddDate(0, 0, -29)

	users, err := s.userOverview(ctx, now, todayStart, yesterdayStart, weekStart, monthStart)
	if err != nil {
		return nil, err
	}

	rooms, err := s.roomOverview(ctx, todayStart)
	if err != nil {
		return nil, err
	}

	messages, err := s.messageOverview(ctx, todayStart, now)
	if err != nil {
		return nil, err
	}

	ws := metrics.RuntimeSnapshotNow().WebSocket
	if s.hub != nil {
		stats := s.hub.Stats()
		ws.CurrentConnections = int64(stats.ClientCount)
	}

	moderation := ModerationOverview{}
	if s.moderation != nil {
		moderation, err = s.moderation.Overview(ctx)
		if err != nil {
			return nil, err
		}
	}

	rag := &RAGDashboard{}
	if s.rag != nil {
		rag, err = s.rag.Dashboard(ctx)
		if err != nil {
			return nil, err
		}
	}

	system := []HealthItem{}
	if s.health != nil {
		system = s.health.CheckAll(ctx)
	}

	return &DashboardOverview{
		GeneratedAt: now,
		Sections: OverviewSection{
			Users:    users,
			Rooms:    rooms,
			Messages: messages,
			Realtime: RealtimeOverview{WebSocketConnections: ws.CurrentConnections},
		},
		Websocket:  ws,
		Moderation: moderation,
		Agent:      metrics.AgentRuntimeSnapshotNow(),
		RAG:        rag,
		System:     system,
	}, nil
}

func (s *DashboardService) userOverview(ctx context.Context, now, todayStart, yesterdayStart, weekStart, monthStart time.Time) (UserOverview, error) {
	var out UserOverview

	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("is_bot = ?", false).Count(&out.Total).Error; err != nil {
		return out, err
	}
	newCounts := []struct {
		Since time.Time
		Out   *int64
	}{
		{Since: todayStart, Out: &out.NewToday},
		{Since: weekStart, Out: &out.NewLast7Days},
		{Since: monthStart, Out: &out.NewLast30Days},
	}
	for _, item := range newCounts {
		if err := s.db.WithContext(ctx).Model(&models.User{}).Where("is_bot = ? AND created_at >= ?", false, item.Since).Count(item.Out).Error; err != nil {
			return out, err
		}
	}

	out.Online = s.onlineUserCount(ctx)

	activeCounts := []struct {
		Since time.Time
		Out   *int64
	}{
		{Since: todayStart, Out: &out.ActiveToday},
		{Since: weekStart, Out: &out.ActiveLast7Days},
		{Since: monthStart, Out: &out.ActiveLast30Days},
	}
	for _, item := range activeCounts {
		count, err := s.messageStore.ActiveSenders(ctx, item.Since)
		if err != nil {
			return out, err
		}
		*item.Out = count
	}

	return out, nil
}

func (s *DashboardService) roomOverview(ctx context.Context, todayStart time.Time) (RoomOverview, error) {
	var out RoomOverview
	if err := s.db.WithContext(ctx).Model(&models.Room{}).Where("type = ?", 2).Count(&out.Total).Error; err != nil {
		return out, err
	}
	if err := s.db.WithContext(ctx).Model(&models.Room{}).Where("type = ? AND created_at >= ?", 2, todayStart).Count(&out.NewToday).Error; err != nil {
		return out, err
	}
	return out, nil
}

func (s *DashboardService) messageOverview(ctx context.Context, todayStart, now time.Time) (MessageOverview, error) {
	var out MessageOverview
	count, err := s.messageStore.CountMessages(ctx, todayStart, now)
	if err != nil {
		return out, err
	}
	out.Today = count

	privateCount, groupCount, err := s.messageStore.CountPrivateGroupMessages(ctx, todayStart, now)
	if err != nil {
		return out, err
	}
	out.PrivateToday = privateCount
	out.GroupToday = groupCount

	byType, err := s.messageStore.CountMessagesByType(ctx, todayStart, now)
	if err != nil {
		return out, err
	}
	out.FileToday = byType[2]

	agentCount, err := s.messageStore.CountAgentMentions(ctx, todayStart, now)
	if err != nil {
		return out, err
	}
	out.AgentToday = agentCount

	return out, nil
}

func (s *DashboardService) onlineUserCount(ctx context.Context) int64 {
	iterator := config.RedisClient.Scan(ctx, 0, "im:user:online:*", 500).Iterator()
	var count int64
	for iterator.Next(ctx) {
		count++
	}
	return count
}

// MessageTraffic 返回消息流量图表数据，并通过缓存避免多个图表重复查询。
func (s *DashboardService) MessageTraffic(ctx context.Context) (*MessageTraffic, error) {
	if cached, ok := s.getTrafficCache(); ok {
		return cached, nil
	}

	data, err := s.buildMessageTraffic(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.trafficCache = &messageTrafficCache{data: data, expiresAt: time.Now().Add(dashboardCacheTTL)}
	s.mu.Unlock()
	return data, nil
}

func (s *DashboardService) getTrafficCache() (*MessageTraffic, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.trafficCache == nil || time.Now().After(s.trafficCache.expiresAt) {
		return nil, false
	}
	return s.trafficCache.data, true
}

// DashboardTimeSeries 用于前端图表的时间序列数据。
type DashboardTimeSeries struct {
	Metric      string      `json:"metric"`
	Period      string      `json:"period"`
	Points      []TimeCount `json:"points"`
	GeneratedAt time.Time   `json:"generated_at"`
}

// TimeSeries 返回指定指标的时间序列数据。
// 目前支持 metric=messages，period 支持 1h、24h、7d、30d。
func (s *DashboardService) TimeSeries(ctx context.Context, metric, period string) (*DashboardTimeSeries, error) {
	if metric != "messages" {
		return nil, fmt.Errorf("暂不支持的指标: %s", metric)
	}

	now := time.Now()
	window, hourly, err := timeSeriesWindow(period)
	if err != nil {
		return nil, err
	}

	var points []TimeCount
	if hourly {
		points, err = s.messageStore.HourlyCounts(ctx, now.Add(-window), now)
	} else {
		points, err = s.messageStore.DailyCounts(ctx, now.Add(-window), now)
	}
	if err != nil {
		return nil, err
	}

	return &DashboardTimeSeries{
		Metric:      metric,
		Period:      period,
		Points:      points,
		GeneratedAt: now,
	}, nil
}

func timeSeriesWindow(period string) (time.Duration, bool, error) {
	switch period {
	case "1h":
		return time.Hour, true, nil
	case "24h":
		return 24 * time.Hour, true, nil
	case "7d":
		return 7 * 24 * time.Hour, false, nil
	case "30d":
		return 30 * 24 * time.Hour, false, nil
	default:
		return 24 * time.Hour, true, nil
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
