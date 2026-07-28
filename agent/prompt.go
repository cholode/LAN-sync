package agent

import "strings"

// ChatPromptTemplate 对话 Prompt 模板
const ChatPromptTemplate = "### 系统角色\n{{.SystemPrompt}}\n\n当前群聊: {{.RoomName}}\n当前时间: {{.CurrentTime}}\n\n### 群知识库上下文\n{{.RAGSection}}\n\n### 最近对话历史\n{{.HistorySection}}\n\n### 当前问题\n[{{.SenderName}}] {{.Question}}\n\n请基于上述信息回答。如需查特定时间段的消息，调用 get_messages 函数获取原文后再作答。"

// BuildRAGSection 构建 RAG 片段
func BuildRAGSection(chunks []string) string {
	if len(chunks) == 0 {
		return "(暂无相关知识库内容)"
	}
	var sb strings.Builder
	for i, c := range chunks {
		sb.WriteString(c + "\n")
		if i < len(chunks)-1 {
			sb.WriteString("---\n")
		}
	}
	return sb.String()
}

// BuildHistorySection 构建历史消息片段
func BuildHistorySection(history []string) string {
	if len(history) == 0 {
		return "(暂无历史消息)"
	}
	return strings.Join(history, "\n")
}