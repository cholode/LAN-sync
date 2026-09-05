from __future__ import annotations

from datetime import datetime, timezone

from langchain_core.tools import tool

from app.im_client import get_im_client
from app.time_utils import utc_from_timestamp


def _parse_iso(value: str) -> datetime:
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def build_get_messages_tool(room_id: int):
    @tool("get_messages")
    def get_messages(start_time: str, end_time: str) -> str:
        """获取群聊中指定时间段的原始消息。

        当用户询问带有限定时间（如昨天、上周三、7月24号下午）的问题时，
        先调用此函数获取该时间段的消息原文，再基于原文作答。
        """
        start = _parse_iso(start_time)
        end = _parse_iso(end_time)

        messages = get_im_client().fetch_messages(
            room_id=room_id,
            start_time_unix_ms=int(start.timestamp() * 1000),
            end_time_unix_ms=int(end.timestamp() * 1000),
            limit=200,
        )

        if not messages:
            return "该时间段内没有消息记录。"

        lines = [
            f"时间段 {start.strftime('%Y-%m-%d %H:%M')} ~ {end.strftime('%Y-%m-%d %H:%M')} 的消息记录：\n"
        ]
        for message in messages:
            created = utc_from_timestamp(message["created_at_unix_ms"] / 1000)
            sender_name = message.get("sender_name") or f"用户{message['sender_id']}"
            lines.append(
                f"[{created.strftime('%Y-%m-%d %H:%M')}] {sender_name}: {message['content']}"
            )
        return "\n".join(lines)

    return get_messages
