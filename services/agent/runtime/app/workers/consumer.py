from __future__ import annotations

import json
from datetime import datetime

from aiokafka import AIOKafkaConsumer
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError

from app.settings import get_settings
from app.storage.database import session_factory
from app.storage.models import AgentMessageInbox, RoomAgentBinding


def decode_message(value: bytes) -> dict:
    try:
        from im.v1.message_pb2 import ChatMessage

        message = ChatMessage()
        message.ParseFromString(value)
        if message.room_id:
            return {
                "room_id": message.room_id,
                "sender_id": message.sender_id,
                "message_id": message.client_msg_id,
                "content": message.content,
                "message_time": datetime.fromtimestamp(message.created_at_ns / 1_000_000_000),
            }
    except Exception:
        pass

    raw = json.loads(value.decode("utf-8"))
    timestamp = int(raw.get("timestamp") or 0)
    return {
        "room_id": int(raw["room_id"]),
        "sender_id": int(raw["sender_id"]),
        "message_id": str(raw.get("client_msg_id") or ""),
        "content": str(raw.get("content") or ""),
        "message_time": datetime.fromtimestamp(timestamp / 1_000_000_000) if timestamp else datetime.now(),
    }


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
                event_id = f"{record.topic}:{record.partition}:{record.offset}"
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
                await consumer.commit()
        finally:
            await consumer.stop()
