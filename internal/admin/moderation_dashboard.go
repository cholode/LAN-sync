package admin

import (
	"context"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// ModerationService ?????? Dashboard ??????????
type ModerationService struct {
	db *gorm.DB
}

// NewModerationService ?????????
func NewModerationService(db *gorm.DB) *ModerationService {
	return &ModerationService{db: db}
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

// Dashboard ???????????
func (s *ModerationService) Dashboard(ctx context.Context) (*ModerationDashboard, error) {
	start := startOfDay(time.Now())

	var total, violations, kicks, bans, manual, revoked, toolFailures int64
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ?", start).Count(&total).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND model_result <> ?", start, "safe").Count(&violations).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND penalty_status = ?", start, "kicked").Count(&kicks).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND penalty_status = ?", start, "banned").Count(&bans).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND review_status <> ?", start, "pending").Count(&manual).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND review_status = ?", start, "revoked").Count(&revoked).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&models.ModerationEvent{}).
		Where("created_at >= ? AND tool_result LIKE ?", start, "%??%").Count(&toolFailures).Error; err != nil {
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
	if total > 0 {
		violationRate = float64(violations) / float64(total)
	}

	return &ModerationDashboard{
		TodayReviewed:     total,
		TodayViolations:   violations,
		ViolationRate:     violationRate,
		AutoKickCount:     kicks,
		AutoBanCount:      bans,
		ManualReviewCount: manual,
		RevokedCount:      revoked,
		ToolFailureCount:  toolFailures,
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
