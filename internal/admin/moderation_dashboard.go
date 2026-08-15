package admin

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// ModerationService ?????? Dashboard ??????????
type ModerationService struct {
	db    *gorm.DB
	audit *AuditService
}

// NewModerationService ?????????
func NewModerationService(db *gorm.DB, audit *AuditService) *ModerationService {
	return &ModerationService{db: db, audit: audit}
}

// ModerationDashboard ???? Dashboard ?????
type ModerationDashboard struct {
	TodayReviewed     int64                 `json:"today_reviewed"`
	TodayViolations   int64                 `json:"today_violations"`
	ViolationRate     float64               `json:"violation_rate"`
	AutoKickCount     int64                 `json:"auto_kick_count"`
	AutoBanCount      int64                 `json:"auto_ban_count"`
	ManualReviewCount int64                 `json:"manual_review_count"`
	RevokedCount      int64                 `json:"revoked_count"`
	ToolFailureCount  int64                 `json:"tool_failure_count"`
	CategoryStats     []CategoryCount       `json:"category_stats"`
	RecentViolations  []ModerationEventItem `json:"recent_violations"`
	GeneratedAt       time.Time             `json:"generated_at"`
}

// CategoryCount ???????
type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// ModerationEventItem ????????????
type ModerationEventItem struct {
	ID            int64     `json:"id"`
	MessageID     int64     `json:"message_id"`
	RoomID        int64     `json:"room_id"`
	UserID        int64     `json:"user_id"`
	Username      string    `json:"username"`
	RoomName      string    `json:"room_name"`
	OriginalMsg   string    `json:"original_msg"`
	Category      string    `json:"category"`
	RiskLevel     string    `json:"risk_level"`
	RiskScore     float64   `json:"risk_score"`
	ModelName     string    `json:"model_name"`
	ModelResult   string    `json:"model_result"`
	ModelReason   string    `json:"model_reason"`
	ToolParams    string    `json:"tool_params"`
	ToolResult    string    `json:"tool_result"`
	PenaltyStatus string    `json:"penalty_status"`
	ReviewStatus  string    `json:"review_status"`
	CreatedAt     time.Time `json:"created_at"`
}

// ModerationOverview 用于管理员首页的轻量级审核概览。
type ModerationOverview struct {
	TodayReviewed   int64     `json:"today_reviewed"`
	TodayViolations int64     `json:"today_violations"`
	ViolationRate   float64   `json:"violation_rate"`
	PendingReviews  int64     `json:"pending_reviews"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// Overview 通过一次聚合查询返回首页需要的审核核心指标。
func (s *ModerationService) Overview(ctx context.Context) (ModerationOverview, error) {
	start := startOfDay(time.Now())
	var row struct {
		Total      int64 `gorm:"column:total"`
		Violations int64 `gorm:"column:violations"`
		Pending    int64 `gorm:"column:pending"`
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN model_result <> ? THEN 1 ELSE 0 END), 0) AS violations,
			COALESCE(SUM(CASE WHEN review_status = ? THEN 1 ELSE 0 END), 0) AS pending`, "safe", "pending").
		Where("created_at >= ?", start).
		Scan(&row).Error; err != nil {
		return ModerationOverview{}, err
	}

	var rate float64
	if row.Total > 0 {
		rate = float64(row.Violations) / float64(row.Total)
	}
	return ModerationOverview{
		TodayReviewed:   row.Total,
		TodayViolations: row.Violations,
		ViolationRate:   rate,
		PendingReviews:  row.Pending,
		GeneratedAt:     time.Now(),
	}, nil
}

