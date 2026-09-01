package messages

import (
	"time"

	"gorm.io/gorm"
	"lan-im-go/models"
	"lan-im-go/repository"
)

type messageRepoImpl struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) repository.MessageRepository {
	return &messageRepoImpl{db: db}
}

func (r *messageRepoImpl) SaveMessage(msg *models.Message) error {
	// 消息持久化方法，提供高性能写入能力
	return r.db.Create(msg).Error
}

// 批量保存消息（高性能写入）
func (r *messageRepoImpl) SaveMessageBatch(msgs []*models.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	// gorm 批量插入，只执行一条SQL
	return r.db.CreateInBatches(msgs, 100).Error
}

// GetHistoryByCursor 基于游标分页查询历史消息
// 使用 idx_room_created (room_id, created_at) 二级索引保证时间顺序
func (r *messageRepoImpl) GetHistoryByCursor(roomID int64, cursorMsgID int64, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	query := r.db.Model(&models.Message{}).Where("room_id = ?", roomID)

	if cursorMsgID > 0 {
		// 查找游标消息的 created_at，用 (created_at, id) 复合游标走 idx_room_created 索引
		var cursor models.Message
		if err := r.db.Where("id = ?", cursorMsgID).Select("created_at").First(&cursor).Error; err == nil {
			query = query.Where(
				"(created_at < ? OR (created_at = ? AND id < ?))",
				cursor.CreatedAt, cursor.CreatedAt, cursorMsgID,
			)
		} else {
			// 游标消息不存在，降级为 id 游标
			query = query.Where("id < ?", cursorMsgID)
		}
	}

	err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&messages).Error

	// 翻转为旧→新顺序（前端 append 渲染）
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, err
}

// SoftDeleteUserMessagesInRoom 软删除指定用户在群聊内的所有消息
// GetMessagesByTimeRange 返回 created_at 在 [start, end) 范围内的消息，按升序排列。
func (r *messageRepoImpl) GetMessagesByTimeRange(roomID int64, start, end time.Time, limit int) ([]models.Message, error) {
	var messages []models.Message
	err := r.db.Model(&models.Message{}).
		Where("room_id = ? AND created_at >= ? AND created_at < ?", roomID, start, end).
		Order("created_at ASC, id ASC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

// GetMessagesAfterID 返回 id 大于 sinceID 的消息，按升序排列。
func (r *messageRepoImpl) GetMessagesAfterID(roomID int64, sinceID int64, limit int) ([]models.Message, error) {
	var messages []models.Message
	query := r.db.Model(&models.Message{}).Where("room_id = ?", roomID)
	if sinceID > 0 {
		query = query.Where("id > ?", sinceID)
	}
	err := query.Order("id ASC").Limit(limit).Find(&messages).Error
	return messages, err
}

// CountMessagesAfterID 统计 id 大于 sinceID 的消息数量。
func (r *messageRepoImpl) CountMessagesAfterID(roomID int64, sinceID int64) (int64, error) {
	var count int64
	query := r.db.Model(&models.Message{}).Where("room_id = ?", roomID)
	if sinceID > 0 {
		query = query.Where("id > ?", sinceID)
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *messageRepoImpl) SearchMessages(params repository.MessageSearchParams) ([]*models.Message, int64, error) {
	query := r.db.Model(&models.Message{}).Where("room_id = ?", params.RoomID)
	if params.Keyword != "" {
		// LOCATE treats %, _ and backslashes as plain user input instead of LIKE wildcards.
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

	var messages []*models.Message
	err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&messages).Error
	return messages, total, err
}

func (r *messageRepoImpl) SoftDeleteUserMessagesInRoom(roomID int64, userID int64) error {
	// 采用软删除而非物理删除：
	// 1. 保留数据记录，满足数据追溯需求
	// 2. 避免物理删除导致的数据库索引结构变动，保证高并发场景下的数据库性能稳定
	return r.db.Model(&models.Message{}).
		Where("room_id = ? AND sender_id = ?", roomID, userID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
