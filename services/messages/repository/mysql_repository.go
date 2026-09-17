package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	messagesmodel "lan-im-go/services/messages/models"
)

type messageRepoImpl struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) MessageRepository {
	return &messageRepoImpl{db: db}
}

func (r *messageRepoImpl) SaveMessage(msg *messagesmodel.Message) error {
	return r.SaveMessageBatch([]*messagesmodel.Message{msg})
}

// 批量保存消息（高性能写入）
func (r *messageRepoImpl) SaveMessageBatch(msgs []*messagesmodel.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	// 进程在落库后、提交 Kafka 游标前退出时会重放；复用原消息 ID 保持索引和缓存一致。
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(msgs, 100).Error; err != nil {
			return err
		}
		keys := make([]string, 0, len(msgs))
		for _, msg := range msgs {
			keys = append(keys, msg.ClientMsgID)
		}
		var persisted []messagesmodel.Message
		if err := tx.Unscoped().Where("client_msg_id IN ?", keys).Find(&persisted).Error; err != nil {
			return err
		}
		byKey := make(map[messagesmodel.MessageRequestKey]messagesmodel.Message, len(persisted))
		for _, msg := range persisted {
			byKey[msg.RequestKey()] = msg
		}
		for _, msg := range msgs {
			stored, ok := byKey[msg.RequestKey()]
			if !ok {
				return gorm.ErrRecordNotFound
			}
			if err := messagesmodel.ReconcileMessage(msg, &stored); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetHistoryByCursor 基于游标分页查询历史消息
// 新消息按群序号分页；未编号的旧消息排在新链路消息之前。
func (r *messageRepoImpl) GetHistoryByCursor(roomID int64, cursorMsgID int64, limit int) ([]*messagesmodel.Message, error) {
	var messages []*messagesmodel.Message
	query := r.db.Model(&messagesmodel.Message{}).Where("room_id = ?", roomID)

	if cursorMsgID > 0 {
		// 游标只允许定位当前群，不能使用另一群的序号改变分页范围。
		var cursor messagesmodel.Message
		if err := r.db.Unscoped().Where("id = ? AND room_id = ?", cursorMsgID, roomID).Select("room_seq").First(&cursor).Error; err == nil {
			if cursor.RoomSeq > 0 {
				query = query.Where("(room_seq < ? OR room_seq IS NULL OR (room_seq = ? AND id < ?))", cursor.RoomSeq, cursor.RoomSeq, cursorMsgID)
			} else {
				query = query.Where("(room_seq IS NULL OR room_seq = 0) AND id < ?", cursorMsgID)
			}
		} else {
			// 游标消息不存在，降级为 id 游标
			query = query.Where("id < ?", cursorMsgID)
		}
	}

	err := query.Order("room_seq DESC, id DESC").Limit(limit).Find(&messages).Error

	// 翻转为旧→新顺序（前端 append 渲染）
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, err
}

// SoftDeleteUserMessagesInRoom 软删除指定用户在群聊内的所有消息
// GetMessagesByTimeRange 返回 created_at 在 [start, end) 范围内的消息，按升序排列。
func (r *messageRepoImpl) GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]messagesmodel.Message, error) {
	var messages []messagesmodel.Message
	err := r.db.Model(&messagesmodel.Message{}).
		Where("room_id = ? AND created_at >= ? AND created_at < ?", roomID, start, end).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GetMessagesAfterID 返回 id 大于 sinceID 的消息，按升序排列。
func (r *messageRepoImpl) GetMessagesAfterID(roomID int64, sinceID int64, limit int) ([]messagesmodel.Message, error) {
	var messages []messagesmodel.Message
	query := r.db.Model(&messagesmodel.Message{}).Where("room_id = ?", roomID)
	if sinceID > 0 {
		query = query.Where("id > ?", sinceID)
	}
	err := query.Order("id ASC").Limit(limit).Find(&messages).Error
	return messages, err
}

// CountMessagesAfterID 统计 id 大于 sinceID 的消息数量。
func (r *messageRepoImpl) CountMessagesAfterID(roomID int64, sinceID int64) (int64, error) {
	var count int64
	query := r.db.Model(&messagesmodel.Message{}).Where("room_id = ?", roomID)
	if sinceID > 0 {
		query = query.Where("id > ?", sinceID)
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *messageRepoImpl) SearchMessages(params MessageSearchParams) ([]*messagesmodel.Message, int64, error) {
	query := r.db.Model(&messagesmodel.Message{}).Where("room_id = ?", params.RoomID)
	if params.Keyword != "" {
		// LOCATE 会把 %、_ 和反斜杠视为普通用户输入，而不是 LIKE 通配符。
		query = query.Where("LOCATE(?, content) > 0", params.Keyword)
	}
	if params.SenderID > 0 {
		query = query.Where("sender_id = ?", params.SenderID)
	}
	if !params.Start.IsZero() {
		query = query.Where("created_at >= ?", params.Start)
	}
	if !params.End.IsZero() {
		query = query.Where("created_at < ?", params.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var messages []*messagesmodel.Message
	err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&messages).Error
	return messages, total, err
}

func (r *messageRepoImpl) SoftDeleteUserMessagesInRoom(roomID int64, userID int64) error {
	// 采用软删除而非物理删除：
	// 1. 保留数据记录，满足数据追溯需求
	// 2. 避免物理删除导致的数据库索引结构变动，保证高并发场景下的数据库性能稳定
	return r.db.Model(&messagesmodel.Message{}).
		Where("room_id = ? AND sender_id = ?", roomID, userID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
