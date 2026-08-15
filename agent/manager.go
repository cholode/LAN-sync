package agent

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"

	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/internal/agentclient"
	"lan-im-go/internal/metrics"
	"lan-im-go/internal/protocol"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"lan-im-go/repository"
)

// AgentManager 管理每个群的 Agent 生命周期：启用、暂停、移除，
// 并监听 Redis 群消息广播后分发给对应群的 Agent。
// 消息处理链（trigger / RAG / LLM / tools）委托给 Python agent-service。
type AgentManager struct {
	agents      map[int64]*RoomAgent
	mu          sync.RWMutex
	agentClient *agentclient.Client
	db          *gorm.DB
	hub         *core.Hub
}

// NewAgentManager 创建 AgentManager。
func NewAgentManager(db *gorm.DB, agentClient *agentclient.Client, hub *core.Hub) *AgentManager {
	return &AgentManager{
		agents:      make(map[int64]*RoomAgent),
		agentClient: agentClient,
		db:          db,
		hub:         hub,
	}
}

// Start 从 DB 加载已启用的群 Agent，并开始监听群消息。
func (m *AgentManager) Start(ctx context.Context) {
	pkg.Infoln("[AgentManager] 正在启动...")
	m.loadEnabledAgents(ctx)
	go m.listenMessages(ctx)
	metrics.SetAgentRoomsEnabled(len(m.agents))
	pkg.Infof("[AgentManager] 启动完成, 已加载 %d 个 Agent", len(m.agents))
}

// loadEnabledAgents 从 DB 查询所有启用了 Agent 的群，逐个启动。
func (m *AgentManager) loadEnabledAgents(ctx context.Context) {
	var rooms []models.Room
	if err := m.db.WithContext(ctx).
		Where("agent_enabled = ? AND type = ?", true, 2).
		Find(&rooms).Error; err != nil {
		pkg.Infof("[AgentManager] 加载已启用 Agent 的群失败: %v", err)
		return
	}

	for _, room := range rooms {
		if err := m.AddAgent(ctx, room.ID); err != nil {
			pkg.Infof("[AgentManager] 添加 Agent room=%d 失败: %v", room.ID, err)
		}
	}
}

// AddAgent 为指定群添加并启动 Agent。
// 若已存在则跳过；否则创建 Bot 用户、加入群、初始化配置并通知 Python 服务。
func (m *AgentManager) AddAgent(ctx context.Context, roomID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.agents[roomID]; exists {
		return nil
	}

	botUserID, err := m.ensureBotUser(ctx, roomID)
	if err != nil {
		return fmt.Errorf("ensure bot user: %w", err)
	}

	if err := m.ensureBotInRoom(ctx, roomID, botUserID); err != nil {
		return fmt.Errorf("ensure bot in room: %w", err)
	}

	agentConfig := m.ensureAgentConfig(ctx, roomID)

	roomAgent := NewRoomAgent(roomID, botUserID, m.db, m.agentClient, agentConfig, m.hub)
	go roomAgent.Start()

	m.agents[roomID] = roomAgent
	metrics.SetAgentRoomsEnabled(len(m.agents))

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"bot_user_id":   botUserID,
			"agent_enabled": true,
		})

	if err := m.agentClient.EnableAgent(ctx, roomID, botUserID, runtimeConfigFromModel(agentConfig)); err != nil {
		pkg.Infof("[AgentManager] 通知 Python 启用 room=%d 失败: %v", roomID, err)
	}

	pkg.Infof("[AgentManager] room=%d Agent 已启用 (botID=%d)", roomID, botUserID)
	return nil
}

// PauseAgent 暂停指定群的 Agent。
func (m *AgentManager) PauseAgent(ctx context.Context, roomID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[roomID]
	if !exists {
		return nil
	}

	agent.Stop()
	delete(m.agents, roomID)
	metrics.SetAgentRoomsEnabled(len(m.agents))

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"agent_enabled": false,
		})

	if err := m.agentClient.PauseAgent(ctx, roomID); err != nil {
		pkg.Infof("[AgentManager] 通知 Python 暂停 room=%d 失败: %v", roomID, err)
	}

	pkg.Infof("[AgentManager] room=%d Agent 已暂停", roomID)
	return nil
}

