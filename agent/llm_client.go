package agent

import "lan-im-go/agent/llm"

// 以下为向后兼容的类型别名，新代码请直接使用 agent/llm 包
type LLMClient = llm.Client
type ChatMessage = llm.ChatMessage
type ChatResponse = llm.ChatResponse

var NewLLMClient = llm.NewClient
