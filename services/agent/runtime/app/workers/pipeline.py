from __future__ import annotations

import asyncio
import logging
import uuid
from collections import defaultdict
from datetime import timedelta

from sqlalchemy import func, select, update

from app.embeddings import get_embedder
from app.im_client import get_im_client
from app.rag import get_retriever
from app.settings import get_settings
from app.storage.database import session_factory
from app.storage.models import (
    AgentChunk,
    AgentMessageInbox,
    ModerationRecord,
    RemovalRequest,
    RoomAgentBinding,
    RoomAgentConfig,
)
from app.workflows.agents import answer_question, chunk_messages, moderate_messages
from app.time_utils import utc_now


logger = logging.getLogger(__name__)


class AgentPipeline:
    def __init__(self) -> None:
        self.settings = get_settings()

    async def run(self) -> None:
        while True:
            try:
                await self._requeue_stale()
                room_ids = await self._ready_rooms()
                for room_id in room_ids:
                    await self._process_room(room_id)
                await self._retry_approved_chunks()
            except Exception:
                logger.exception("agent pipeline iteration failed")
            await asyncio.sleep(self.settings.kafka.poll_interval_seconds)

    async def _requeue_stale(self) -> None:
        cutoff = utc_now() - timedelta(minutes=5)
        async with session_factory() as session:
            await session.execute(
                update(AgentMessageInbox)
                .where(
                    AgentMessageInbox.status == "processing",
                    AgentMessageInbox.updated_at < cutoff,
                )
                .values(status="pending", error_message="stale processing lease recovered")
            )
            await session.commit()

    async def _retry_approved_chunks(self) -> None:
        retry_before = utc_now() - timedelta(seconds=30)
        async with session_factory() as session:
            room_ids = list(await session.scalars(
                select(AgentMessageInbox.room_id)
                .where(
                    AgentMessageInbox.status == "approved",
                    AgentMessageInbox.updated_at < retry_before,
                )
                .distinct()
                .limit(20)
            ))
        for room_id in room_ids:
            async with session_factory() as session:
                messages = list(await session.scalars(
                    select(AgentMessageInbox)
                    .where(AgentMessageInbox.room_id == room_id, AgentMessageInbox.status == "approved")
                    .order_by(AgentMessageInbox.message_time, AgentMessageInbox.id)
                    .limit(self.settings.kafka.batch_size)
                ))
                bindings = list(await session.scalars(select(RoomAgentBinding).where(
                    RoomAgentBinding.room_id == room_id,
                    RoomAgentBinding.enabled.is_(True),
                    RoomAgentBinding.deleted_at == 0,
                )))
            if messages:
                await self._run_chunking(room_id, bindings, messages)

    async def _ready_rooms(self) -> list[int]:
        cutoff = utc_now() - timedelta(seconds=self.settings.kafka.batch_window_seconds)
        async with session_factory() as session:
            rows = await session.execute(
                select(
                    AgentMessageInbox.room_id,
                    func.count(AgentMessageInbox.id),
                    func.min(AgentMessageInbox.created_at),
                )
                .where(AgentMessageInbox.status == "pending")
                .group_by(AgentMessageInbox.room_id)
            )
            ready: list[int] = []
            for room_id, count, oldest in rows:
                if count >= self.settings.kafka.batch_size or oldest <= cutoff:
                    ready.append(room_id)
                    continue
                if await self._room_has_trigger(session, room_id):
                    ready.append(room_id)
            return ready

    async def _room_has_trigger(self, session, room_id: int) -> bool:
        configs = await session.scalars(
            select(RoomAgentConfig)
            .join(RoomAgentBinding, RoomAgentBinding.id == RoomAgentConfig.binding_id)
            .where(
                RoomAgentBinding.room_id == room_id,
                RoomAgentBinding.enabled.is_(True),
                RoomAgentBinding.deleted_at == 0,
                RoomAgentConfig.deleted_at == 0,
            )
        )
        words = [word.lower() for cfg in configs for word in (cfg.trigger_words or []) if word]
        if not words:
            return False
        contents = await session.scalars(
            select(AgentMessageInbox.content).where(
                AgentMessageInbox.room_id == room_id,
                AgentMessageInbox.status == "pending",
            ).limit(self.settings.kafka.batch_size)
        )
        return any(word in content.lower() for content in contents for word in words)

    async def _process_room(self, room_id: int) -> None:
        async with session_factory() as session:
            messages = list(await session.scalars(
                select(AgentMessageInbox)
                .where(AgentMessageInbox.room_id == room_id, AgentMessageInbox.status == "pending")
                .order_by(AgentMessageInbox.message_time, AgentMessageInbox.id)
                .limit(self.settings.kafka.batch_size)
                .with_for_update(skip_locked=True)
            ))
            if not messages:
                return

            bindings = list(await session.scalars(
                select(RoomAgentBinding).where(
                    RoomAgentBinding.room_id == room_id,
                    RoomAgentBinding.enabled.is_(True),
                    RoomAgentBinding.deleted_at == 0,
                ).order_by(RoomAgentBinding.priority, RoomAgentBinding.id)
            ))
            legacy_bot_ids = {item.legacy_bot_user_id for item in bindings if item.legacy_bot_user_id}
            user_messages = []
            for message in messages:
                if message.sender_id in legacy_bot_ids:
                    message.status = "ignored"
                else:
                    message.status = "processing"
                    user_messages.append(message)
            await session.commit()

        if not user_messages:
            return

        decisions = await self._moderate(user_messages)
        approved = await self._save_moderation(user_messages, decisions)
        if not approved:
            return

        try:
            await self._run_conversations(room_id, bindings, approved)
        except Exception:
            logger.exception("conversation pipeline failed for room %s", room_id)
        await self._run_chunking(room_id, bindings, approved)

    async def _moderate(self, messages: list[AgentMessageInbox]) -> dict[str, dict]:
        payload = [{
            "message_id": item.message_id,
            "sender_id": item.sender_id,
            "content": item.content,
            "created_at": item.message_time.isoformat(),
        } for item in messages]
        try:
            result = await asyncio.to_thread(moderate_messages, payload)
            return {item.message_id: item.model_dump() for item in result.results}
        except Exception as exc:
            logger.exception("moderation agent failed")
            return {item.message_id: {
                "message_id": item.message_id,
                "sender_id": item.sender_id,
                "status": "needs_review",
                "rule_code": "AGENT_ERROR",
                "reason": str(exc),
                "evidence": None,
                "confidence": 0.0,
                "request_removal": False,
            } for item in messages}

    async def _save_moderation(self, messages, decisions) -> list[AgentMessageInbox]:
        approved_ids: set[int] = set()
        async with session_factory() as session:
            for source in messages:
                item = decisions.get(source.message_id) or {
                    "status": "needs_review", "rule_code": "MISSING_RESULT",
                    "reason": "moderation result missing", "confidence": 0.0,
                }
                status = item.get("status", "needs_review")
                record = ModerationRecord(
                    inbox_id=source.id,
                    room_id=source.room_id,
                    message_id=source.message_id,
                    sender_id=source.sender_id,
                    status=status,
                    rule_code=item.get("rule_code"),
                    reason=item.get("reason"),
                    evidence=item.get("evidence"),
                    confidence=item.get("confidence"),
                    raw_result=item,
                )
                session.add(record)
                await session.flush()
                db_message = await session.get(AgentMessageInbox, source.id)
                db_message.status = status
                if status == "approved":
                    approved_ids.add(source.id)
                elif status == "rejected" and item.get("request_removal"):
                    session.add(RemovalRequest(
                        room_id=source.room_id,
                        target_user_id=source.sender_id,
                        moderation_record_id=record.id,
                        reason=item.get("reason") or "违反群聊规则",
                        evidence={"message_id": source.message_id, "text": item.get("evidence")},
                    ))
            await session.commit()

        return [item for item in messages if item.id in approved_ids]

    async def _load_config(self, binding_id: int) -> RoomAgentConfig | None:
        async with session_factory() as session:
            return await session.scalar(select(RoomAgentConfig).where(
                RoomAgentConfig.binding_id == binding_id,
                RoomAgentConfig.deleted_at == 0,
            ))

    async def _run_conversations(self, room_id, bindings, messages) -> None:
        for binding in bindings:
            config = await self._load_config(binding.id)
            if config is None or not binding.legacy_bot_user_id:
                continue
            words = [word.lower() for word in (config.trigger_words or []) if word]
            for message in messages:
                if not words or not any(word in message.content.lower() for word in words):
                    continue
                context = ""
                if config.rag_enabled:
                    try:
                        chunks = await asyncio.to_thread(
                            get_retriever().retrieve, message.content, room_id, self.settings.agent.top_k
                        )
                        context = "\n\n".join(chunk.content for chunk in chunks)
                    except Exception:
                        logger.exception("conversation retrieval failed")
                reply = await asyncio.to_thread(
                    answer_question,
                    system_prompt=config.system_prompt or "",
                    question=message.content,
                    context=context,
                    room_id=room_id,
                )
                if reply:
                    await asyncio.to_thread(
                        get_im_client().send_reply,
                        room_id,
                        binding.legacy_bot_user_id,
                        reply,
                        f"agent-{binding.id}-{message.message_id}",
                    )

    async def _run_chunking(self, room_id, bindings, messages) -> None:
        payload = [{
            "message_id": item.message_id,
            "sender_id": item.sender_id,
            "content": item.content,
            "created_at": item.message_time.isoformat(),
        } for item in messages]
        allowed_ids = {item.message_id for item in messages}
        rag_bindings = []
        for binding in bindings:
            config = await self._load_config(binding.id)
            if config is not None and config.rag_enabled:
                rag_bindings.append(binding)

        if not rag_bindings:
            await self._mark_chunked(messages)
            return

        try:
            result = await asyncio.to_thread(chunk_messages, payload)
            chunks = [item for item in result.chunks if item.message_ids and set(item.message_ids) <= allowed_ids]
            if not chunks:
                raise ValueError("chunk agent returned no valid chunks")
            for binding in rag_bindings:
                for chunk in chunks:
                    selected = [item for item in messages if item.message_id in chunk.message_ids]
                    vector = await asyncio.to_thread(get_embedder().embed, chunk.content)
                    point_id = str(uuid.uuid5(uuid.NAMESPACE_URL, f"{binding.id}:{','.join(chunk.message_ids)}"))
                    await asyncio.to_thread(
                        get_retriever().vector_store.upsert_chunk,
                        point_id=point_id,
                        vector=vector,
                        room_id=room_id,
                        binding_id=binding.id,
                        topic_name=chunk.topic_name,
                        content=chunk.content,
                        message_ids=chunk.message_ids,
                        start_time=min(item.message_time for item in selected),
                        end_time=max(item.message_time for item in selected),
                    )
                    async with session_factory() as session:
                        session.add(AgentChunk(
                            room_id=room_id,
                            binding_id=binding.id,
                            topic_name=chunk.topic_name,
                            content=chunk.content,
                            message_ids=chunk.message_ids,
                            start_time=min(item.message_time for item in selected),
                            end_time=max(item.message_time for item in selected),
                            qdrant_point_id=point_id,
                        ))
                        await session.commit()
            await self._mark_chunked(messages)
        except Exception as exc:
            logger.exception("chunk pipeline failed")
            async with session_factory() as session:
                for item in messages:
                    db_message = await session.get(AgentMessageInbox, item.id)
                    db_message.error_message = str(exc)
                await session.commit()

    async def _mark_chunked(self, messages) -> None:
        async with session_factory() as session:
            for item in messages:
                db_message = await session.get(AgentMessageInbox, item.id)
                db_message.status = "chunked"
                db_message.error_message = None
            await session.commit()
