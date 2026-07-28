package chunker

import (
	"context"
	"encoding/json"
	"fmt"
	"lan-im-go/agent/llm"
	"lan-im-go/agent/rag"
	"lan-im-go/agent/tool"
	"lan-im-go/core"
	"lan-im-go/models"
	"log"
	"strings"

	"gorm.io/gorm"
)

// TopicChunker 话题分块器（LLM-based），附带内容审核
type TopicChunker struct {
	roomID      int64
	db          *gorm.DB
	llmClient   *llm.Client
	formatter   *rag.ContentFormatter
	chunkStore  *ChunkStore
	reviewTools *tool.Registry
	lastID      int64
	model       string
}

// NewTopicChunker 创建话题分块器，自动注册 kick_user 审核工具
func NewTopicChunker(
	roomID int64, db *gorm.DB,
	llmClient *llm.Client,
	chunkStore *ChunkStore,
	hub *core.Hub,
	model string,
) *TopicChunker {
	rt := tool.NewRegistry()
	rt.Register(tool.NewKickUserHandler(db, hub, roomID))

	return &TopicChunker{
		roomID:      roomID,
		db:          db,
		llmClient:   llmClient,
		formatter:   rag.NewContentFormatter(),
		chunkStore:  chunkStore,
		reviewTools: rt,
		model:       model,
	}
}

func (c *TopicChunker) ChunkType() models.ChunkType { return models.ChunkTypeTopic }

// Chunk 执行话题分块 + 内容审核（合并为一次 LLM 调用）
func (c *TopicChunker) Chunk(ctx context.Context) error {
	messages, err := c.fetchNewMessages(ctx)
	if err != nil {
		return fmt.Errorf("fetch messages: %w", err)
	}
	if len(messages) == 0 {
		return nil
	}

	userMap, err := c.loadUserNames(ctx, messages)
	if err != nil {
		return fmt.Errorf("load user names: %w", err)
	}

	msgWithUsers := c.toMessageWithUsers(messages, userMap)

	// --- 审核 + 话题分块合并为一次 LLM 调用 ---
	segments, err := c.callLLMCombined(ctx, msgWithUsers)
	if err != nil {
		return fmt.Errorf("llm combined call: %w", err)
	}

	log.Printf("[TopicChunker] room=%d 识别到 %d 个话题", c.roomID, len(segments))

	var chunks []*models.RAGChunk
	for _, seg := range segments {
		if seg.StartIndex < 0 || seg.EndIndex >= len(msgWithUsers) || seg.StartIndex > seg.EndIndex {
			continue
		}

		segMsgs := msgWithUsers[seg.StartIndex : seg.EndIndex+1]
		speakers := extractSpeakers(segMsgs)
		startTime := segMsgs[0].CreatedAt
		endTime := segMsgs[len(segMsgs)-1].CreatedAt

		content := c.formatter.FormatTopicChunk(seg.TopicName, speakers, startTime, endTime, segMsgs)

		msgIDs := make([]int64, len(segMsgs))
		for j, m := range segMsgs {
			msgIDs[j] = m.ID
		}
		msgIDsJSON, _ := json.Marshal(msgIDs)
		speakersJSON, _ := json.Marshal(speakers)

		chunks = append(chunks, &models.RAGChunk{
			RoomID:     c.roomID,
			ChunkType:  string(models.ChunkTypeTopic),
			TopicName:  seg.TopicName,
			Speakers:   string(speakersJSON),
			Content:    content,
			StartTime:  startTime,
			EndTime:    endTime,
			MessageIDs: string(msgIDsJSON),
			TokenCount: estimateTokenCount(content),
		})
	}

	if len(chunks) == 0 {
		return nil
	}

	if err := c.chunkStore.BatchSave(ctx, chunks); err != nil {
		return fmt.Errorf("batch save chunks: %w", err)
	}

	if len(messages) > 0 {
		c.lastID = messages[len(messages)-1].ID
	}

	return nil
}

