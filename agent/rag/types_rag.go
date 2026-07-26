package rag

import "lan-im-go/models"

// SearchOptions 检索参数
type SearchOptions struct {
	ChunkTypes []string
	TopK       int
}

// ChunkResult 检索结果
type ChunkResult struct {
	Chunk      *models.RAGChunk
	Similarity float64
	Score      float64
}