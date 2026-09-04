from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, Response, status
from sqlalchemy import select, text
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.exc import IntegrityError

from app.api.schemas import (
    BindingCreate,
    BindingView,
    BotCreate,
    BotView,
    ConfigView,
    ConfigWrite,
    RemovalRequestView,
)
from app.storage.database import get_session
from app.storage.models import AgentBot, RemovalRequest, RoomAgentBinding
from app.storage.repositories import RoomAgentConfigRepository


router = APIRouter(prefix="/api/v1")


@router.get("/health/live")
async def live() -> dict[str, str]:
    return {"status": "ok"}


@router.get("/health/ready")
async def ready(session: AsyncSession = Depends(get_session)) -> dict[str, str]:
    await session.execute(text("SELECT 1"))
    return {"status": "ready"}


@router.post("/bots", response_model=BotView, status_code=status.HTTP_201_CREATED)
async def create_bot(payload: BotCreate, session: AsyncSession = Depends(get_session)):
    bot = AgentBot(**payload.model_dump())
    session.add(bot)
    try:
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise HTTPException(status_code=409, detail="bot code already exists") from exc
    await session.refresh(bot)
    return bot


@router.get("/bots", response_model=list[BotView])
async def list_bots(session: AsyncSession = Depends(get_session)):
    result = await session.scalars(select(AgentBot).where(AgentBot.deleted_at == 0).order_by(AgentBot.id))
    return list(result)


@router.post("/room-agent-bindings", response_model=BindingView, status_code=status.HTTP_201_CREATED)
async def create_binding(payload: BindingCreate, session: AsyncSession = Depends(get_session)):
    binding = RoomAgentBinding(**payload.model_dump())
    session.add(binding)
    try:
        await session.commit()
    except IntegrityError as exc:
        await session.rollback()
        raise HTTPException(status_code=409, detail="room agent binding already exists") from exc
    await session.refresh(binding)
    return binding


@router.get("/rooms/{room_id}/agents", response_model=list[BindingView])
async def list_room_agents(room_id: int, session: AsyncSession = Depends(get_session)):
    result = await session.scalars(
        select(RoomAgentBinding)
        .where(RoomAgentBinding.room_id == room_id, RoomAgentBinding.deleted_at == 0)
        .order_by(RoomAgentBinding.priority, RoomAgentBinding.id)
    )
    return list(result)


@router.put("/room-agent-bindings/{binding_id}/config", response_model=ConfigView)
async def put_config(
    binding_id: int,
    payload: ConfigWrite,
    session: AsyncSession = Depends(get_session),
):
    binding = await session.scalar(
        select(RoomAgentBinding).where(RoomAgentBinding.id == binding_id, RoomAgentBinding.deleted_at == 0)
    )
    if binding is None:
        raise HTTPException(status_code=404, detail="binding not found")
    repository = RoomAgentConfigRepository(session)
    config = await repository.get_by_binding_id(binding_id)
    values = payload.model_dump()
    if config is None:
        config = await repository.create(binding_id=binding_id, **values)
    else:
        config = await repository.update(config, **values)
    await session.commit()
    await session.refresh(config)
    return config


@router.get("/room-agent-bindings/{binding_id}/config", response_model=ConfigView)
async def get_config(binding_id: int, session: AsyncSession = Depends(get_session)):
    config = await RoomAgentConfigRepository(session).get_by_binding_id(binding_id)
    if config is None:
        raise HTTPException(status_code=404, detail="config not found")
    return config


@router.delete("/room-agent-bindings/{binding_id}/config", status_code=status.HTTP_204_NO_CONTENT)
async def delete_config(binding_id: int, session: AsyncSession = Depends(get_session)):
    repository = RoomAgentConfigRepository(session)
    config = await repository.get_by_binding_id(binding_id)
    if config is None:
        raise HTTPException(status_code=404, detail="config not found")
    await repository.soft_delete(config)
    await session.commit()
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.get("/removal-requests", response_model=list[RemovalRequestView])
async def list_removal_requests(
    room_id: int | None = None,
    request_status: str = "pending",
    session: AsyncSession = Depends(get_session),
):
    statement = select(RemovalRequest).where(
        RemovalRequest.deleted_at == 0,
        RemovalRequest.status == request_status,
    )
    if room_id is not None:
        statement = statement.where(RemovalRequest.room_id == room_id)
    result = await session.scalars(statement.order_by(RemovalRequest.created_at.desc()).limit(200))
    return list(result)
