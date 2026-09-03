package rag

import (
	"context"
	"fmt"
	"lan-im-go/services/agent/application/llm"
)

// Embedder 嵌入服务封装
type Embedder struct {
	client *llm.EmbedClient
	dim    int
}

// NewEmbedder 创建嵌入服务
func NewEmbedder(client *llm.EmbedClient) *Embedder {
	return &Embedder{client: client, dim: client.Dim()}
}

// Embed 单条文本向量化
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.client.Embed(ctx, text)
}

// EmbedBatch 批量文本向量化
// 每次请求最大 100 条，按序返回
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 100
	result := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		vecs, err := e.client.EmbedMulti(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}

		result = append(result, vecs...)
	}

	return result, nil
}
