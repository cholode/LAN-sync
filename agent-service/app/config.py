from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from agent.v1 import agent_pb2


@dataclass
class RuntimeConfig:
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

    @classmethod
    def from_pb(cls, pb: agent_pb2.AgentRuntimeConfig | None) -> "RuntimeConfig":
        if pb is None:
            return cls()

        return cls(
            system_prompt=pb.system_prompt,
            trigger_mode=pb.trigger_mode,
            trigger_words=list(pb.trigger_words),
            max_history=pb.max_history or 20,
            temperature=pb.temperature or 0.7,
            model_name=pb.model_name or "deepseek-chat",
            rag_enabled=pb.rag_enabled,
            top_k=pb.top_k or 5,
            similarity_thold=pb.similarity_thold or 0.7,
            rerank_enabled=pb.rerank_enabled,
            max_chunk_tokens=pb.max_chunk_tokens or 4000,
            topic_chunk_min_msgs=pb.topic_chunk_min_msgs or 30,
            topic_chunk_model=pb.topic_chunk_model or "deepseek-chat",
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "system_prompt": self.system_prompt,
            "trigger_mode": self.trigger_mode,
            "trigger_words": self.trigger_words,
            "max_history": self.max_history,
            "temperature": self.temperature,
            "model_name": self.model_name,
            "rag_enabled": self.rag_enabled,
            "top_k": self.top_k,
            "similarity_thold": self.similarity_thold,
            "rerank_enabled": self.rerank_enabled,
            "max_chunk_tokens": self.max_chunk_tokens,
            "topic_chunk_min_msgs": self.topic_chunk_min_msgs,
            "topic_chunk_model": self.topic_chunk_model,
        }
