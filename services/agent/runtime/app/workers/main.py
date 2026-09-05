from __future__ import annotations

import asyncio
import logging

from prometheus_client import start_http_server

from app.metrics import WORKER_UP
from app.storage.database import close_database, init_database
from app.workers.consumer import InboxConsumer
from app.workers.pipeline import AgentPipeline


async def run() -> None:
    logging.basicConfig(level=logging.INFO)
    start_http_server(8001)
    WORKER_UP.set(1)
    await init_database()
    try:
        await asyncio.gather(InboxConsumer().run(), AgentPipeline().run())
    finally:
        WORKER_UP.set(0)
        await close_database()


if __name__ == "__main__":
    asyncio.run(run())
