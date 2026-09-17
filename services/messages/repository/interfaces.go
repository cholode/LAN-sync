package repository

import (
	messagesmodel "lan-im-go/services/messages/models"
	"time"
)

// MessageRepository 消息数据访问接口
type MessageRepository interface {
	// 异步保存消息
	SaveMessage(msg *messagesmodel.Message) error

	SaveMessageBatch(msgs []*messagesmodel.Message) error
	// 基于游标分页查询历史消息，避免深分页性能问题
	GetHistoryByCursor(roomID int64, cursorMsgID int64, limit int) ([]*messagesmodel.Message, error)
	// GetMessagesByTimeRange 返回 created_at 在 [start, end) 范围内的消息，按升序排列。
	GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]messagesmodel.Message, error)
	// GetMessagesAfterID 返回 id 大于 sinceID 的消息，按升序排列。
	GetMessagesAfterID(roomID int64, sinceID int64, limit int) ([]messagesmodel.Message, error)
	// CountMessagesAfterID 统计 id 大于 sinceID 的消息数量。
	CountMessagesAfterID(roomID int64, sinceID int64) (int64, error)
	// SearchMessages 按房间、正文和可选条件搜索消息。它既是 Elasticsearch
	// 不可用时的可靠回退，也覆盖尚未完成异步索引的历史消息。
	SearchMessages(params MessageSearchParams) ([]*messagesmodel.Message, int64, error)
	// 批量软删除指定用户在群组内的消息
	SoftDeleteUserMessagesInRoom(roomID int64, userID int64) error
}

type MessageSearchParams struct {
	RoomID   int64
	Keyword  string
	SenderID int64
	Start    time.Time
	End      time.Time
	Offset   int
	Limit    int
}
