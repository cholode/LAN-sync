package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/internal/agentclient"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
)

// RoomAgent 单个群的 Agent 实例。
// 触发判断和冷却在 Go 侧完成，LLM/RAG/tools 处理链委托给 Python agent-service。
type RoomAgent struct {
	roomID    int64
	botUserID int64
	client    *agentclient.Client
	config    *models.AgentConfig

	lastReplyTime time.Time
	coolDown      time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// NewRoomAgent 创建 RoomAgent。
// db 和 hub 参数保留用于保持调用面稳定，后续如有 Go 侧工具可复用。
func NewRoomAgent(roomID, botUserID int64, db *gorm.DB, client *agentclient.Client, agentConfig *models.AgentConfig, hub *core.Hub) *RoomAgent {
	ctx, cancel := context.WithCancel(context.Background())

	return &RoomAgent{
		roomID:    roomID,
		botUserID: botUserID,
		client:    client,
		config:    agentConfig,
		coolDown:  5 * time.Second,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动 RoomAgent。
func (a *RoomAgent) Start() {
	pkg.Infof("[RoomAgent] room=%d 启动", a.roomID)
}

// Stop 停止 RoomAgent。
func (a *RoomAgent) Stop() { a.cancel() }

// HandleMessage 处理群消息：触发判断 + 冷却，通过后异步交给 Python。
func (a *RoomAgent) HandleMessage(msg AgentMessage) {
	if !a.shouldTrigger(msg) {
		return
	}
	if time.Since(a.lastReplyTime) < a.coolDown {
		return
	}
	go a.processMessage(msg)
}

func (a *RoomAgent) shouldTrigger(msg AgentMessage) bool {
	switch a.config.TriggerMode {
	case 1:
		botName := fmt.Sprintf("@AI助手_群%d", a.roomID)
		return strings.Contains(msg.Content, "@agent") ||
			strings.Contains(msg.Content, "@AI助手") ||
			strings.Contains(msg.Content, botName)
	case 2:
		return true
	case 3:
		for _, kw := range agentclient.ParseTriggerWords(a.config.TriggerWords) {
			if strings.Contains(msg.Content, kw) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (a *RoomAgent) processMessage(msg AgentMessage) {
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	a.lastReplyTime = time.Now()

	in := agentclient.IncomingMessage{
		RoomID:     a.roomID,
		RoomName:   a.getRoomName(),
		BotUserID:  a.botUserID,
		SenderID:   msg.SenderID,
		SenderName: a.getSenderName(msg.SenderID),
		Content:    msg.Content,
		Time:       msg.Time,
		Config:     runtimeConfigFromModel(a.config),
	}

	reply, skip, err := a.client.ProcessMessage(ctx, in)
	if err != nil {
		pkg.Infof("[RoomAgent] room=%d agent 调用失败: %v", a.roomID, err)
		a.sendReply("AI 助手暂时不可用，请稍后再试。")
		return
	}
	if skip {
		return
	}
	if reply != "" {
		a.sendReply(reply)
		pkg.Infof("[RoomAgent] room=%d 回复: %s", a.roomID, truncate(reply, 50))
	}
}

func (a *RoomAgent) sendReply(content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := config.KafkaProducer.HandleIncomingMessage(ctx,
		fmt.Sprintf("%d", a.roomID), int(a.botUserID), content,
		fmt.Sprintf("agent-%d-%d", a.roomID, time.Now().UnixNano()))
	if err != nil {
		pkg.Infof("[RoomAgent] room=%d 发送失败: %v", a.roomID, err)
	}
}

func (a *RoomAgent) getSenderName(userID int64) string {
	user, err := repository.User.GetByID(userID)
	if err != nil || user == nil || user.Username == "" {
		return fmt.Sprintf("用户%d", userID)
	}
	return user.Username
}

func (a *RoomAgent) getRoomName() string {
	room, err := repository.Room.GetRoomByID(a.roomID)
	if err != nil || room == nil || room.Name == "" {
		return fmt.Sprintf("群聊%d", a.roomID)
	}
	return room.Name
}

// runtimeConfigFromModel 将数据库配置转换为 gRPC 运行配置。
func runtimeConfigFromModel(cfg *models.AgentConfig) agentclient.RuntimeConfig {
	if cfg == nil {
		cfg = models.DefaultAgentConfig(0)
	}
	return agentclient.RuntimeConfig{
		SystemPrompt:      cfg.SystemPrompt,
		TriggerMode:       cfg.TriggerMode,
		TriggerWords:      agentclient.ParseTriggerWords(cfg.TriggerWords),
		MaxHistory:        cfg.MaxHistory,
		Temperature:       cfg.Temperature,
		ModelName:         cfg.ModelName,
		RAGEnabled:        cfg.RAGEnabled,
		TopK:              cfg.TopK,
		SimilarityThold:   cfg.SimilarityThold,
		RerankEnabled:     cfg.RerankEnabled,
		MaxChunkTokens:    cfg.MaxChunkTokens,
		TopicChunkMinMsgs: cfg.TopicChunkMinMsgs,
		TopicChunkModel:   cfg.TopicChunkModel,
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}