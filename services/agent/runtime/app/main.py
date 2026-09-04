from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api.router import router
from app.storage.database import close_database, init_database


@asynccontextmanager
async def lifespan(_: FastAPI):
    await init_database()
    try:
        yield
    finally:
        await close_database()


app = FastAPI(title="LAN IM Agent Service", version="0.2.0", lifespan=lifespan)
app.include_router(router)
