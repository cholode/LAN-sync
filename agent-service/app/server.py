from __future__ import annotations

import threading

import grpc

from agent.v1 import agent_pb2, agent_pb2_grpc
from app.config import RuntimeConfig
from app.graph import get_graph

SERVICE_VERSION = "0.1.0"

_paused_rooms = set()
_paused_rooms_lock = threading.Lock()


class AgentServiceServicer(agent_pb2_grpc.AgentServiceServicer):
    def ProcessMessage(self, request, context):
        if _is_paused(request.room_id):
            return agent_pb2.ProcessMessageResponse(reply="", skip=True)

        cfg = RuntimeConfig.from_pb(request.config)

        state = {
            "room_id": request.room_id,
            "room_name": request.room_name,
            "bot_user_id": request.bot_user_id,
            "sender_id": request.sender_id,
            "sender_name": request.sender_name,
            "content": request.content,
            "config": cfg.to_dict(),
            "rag_context": "",
            "query_vector": [],
            "history": [],
            "messages": [],
            "last_reply_at": 0.0,
            "prompt_ready": False,
            "reply": "",
            "skip": False,
        }

        try:
            result = get_graph().invoke(
                state,
                {"configurable": {"thread_id": f"room-{request.room_id}"}},
            )
        except Exception as exc:  # pragma: no cover - exercised by integration tests
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(exc))
            return agent_pb2.ProcessMessageResponse(error=str(exc))

        return agent_pb2.ProcessMessageResponse(
            reply=result.get("reply", ""),
            skip=bool(result.get("skip", False)),
        )

    def EnableAgent(self, request, context):
        _set_paused(request.room_id, False)
        return agent_pb2.EnableAgentResponse()

    def PauseAgent(self, request, context):
        _set_paused(request.room_id, True)
        return agent_pb2.PauseAgentResponse()

    def RemoveAgent(self, request, context):
        _set_paused(request.room_id, True)
        return agent_pb2.RemoveAgentResponse()

    def TriggerChunking(self, request, context):
        if _is_paused(request.room_id):
            return agent_pb2.TriggerChunkingResponse(chunked_messages=0)
        return agent_pb2.TriggerChunkingResponse(chunked_messages=0)

    def Health(self, request, context):
        return agent_pb2.HealthResponse(status="ok", version=SERVICE_VERSION)


def _is_paused(room_id: int) -> bool:
    with _paused_rooms_lock:
        return room_id in _paused_rooms


def _set_paused(room_id: int, paused: bool) -> None:
    with _paused_rooms_lock:
        if paused:
            _paused_rooms.add(room_id)
        else:
            _paused_rooms.discard(room_id)
