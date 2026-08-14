from __future__ import annotations

from datetime import datetime
from typing import Any

CHAT_PROMPT_TEMPLATE = """### 系统角色
{{.SystemPrompt}}

当前群聊: {{.RoomName}}
当前时间: {{.CurrentTime}}

### 群知识库上下文
{{.RAGSection}}

### 最近对话历史
{{.HistorySection}}

### 当前问题
[{{.SenderName}}] {{.Question}}

请基于上述信息回答。如需查特定时间段的消息，调用 get_messages 函数获取原文后再作答。注意作答的时候要不要用 markdown 的格式，聊天消息不能用 markdown 的格式"""


def build_rag_section(chunks: list[str]) -> str:
    if not chunks:
        return "(暂无相关知识库内容)"
    return "\n---\n".join(chunks)


def build_history_section(history: list[str]) -> str:
    if not history:
        return "(暂无历史消息)"
    return "\n".join(history)


def build_prompt(
    system_prompt: str,
    room_name: str,
    rag_section: str,
    history_section: str,
    sender_name: str,
    question: str,
) -> str:
    replacements: dict[str, Any] = {
        "{{.SystemPrompt}}": system_prompt,
        "{{.RoomName}}": room_name,
        "{{.CurrentTime}}": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        "{{.RAGSection}}": rag_section,
        "{{.HistorySection}}": history_section,
        "{{.SenderName}}": sender_name,
        "{{.Question}}": question,
    }

    prompt = CHAT_PROMPT_TEMPLATE
    for key, value in replacements.items():
        prompt = prompt.replace(key, str(value))
    return prompt
