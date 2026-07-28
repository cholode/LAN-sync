package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// EmbedClient 向量嵌入客户端，默认使用硅基流动的中文 embedding 模型
type EmbedClient struct {
	baseURL    string
	apiKey     string
	model      string
	dim        int
	httpClient *http.Client
}

// EmbeddingRequest 嵌入请求
type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbeddingResponse 嵌入响应
type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// NewEmbedClient 创建 Embedding 客户端
// EMBED_BASE_URL — 默认 https://api.siliconflow.cn/v1（硅基流动）
// EMBED_API_KEY — API Key
// EMBED_MODEL   — 默认 BAAI/bge-large-zh-v1.5（1024维，中文优化）
func NewEmbedClient() *EmbedClient {
	baseURL := os.Getenv("EMBED_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.siliconflow.cn/v1"
	}
	apiKey := os.Getenv("EMBED_API_KEY")
	model := os.Getenv("EMBED_MODEL")
	if model == "" {
		model = "BAAI/bge-large-zh-v1.5"
	}

	return &EmbedClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		dim:     1024,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Dim 返回向量维度
func (c *EmbedClient) Dim() int { return c.dim }

// Embed 单条文本向量化
func (c *EmbedClient) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedMulti(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return vecs[0], nil
}

// EmbedMulti 批量文本向量化，一次请求传入整个 batch（最多100条）
func (c *EmbedClient) EmbedMulti(ctx context.Context, inputs []string) ([][]float32, error) {
	req := EmbeddingRequest{
		Model: c.model,
		Input: inputs,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/embeddings", body)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed API error %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal embed response: %w", err)
	}

	vecs := make([][]float32, len(embedResp.Data))
	for _, d := range embedResp.Data {
		if d.Index < 0 || d.Index >= len(vecs) {
			continue
		}
		vecs[d.Index] = d.Embedding
	}

	return vecs, nil
}

func (c *EmbedClient) doRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.httpClient.Do(req)
}