package models

import (
	"time"

	"gorm.io/plugin/soft_delete"
)

// Message 统一消息表（系统的绝对核心）
type Message struct {
	ID          int64                 `json:"id,string" gorm:"primaryKey;autoIncrement:false;index:idx_room_id_id,priority:2;comment:'编号器分配的消息ID'"`
	RoomSeq     int64                 `json:"room_seq,string" gorm:"default:null;uniqueIndex:idx_room_seq,priority:2;comment:'群内序号，旧消息为空'"`
	RoomID      int64                 `json:"room_id,string" gorm:"type:bigint;not null;uniqueIndex:idx_room_seq,priority:1;index:idx_room_id_id,priority:1;index:idx_room_created,priority:1;comment:'所属房间'"`
	SenderID    int64                 `json:"sender_id,string" gorm:"type:bigint;not null;uniqueIndex:idx_sender_client,priority:1;index:idx_sender_id;comment:'发送者ID'"`
	ClientMsgID string                `json:"client_msg_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_sender_client,priority:2;comment:'客户端防重发凭证'"`
	Type        int8                  `gorm:"type:tinyint;not null;default:1;comment:'1:文本 2:文件/图片 3:系统通知'"`
	Content     string                `gorm:"type:text;not null;comment:'消息内容或文件JSON载荷'"`
	CreatedAt   time.Time             `gorm:"index:idx_room_created,priority:2;index:idx_messages_created;comment:'创建时间'"`
	DeletedAt   soft_delete.DeletedAt `gorm:"type:bigint unsigned;index:idx_msg_deleted;softDelete:milli;comment:'用于实现消息撤回的软删除'"`
}
