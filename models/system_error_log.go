package models

import "time"

// SystemErrorLog \u96c6\u4e2d\u8bb0\u5f55\u7cfb\u7edf\u8fd0\u884c\u8fc7\u7a0b\u4e2d\u7684\u5f02\u5e38\u4e8b\u4ef6\uff0c\u7528\u4e8e\u8d85\u7ea7\u7ba1\u7406\u5458\u540e\u53f0\u6392\u67e5\u3002
type SystemErrorLog struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Timestamp    time.Time  `gorm:"index:idx_system_error_time;not null" json:"timestamp"`
	Module       string     `gorm:"type:varchar(64);index:idx_system_error_module;not null" json:"module"`
	ErrorType    string     `gorm:"type:varchar(64);index:idx_system_error_type;not null" json:"error_type"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	RequestID    string     `gorm:"type:varchar(128);index:idx_system_error_request;default:''" json:"request_id"`
	UserID       int64      `gorm:"type:bigint;index:idx_system_error_user;not null;default:0" json:"user_id"`
	RoomID       int64      `gorm:"type:bigint;index:idx_system_error_room;not null;default:0" json:"room_id"`
	StackTrace   string     `gorm:"type:longtext" json:"stack_trace"`
	Resolved     bool       `gorm:"type:tinyint(1);index:idx_system_error_resolved;default:0" json:"resolved"`
	ResolvedBy   int64      `gorm:"type:bigint;default:0" json:"resolved_by"`
	ResolvedAt   *time.Time `gorm:"type:datetime(3)" json:"resolved_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (SystemErrorLog) TableName() string {
	return "system_error_logs"
}
