from __future__ import annotations

import grpc

from agent.v1 import agent_pb2, agent_pb2_grpc
from app.config import RuntimeConfig
from app.graph import get_graph

SERVICE_VERSION = "0.1.0"


class AgentServiceServicer(agent_pb2_grpc.AgentServiceServicer):
    def ProcessMessage(self, request, context):
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
        return agent_pb2.EnableAgentResponse()

    def PauseAgent(self, request, context):
        return agent_pb2.PauseAgentResponse()

    def RemoveAgent(self, request, context):
        return agent_pb2.RemoveAgentResponse()

    def TriggerChunking(self, request, context):
        return agent_pb2.TriggerChunkingResponse(chunked_messages=0)

    def Health(self, request, context):
        return agent_pb2.HealthResponse(status="ok", version=SERVICE_VERSION)
