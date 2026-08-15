package models

import "time"

// AlertEvent \u544a\u8b66\u4e2d\u5fc3\u4e8b\u4ef6\uff0c\u7531\u540e\u53f0\u5b9a\u671f\u6216\u8c03\u7528 Evaluate \u751f\u6210\u3002
type AlertEvent struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string     `gorm:"type:varchar(128);uniqueIndex:uk_alert_name;not null" json:"name"`
	Level      string     `gorm:"type:varchar(16);index:idx_alert_level;not null" json:"level"`
	Source     string     `gorm:"type:varchar(64);index:idx_alert_source;not null" json:"source"`
	Message    string     `gorm:"type:text" json:"message"`
	Status     string     `gorm:"type:varchar(32);uniqueIndex:uk_alert_name;index:idx_alert_status;not null;default:'unresolved'" json:"status"`
	ResolvedAt *time.Time `gorm:"type:datetime(3)" json:"resolved_at"`
	CreatedAt  time.Time  `gorm:"index:idx_alert_created" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AlertEvent) TableName() string {
	return "alert_events"
}
