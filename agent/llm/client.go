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

// Client 封装 OpenAI 兼容的 LLM API
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// ChatMessage 消息结构
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 请求体
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

// ChatResponse 响应体
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
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

// NewClient 创建 LLM 客户端
func NewClient() *Client {
	baseURL := os.Getenv("LLM_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   "gpt-4o-mini",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetModel 设置模型
func (c *Client) SetModel(model string) {
	c.model = model
}

// Chat 对话补全
func (c *Client) Chat(ctx context.Context, messages []ChatMessage, temperature float64) (*ChatResponse, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		Stream:      false,
	}

	// 序列化
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	// 发送 http 请求
	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/chat/completions", body)
	if err != nil {
		return nil, fmt.Errorf(" chat request: %w", err)
	}
	defer resp.Body.Close()

	// 读入内存
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(" read chat response: %w", err)
	}

	// 确认 http 返回成功
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(" chat API error %d: %s", resp.StatusCode, string(respBody))
	}

	// 序列化 ai 返回的答案
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf(" unmarshal chat response: %w", err)
	}

	// 返回 ai 的回复
	return &chatResp, nil
}

// Embed 文本向量化
func (c *Client) Embed(ctx context.Context, inputs []string) ([]float32, int, error) {

	// 初始化请求
	req := EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: inputs,
	}

	// 序列化
	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal embed request: %w", err)
	}

	// 发起向量化请求
	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/embeddings", body)
	if err != nil {
		return nil, 0, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	// 读入内存
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read embed response: %w", err)
	}

	// 确认状态
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("embed API error %d: %s", resp.StatusCode, string(respBody))
	}

	// 反序列化
	var embedResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, 0, fmt.Errorf("unmarshal embed response: %w", err)
	}

	// 返回错误
	if len(embedResp.Data) == 0 {
		return nil, 0, fmt.Errorf("empty embedding response")
	}

	return embedResp.Data[0].Embedding, embedResp.Usage.TotalTokens, nil
}

// doRequest 通用 HTTP 请求
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return c.httpClient.Do(req)
}
