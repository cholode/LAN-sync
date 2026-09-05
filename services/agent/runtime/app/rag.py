from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any

from qdrant_client import QdrantClient
from qdrant_client.models import (
    DatetimeRange,
    Distance,
    FieldCondition,
    Filter,
    MatchValue,
    PayloadSchemaType,
    PointStruct,
    VectorParams,
)

from app.embeddings import Embedder, get_embedder
from app.metrics import observe_qdrant
from app.settings import get_settings
from app.time_utils import utc_from_timestamp, utc_now


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
        self.client = QdrantClient(host=host, grpc_port=port, prefer_grpc=True)

    @staticmethod
    def collection_name(room_id: int) -> str:
        return f"rag_room_{room_id}"

    def ensure_collection(self, room_id: int, vector_size: int | None = None) -> None:
        if vector_size is None:
            vector_size = get_settings().qdrant.vector_size
        name = self.collection_name(room_id)
        with observe_qdrant("ensure_collection"):
            if not self.client.collection_exists(name):
                self.client.create_collection(
                    collection_name=name,
                    vectors_config=VectorParams(size=vector_size, distance=Distance.COSINE),
                )
        self.client.create_payload_index(
            collection_name=name,
            field_name="chunk_type",
            field_schema="keyword",
        )
        for field_name in ("room_id", "binding_id"):
            self.client.create_payload_index(
                collection_name=name,
                field_name=field_name,
                field_schema=PayloadSchemaType.INTEGER,
            )
        for field_name in ("start_time", "end_time"):
            self.client.create_payload_index(
                collection_name=name,
                field_name=field_name,
                field_schema=PayloadSchemaType.DATETIME,
            )

    def upsert_chunk(
        self,
        *,
        point_id: str,
        vector: list[float],
        room_id: int,
        binding_id: int,
        topic_name: str,
        content: str,
        message_ids: list[str],
        start_time: datetime,
        end_time: datetime,
    ) -> None:
        self.ensure_collection(room_id, len(vector))
        with observe_qdrant("upsert"):
            self.client.upsert(
                collection_name=self.collection_name(room_id),
                wait=True,
                points=[PointStruct(
                    id=point_id,
                    vector=vector,
                    payload={
                        "room_id": room_id,
                        "binding_id": binding_id,
                        "chunk_type": "topic",
                        "topic_name": topic_name,
                        "content": content,
                        "message_ids": message_ids,
                        "start_time": start_time.isoformat(),
                        "end_time": end_time.isoformat(),
                        "moderation_status": "approved",
                    },
                )],
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

        with observe_qdrant("search"):
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
            start_value = payload.get("start_time")
            end_value = payload.get("end_time")

            def parse_time(value: Any) -> datetime:
                if isinstance(value, str):
                    return datetime.fromisoformat(value.replace("Z", "+00:00"))
                if value:
                    return utc_from_timestamp(int(value) / 1000)
                return utc_now()

            results.append(
                ChunkResult(
                    id=point.id or 0,
                    content=str(payload.get("content") or ""),
                    topic_name=str(payload.get("topic_name") or ""),
                    start_time=parse_time(start_value),
                    end_time=parse_time(end_value),
                    similarity=float(point.score or 0.0),
                    score=float(point.score or 0.0),
                )
            )

        return results

    def search_time_range(
        self,
        *,
        query_vector: list[float],
        room_id: int,
        binding_id: int,
        start_time: datetime,
        end_time: datetime,
        top_k: int = 5,
    ) -> list[ChunkResult]:
        with observe_qdrant("search_time_range"):
            response = self.client.query_points(
                collection_name=self.collection_name(room_id),
                query=query_vector,
                limit=top_k,
                query_filter=Filter(must=[
                    FieldCondition(key="binding_id", match=MatchValue(value=binding_id)),
                    FieldCondition(key="start_time", range=DatetimeRange(lte=end_time)),
                    FieldCondition(key="end_time", range=DatetimeRange(gte=start_time)),
                ]),
                with_payload=True,
            )
        results: list[ChunkResult] = []
        for point in response.points:
            payload = point.payload or {}
            results.append(ChunkResult(
                id=0,
                content=str(payload.get("content") or ""),
                topic_name=str(payload.get("topic_name") or ""),
                start_time=datetime.fromisoformat(str(payload["start_time"])),
                end_time=datetime.fromisoformat(str(payload["end_time"])),
                similarity=float(point.score or 0.0),
                score=float(point.score or 0.0),
            ))
        return results

    def delete_by_room(self, room_id: int) -> None:
        with observe_qdrant("delete_room"):
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
