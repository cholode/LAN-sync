from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any

from qdrant_client import QdrantClient
from qdrant_client.models import Distance, FieldCondition, Filter, MatchValue, VectorParams

from app.embeddings import Embedder, get_embedder
from app.settings import get_settings


@dataclass
class ChunkResult:
    content: str
    topic_name: str
    start_time: datetime
    end_time: datetime
    similarity: float
    score: float
    id: int = 0


class QdrantVectorStore:
    def __init__(self, host: str | None = None, port: int | None = None) -> None:
        settings = get_settings()
        host = host or settings.qdrant.host
        port = port or settings.qdrant.grpc_port
        self.client = QdrantClient(host=host, port=port)

    @staticmethod
    def collection_name(room_id: int) -> str:
        return f"rag_room_{room_id}"

    def ensure_collection(self, room_id: int, vector_size: int | None = None) -> None:
        if vector_size is None:
            vector_size = get_settings().qdrant.vector_size
        name = self.collection_name(room_id)
        if self.client.collection_exists(name):
            return

        self.client.create_collection(
            collection_name=name,
            vectors_config=VectorParams(size=vector_size, distance=Distance.COSINE),
        )
        self.client.create_payload_index(
            collection_name=name,
            field_name="chunk_type",
            field_schema="keyword",
        )

    def search(
        self,
        query_vector: list[float],
        room_id: int,
        top_k: int = 5,
        chunk_types: list[str] | None = None,
    ) -> list[ChunkResult]:
        name = self.collection_name(room_id)
        query_filter = None
        if chunk_types:
            query_filter = Filter(
                must=[
                    FieldCondition(
                        key="chunk_type",
                        match=MatchValue(value=chunk_type),
                    )
                    for chunk_type in chunk_types
                ]
            )

        response = self.client.query_points(
            collection_name=name,
            query=query_vector,
            limit=top_k,
            query_filter=query_filter,
            with_payload=True,
        )

        results: list[ChunkResult] = []
        for point in response.points:
            payload = point.payload or {}
            start_ms = int(payload.get("start_time") or 0)
            end_ms = int(payload.get("end_time") or 0)

            results.append(
                ChunkResult(
                    id=point.id or 0,
                    content=str(payload.get("content") or ""),
                    topic_name=str(payload.get("topic_name") or ""),
                    start_time=datetime.fromtimestamp(start_ms / 1000) if start_ms else datetime.now(),
                    end_time=datetime.fromtimestamp(end_ms / 1000) if end_ms else datetime.now(),
                    similarity=float(point.score or 0.0),
                    score=float(point.score or 0.0),
                )
            )

        return results

    def delete_by_room(self, room_id: int) -> None:
        self.client.delete_collection(self.collection_name(room_id))


class Retriever:
    def __init__(self, embedder: Embedder, vector_store: QdrantVectorStore) -> None:
        self.embedder = embedder
        self.vector_store = vector_store

    def retrieve(self, query: str, room_id: int, top_k: int) -> list[ChunkResult]:
        query_vector = self.embedder.embed(query)
        results = self.vector_store.search(
            query_vector=query_vector,
            room_id=room_id,
            top_k=top_k,
            chunk_types=["topic"],
        )

        seen: set[str] = set()
        output: list[ChunkResult] = []
        for result in results:
            if result.content in seen:
                continue
            seen.add(result.content)
            result.score = result.similarity
            output.append(result)
        return output

    @staticmethod
    def format_chunk_for_prompt(chunk: ChunkResult) -> str:
        start = chunk.start_time.strftime("%Y-%m-%d %H:%M")
        end = chunk.end_time.strftime("%Y-%m-%d %H:%M")
        return f"【话题: {chunk.topic_name}】\n时间: {start} ~ {end}\n{chunk.content}"


_client: QdrantVectorStore | None = None
_retriever: Retriever | None = None


def get_retriever() -> Retriever:
    global _client, _retriever
    if _retriever is None:
        _client = QdrantVectorStore()
        _retriever = Retriever(embedder=get_embedder(), vector_store=_client)
    return _retriever
