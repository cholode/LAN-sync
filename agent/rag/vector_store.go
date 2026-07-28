package rag

import (
	"context"
	"lan-im-go/models"
)

// VectorStore 向量存储接口，当前由 Qdrant 实现
type VectorStore interface {
	// 确保存在，不在就创建
	EnsureIndex(ctx context.Context, roomID int64) error

	// 插入向量和向量代表的内容
	Insert(ctx context.Context, chunks []*models.RAGChunk, vectors [][]float32) error

	// 根据向量和查找与集合找到相应数量的chunk
	Search(ctx context.Context, queryVec []float32, roomID int64, opts SearchOptions) ([]ChunkResult, error)

	// 根据房间号删除
	DeleteByRoom(ctx context.Context, roomID int64) error
}