// callLLMCombined 审核 + 话题分块合并为一次 LLM 调用
// 消息只发一次，同时下达审核和分块两条指令。
// kick_user 作为 tool 注册——LLM 发现违规时自动调用。
// LLM 在 content 中返回话题 JSON；
// 若违规触发 tool_call 导致 content 为空，做一次轻量补全拿 JSON。
func (c *TopicChunker) callLLMCombined(ctx context.Context, msgWithUsers []rag.MessageWithUser) ([]TopicSegment, error) {
	msgText := c.formatter.FormatMessagesForChunking(msgWithUsers)

	prompt := fmt.Sprintf(`完成两项任务：
1. 审核消息：发现侮辱、人身攻击、歧视、骚扰言论时，调用 kick_user 移除对应用户
2. 话题分块：输出 JSON 数组 [{"topic_name":"简短话题(≤10字)","start_index":0,"end_index":3}]

分块要求：
- topic_name 准确概括讨论主题
- start_index/end_index 从0开始左闭右闭
- 闲聊、问候也算独立话题

聊天记录：
%s`, msgText)

	messages := []llm.ChatMessage{
		{Role: "system", Content: "你是群聊分析器。同时做内容审核和话题分块。审核发现违规就踢，不要犹豫。分块只输出JSON数组，不要额外解释。"},
		{Role: "user", Content: prompt},
	}

	resp, err := c.llmClient.Chat(ctx, messages, 0.0, c.reviewTools.AllTools())
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices")
	}

	// 1. 处理审核 tool_calls
	for _, tc := range resp.Choices[0].Message.ToolCalls {
		result, dispatchErr := c.reviewTools.Dispatch(tc.Function.Name, []byte(tc.Function.Arguments))
		if dispatchErr != nil {
			log.Printf("[TopicChunker] room=%d %s 失败: %v", c.roomID, tc.Function.Name, dispatchErr)
			continue
		}
		log.Printf("[TopicChunker] room=%d %s: %s", c.roomID, tc.Function.Name, result)
	}

	// 2. 提取话题 JSON
	content := resp.Choices[0].Message.Content
	if content == "" {
		// 违规触发了 tool_call，content 被占，做一次轻量补全
		messages = append(messages, llm.ChatMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: resp.Choices[0].Message.ToolCalls,
		})
		for _, tc := range resp.Choices[0].Message.ToolCalls {
			messages = append(messages, llm.ChatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    "已执行",
			})
		}
		messages = append(messages, llm.ChatMessage{
			Role:    "user",
			Content: "请输出话题分块 JSON 数组",
		})

		resp2, err2 := c.llmClient.Chat(ctx, messages, 0.0, nil)
		if err2 != nil {
			return nil, fmt.Errorf("second round for json: %w", err2)
		}
		if len(resp2.Choices) == 0 {
			return nil, fmt.Errorf("second round empty")
		}
		content = resp2.Choices[0].Message.Content
	}

	return parseTopicSegments(content)
}

// parseTopicSegments 从 LLM 回复中提取并解析话题分段 JSON
func parseTopicSegments(content string) ([]TopicSegment, error) {
	jsonStr := extractJSON(content)
	var segments []TopicSegment
	if err := json.Unmarshal([]byte(jsonStr), &segments); err != nil {
		return nil, fmt.Errorf("parse topic segments: %w\nraw: %s", err, jsonStr)
	}
	return segments, nil
}

func (c *TopicChunker) fetchNewMessages(ctx context.Context) ([]models.Message, error) {
	var messages []models.Message
	query := c.db.WithContext(ctx).
		Where("room_id = ? AND id > ?", c.roomID, c.lastID).
		Order("id ASC").Limit(200)
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (c *TopicChunker) loadUserNames(ctx context.Context, messages []models.Message) (map[int64]string, error) {
	userIDSet := make(map[int64]bool)
	for _, m := range messages {
		userIDSet[m.SenderID] = true
	}
	userIDs := make([]int64, 0, len(userIDSet))
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}
	var users []models.User
	if err := c.db.WithContext(ctx).Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	userMap := make(map[int64]string)
	for _, u := range users {
		userMap[u.ID] = u.Username
	}
	return userMap, nil
}

func (c *TopicChunker) toMessageWithUsers(messages []models.Message, userMap map[int64]string) []rag.MessageWithUser {
	result := make([]rag.MessageWithUser, len(messages))
	for i, m := range messages {
		result[i] = rag.MessageWithUser{
			Message:  m,
			UserName: userMap[m.SenderID],
		}
	}
	return result
}

// extractJSON 从 LLM 回复中提取有效 JSON 部分
func extractJSON(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```javascript")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start != -1 && end != -1 && start < end {
		return s[start : end+1]
	}
	return s
}

func extractSpeakers(messages []rag.MessageWithUser) []string {
	seen := make(map[string]bool)
	var speakers []string
	for _, m := range messages {
		if !seen[m.UserName] {
			seen[m.UserName] = true
			speakers = append(speakers, m.UserName)
		}
	}
	return speakers
}

func estimateTokenCount(text string) int { return len([]rune(text)) / 2 }

func (c *TopicChunker) SetLastID(id int64) { c.lastID = id }
func (c *TopicChunker) LastID() int64      { return c.lastID }
func (c *TopicChunker) SetModel(m string)  { c.model = m; c.llmClient.SetModel(m) }