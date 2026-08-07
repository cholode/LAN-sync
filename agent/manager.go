package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"lan-im-go/agent/llm"
	"lan-im-go/agent/rag"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/models"
	"lan-im-go/repository"
	"lan-im-go/pkg"
	"sync"
	"time"

	"gorm.io/gorm"
)

// AgentManager Agent 生命周期管理器
// 负责群的 Agent 启停、消息监听与分发
type AgentManager struct {
	agents    map[int64]*RoomAgent // roomID → Agent 实例
	mu        sync.RWMutex
	llmClient *llm.Client
	db        *gorm.DB
	hub       *core.Hub
}

// NewAgentManager 创建 AgentManager
func NewAgentManager(db *gorm.DB, llmClient *llm.Client, hub *core.Hub) *AgentManager {
	return &AgentManager{
		agents:    make(map[int64]*RoomAgent),
		llmClient: llmClient,
		db:        db,
		hub:       hub,
	}
}

// Start 启动 AgentManager
// 从 DB 加载已启用的群 Agent，并开始监听群消息
func (m *AgentManager) Start(ctx context.Context) {
	pkg.Infoln("[AgentManager] 正在启动...")
	m.loadEnabledAgents(ctx)
	go m.listenMessages(ctx)
	pkg.Infof("[AgentManager] 启动完成, 已加载 %d 个 Agent", len(m.agents))
}

// loadEnabledAgents 从 DB 查询所有启用了 Agent 的群，逐个启动
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

// AddAgent 为指定群添加并启动 Agent
// 若已存在则跳过；否则自动创建 Bot 用户、加入群、初始化 RoomAgent 并启动流水线
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

	roomAgent := NewRoomAgent(roomID, botUserID, m.db, m.llmClient, agentConfig, m.hub)
	go roomAgent.Start(ctx)

	m.agents[roomID] = roomAgent

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"bot_user_id":   botUserID,
			"agent_enabled": true,
		})

	pkg.Infof("[AgentManager] room=%d Agent 已启用 (botID=%d)", roomID, botUserID)
	return nil
}

// PauseAgent 暂停指定群的 Agent
// 停止 RoomAgent、标记禁用，但保留 Qdrant 向量和 MySQL 分块数据，可以重新启用
func (m *AgentManager) PauseAgent(ctx context.Context, roomID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[roomID]
	if !exists {
		return nil
	}

	agent.Stop()
	delete(m.agents, roomID)

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"agent_enabled": false,
		})

	pkg.Infof("[AgentManager] room=%d Agent 已暂停", roomID)
	return nil
}

// RemoveAgent 停止并移除指定群的 Agent
// 同时清理 Qdrant 向量集合和 MySQL 分块记录，不可恢复
func (m *AgentManager) RemoveAgent(ctx context.Context, roomID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[roomID]
	if !exists {
		return nil
	}

	agent.Stop()

	// 清理 Qdrant 向量集合 + MySQL 分块
	if vs, err := rag.NewQdrantVectorStore(); err == nil {
		vs.DeleteByRoom(ctx, roomID)
	}
	m.db.WithContext(ctx).Where("room_id = ?", roomID).Delete(&models.RAGChunk{})

	delete(m.agents, roomID)

	m.db.WithContext(ctx).Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"agent_enabled": false,
		})

	pkg.Infof("[AgentManager] room=%d Agent 已移除（含数据清理）", roomID)
	return nil
}

// GetAgent 获取指定群的 Agent 实例，不存在返回 nil
func (m *AgentManager) GetAgent(roomID int64) *RoomAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[roomID]
}

// listenMessages 通过 Redis Pub/Sub 监听群消息广播，分发给对应群的 Agent
func (m *AgentManager) listenMessages(ctx context.Context) {
	pubsub := config.RedisClient.PSubscribe(ctx, "im:broadcast:room:*")
	defer pubsub.Close()

	_, err := pubsub.Receive(ctx)
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

// handleRedisMessage 解析 Pub/Sub 消息，路由到对应群的 Agent.HandleMessage
// 过滤 bot 自身消息，避免 Agent 回环
func (m *AgentManager) handleRedisMessage(ctx context.Context, payload string) {
	var raw struct {
		RoomID   string `json:"room_id"`
		SenderID int64  `json:"sender_id"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return
	}

	roomID := parseInt64(raw.RoomID)
	if roomID == 0 {
		return
	}

	agent := m.GetAgent(roomID)
	if agent == nil {
		return
	}

	if raw.SenderID == agent.botUserID {
		return
	}

	msg := AgentMessage{
		RoomID:   roomID,
		SenderID: raw.SenderID,
		Content:  raw.Content,
		Time:     time.Now(),
	}

	agent.HandleMessage(msg)
}

// ensureBotUser 获取或创建群的 Bot 用户
// 如果 Room 已有 BotUserID 则直接返回；否则新建 IsBot=true 的用户
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

// ensureBotInRoom 确保 Bot 用户已加入群，未加入则添加为普通成员
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

// ensureAgentConfig 获取或创建群的 Agent 配置，不存在时写入默认配置
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

// parseInt64 从字符串中提取前导数字部分转为 int64
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