package models

import "time"

// ModerationEvent AI ??????????????????? Dashboard ???????
type ModerationEvent struct {
	ID            int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	MessageID     int64      `gorm:"type:bigint;index:idx_moderation_message;not null;default:0" json:"message_id"`
	RoomID        int64      `gorm:"type:bigint;index:idx_moderation_room;not null;default:0" json:"room_id"`
	UserID        int64      `gorm:"type:bigint;index:idx_moderation_user;not null;default:0" json:"user_id"`
	Username      string     `gorm:"type:varchar(64);default:''" json:"username"`
	RoomName      string     `gorm:"type:varchar(128);default:''" json:"room_name"`
	OriginalMsg   string     `gorm:"type:text" json:"original_msg"`
	Category      string     `gorm:"type:varchar(32);index:idx_moderation_category;default:'other'" json:"category"`
	RiskLevel     string     `gorm:"type:varchar(16);index:idx_moderation_risk;default:'low'" json:"risk_level"`
	RiskScore     float64    `gorm:"type:double;default:0" json:"risk_score"`
	ModelName     string     `gorm:"type:varchar(64);default:''" json:"model_name"`
	ModelResult   string     `gorm:"type:varchar(32);default:''" json:"model_result"`
	ModelReason   string     `gorm:"type:text" json:"model_reason"`
	ToolParams    string     `gorm:"type:text" json:"tool_params"`
	ToolResult    string     `gorm:"type:text" json:"tool_result"`
	PenaltyStatus string     `gorm:"type:varchar(32);index:idx_moderation_penalty;default:'none'" json:"penalty_status"`
	ReviewStatus  string     `gorm:"type:varchar(32);index:idx_moderation_review;default:'pending'" json:"review_status"`
	ReviewedBy    int64      `gorm:"type:bigint;default:0" json:"reviewed_by"`
	ReviewedAt    *time.Time `gorm:"type:datetime(3)" json:"reviewed_at"`
	CreatedAt     time.Time  `gorm:"index:idx_moderation_created" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName ???????????
func (ModerationEvent) TableName() string {
	return "moderation_events"
}
