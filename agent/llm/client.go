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
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

// ToolCall 函数调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用详情
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool 工具定义
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ChatRequest 请求体
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
	Tools       []Tool        `json:"tools,omitempty"`
}

// ChatResponse 响应体
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
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

// Chat 对话补全，支持 function calling
func (c *Client) Chat(ctx context.Context, messages []ChatMessage, temperature float64, tools []Tool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: temperature,
		Stream:      false,
		Tools:       tools,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/chat/completions", body)
	if err != nil {
		return nil, fmt.Errorf("chat request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read chat response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal chat response: %w", err)
	}

	return &chatResp, nil
}

// Embed 单条文本向量化，返回第一条结果
func (c *Client) Embed(ctx context.Context, inputs []string) ([]float32, int, error) {
	vecs, tokens, err := c.EmbedMulti(ctx, inputs)
	if err != nil {
		return nil, 0, err
	}
	if len(vecs) == 0 {
		return nil, 0, fmt.Errorf("empty embedding response")
	}
	return vecs[0], tokens, nil
}

// EmbedMulti 批量文本向量化，一次请求传入整个 batch
func (c *Client) EmbedMulti(ctx context.Context, inputs []string) ([][]float32, int, error) {
	req := EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: inputs,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal embed request: %w", err)
	}

	resp, err := c.doRequest(ctx, "POST", c.baseURL+"/embeddings", body)
	if err != nil {
		return nil, 0, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read embed response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("embed API error %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp EmbeddingResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, 0, fmt.Errorf("unmarshal embed response: %w", err)
	}

	vecs := make([][]float32, len(embedResp.Data))
	for _, d := range embedResp.Data {
		if d.Index < 0 || d.Index >= len(vecs) {
			continue
		}
		vecs[d.Index] = d.Embedding
	}

	return vecs, embedResp.Usage.TotalTokens, nil
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