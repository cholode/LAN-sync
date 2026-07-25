package rag

import (
	"context"
	"fmt"
	"lan-im-go/agent/llm"
	"math"
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
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))
	batchSize := 100
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		for _, t := range texts[i:end] {
			vec, _, err := e.llmClient.Embed(ctx, []string{t})
			if err != nil {
				return nil, fmt.Errorf("embed batch [%d]: %w", i, err)
			}
			result = append(result, vec)
		}
	}
	return result, nil
}

// CosineSimilarity 余弦相似度
func (e *Embedder) CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot float64
	var normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
