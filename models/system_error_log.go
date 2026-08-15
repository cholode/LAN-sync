package models

import "time"

// SystemErrorLog 集中记录系统运行过程中的异常事件，用于超级管理员后台排查。

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
