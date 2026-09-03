from __future__ import annotations

from typing import Any

import grpc

from agent.v1 import agent_pb2, agent_pb2_grpc
from app.settings import get_settings


class IMServiceClient:
    """python 调用 go 的工具"""

    def __init__(self, addr: str | None = None) -> None:
        addr = addr or get_settings().im_service.grpc_addr
        self.channel = grpc.insecure_channel(addr)
        self.stub = agent_pb2_grpc.IMServiceStub(self.channel)

    def fetch_messages(
        self,
        room_id: int,
        start_time_unix_ms: int,
        end_time_unix_ms: int,
        limit: int = 200,
    ) -> list[dict[str, Any]]:
        response = self.stub.FetchMessages(
            agent_pb2.FetchMessagesRequest(
                room_id=room_id,
                start_time_unix_ms=start_time_unix_ms,
                end_time_unix_ms=end_time_unix_ms,
                limit=limit,
            )
        )
        return [
            {
                "message_id": message.message_id,
                "sender_id": message.sender_id,
                "sender_name": message.sender_name,
                "content": message.content,
                "created_at_unix_ms": message.created_at_unix_ms,
            }
            for message in response.messages
        ]

    def kick_user(self, room_id: int, user_id: int, reason: str) -> dict[str, Any]:
        response = self.stub.KickUser(
            agent_pb2.KickUserRequest(room_id=room_id, user_id=user_id, reason=reason)
        )
        return {"removed": response.removed, "message": response.message}

    def send_reply(
        self,
        room_id: int,
        bot_user_id: int,
        content: str,
        message_id: str,
    ) -> None:
        self.stub.SendReply(
            agent_pb2.SendReplyRequest(
                room_id=room_id,
                bot_user_id=bot_user_id,
                content=content,
                message_id=message_id,
            )
        )

    def close(self) -> None:
        self.channel.close()


_client: IMServiceClient | None = None


def get_im_client() -> IMServiceClient:
    global _client
    if _client is None:
        _client = IMServiceClient()
    return _client
