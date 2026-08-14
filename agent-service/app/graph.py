from __future__ import annotations

import operator
import time
from datetime import datetime
from typing import Annotated, Any, TypedDict

from langchain_core.messages import BaseMessage, HumanMessage, SystemMessage, ToolMessage
from langchain_openai import ChatOpenAI
from langgraph.checkpoint.memory import MemorySaver
from langgraph.graph import END, START, StateGraph

from app.config import RuntimeConfig
from app.settings import get_settings
from app.prompt import build_history_section, build_prompt, build_rag_section
from app.rag import get_retriever
from app.tools import build_get_messages_tool

COOLDOWN_SECONDS = get_settings().agent.cooldown_seconds


class AgentState(TypedDict):
    room_id: int
    room_name: str
    bot_user_id: int
    sender_id: int
    sender_name: str
    content: str
    config: dict[str, Any]
    rag_context: str
    query_vector: list[float]
    history: Annotated[list[dict[str, Any]], operator.add]
    messages: Annotated[list[BaseMessage], operator.add]
    last_reply_at: float
    prompt_ready: bool
    reply: str
    skip: bool


_memory = MemorySaver()
_graph = None


def get_graph():
    global _graph
    if _graph is None:
        _graph = _build_graph()
    return _graph


def _build_graph():
    workflow = StateGraph(AgentState)

    workflow.add_node("trigger", trigger_node)
    workflow.add_node("retrieve", retrieve_node)
    workflow.add_node("prepare", prepare_node)
    workflow.add_node("agent", agent_node)
    workflow.add_node("tools", tool_node)

    workflow.add_edge(START, "trigger")
    workflow.add_conditional_edges(
        "trigger",
        lambda state: "end" if state.get("skip") else "retrieve",
        {"end": END, "retrieve": "retrieve"},
    )
    workflow.add_edge("retrieve", "prepare")
    workflow.add_edge("prepare", "agent")
    workflow.add_conditional_edges(
        "agent",
        should_continue_after_agent,
        {"tools": "tools", "end": END},
    )
    workflow.add_edge("tools", "agent")

    return workflow.compile(checkpointer=_memory)


def trigger_node(state: AgentState) -> dict[str, Any]:
    cfg = RuntimeConfig(**state["config"])
    entry = {
        "sender_name": state.get("sender_name") or f"用户{state['sender_id']}",
        "content": state["content"],
        "time": datetime.now().strftime("%Y-%m-%d %H:%M"),
    }

    if not should_trigger(state["content"], cfg, state["room_id"]):
        return {"history": [entry], "skip": True}

    now = time.time()
    if now - float(state.get("last_reply_at") or 0) < COOLDOWN_SECONDS:
        return {"history": [entry], "skip": True}

    return {"history": [entry], "skip": False}


def should_trigger(content: str, cfg: RuntimeConfig, room_id: int) -> bool:
    if cfg.trigger_mode == 1:
        bot_name = f"@AI助手_群{room_id}"
        return any(
            keyword in content
            for keyword in ("@agent", "@AI助手", bot_name)
        )

    if cfg.trigger_mode == 2:
        return True

    if cfg.trigger_mode == 3:
        return any(keyword in content for keyword in cfg.trigger_words)

    return False


def retrieve_node(state: AgentState) -> dict[str, Any]:
    cfg = RuntimeConfig(**state["config"])
    if not cfg.rag_enabled:
        return {"rag_context": "", "query_vector": []}

    try:
        results = get_retriever().retrieve(
            query=state["content"],
            room_id=state["room_id"],
            top_k=cfg.top_k,
        )
        chunks = [get_retriever().format_chunk_for_prompt(result) for result in results]
    except Exception:
        chunks = []

    return {"rag_context": build_rag_section(chunks), "query_vector": []}


def prepare_node(state: AgentState) -> dict[str, Any]:
    cfg = RuntimeConfig(**state["config"])

    history_entries = state.get("history", [])
    if cfg.max_history > 0:
        history_entries = history_entries[-cfg.max_history :]

    history_lines = [
        f"[{entry['time']}] {entry['sender_name']}: {entry['content']}"
        for entry in history_entries
    ]
    history_section = build_history_section(history_lines)

    system_prompt = cfg.system_prompt or "你是本群的 AI 助手，帮助群成员解答问题、总结讨论、提供建议。"
    room_name = state.get("room_name") or f"群聊{state['room_id']}"

    prompt = build_prompt(
        system_prompt=system_prompt,
        room_name=room_name,
        rag_section=state.get("rag_context") or "(暂无相关知识库内容)",
        history_section=history_section,
        sender_name=state.get("sender_name") or f"用户{state['sender_id']}",
        question=state["content"],
    )

    return {
        "messages": [HumanMessage(content=prompt)],
        "prompt_ready": True,
        "last_reply_at": time.time(),
    }


def agent_node(state: AgentState) -> dict[str, Any]:
    cfg = RuntimeConfig(**state["config"])
    system_prompt = cfg.system_prompt or "你是本群的 AI 助手，帮助群成员解答问题、总结讨论、提供建议。"

    settings = get_settings()
    llm = ChatOpenAI(
        model=cfg.model_name,
        temperature=cfg.temperature,
        base_url=settings.llm.base_url,
        api_key=settings.llm.api_key,
        timeout=settings.llm.timeout_seconds,
    )

    tools = [build_get_messages_tool(state["room_id"])]
    llm_with_tools = llm.bind_tools(tools)

    messages = [SystemMessage(content=system_prompt), *state.get("messages", [])]
    response = llm_with_tools.invoke(messages)

    return {
        "messages": [response],
        "reply": str(response.content or ""),
    }


def should_continue_after_agent(state: AgentState) -> str:
    messages = state.get("messages", [])
    if not messages:
        return "end"

    last_message = messages[-1]
    tool_calls = getattr(last_message, "tool_calls", None)
    return "tools" if tool_calls else "end"


def tool_node(state: AgentState) -> dict[str, Any]:
    messages = state.get("messages", [])
    if not messages:
        return {"messages": []}

    last_message = messages[-1]
    tool_calls = getattr(last_message, "tool_calls", None) or []

    tool_results: list[ToolMessage] = []
    tool_by_name = {
        "get_messages": build_get_messages_tool(state["room_id"]),
    }

    for tool_call in tool_calls:
        name = tool_call.get("name")
        args = tool_call.get("args") or {}
        call_id = tool_call.get("id") or ""

        tool_fn = tool_by_name.get(name)
        if tool_fn is None:
            content = f"未知工具: {name}"
        else:
            try:
                result = tool_fn.invoke(args)
                content = result.content if hasattr(result, "content") else str(result)
            except Exception as exc:
                content = f"执行失败: {exc}"

        tool_results.append(ToolMessage(content=content, tool_call_id=call_id, name=name))

    return {"messages": tool_results}