// Dashboard 聚合返回审核 Dashboard 数据，避免对同一张表执行多次 COUNT。
func (s *ModerationService) Dashboard(ctx context.Context) (*ModerationDashboard, error) {
	start := startOfDay(time.Now())

	var row struct {
		Total         int64 `gorm:"column:total"`
		Violations    int64 `gorm:"column:violations"`
		Kicks         int64 `gorm:"column:kicks"`
		Bans          int64 `gorm:"column:bans"`
		ManualReviews int64 `gorm:"column:manual_reviews"`
		Revoked       int64 `gorm:"column:revoked"`
		ToolFailures  int64 `gorm:"column:tool_failures"`
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN model_result <> ? THEN 1 ELSE 0 END), 0) AS violations,
			COALESCE(SUM(CASE WHEN penalty_status = ? THEN 1 ELSE 0 END), 0) AS kicks,
			COALESCE(SUM(CASE WHEN penalty_status = ? THEN 1 ELSE 0 END), 0) AS bans,
			COALESCE(SUM(CASE WHEN review_status <> ? THEN 1 ELSE 0 END), 0) AS manual_reviews,
			COALESCE(SUM(CASE WHEN review_status = ? THEN 1 ELSE 0 END), 0) AS revoked,
			COALESCE(SUM(CASE WHEN tool_result LIKE ? THEN 1 ELSE 0 END), 0) AS tool_failures`,
			"safe", "kicked", "banned", "pending", "revoked", "%失败%").
		Where("created_at >= ?", start).
		Scan(&row).Error; err != nil {
		return nil, err
	}

	var categoryRows []CategoryCount
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Select("category, COUNT(*) AS count").
		Where("created_at >= ?", start).
		Group("category").
		Order("count DESC").
		Scan(&categoryRows).Error; err != nil {
		return nil, err
	}

	var recent []models.ModerationEvent
	if err := s.db.WithContext(ctx).Where("model_result <> ?", "safe").
		Order("created_at DESC").
		Limit(20).
		Find(&recent).Error; err != nil {
		return nil, err
	}

	var violationRate float64
	if row.Total > 0 {
		violationRate = float64(row.Violations) / float64(row.Total)
	}

	return &ModerationDashboard{
		TodayReviewed:     row.Total,
		TodayViolations:   row.Violations,
		ViolationRate:     violationRate,
		AutoKickCount:     row.Kicks,
		AutoBanCount:      row.Bans,
		ManualReviewCount: row.ManualReviews,
		RevokedCount:      row.Revoked,
		ToolFailureCount:  row.ToolFailures,
		CategoryStats:     categoryRows,
		RecentViolations:  moderationItems(recent),
		GeneratedAt:       time.Now(),
	}, nil
}

func moderationItems(events []models.ModerationEvent) []ModerationEventItem {
	out := make([]ModerationEventItem, 0, len(events))
	for _, item := range events {
		out = append(out, moderationItem(item))
	}
	return out
}

func moderationItem(item models.ModerationEvent) ModerationEventItem {
	return ModerationEventItem{
		ID:            item.ID,
		MessageID:     item.MessageID,
		RoomID:        item.RoomID,
		UserID:        item.UserID,
		Username:      item.Username,
		RoomName:      item.RoomName,
		OriginalMsg:   item.OriginalMsg,
		Category:      item.Category,
		RiskLevel:     item.RiskLevel,
		RiskScore:     item.RiskScore,
		ModelName:     item.ModelName,
		ModelResult:   item.ModelResult,
		ModelReason:   item.ModelReason,
		ToolParams:    item.ToolParams,
		ToolResult:    item.ToolResult,
		PenaltyStatus: item.PenaltyStatus,
		ReviewStatus:  item.ReviewStatus,
		CreatedAt:     item.CreatedAt,
	}
}

// ModerationListQuery ?????????
type ModerationListQuery struct {
	Page          int
	PageSize      int
	Username      string
	UserID        int64
	RoomID        int64
	Category      string
	RiskLevel     string
	PenaltyStatus string
	Start         time.Time
	End           time.Time
}

// ListEvents ?????????
func (s *ModerationService) ListEvents(ctx context.Context, q ModerationListQuery) ([]ModerationEventItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.ModerationEvent{})
	if q.UserID > 0 {
		query = query.Where("user_id = ?", q.UserID)
	}
	if q.RoomID > 0 {
		query = query.Where("room_id = ?", q.RoomID)
	}
	if q.Username != "" {
		query = query.Where("username LIKE ?", q.Username+"%")
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}
	if q.RiskLevel != "" {
		query = query.Where("risk_level = ?", q.RiskLevel)
	}
	if q.PenaltyStatus != "" {
		query = query.Where("penalty_status = ?", q.PenaltyStatus)
	}
	if !q.Start.IsZero() {
		query = query.Where("created_at >= ?", q.Start)
	}
	if !q.End.IsZero() {
		query = query.Where("created_at < ?", q.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var events []models.ModerationEvent
	if err := query.Order("created_at DESC").
		Offset((q.Page - 1) * q.PageSize).
		Limit(q.PageSize).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return moderationItems(events), total, nil
}

// GetEvent ???????????
func (s *ModerationService) GetEvent(ctx context.Context, id int64) (*ModerationEventItem, error) {
	var event models.ModerationEvent
	if err := s.db.WithContext(ctx).First(&event, id).Error; err != nil {
		return nil, err
	}
	item := moderationItem(event)
	return &item, nil
}

// ModerationAction ????????
type ModerationAction struct {
	Action      string
	AdminUserID int64
	AdminName   string
	RequestID   string
	RemoteIP    string
	UserAgent   string
}

// ApplyAction ?????????/????????????
func (s *ModerationService) ApplyAction(ctx context.Context, id int64, action ModerationAction) error {
	var event models.ModerationEvent
	if err := s.db.WithContext(ctx).First(&event, id).Error; err != nil {
		return err
	}
	before := moderationItem(event)

	switch action.Action {
	case "warn":
		event.PenaltyStatus = "warned"
		event.ReviewStatus = "reviewed"
	case "mute":
		event.PenaltyStatus = "muted"
		event.ReviewStatus = "reviewed"
	case "kick":
		event.PenaltyStatus = "kicked"
		event.ReviewStatus = "reviewed"
	case "ban":
		event.PenaltyStatus = "banned"
		event.ReviewStatus = "reviewed"
	case "revoke":
		event.PenaltyStatus = "none"
		event.ReviewStatus = "revoked"
	case "false_positive":
		event.ReviewStatus = "false_positive"
		event.PenaltyStatus = "none"
	case "confirmed":
		event.ReviewStatus = "confirmed"
	default:
		event.ReviewStatus = "reviewed"
	}

	event.ReviewedBy = action.AdminUserID
	now := time.Now()
	event.ReviewedAt = &now

	if err := s.db.WithContext(ctx).Save(&event).Error; err != nil {
		return err
	}

	if s.audit != nil {
		_ = s.audit.Log(ctx, AuditEntry{
			AdminUserID:   action.AdminUserID,
			AdminUsername: action.AdminName,
			Action:        "moderation." + action.Action,
			TargetType:    "moderation_event",
			TargetID:      strconv.FormatInt(id, 10),
			BeforeData:    before,
			AfterData:     moderationItem(event),
			RequestID:     action.RequestID,
			RemoteIP:      action.RemoteIP,
			UserAgent:     action.UserAgent,
			Result:        "success",
		})
	}
	return nil
}
