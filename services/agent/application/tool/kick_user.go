package tool

import (
	"encoding/json"
	"fmt"
	"lan-im-go/pkg"
	"lan-im-go/repository"
	"lan-im-go/services/agent/application/llm"
	"lan-im-go/services/gateway/websocket"

	"gorm.io/gorm"
)

// KickUserHandler 移除违规用户，实现 Handler 接口
type KickUserHandler struct {
	db     *gorm.DB
	hub    *core.Hub
	roomID int64
}

// NewKickUserHandler 创建 handler
func NewKickUserHandler(db *gorm.DB, hub *core.Hub, roomID int64) *KickUserHandler {
	return &KickUserHandler{db: db, hub: hub, roomID: roomID}
}

func (h *KickUserHandler) Name() string { return "kick_user" }

func (h *KickUserHandler) Definition() llm.FunctionDef {
	return llm.FunctionDef{
		Name:        "kick_user",
		Description: "将发布侮辱性、攻击性、歧视性言论的用户移出群聊。仅在确认消息违规时调用。",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"user_id": map[string]interface{}{
					"type":        "integer",
					"description": "被移除的用户ID（消息中 sender_id 的值）",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "移除原因，引用违规言论原文",
				},
			},
			"required": []string{"user_id", "reason"},
		},
	}
}

func (h *KickUserHandler) Handle(args json.RawMessage) (string, error) {
	var params struct {
		UserID int64  `json:"user_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if err := repository.RoomMember.RemoveMember(h.roomID, params.UserID); err != nil {
		pkg.Infof("[KickUser] room=%d 移除成员 %d 失败: %v", h.roomID, params.UserID, err)
		return "", err
	}

	h.hub.Kick(params.UserID)

	pkg.Infof("[KickUser] room=%d 已移除用户 %d, 原因: %s", h.roomID, params.UserID, params.Reason)
	return fmt.Sprintf("%d 已被移出群聊，原因: %s", params.UserID, params.Reason), nil
}
