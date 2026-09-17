package models

import "time"

// AlertEvent 告警中心事件，由后台定期或调用 Evaluate 生成。
type AlertEvent struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`                                                                            // 主键自增 ID，告警事件唯一标识
	Name       string     `gorm:"type:varchar(128);uniqueIndex:uk_alert_name;not null" json:"name"`                                              // 告警名称，与 Status 组成联合唯一索引（同一名称同时只能有一个未解决事件）
	Level      string     `gorm:"type:varchar(16);index:idx_alert_level;not null" json:"level"`                                                  // 告警级别（如 info/warning/critical），建立索引便于按级别筛选
	Source     string     `gorm:"type:varchar(64);index:idx_alert_source;not null" json:"source"`                                                // 告警来源（如系统模块名称：agent/moderation/rag 等），建立索引便于按来源统计
	Message    string     `gorm:"type:text" json:"message"`                                                                                      // 告警详细信息描述
	Status     string     `gorm:"type:varchar(32);uniqueIndex:uk_alert_name;index:idx_alert_status;not null;default:'unresolved'" json:"status"` // 告警状态（如 unresolved/resolved），与 Name 组成联合唯一索引
	ResolvedAt *time.Time `gorm:"type:datetime(3)" json:"resolved_at"`                                                                           // 告警解决时间（指针类型，可为空表示未解决）
	CreatedAt  time.Time  `gorm:"index:idx_alert_created" json:"created_at"`                                                                     // 告警创建时间，建立索引便于按时间范围检索
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`                                                                              // 记录最后更新时间，由数据库自动维护
}

// TableName 返回该结构体对应的数据库表名
func (AlertEvent) TableName() string {
	return "alert_events"
}
