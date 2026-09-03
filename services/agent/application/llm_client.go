package agent

import "lan-im-go/services/agent/application/llm"

// 类型别名，新代码可直接用 agent.LLMClient 等，无需改引用
type LLMClient = llm.Client
type EmbedClient = llm.EmbedClient
type ChatMessage = llm.ChatMessage
type ChatResponse = llm.ChatResponse
type ToolCall = llm.ToolCall
type Tool = llm.Tool

var NewLLMClient = llm.NewClient
var NewEmbedClient = llm.NewEmbedClient
