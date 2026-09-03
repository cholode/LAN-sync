package tool

import (
	"encoding/json"
	"fmt"
	"lan-im-go/models"
	"lan-im-go/repository"
	"lan-im-go/services/agent/application/llm"
	"strings"
	"time"

	"gorm.io/gorm"
)

// GetMessagesHandler 查询群聊指定时间段的原始消息
type GetMessagesHandler struct {
	db     *gorm.DB
	roomID int64
}

// NewGetMessagesHandler 创建 handler
func NewGetMessagesHandler(db *gorm.DB, roomID int64) *GetMessagesHandler {
	return &GetMessagesHandler{db: db, roomID: roomID}
}

func (h *GetMessagesHandler) Name() string { return "get_messages" }

func (h *GetMessagesHandler) Definition() llm.FunctionDef {
	return llm.FunctionDef{
		Name:        "get_messages",
		Description: "获取群聊中指定时间段的原始消息。当用户询问带有限定时间(如昨天、上周三、7月24号下午)的问题时，先调用此函数获取该时间段的消息原文，再基于原文作答。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "起始时间，ISO 8601 格式，如 2025-07-24T14:00:00+08:00",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "结束时间，ISO 8601 格式",
				},
			},
			"required": []string{"start_time", "end_time"},
		},
	}
}

func (h *GetMessagesHandler) Handle(args json.RawMessage) (string, error) {
	var params struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	startTime, err := time.Parse(time.RFC3339, params.StartTime)
	if err != nil {
		return "", fmt.Errorf("parse start_time: %w", err)
	}
	endTime, err := time.Parse(time.RFC3339, params.EndTime)
	if err != nil {
		return "", fmt.Errorf("parse end_time: %w", err)
	}

	return h.queryMessages(startTime, endTime), nil
}

func (h *GetMessagesHandler) queryMessages(startTime, endTime time.Time) string {
	msgs, err := repository.Message.GetMessagesByTimeRange(h.roomID, startTime, endTime, 200)
	if err != nil {
		return fmt.Sprintf("查询消息失败: %v", err)
	}

	if len(msgs) == 0 {
		return "该时间段内没有消息记录。"
	}

	userIDs := make(map[int64]bool)
	for _, m := range msgs {
		userIDs[m.SenderID] = true
	}
	uidList := make([]int64, 0, len(userIDs))
	for uid := range userIDs {
		uidList = append(uidList, uid)
	}

	var users []models.User
	h.db.Where("id IN ?", uidList).Find(&users)
	nameMap := make(map[int64]string)
	for _, u := range users {
		nameMap[u.ID] = u.Username
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("时间段 %s ~ %s 的消息记录：\n\n",
		startTime.Format("2006-01-02 15:04"),
		endTime.Format("2006-01-02 15:04")))

	for _, m := range msgs {
		name := nameMap[m.SenderID]
		if name == "" {
			name = fmt.Sprintf("用户%d", m.SenderID)
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n",
			m.CreatedAt.Format("2006-01-02 15:04"),
			name, m.Content))
	}

	return sb.String()
}
