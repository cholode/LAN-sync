package agent

import (
	"lan-im-go/services/agent/application/rag"
	"time"
)

type SearchOptions = rag.SearchOptions
type ChunkResult = rag.ChunkResult

// AgentMessage Agent 处理的消息上下文
type AgentMessage struct {
	RoomID   int64
	SenderID int64
	Content  string
	Time     time.Time
}
