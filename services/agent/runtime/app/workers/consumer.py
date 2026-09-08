from __future__ import annotations

import json
from aiokafka import AIOKafkaConsumer
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError

from app.settings import get_settings
from app.storage.database import session_factory
from app.storage.models import AgentMessageInbox, RoomAgentBinding
from app.time_utils import utc_from_timestamp, utc_now


def decode_message(value: bytes) -> dict:
    try:
        from im.v1.message_pb2 import ChatMessage

        message = ChatMessage()
        message.ParseFromString(value)
        if message.room_id:
            return {
                "room_id": message.room_id,
                "sender_id": message.sender_id,
                "message_id": str(message.message_id) if message.message_id else "",
                "client_msg_id": message.client_msg_id,
                "room_seq": message.room_seq,
                "content": message.content,
                "message_time": utc_from_timestamp(message.created_at_ns / 1_000_000_000),
            }
    except Exception:
        pass

    raw = json.loads(value.decode("utf-8"))
    timestamp = int(raw.get("timestamp") or 0)
    return {
        "room_id": int(raw["room_id"]),
        "sender_id": int(raw["sender_id"]),
        "message_id": str(raw.get("message_id") or raw.get("id") or ""),
        "client_msg_id": str(raw.get("client_msg_id") or ""),
        "room_seq": int(raw.get("room_seq") or 0),
        "content": str(raw.get("content") or ""),
        "message_time": utc_from_timestamp(timestamp / 1_000_000_000) if timestamp else utc_now(),
    }


def inbox_event_id(payload: dict, topic: str, partition: int, offset: int) -> str:
    # 正式消息重投会改变 Kafka offset，业务幂等键必须保持不变。
    if payload.get("message_id"):
        return f"message:{payload['message_id']}"
    if payload.get("client_msg_id"):
        return f"client:{payload['sender_id']}:{payload['client_msg_id']}"
    return f"{topic}:{partition}:{offset}"


class InboxConsumer:
    async def run(self) -> None:
        settings = get_settings().kafka
        consumer = AIOKafkaConsumer(
            settings.topic,
            bootstrap_servers=settings.brokers,
            group_id=settings.group_id,
            enable_auto_commit=False,
            auto_offset_reset=settings.auto_offset_reset,
        )
        await consumer.start()
        try:
            async for record in consumer:
                payload = decode_message(record.value)
                event_id = inbox_event_id(payload, record.topic, record.partition, record.offset)
                async with session_factory() as session:
                    bindings = list(await session.scalars(select(RoomAgentBinding).where(
                        RoomAgentBinding.room_id == payload["room_id"],
                        RoomAgentBinding.enabled.is_(True),
                        RoomAgentBinding.deleted_at == 0,
                    )))
                    if not bindings or payload["sender_id"] in {
                        binding.legacy_bot_user_id for binding in bindings if binding.legacy_bot_user_id
                    }:
                        await consumer.commit()
                        continue
                    session.add(AgentMessageInbox(
                        event_id=event_id,
                        topic=record.topic,
                        partition_id=record.partition,
                        kafka_offset=record.offset,
                        room_id=payload["room_id"],
                        message_id=payload["message_id"] or event_id,
                        sender_id=payload["sender_id"],
                        sender_type="user",
                        content=payload["content"],
                        message_time=payload["message_time"],
                    ))
                    try:
                        await session.commit()
                    except IntegrityError:
                        await session.rollback()
                        # 只有相同业务事件已存在才能确认；其他约束错误不得被当成去重成功。
                        existing = await session.scalar(select(AgentMessageInbox.id).where(
                            AgentMessageInbox.event_id == event_id,
                        ))
                        if existing is None:
                            raise
                await consumer.commit()
        finally:
            await consumer.stop()
