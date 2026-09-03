package agentclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	agentv1 "lan-im-go/proto/agent/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client 是对 Python Agent 服务的一个薄 gRPC 封装。
// 它使 IM 侧不依赖 proto 类型。
type Client struct {
	conn  *grpc.ClientConn
	agent agentv1.AgentServiceClient
}

// IncomingMessage 是发送给 Agent 服务的消息上下文。
type IncomingMessage struct {
	RoomID     int64
	RoomName   string
	BotUserID  int64
	SenderID   int64
	SenderName string
	Content    string
	Time       time.Time
	Config     RuntimeConfig
}

// RuntimeConfig 对应 Python LangGraph 服务所需的 models.AgentConfig 运行时部分。
// 将此结构定义在本地可以避免从 gRPC 边界导入 GORM 模型。
type RuntimeConfig struct {
	SystemPrompt      string
	TriggerMode       int8
	TriggerWords      []string
	MaxHistory        int
	Temperature       float64
	ModelName         string
	RAGEnabled        bool
	TopK              int
	SimilarityThold   float64
	RerankEnabled     bool
	MaxChunkTokens    int
	TopicChunkMinMsgs int
	TopicChunkModel   string
}

// New 创建一个 gRPC Agent 服务客户端。连接是惰性的，因此在第一次 RPC 之前不会发起网络调用。
func New(addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("agent grpc addr is empty")
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create agent grpc client: %w", err)
	}

	return &Client{
		conn:  conn,
		agent: agentv1.NewAgentServiceClient(conn),
	}, nil
}

// Close 释放底层的 gRPC 连接。
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// ProcessMessage 请求 Agent 服务运行 LangGraph 流水线并返回最终回复。
// 当消息被跳过时，回复可能为空。
func (c *Client) ProcessMessage(ctx context.Context, msg IncomingMessage) (string, bool, error) {
	resp, err := c.agent.ProcessMessage(ctx, &agentv1.ProcessMessageRequest{
		RoomId:            msg.RoomID,
		BotUserId:         msg.BotUserID,
		SenderId:          msg.SenderID,
		SenderName:        msg.SenderName,
		Content:           msg.Content,
		MessageTimeUnixMs: msg.Time.UnixMilli(),
		Config:            runtimeConfigToProto(msg.Config),
		RoomName:          msg.RoomName,
	})
	if err != nil {
		return "", false, err
	}
	return resp.GetReply(), resp.GetSkip(), nil
}

// EnableAgent 通知 Agent 服务某个房间的智能体已启用。
func (c *Client) EnableAgent(ctx context.Context, roomID, botUserID int64, cfg RuntimeConfig) error {
	_, err := c.agent.EnableAgent(ctx, &agentv1.EnableAgentRequest{
		RoomId:    roomID,
		BotUserId: botUserID,
		Config:    runtimeConfigToProto(cfg),
	})
	return err
}

// PauseAgent 通知 Agent 服务某个房间的智能体已暂停。
func (c *Client) PauseAgent(ctx context.Context, roomID int64) error {
	_, err := c.agent.PauseAgent(ctx, &agentv1.PauseAgentRequest{RoomId: roomID})
	return err
}

// RemoveAgent 通知 Agent 服务某个房间的智能体已移除。
func (c *Client) RemoveAgent(ctx context.Context, roomID int64) error {
	_, err := c.agent.RemoveAgent(ctx, &agentv1.RemoveAgentRequest{RoomId: roomID})
	return err
}

// TriggerChunking 请求 Agent 服务为某个房间执行话题分块。
func (c *Client) TriggerChunking(ctx context.Context, roomID int64) (int32, error) {
	resp, err := c.agent.TriggerChunking(ctx, &agentv1.TriggerChunkingRequest{RoomId: roomID})
	if err != nil {
		return 0, err
	}
	return resp.GetChunkedMessages(), nil
}

// Health 返回 Agent 服务的健康状态和版本。
func (c *Client) Health(ctx context.Context) (string, string, error) {
	resp, err := c.agent.Health(ctx, &agentv1.HealthRequest{})
	if err != nil {
		return "", "", err
	}
	return resp.GetStatus(), resp.GetVersion(), nil
}

func runtimeConfigToProto(cfg RuntimeConfig) *agentv1.AgentRuntimeConfig {
	words := append([]string(nil), cfg.TriggerWords...)

	return &agentv1.AgentRuntimeConfig{
		SystemPrompt:      cfg.SystemPrompt,
		TriggerMode:       int32(cfg.TriggerMode),
		TriggerWords:      words,
		MaxHistory:        int32(cfg.MaxHistory),
		Temperature:       cfg.Temperature,
		ModelName:         cfg.ModelName,
		RagEnabled:        cfg.RAGEnabled,
		TopK:              int32(cfg.TopK),
		SimilarityThold:   cfg.SimilarityThold,
		RerankEnabled:     cfg.RerankEnabled,
		MaxChunkTokens:    int32(cfg.MaxChunkTokens),
		TopicChunkMinMsgs: int32(cfg.TopicChunkMinMsgs),
		TopicChunkModel:   cfg.TopicChunkModel,
	}
}

// ParseTriggerWords 将存储在 IM 数据库中的 JSON 数组转换为适合 proto RuntimeConfig 的 []string。
func ParseTriggerWords(raw string) []string {
	var words []string
	if raw == "" {
		return words
	}
	_ = json.Unmarshal([]byte(raw), &words)
	return words
}
