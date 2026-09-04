from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, ConfigDict, Field


class BotCreate(BaseModel):
    code: str = Field(min_length=1, max_length=64)
    name: str = Field(min_length=1, max_length=100)
    description: str | None = Field(default=None, max_length=500)
    avatar_url: str | None = Field(default=None, max_length=500)


class BotView(BotCreate):
    model_config = ConfigDict(from_attributes=True)
    id: int
    created_at: datetime
    updated_at: datetime


class BindingCreate(BaseModel):
    room_id: int = Field(gt=0)
    bot_id: int = Field(gt=0)
    legacy_bot_user_id: int | None = Field(default=None, gt=0)
    enabled: bool = True
    priority: int = 100


class BindingView(BindingCreate):
    model_config = ConfigDict(from_attributes=True)
    id: int
    created_at: datetime
    updated_at: datetime


class ConfigWrite(BaseModel):
    system_prompt: str | None = None
    trigger_words: list[str] = Field(default_factory=list)
    rag_enabled: bool = False
    extra_config: dict[str, Any] = Field(default_factory=dict)


class ConfigView(ConfigWrite):
    model_config = ConfigDict(from_attributes=True)
    id: int
    binding_id: int
    created_at: datetime
    updated_at: datetime


class RemovalRequestView(BaseModel):
    model_config = ConfigDict(from_attributes=True)
    id: int
    room_id: int
    target_user_id: int
    moderation_record_id: int
    reason: str
    evidence: dict[str, Any] | None
    status: str
    created_at: datetime
