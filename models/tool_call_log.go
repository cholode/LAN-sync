package models

import "time"

// ToolCallLog \u8bb0\u5f55 Agent Tool Calling \u7684\u6267\u884c\u60c5\u51b5\uff0c\u7528\u4e8e\u8d85\u7ea7\u7ba1\u7406\u5458\u540e\u53f0\u6392\u67e5\u3002
type ToolCallLog struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ToolCallID     string    `gorm:"type:varchar(128);index:idx_tool_call_id;not null" json:"tool_call_id"`
	UserID         int64     `gorm:"type:bigint;index:idx_tool_call_user;not null;default:0" json:"user_id"`
	RoomID         int64     `gorm:"type:bigint;index:idx_tool_call_room;not null;default:0" json:"room_id"`
	AgentRequestID string    `gorm:"type:varchar(128);index:idx_tool_call_agent;default:''" json:"agent_request_id"`
	ToolName       string    `gorm:"type:varchar(64);index:idx_tool_call_name;not null" json:"tool_name"`
	Arguments      string    `gorm:"type:longtext" json:"arguments"`
	StartedAt      time.Time `gorm:"index:idx_tool_call_started" json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	LatencyMS      float64   `gorm:"type:double;default:0" json:"latency_ms"`
	Success        bool      `gorm:"type:tinyint(1);default:0" json:"success"`
	Error          string    `gorm:"type:text" json:"error"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ToolCallLog) TableName() string {
	return "tool_call_logs"
}
