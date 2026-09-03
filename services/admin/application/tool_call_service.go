package admin

import (
	"context"
	"time"

	"gorm.io/gorm"

	"lan-im-go/models"
)

// ToolCallService 管理 Agent Tool Calling 运行记录。
type ToolCallService struct {
	db *gorm.DB
}

func NewToolCallService(db *gorm.DB) *ToolCallService {
	return &ToolCallService{db: db}
}

// ToolCallListQuery 工具调用记录查询条件。
type ToolCallListQuery struct {
	Page     int
	PageSize int
	ToolName string
	UserID   int64
	RoomID   int64
	Success  *bool
	Start    time.Time
	End      time.Time
}

// ToolCallListItem 带用户名和群名的展示项。
type ToolCallListItem struct {
	models.ToolCallLog
	Username string `json:"username"`
	RoomName string `json:"room_name"`
}

// List 分页查询 Tool Call 记录。
func (s *ToolCallService) List(ctx context.Context, q ToolCallListQuery) ([]ToolCallListItem, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.ToolCallLog{})
	if q.ToolName != "" {
		query = query.Where("tool_name = ?", q.ToolName)
	}
	if q.UserID > 0 {
		query = query.Where("user_id = ?", q.UserID)
	}
	if q.RoomID > 0 {
		query = query.Where("room_id = ?", q.RoomID)
	}
	if q.Success != nil {
		query = query.Where("success = ?", *q.Success)
	}
	if !q.Start.IsZero() {
		query = query.Where("started_at >= ?", q.Start)
	}
	if !q.End.IsZero() {
		query = query.Where("started_at < ?", q.End)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.ToolCallLog
	if err := query.Order("id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]ToolCallListItem, 0, len(rows))
	for _, row := range rows {
		item := ToolCallListItem{ToolCallLog: row}
		item.Username = s.username(ctx, row.UserID)
		item.RoomName = s.roomName(ctx, row.RoomID)
		items = append(items, item)
	}
	return items, total, nil
}

// RecordInput 用于在 Go 侧记录工具调用结果。
type RecordInput struct {
	ToolCallID     string
	UserID         int64
	RoomID         int64
	AgentRequestID string
	ToolName       string
	Arguments      string
	StartedAt      time.Time
	FinishedAt     time.Time
	Success        bool
	Error          string
}

// Record 写入一条 Tool Call 运行记录。
func (s *ToolCallService) Record(ctx context.Context, input RecordInput) error {
	if input.StartedAt.IsZero() {
		input.StartedAt = time.Now()
	}
	if input.FinishedAt.IsZero() {
		input.FinishedAt = time.Now()
	}
	latency := float64(input.FinishedAt.Sub(input.StartedAt).Microseconds()) / 1000.0
	record := &models.ToolCallLog{
		ToolCallID:     input.ToolCallID,
		UserID:         input.UserID,
		RoomID:         input.RoomID,
		AgentRequestID: input.AgentRequestID,
		ToolName:       input.ToolName,
		Arguments:      input.Arguments,
		StartedAt:      input.StartedAt,
		FinishedAt:     input.FinishedAt,
		LatencyMS:      latency,
		Success:        input.Success,
		Error:          input.Error,
	}
	return s.db.WithContext(ctx).Create(record).Error
}

func (s *ToolCallService) username(ctx context.Context, userID int64) string {
	var user models.User
	if err := s.db.WithContext(ctx).Select("username").First(&user, userID).Error; err == nil {
		return user.Username
	}
	return ""
}

func (s *ToolCallService) roomName(ctx context.Context, roomID int64) string {
	var room models.Room
	if err := s.db.WithContext(ctx).Select("name").First(&room, roomID).Error; err == nil {
		return room.Name
	}
	return ""
}
