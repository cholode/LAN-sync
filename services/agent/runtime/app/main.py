from __future__ import annotations

from contextlib import asynccontextmanager
from concurrent import futures

import grpc
from fastapi import FastAPI

from agent.v1 import agent_pb2_grpc
from app.api.router import router
from app.server import AgentServiceServicer
from app.settings import get_settings
from app.storage.database import close_database, init_database


@asynccontextmanager
async def lifespan(_: FastAPI):
    settings = get_settings()
    await init_database()
    grpc_server = grpc.server(futures.ThreadPoolExecutor(max_workers=settings.grpc.max_workers))
    agent_pb2_grpc.add_AgentServiceServicer_to_server(AgentServiceServicer(), grpc_server)
    grpc_server.add_insecure_port(f"{settings.grpc.host}:{settings.grpc.port}")
    grpc_server.start()
    try:
        yield
    finally:
        grpc_server.stop(grace=5)
        await close_database()


app = FastAPI(title="LAN IM Agent Service", version="0.2.0", lifespan=lifespan)
app.include_router(router)
