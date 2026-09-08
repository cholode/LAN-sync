from __future__ import annotations

from contextlib import asynccontextmanager
from time import perf_counter

from fastapi import FastAPI
from prometheus_client import make_asgi_app

from app.api.router import router
from app.metrics import observe_api_request, register_api_metrics_collector
from app.storage.database import close_database, init_database


@asynccontextmanager
async def lifespan(_: FastAPI):
    await init_database()
    try:
        yield
    finally:
        await close_database()


register_api_metrics_collector()
app = FastAPI(title="LAN IM Agent Service", version="0.2.0", lifespan=lifespan)
app.include_router(router)
app.mount("/metrics", make_asgi_app())


@app.middleware("http")
async def observe_http(request, call_next):
    if not request.url.path.startswith("/api/"):
        return await call_next(request)
    started_at = perf_counter()
    status = 500
    try:
        response = await call_next(request)
        status = response.status_code
        return response
    finally:
        route = request.scope.get("route")
        path = getattr(route, "path", "unmatched")
        observe_api_request(request.method, path, status, perf_counter() - started_at)
