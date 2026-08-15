package models

import "time"

// FileRecord 统一记录已上传到对象存储的文件元数据，
// 用于超级管理员后台进行检索、删除和异常检测。
type FileRecord struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ObjectKey    string    `gorm:"type:varchar(512);uniqueIndex:uk_file_object_key;not null" json:"object_key"`
	OriginalName string    `gorm:"type:varchar(255);not null" json:"original_name"`
	SHA256       string    `gorm:"type:varchar(64);default:''" json:"sha256"`
	Size         int64     `gorm:"type:bigint;default:0" json:"size"`
	UploaderID   int64     `gorm:"type:bigint;index:idx_file_uploader;not null;default:0" json:"uploader_id"`
	RoomID       int64     `gorm:"type:bigint;index:idx_file_room;not null;default:0" json:"room_id"`
	Backend      string    `gorm:"type:varchar(32);default:'minio'" json:"backend"`
	Status       string    `gorm:"type:varchar(32);index:idx_file_status;default:'uploaded'" json:"status"`
	MessageID    int64     `gorm:"type:bigint;default:0;index:idx_file_message" json:"message_id"`
	CreatedAt    time.Time `gorm:"index:idx_file_created" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (FileRecord) TableName() string {
	return "file_records"
}
