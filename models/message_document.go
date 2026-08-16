package models

import "time"

// MessageDocument 是聊天消息在 MongoDB 中的存储结构。
// 消息存储可以独立于关系型模型进行切换。
type MessageDocument struct {
	ID          int64     `bson:"_id,omitempty"`           // 消息 ID，作为 MongoDB 文档主键
	RoomID      int64     `bson:"room_id,omitempty"`       // 聊天室 ID
	SenderID    int64     `bson:"sender_id,omitempty"`     // 发送者用户 ID
	ClientMsgID string    `bson:"client_msg_id,omitempty"` // 客户端生成的消息唯一标识，用于幂等去重
	Type        int8      `bson:"type,omitempty"`          // 消息类型（如文本、图片、语音等）
	Content     string    `bson:"content,omitempty"`       // 消息内容
	CreatedAt   time.Time `bson:"created_at,omitempty"`    // 消息创建时间
	DeletedAt   int64     `bson:"deleted_at,omitempty"`    // 软删除时间戳（Unix 毫秒），0 表示未删除
}

// ToMessageDocument 将 GORM 领域模型转换为 MongoDB 文档结构。
// DeletedAt 字段以 Unix 毫秒时间戳存储；0 表示未删除。
func (m *Message) ToMessageDocument() *MessageDocument {
	if m == nil {
		return nil
	}

	return &MessageDocument{
		ID:          m.ID,
		RoomID:      m.RoomID,
		SenderID:    m.SenderID,
		ClientMsgID: m.ClientMsgID,
		Type:        m.Type,
		Content:     m.Content,
		CreatedAt:   m.CreatedAt,
		DeletedAt:   int64(m.DeletedAt),
	}
}

// ToMessage 将 MongoDB 文档转换回领域模型。
func (d *MessageDocument) ToMessage() *Message {
	if d == nil {
		return nil
	}

	return &Message{
		ID:          d.ID,
		RoomID:      d.RoomID,
		SenderID:    d.SenderID,
		ClientMsgID: d.ClientMsgID,
		Type:        d.Type,
		Content:     d.Content,
		CreatedAt:   d.CreatedAt,
		DeletedAt:   softDeleteAt(d.DeletedAt),
	}
}
