package models

import "time"

// AdminAuditLog 超级管理员操作审计日志，记录所有管理后台的敏感操作。
type AdminAuditLog struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`                                        // 主键自增 ID，审计日志唯一标识
	AdminUserID   int64     `gorm:"type:bigint;index:idx_admin_audit_admin;not null" json:"admin_user_id"`     // 执行操作的管理员用户 ID，建立索引便于按管理员查询
	AdminUsername string    `gorm:"type:varchar(64);not null" json:"admin_username"`                           // 执行操作的管理员用户名（冗余存储）
	Action        string    `gorm:"type:varchar(64);index:idx_admin_audit_action;not null" json:"action"`      // 操作类型（如 ban_user、delete_message 等），建立索引便于按操作筛选
	TargetType    string    `gorm:"type:varchar(32);index:idx_admin_audit_target;not null" json:"target_type"` // 操作目标类型（如 user/room/message 等），建立索引便于按目标类型查询
	TargetID      string    `gorm:"type:varchar(64);not null;default:''" json:"target_id"`                     // 操作目标 ID（如用户 ID、聊天室 ID 等）
	BeforeData    string    `gorm:"type:mediumtext" json:"before_data"`                                        // 操作前的数据快照（JSON 字符串）
	AfterData     string    `gorm:"type:mediumtext" json:"after_data"`                                         // 操作后的数据快照（JSON 字符串）
	RequestID     string    `gorm:"type:varchar(64);default:''" json:"request_id"`                             // 请求 ID，用于关联日志和链路追踪
	RemoteIP      string    `gorm:"type:varchar(64);default:''" json:"remote_ip"`                              // 操作者的远程 IP 地址
	UserAgent     string    `gorm:"type:varchar(255);default:''" json:"user_agent"`                            // 操作者的浏览器 User-Agent
	Result        string    `gorm:"type:varchar(16);default:'success'" json:"result"`                          // 操作结果（如 success/failure）
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`                                            // 操作失败时的错误信息
	CreatedAt     time.Time `gorm:"index:idx_admin_audit_created" json:"created_at"`                           // 审计日志创建时间，建立索引便于按时间范围检索
}

// TableName 返回该结构体对应的数据库表名
func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
