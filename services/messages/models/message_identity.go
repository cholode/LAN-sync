package models

import "fmt"

// MessageRequestKey 保留用户作用域，避免不同用户的客户端凭证相互覆盖。
type MessageRequestKey struct {
	SenderID    int64
	ClientMsgID string
}

func (m *Message) RequestKey() MessageRequestKey {
	return MessageRequestKey{m.SenderID, m.ClientMsgID}
}

// ReconcileMessage 正式消息的编号与内容不可变；旧版未编号消息继续兼容原归档行为。
func ReconcileMessage(incoming, stored *Message) error {
	if incoming.RoomSeq > 0 && (incoming.ID != stored.ID || incoming.RoomSeq != stored.RoomSeq ||
		incoming.RoomID != stored.RoomID || incoming.SenderID != stored.SenderID ||
		incoming.ClientMsgID != stored.ClientMsgID || incoming.Content != stored.Content || incoming.Type != stored.Type) {
		return fmt.Errorf("正式消息幂等冲突：message_id=%d room_id=%d room_seq=%d", incoming.ID, incoming.RoomID, incoming.RoomSeq)
	}
	*incoming = *stored
	return nil
}
