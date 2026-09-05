from __future__ import annotations

from contextlib import contextmanager
from time import perf_counter

from prometheus_client import Counter, Gauge, Histogram


HTTP_REQUESTS = Counter(
    "agent_http_requests_total",
    "Agent HTTP 请求数",
    ("method", "path", "status"),
)
HTTP_DURATION = Histogram(
    "agent_http_request_duration_seconds",
    "Agent HTTP 请求耗时",
    ("method", "path"),
)
PIPELINE_RUNS = Counter(
    "agent_pipeline_runs_total",
    "Agent 流水线阶段执行数",
    ("stage", "result"),
)
PIPELINE_DURATION = Histogram(
    "agent_pipeline_duration_seconds",
    "Agent 流水线阶段耗时",
    ("stage",),
)
MODERATION_MESSAGES = Counter(
    "agent_moderation_messages_total",
    "审核结果消息数",
    ("status", "rule_code"),
)
REMOVAL_REQUESTS = Counter(
    "agent_removal_requests_total",
    "向群管理员提交的移除成员申请数",
)
CHUNKS = Counter("agent_chunks_total", "分块处理结果数", ("result",))
QDRANT_OPERATIONS = Counter(
    "agent_qdrant_operations_total",
    "Qdrant 操作数",
    ("operation", "result"),
)
QDRANT_DURATION = Histogram(
    "agent_qdrant_operation_duration_seconds",
    "Qdrant 操作耗时",
    ("operation",),
)
WORKER_UP = Gauge("agent_worker_up", "Agent Worker 是否正在运行")


@contextmanager
def observe_stage(stage: str):
    started_at = perf_counter()
    result = "success"
    try:
        yield
    except Exception:
        result = "error"
        raise
    finally:
        PIPELINE_DURATION.labels(stage=stage).observe(perf_counter() - started_at)
        PIPELINE_RUNS.labels(stage=stage, result=result).inc()


@contextmanager
def observe_qdrant(operation: str):
    started_at = perf_counter()
    result = "success"
    try:
        yield
    except Exception:
        result = "error"
        raise
    finally:
        QDRANT_DURATION.labels(operation=operation).observe(perf_counter() - started_at)
        QDRANT_OPERATIONS.labels(operation=operation, result=result).inc()
