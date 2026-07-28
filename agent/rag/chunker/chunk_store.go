package chunker

import (
	"context"
	"fmt"
	rag "lan-im-go/agent/rag"
	"lan-im-go/models"
	"log"

	"gorm.io/gorm"
)

// ChunkStore 分块持久化层
// 负责：MySQL 存储 + 向量嵌入 + Qdrant 向量写入
type ChunkStore struct {
	db          *gorm.DB
	embedder    *rag.Embedder
	vectorStore rag.VectorStore
}

// NewChunkStore 创建分块存储
func NewChunkStore(db *gorm.DB, embedder *rag.Embedder, vectorStore rag.VectorStore) *ChunkStore {
	return &ChunkStore{
		db:          db,
		embedder:    embedder,
		vectorStore: vectorStore,
	}
}

// BatchSave 批量保存分块
// 流程: MySQL INSERT -> Embedding -> Qdrant Vector INSERT
func (s *ChunkStore) BatchSave(ctx context.Context, chunks []*models.RAGChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// 1. MySQL 批量插入（先获得自增 ID）
	if err := s.db.WithContext(ctx).Create(&chunks).Error; err != nil {
		return fmt.Errorf("mysql insert chunks: %w", err)
	}

	// 2. 提取文本内容用于向量化
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}

	// 3. 批量向量嵌入
	vectors, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed batch: %w", err)
	}

	// 4. 回填 VectorID
	for _, chunk := range chunks {
		chunk.VectorID = fmt.Sprintf("chunk:room:%d:id:%d", chunk.RoomID, chunk.ID)
	}

	// 5. 写入 Qdrant 向量存储
	roomID := chunks[0].RoomID
	if err := s.vectorStore.EnsureIndex(ctx, roomID); err != nil {
		log.Printf("[ChunkStore] ensure index warning: %v", err)
	}

	if err := s.vectorStore.Insert(ctx, chunks, vectors); err != nil {
		return fmt.Errorf("qdrant vector insert: %w", err)
	}

	// 6. 更新 MySQL 中 chunk 的 vector_id
	for _, chunk := range chunks {
		s.db.WithContext(ctx).Model(chunk).Update("vector_id", chunk.VectorID)
	}

	log.Printf("[ChunkStore] 已保存 %d 个分块 (room=%d)", len(chunks), roomID)
	return nil
}

// GetByRoomID 按房间获取分块列表
func (s *ChunkStore) GetByRoomID(ctx context.Context, roomID int64, chunkType string) ([]*models.RAGChunk, error) {
	var chunks []*models.RAGChunk
	query := s.db.WithContext(ctx).Where("room_id = ?", roomID)
	if chunkType != "" {
		query = query.Where("chunk_type = ?", chunkType)
	}
	if err := query.Order("start_time ASC").Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

// DeleteByRoom 删除房间所有分块
func (s *ChunkStore) DeleteByRoom(ctx context.Context, roomID int64) error {
	if err := s.db.WithContext(ctx).Where("room_id = ?", roomID).Delete(&models.RAGChunk{}).Error; err != nil {
		return err
	}
	return s.vectorStore.DeleteByRoom(ctx, roomID)
}