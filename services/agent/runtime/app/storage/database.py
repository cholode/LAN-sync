from __future__ import annotations

from collections.abc import AsyncIterator

from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.settings import get_settings


_settings = get_settings().database
engine = create_async_engine(
    _settings.url,
    echo=_settings.echo,
    pool_pre_ping=True,
    pool_recycle=_settings.pool_recycle_seconds,
    pool_size=_settings.pool_size,
    max_overflow=_settings.max_overflow,
)
session_factory = async_sessionmaker(engine, expire_on_commit=False)


async def get_session() -> AsyncIterator[AsyncSession]:
    async with session_factory() as session:
        yield session


async def close_database() -> None:
    await engine.dispose()


async def init_database() -> None:
    from app.storage.models import Base

    async with engine.begin() as connection:
        await connection.run_sync(Base.metadata.create_all)
