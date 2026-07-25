package rag

import (
	"context"
	"fmt"
	"lan-im-go/agent/llm"
)

// Embedder 嵌入服务封装
type Embedder struct {
	llmClient *llm.Client
	dim       int
}

// NewEmbedder 创建嵌入服务
func NewEmbedder(llmClient *llm.Client) *Embedder {
	return &Embedder{
		llmClient: llmClient,
		dim:       1536,
	}
}

// Embed 单条文本向量化
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vec, _, err := e.llmClient.Embed(ctx, []string{text})
	return vec, err
}

// EmbedBatch 批量文本向量化
// 利用 Embedding API 原生批量能力，每次请求最多 100 条，按序返回
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	const batchSize = 100
	result := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		vecs, _, err := e.llmClient.EmbedMulti(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}

		result = append(result, vecs...)
	}

	return result, nil
}