package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"lan-im-go/agent/llm"
	"lan-im-go/agent/rag"
	"lan-im-go/agent/rag/chunker"
	"lan-im-go/agent/tool"
	"lan-im-go/config"
	"lan-im-go/core"
	"lan-im-go/models"
	"lan-im-go/repository"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// RoomAgent 单群 Agent 实例
type RoomAgent struct {
	roomID    int64
	botUserID int64
	db        *gorm.DB
	llmClient *LLMClient
	config    *models.AgentConfig

	retriever *rag.Retriever
	pipeline  *chunker.ChunkingPipeline
	tools     *tool.Registry

	historyBuf  []historyEntry
	historyMu   sync.RWMutex
	historyIdx  int
	historyFull bool

	lastReplyTime time.Time
	coolDown      time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

type historyEntry struct {
	SenderName string
	Content    string
	Time       time.Time
}

// NewRoomAgent 创建 RoomAgent，注册所有可用的 function call handler
func NewRoomAgent(roomID, botUserID int64, db *gorm.DB, llmClient *LLMClient, agentConfig *models.AgentConfig, hub *core.Hub) *RoomAgent {
	ctx, cancel := context.WithCancel(context.Background())

	embedClient := llm.NewEmbedClient()
	vs, _ := rag.NewQdrantVectorStore()
	embedder := rag.NewEmbedder(embedClient)
	retriever := rag.NewRetriever(embedder, vs)
	chunkStore := chunker.NewChunkStore(db, embedder, vs)
	topicChunker := chunker.NewTopicChunker(roomID, db, llmClient, chunkStore, hub, agentConfig.TopicChunkModel)
	pipeline := chunker.NewChunkingPipeline(roomID, db, topicChunker, agentConfig.TopicChunkMinMsgs)

	// 注册 function call handler —— 后续新增工具只需在此加一行
	tools := tool.NewRegistry()
	tools.Register(tool.NewGetMessagesHandler(db, roomID))

	return &RoomAgent{
		roomID:        roomID,
		botUserID:     botUserID,
		db:            db,
		llmClient:     llmClient,
		config:        agentConfig,
		retriever:     retriever,
		pipeline:      pipeline,
		tools:         tools,
		historyBuf:    make([]historyEntry, agentConfig.MaxHistory),
		coolDown:      5 * time.Second,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (a *RoomAgent) Start(ctx context.Context) {
	log.Printf("[RoomAgent] room=%d 启动", a.roomID)
	go a.pipeline.Start(a.ctx)
}

func (a *RoomAgent) Stop() { a.cancel() }

func (a *RoomAgent) HandleMessage(msg AgentMessage) {
	a.appendHistory(msg)
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
		var keywords []string
		if err := json.Unmarshal([]byte(a.config.TriggerWords), &keywords); err == nil {
			for _, kw := range keywords {
				if strings.Contains(msg.Content, kw) {
					return true
				}
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

	ragChunks := a.retrieveRAG(ctx, msg.Content)
	ragSection := BuildRAGSection(ragChunks)
	historySection := BuildHistorySection(a.getHistory())

	systemPrompt := a.config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "你是本群的 AI 助手，帮助群成员解答问题、总结讨论、提供建议。"
	}

	prompt := buildPrompt(systemPrompt, a.getRoomName(), ragSection, historySection,
		a.getSenderName(msg.SenderID), msg.Content)

	messages := []ChatMessage{{Role: "user", Content: prompt}}

	resp, err := a.llmClient.Chat(ctx, messages, a.config.Temperature, a.tools.AllTools())
	if err != nil {
		log.Printf("[RoomAgent] room=%d LLM 失败: %v", a.roomID, err)
		a.sendReply("AI 助手暂时不可用，请稍后再试。")
		return
	}

	if len(resp.Choices) == 0 {
		return
	}

	choice := resp.Choices[0]

	if len(choice.Message.ToolCalls) > 0 {
		a.handleToolCalls(ctx, messages, choice)
		return
	}

	if reply := choice.Message.Content; reply != "" {
		a.sendReply(reply)
		log.Printf("[RoomAgent] room=%d 回复: %s", a.roomID, truncate(reply, 50))
	}
}

func (a *RoomAgent) handleToolCalls(ctx context.Context, messages []ChatMessage, choice struct {
	Message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}) {
	messages = append(messages, ChatMessage{
		Role:      "assistant",
		ToolCalls: choice.Message.ToolCalls,
	})

	for _, tc := range choice.Message.ToolCalls {
		result, err := a.tools.Dispatch(tc.Function.Name, []byte(tc.Function.Arguments))
		if err != nil {
			log.Printf("[RoomAgent] tool=%s 执行失败: %v", tc.Function.Name, err)
			result = fmt.Sprintf("执行失败: %v", err)
		}
		messages = append(messages, ChatMessage{
			Role:       "tool",
			ToolCallID: tc.ID,
			Content:    result,
		})
	}

	resp2, err := a.llmClient.Chat(ctx, messages, a.config.Temperature, nil)
	if err != nil {
		log.Printf("[RoomAgent] room=%d 第二轮 LLM 失败: %v", a.roomID, err)
		a.sendReply("AI 助手暂时不可用，请稍后再试。")
		return
	}

	if len(resp2.Choices) > 0 && resp2.Choices[0].Message.Content != "" {
		reply := resp2.Choices[0].Message.Content
		a.sendReply(reply)
		log.Printf("[RoomAgent] room=%d function call 回复: %s", a.roomID, truncate(reply, 50))
	}
}

func (a *RoomAgent) retrieveRAG(ctx context.Context, query string) []string {
	if !a.config.RAGEnabled {
		return nil
	}
	results, err := a.retriever.Retrieve(ctx, query, a.roomID, a.config.TopK)
	if err != nil {
		log.Printf("[RoomAgent] room=%d RAG 检索失败: %v", a.roomID, err)
		return nil
	}
	var chunks []string
	for _, r := range results {
		chunks = append(chunks, a.retriever.FormatChunkForPrompt(r.Chunk))
	}
	return chunks
}

func (a *RoomAgent) sendReply(content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := config.KafkaProducer.HandleIncomingMessage(ctx,
		fmt.Sprintf("%d", a.roomID), int(a.botUserID), content,
		fmt.Sprintf("agent-%d-%d", a.roomID, time.Now().UnixNano()))
	if err != nil {
		log.Printf("[RoomAgent] room=%d 发送失败: %v", a.roomID, err)
	}
}

func (a *RoomAgent) appendHistory(msg AgentMessage) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	a.historyBuf[a.historyIdx] = historyEntry{
		SenderName: a.getSenderName(msg.SenderID),
		Content:    msg.Content,
		Time:       msg.Time,
	}
	a.historyIdx = (a.historyIdx + 1) % len(a.historyBuf)
	if a.historyIdx == 0 {
		a.historyFull = true
	}
}

func (a *RoomAgent) getHistory() []string {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	var entries []historyEntry
	if a.historyFull {
		for i := 0; i < len(a.historyBuf); i++ {
			idx := (a.historyIdx + i) % len(a.historyBuf)
			if a.historyBuf[idx].Content != "" {
				entries = append(entries, a.historyBuf[idx])
			}
		}
	} else {
		for i := 0; i < a.historyIdx; i++ {
			if a.historyBuf[i].Content != "" {
				entries = append(entries, a.historyBuf[i])
			}
		}
	}

	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = fmt.Sprintf("[%s] %s: %s", e.Time.Format("2006-01-02 15:04"), e.SenderName, e.Content)
	}
	return result
}

func (a *RoomAgent) getSenderName(userID int64) string {
	user, err := repository.User.GetByID(userID)
	if err != nil || user == nil {
		return fmt.Sprintf("用户%d", userID)
	}
	return user.Username
}

func (a *RoomAgent) getRoomName() string {
	room, err := repository.Room.GetRoomByID(a.roomID)
	if err != nil || room == nil {
		return fmt.Sprintf("群聊%d", a.roomID)
	}
	if room.Name == "" {
		return fmt.Sprintf("群聊%d", a.roomID)
	}
	return room.Name
}

func buildPrompt(systemPrompt, roomName, ragSection, historySection, senderName, question string) string {
	p := ChatPromptTemplate
	p = strings.ReplaceAll(p, "{{.SystemPrompt}}", systemPrompt)
	p = strings.ReplaceAll(p, "{{.RoomName}}", roomName)
	p = strings.ReplaceAll(p, "{{.CurrentTime}}", time.Now().Format("2006-01-02 15:04:05"))
	p = strings.ReplaceAll(p, "{{.RAGSection}}", ragSection)
	p = strings.ReplaceAll(p, "{{.HistorySection}}", historySection)
	p = strings.ReplaceAll(p, "{{.SenderName}}", senderName)
	p = strings.ReplaceAll(p, "{{.Question}}", question)
	return p
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}