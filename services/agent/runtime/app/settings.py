from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path
from typing import Any

import yaml

_ENV_PATTERN = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-(.*?))?\}")


@dataclass(frozen=True)
class GRPCSettings:
    host: str = "[::]"
    port: int = 50051
    max_workers: int = 10


@dataclass(frozen=True)
class LLMSettings:
    base_url: str
    api_key: str = ""
    model: str = "deepseek-chat"
    timeout_seconds: float = 60.0


@dataclass(frozen=True)
class EmbeddingSettings:
    base_url: str
    api_key: str = ""
    model: str = "doubao-embedding-vision-251215"
    dimension: int = 1024
    batch_size: int = 100
    timeout_seconds: float = 30.0


@dataclass(frozen=True)
class QdrantSettings:
    host: str = "qdrant"
    grpc_port: int = 6334
    vector_size: int = 1024


@dataclass(frozen=True)
class RedisSettings:
    addr: str = "redis:6379"


@dataclass(frozen=True)
class KafkaSettings:
    brokers: list[str] = field(default_factory=lambda: ["kafka:9092"])
    topic: str = "im_chat_messages"
    group_id: str = "agent_service"
    batch_size: int = 50
    batch_window_seconds: float = 60.0
    poll_interval_seconds: float = 1.0
    auto_offset_reset: str = "latest"


@dataclass(frozen=True)
class IMServiceSettings:
    grpc_addr: str = "backend:50052"


@dataclass(frozen=True)
class DatabaseSettings:
    url: str
    pool_size: int = 10
    max_overflow: int = 20
    pool_recycle_seconds: int = 3600
    echo: bool = False


@dataclass(frozen=True)
class AgentSettings:
    cooldown_seconds: float = 5.0
    system_prompt: str = ""
    trigger_mode: int = 1
    trigger_words: list[str] = field(default_factory=list)
    max_history: int = 20
    temperature: float = 0.7
    model_name: str = "deepseek-chat"
    rag_enabled: bool = True
    top_k: int = 5
    similarity_thold: float = 0.7
    rerank_enabled: bool = True
    max_chunk_tokens: int = 4000
    topic_chunk_min_msgs: int = 30
    topic_chunk_model: str = "deepseek-chat"


@dataclass(frozen=True)
class Settings:
    grpc: GRPCSettings
    llm: LLMSettings
    embedding: EmbeddingSettings
    qdrant: QdrantSettings
    redis: RedisSettings
    kafka: KafkaSettings
    im_service: IMServiceSettings
    database: DatabaseSettings
    agent: AgentSettings


def _expand_env(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: _expand_env(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_expand_env(item) for item in value]
    if isinstance(value, str):
        def replace(match: re.Match[str]) -> str:
            name = match.group(1)
            default = match.group(2)
            env_value = os.environ.get(name)
            if env_value is not None and env_value != "":
                return env_value
            return default if default is not None else ""
        return _ENV_PATTERN.sub(replace, value)
    return value


def _default_config_path() -> Path:
    env_path = os.getenv("AGENT_CONFIG_PATH")
    if env_path:
        return Path(env_path)
    return Path(__file__).resolve().parent.parent / "config.yaml"


def _section(raw: dict[str, Any], name: str) -> dict[str, Any]:
    value = raw.get(name) or {}
    return value if isinstance(value, dict) else {}


def _int(value: Any, default: int) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def _float(value: Any, default: float) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return default


def _bool(value: Any, default: bool) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, str):
        return value.strip().lower() in {"1", "true", "yes", "on"}
    return default


def _database_url(value: Any) -> str:
    configured = str(value or "").strip()
    if configured:
        return configured
    password = os.getenv("DB_PASSWORD", "")
    database = os.getenv("DB_NAME", "lan_im")
    return f"mysql+aiomysql://root:{password}@db:3306/{database}?charset=utf8mb4"


