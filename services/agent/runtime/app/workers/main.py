from __future__ import annotations

import asyncio
import logging

from app.storage.database import close_database, init_database
from app.workers.consumer import InboxConsumer
from app.workers.pipeline import AgentPipeline


async def run() -> None:
    logging.basicConfig(level=logging.INFO)
    await init_database()
    try:
        await asyncio.gather(InboxConsumer().run(), AgentPipeline().run())
    finally:
        await close_database()


if __name__ == "__main__":
    asyncio.run(run())
