package agent

import (
	"lan-im-go/agent/rag"
	"time"
)

// 对 rag 进行封装
type SearchOptions = rag.SearchOptions
type TimeFilter = rag.TimeFilter
type ChunkResult = rag.ChunkResult
type SearchMode = rag.SearchMode

const (
	SearchTopicOnly = rag.SearchTopicOnly
	SearchTimeOnly  = rag.SearchTimeOnly
	SearchBoth      = rag.SearchBoth
)

// AgentMessage Agent 处理的消息上下文
type AgentMessage struct {
	RoomID   int64
	SenderID int64
	Content  string
	Time     time.Time
}