def _load_settings() -> Settings:
    path = _default_config_path()
    raw_text = path.read_text(encoding="utf-8")
    raw = yaml.safe_load(raw_text) or {}
    data = _expand_env(raw)

    grpc = _section(data, "grpc")
    llm = _section(data, "llm")
    embedding = _section(data, "embedding")
    qdrant = _section(data, "qdrant")
    redis = _section(data, "redis")
    kafka = _section(data, "kafka")
    im_service = _section(data, "im_service")
    database = _section(data, "database")
    agent = _section(data, "agent")

    return Settings(
        grpc=GRPCSettings(
            host=str(grpc.get("host") or "[::]"),
            port=_int(grpc.get("port"), 50051),
            max_workers=_int(grpc.get("max_workers"), 10),
        ),
        llm=LLMSettings(
            base_url=str(llm.get("base_url") or "https://api.deepseek.com/v1"),
            api_key=str(llm.get("api_key") or ""),
            model=str(llm.get("model") or "deepseek-chat"),
            timeout_seconds=_float(llm.get("timeout_seconds"), 60.0),
        ),
        embedding=EmbeddingSettings(
            base_url=str(embedding.get("base_url") or "https://ark.cn-beijing.volces.com/api/v3"),
            api_key=str(embedding.get("api_key") or ""),
            model=str(embedding.get("model") or "doubao-embedding-vision-251215"),
            dimension=_int(embedding.get("dimension"), 1024),
            batch_size=_int(embedding.get("batch_size"), 100),
            timeout_seconds=_float(embedding.get("timeout_seconds"), 30.0),
        ),
        qdrant=QdrantSettings(
            host=str(qdrant.get("host") or "qdrant"),
            grpc_port=_int(qdrant.get("grpc_port"), 6334),
            vector_size=_int(qdrant.get("vector_size"), 1024),
        ),
        redis=RedisSettings(addr=str(redis.get("addr") or "redis:6379")),
        kafka=KafkaSettings(
            brokers=[item.strip() for item in str(kafka.get("brokers") or "kafka:9092").split(",") if item.strip()],
            topic=str(kafka.get("topic") or "im_chat_messages"),
            group_id=str(kafka.get("group_id") or "agent_service"),
            batch_size=_int(kafka.get("batch_size"), 50),
            batch_window_seconds=_float(kafka.get("batch_window_seconds"), 60.0),
            poll_interval_seconds=_float(kafka.get("poll_interval_seconds"), 1.0),
            auto_offset_reset=str(kafka.get("auto_offset_reset") or "latest"),
        ),
        im_service=IMServiceSettings(grpc_addr=str(im_service.get("grpc_addr") or "backend:50052")),
        database=DatabaseSettings(
            url=_database_url(database.get("url")),
            pool_size=_int(database.get("pool_size"), 10),
            max_overflow=_int(database.get("max_overflow"), 20),
            pool_recycle_seconds=_int(database.get("pool_recycle_seconds"), 3600),
            echo=_bool(database.get("echo"), False),
        ),
        agent=AgentSettings(
            cooldown_seconds=_float(agent.get("cooldown_seconds"), 5.0),
            system_prompt=str(agent.get("system_prompt") or ""),
            trigger_mode=_int(agent.get("trigger_mode"), 1),
            trigger_words=list(agent.get("trigger_words") or []),
            max_history=_int(agent.get("max_history"), 20),
            temperature=_float(agent.get("temperature"), 0.7),
            model_name=str(agent.get("model_name") or "deepseek-chat"),
            rag_enabled=_bool(agent.get("rag_enabled"), True),
            top_k=_int(agent.get("top_k"), 5),
            similarity_thold=_float(agent.get("similarity_thold"), 0.7),
            rerank_enabled=_bool(agent.get("rerank_enabled"), True),
            max_chunk_tokens=_int(agent.get("max_chunk_tokens"), 4000),
            topic_chunk_min_msgs=_int(agent.get("topic_chunk_min_msgs"), 30),
            topic_chunk_model=str(agent.get("topic_chunk_model") or "deepseek-chat"),
        ),
    )


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return _load_settings()
