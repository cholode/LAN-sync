from __future__ import annotations

import logging
from concurrent import futures

import grpc

from agent.v1 import agent_pb2_grpc
from app.server import AgentServiceServicer
from app.settings import get_settings


def serve() -> None:
    logging.basicConfig(level=logging.INFO)
    settings = get_settings()
    port = settings.grpc.port
    max_workers = settings.grpc.max_workers

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    agent_pb2_grpc.add_AgentServiceServicer_to_server(AgentServiceServicer(), server)
    server.add_insecure_port(f"{settings.grpc.host}:{port}")

    server.start()
    logging.info("Agent gRPC service listening on %s", port)
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
