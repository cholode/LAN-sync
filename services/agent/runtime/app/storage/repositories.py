from __future__ import annotations

from typing import Any

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.storage.models import RoomAgentConfig


class RoomAgentConfigRepository:
    def __init__(self, session: AsyncSession) -> None:
        self.session = session

    async def get_by_binding_id(self, binding_id: int) -> RoomAgentConfig | None:
        statement = select(RoomAgentConfig).where(
            RoomAgentConfig.binding_id == binding_id,
            RoomAgentConfig.deleted_at == 0,
        )
        return await self.session.scalar(statement)

    async def create(
        self,
        *,
        binding_id: int,
        system_prompt: str | None = None,
        trigger_words: list[str] | None = None,
        rag_enabled: bool = False,
        extra_config: dict[str, Any] | None = None,
    ) -> RoomAgentConfig:
        config = RoomAgentConfig(
            binding_id=binding_id,
            system_prompt=system_prompt,
            trigger_words=trigger_words,
            rag_enabled=rag_enabled,
            extra_config=extra_config,
        )
        self.session.add(config)
        await self.session.flush()
        return config

    async def update(
        self,
        config: RoomAgentConfig,
        *,
        system_prompt: str | None,
        trigger_words: list[str] | None,
        rag_enabled: bool,
        extra_config: dict[str, Any] | None,
    ) -> RoomAgentConfig:
        config.system_prompt = system_prompt
        config.trigger_words = trigger_words
        config.rag_enabled = rag_enabled
        config.extra_config = extra_config
        await self.session.flush()
        return config

    async def soft_delete(self, config: RoomAgentConfig) -> None:
        config.soft_delete()
        await self.session.flush()
