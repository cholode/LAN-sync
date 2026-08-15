package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// User 用户表
type User struct {
	ID           int64                 `gorm:"primaryKey;autoIncrement;comment:'内部流转使用的UID'"`
	Username     string                `gorm:"type:varchar(64);uniqueIndex:idx_username_deleted,priority:1;not null;comment:'登录用户名'"`
	Password     string                `gorm:"type:varchar(255);not null;comment:'Bcrypt哈希密码'"`
	Avatar       string                `gorm:"type:varchar(255);default:'';comment:'用户头像URL'"`
	Role         int8                  `gorm:"type:tinyint;not null;default:0;comment:'0:???? 1:?? 2:??? 3:??'"`
	IsBot        bool                  `gorm:"type:tinyint(1);default:0;comment:'是否为Bot/AI用户'"`
	Status       int8                  `gorm:"type:tinyint;not null;default:0;comment:'0:?? 1:??'"`
	LastLoginAt  time.Time             `gorm:"type:datetime(3);comment:'??????'"`
	LastActiveAt time.Time             `gorm:"type:datetime(3);comment:'??????'"`
	CreatedAt    time.Time             `gorm:"index:idx_users_created;comment:'创建时间'"`
	UpdatedAt    time.Time             `gorm:"comment:'更新时间'"`
	DeletedAt    soft_delete.DeletedAt `gorm:"type:bigint unsigned;uniqueIndex:idx_username_deleted,priority:2;softDelete:milli;comment:'毫秒级软删除标记'"`
}