// RemoveAgent 停止并移除指定群的 Agent。
// Python 侧负责清理 Qdrant 向量；Go 侧清理 MySQL 分块记录。
func (m *AgentManager) RemoveAgent(ctx context.Context, roomID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[roomID]
	if exists {
		agent.Stop()
		delete(m.agents, roomID)
	}
	metrics.SetAgentRoomsEnabled(len(m.agents))

	if err := m.agentClient.RemoveAgent(ctx, roomID); err != nil {
		pkg.Infof("[AgentManager] 通知 Python 移除 room=%d 失败: %v", roomID, err)
	}

	m.db.WithContext(ctx).Where("room_id = ?", roomID).Delete(&models.RAGChunk{})

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"agent_enabled": false,
		})

	pkg.Infof("[AgentManager] room=%d Agent 已移除（含数据清理）", roomID)
	return nil
}

// GetAgent 获取指定群的 Agent 实例，不存在返回 nil。
func (m *AgentManager) GetAgent(roomID int64) *RoomAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[roomID]
}

// listenMessages 通过 Redis Pub/Sub 监听群消息广播，分发给对应群的 Agent。
func (m *AgentManager) listenMessages(ctx context.Context) {
	pubsub := config.RedisClient.PSubscribe(ctx, "im:broadcast:room:*")
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
	metrics.ObserveRedisPubSub("im:broadcast:room:*", "subscribe", err)
	if err != nil {
		pkg.Fatalf("[AgentManager] Redis Pub/Sub 连接失败: %v", err)
	}

	pkg.Infoln("[AgentManager] 开始监听群消息...")

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			pkg.Infoln("[AgentManager] 停止监听")
			return
		case redisMsg := <-ch:
			m.handleRedisMessage(ctx, redisMsg.Payload)
		}
	}
}

// handleRedisMessage 解析 Pub/Sub 消息，路由到对应群的 Agent。
// 过滤 bot 自身消息，避免 Agent 回环。
func (m *AgentManager) handleRedisMessage(ctx context.Context, payload string) {
	envelope, err := protocol.Unmarshal([]byte(payload))
	if err != nil {
		return
	}

	roomID := envelope.RoomID
	if roomID == 0 {
		return
	}

	agent := m.GetAgent(roomID)
	if agent == nil {
		return
	}

	metrics.ObserveAgentMessageReceived(roomID, "redis")

	if envelope.SenderID == agent.botUserID {
		return
	}

	msg := AgentMessage{
		RoomID:   roomID,
		SenderID: envelope.SenderID,
		Content:  envelope.Content,
		Time:     envelope.CreatedAt,
	}

	agent.HandleMessage(msg)
}

// ensureBotUser 获取或创建群的 Bot 用户。
func (m *AgentManager) ensureBotUser(ctx context.Context, roomID int64) (int64, error) {
	var room models.Room
	if err := m.db.WithContext(ctx).First(&room, roomID).Error; err != nil {
		return 0, err
	}
	if room.BotUserID > 0 {
		return room.BotUserID, nil
	}

	botName := fmt.Sprintf("AI助手_群%d", roomID)
	bot := &models.User{
		Username: botName,
		Password: "",
		IsBot:    true,
	}
	if err := repository.User.CreateUser(bot); err != nil {
		return 0, fmt.Errorf("create bot user: %w", err)
	}

	return bot.ID, nil
}

// ensureBotInRoom 确保 Bot 用户已加入群，未加入则添加为普通成员。
func (m *AgentManager) ensureBotInRoom(ctx context.Context, roomID, botUserID int64) error {
	isMember, err := repository.RoomMember.CheckIsMember(roomID, botUserID)
	if err != nil {
		return err
	}
	if isMember {
		return nil
	}

	return repository.RoomMember.AddMember(roomID, botUserID, 1)
}

// ensureAgentConfig 获取或创建群的 Agent 配置。
func (m *AgentManager) ensureAgentConfig(ctx context.Context, roomID int64) *models.AgentConfig {
	var cfg models.AgentConfig
	err := m.db.WithContext(ctx).Where("room_id = ?", roomID).First(&cfg).Error
	if err == nil {
		return &cfg
	}

	cfg = *models.DefaultAgentConfig(roomID)
	m.db.WithContext(ctx).Create(&cfg)
	return &cfg
}

// parseInt64 从字符串中提取前导数字部分转为 int64。
func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
