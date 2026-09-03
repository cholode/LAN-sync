package chunker

import (
	"context"
	"lan-im-go/models"
)

// Chunker 分块器接口
type Chunker interface {
	Chunk(ctx context.Context) error
	ChunkType() models.ChunkType
}

// TopicSegment LLM 切分后的话题段
type TopicSegment struct {
	TopicName  string `json:"topic_name"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}
