package rag

import (
	"context"
	"fmt"
	"lan-im-go/models"
	"lan-im-go/pkg"
	"strings"
)

// Retriever RAG 检索器，仅检索 topic 块
type Retriever struct {
	embedder    *Embedder
	vectorStore VectorStore
}

// NewRetriever 创建检索器
func NewRetriever(embedder *Embedder, vectorStore VectorStore) *Retriever {
	return &Retriever{
		embedder:    embedder,
		vectorStore: vectorStore,
	}
}

// Retrieve 语义检索 Qdrant 中的 topic 块
func (r *Retriever) Retrieve(ctx context.Context, query string, roomID int64, topK int) ([]ChunkResult, error) {
	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	opts := SearchOptions{
		ChunkTypes: []string{"topic"},
		TopK:       topK,
	}

	results, err := r.vectorStore.Search(ctx, queryVec, roomID, opts)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	// 去重
	seen := make(map[string]bool)
	out := make([]ChunkResult, 0, len(results))
	for _, res := range results {
		if seen[res.Chunk.Content] {
			continue
		}
		seen[res.Chunk.Content] = true
		res.Score = res.Similarity
		out = append(out, res)
	}

	pkg.Infof("[Retriever] room=%d 检索完成, 返回 %d 条结果", roomID, len(out))
	return out, nil
}

// FormatChunkForPrompt 将检索结果格式化为 Prompt 文本
func (r *Retriever) FormatChunkForPrompt(chunk *models.RAGChunk) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【话题: %s】\n", chunk.TopicName))
	sb.WriteString(fmt.Sprintf("时间: %s ~ %s\n",
		chunk.StartTime.Format("2006-01-02 15:04"),
		chunk.EndTime.Format("2006-01-02 15:04")))
	sb.WriteString(chunk.Content)
	return sb.String()
}