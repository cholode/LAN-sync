from __future__ import annotations

from contextlib import contextmanager
from time import perf_counter

from math import ceil
from threading import Lock
from time import time

from prometheus_client import REGISTRY, Counter, Gauge, Histogram
from prometheus_client.core import GaugeMetricFamily


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
API_REQUESTS = Counter(
    "im_api_requests_total",
    "已完成的 HTTP API 请求数",
    ("component", "method", "route", "status"),
)
API_DURATION = Histogram(
    "im_api_request_duration_seconds",
    "HTTP API 端到端请求耗时",
    ("component", "method", "route"),
    buckets=(0.001, 0.003, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10),
)


class _APISecondWindow:
    _bucket_count = 3
    _latency_bucket_count = 10_001

    def __init__(self) -> None:
        self._lock = Lock()
        self._buckets = [
            {"second": -1, "counts": [0] * self._latency_bucket_count, "samples": 0}
            for _ in range(self._bucket_count)
        ]

    def observe(self, duration_seconds: float) -> None:
        second = int(time())
        index = second % self._bucket_count
        latency_index = min(max(ceil(duration_seconds * 1000) - 1, 0), 10_000)
        with self._lock:
            bucket = self._buckets[index]
            if bucket["second"] != second:
                bucket["second"] = second
                bucket["counts"] = [0] * self._latency_bucket_count
                bucket["samples"] = 0
            bucket["counts"][latency_index] += 1
            bucket["samples"] += 1

    def latest_complete(self) -> tuple[int, int, float, float, float]:
        second = int(time()) - 1
        with self._lock:
            bucket = self._buckets[second % self._bucket_count]
            if bucket["second"] != second or bucket["samples"] == 0:
                return second, 0, 0.0, 0.0, 0.0
            samples = bucket["samples"]
            counts = bucket["counts"]
            targets = [ceil(samples * value) for value in (0.50, 0.95, 0.99)]
            values = [0.0, 0.0, 0.0]
            cumulative = 0
            target_index = 0
            for latency_index, count in enumerate(counts):
                cumulative += count
                while target_index < len(targets) and cumulative >= targets[target_index]:
                    values[target_index] = float(min(latency_index + 1, 10_000))
                    target_index += 1
                if target_index == len(targets):
                    break
            return second, samples, values[0], values[1], values[2]


API_SECOND_WINDOW = _APISecondWindow()


class _APISecondCollector:
    def collect(self):
        second, samples, p50, p95, p99 = API_SECOND_WINDOW.latest_complete()
        qps = GaugeMetricFamily(
            "im_api_completed_qps",
            "最近完整秒内完成全链路处理的 API 请求数",
            labels=("component",),
        )
        qps.add_metric(("agent",), samples)
        yield qps
        sample_metric = GaugeMetricFamily(
            "im_api_completion_samples",
            "最近完整秒内的 API 完成样本数",
            labels=("component",),
        )
        sample_metric.add_metric(("agent",), samples)
        yield sample_metric
        latency = GaugeMetricFamily(
            "im_api_completion_latency_milliseconds",
            "最近完整秒的 API 延迟分位数，最高显示 10000ms",
            labels=("component", "quantile"),
        )
        latency.add_metric(("agent", "0.50"), p50)
        latency.add_metric(("agent", "0.95"), p95)
        latency.add_metric(("agent", "0.99"), p99)
        yield latency
        represented_second = GaugeMetricFamily(
            "im_api_completion_second",
            "API 完成指标所代表的 Unix 秒",
            labels=("component",),
        )
        represented_second.add_metric(("agent",), second)
        yield represented_second


def register_api_metrics_collector() -> None:
    REGISTRY.register(_APISecondCollector())


def observe_api_request(method: str, route: str, status: int, duration_seconds: float) -> None:
    HTTP_DURATION.labels(method=method, path=route).observe(duration_seconds)
    HTTP_REQUESTS.labels(method=method, path=route, status=str(status)).inc()
    API_DURATION.labels(component="agent", method=method, route=route).observe(duration_seconds)
    API_REQUESTS.labels(component="agent", method=method, route=route, status=str(status)).inc()
    API_SECOND_WINDOW.observe(duration_seconds)


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
