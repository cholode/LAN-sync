from __future__ import annotations  # 允许在类型注解中使用尚未定义的类名（如 "EmbeddingClient"）

from typing import List  # 导入 List 类型，用于类型注解

import httpx  # 导入 httpx 库，用于发送 HTTP 请求

from app.settings import get_settings  # 导入配置获取函数


class EmbeddingClient:
    """用于与兼容 OpenAI 的多模态嵌入接口通信的 HTTP 客户端"""

    def __init__(
        self,
        base_url: str,          # API 基础地址
        api_key: str,           # API 密钥
        model: str,             # 使用的嵌入模型名称
        dim: int = 1024,        # 向量维度（默认 1024）
        timeout: float = 30.0,  # 请求超时时间（秒）
    ) -> None:
        self.base_url = base_url.rstrip("/")  # 去除基础地址末尾的斜杠
        self.api_key = api_key
        self.model = model
        self.dim = dim
        self.timeout = timeout

    @classmethod
    def from_env(cls) -> "EmbeddingClient":
        # 从环境配置中读取参数并创建 EmbeddingClient 实例
        settings = get_settings()  # 获取全局配置
        return cls(
            base_url=settings.embedding.base_url,
            api_key=settings.embedding.api_key,
            model=settings.embedding.model,
            dim=settings.embedding.dimension,
            timeout=settings.embedding.timeout_seconds,
        )

    def embed(self, text: str) -> List[float]:
        # 对单个文本进行嵌入，返回向量
        vectors = self.embed_multi([text])  # 调用多文本嵌入方法
        if not vectors:
            raise RuntimeError("empty embedding response")  # 如果返回空则报错
        return vectors[0]  # 返回第一个（也是唯一一个）向量

    def embed_multi(self, inputs: List[str]) -> List[List[float]]:
        # 批量嵌入多个文本，返回向量列表
        payload = {
            "model": self.model,
            # 构造输入格式：每个文本对象包含 type 和 text
            "input": [{"type": "text", "text": text} for text in inputs],
        }

        response = httpx.post(
            f"{self.base_url}/embeddings/multimodal",  # 请求的完整 URL
            json=payload,  # 请求体为 JSON 格式
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}",  # 使用 Bearer Token 认证
            },
            timeout=self.timeout,
        )
        response.raise_for_status()  # 检查 HTTP 状态码，非 2xx 会抛出异常

        data = response.json()  # 解析响应 JSON
        items = data.get("data", [])  # 获取嵌入数据数组
        vectors: List[List[float] | None] = [None] * len(items)  # 初始化结果列表，长度与 items 相同

        for item in items:
            index = int(item.get("index", -1))  # 获取向量对应的索引
            embedding = item.get("embedding")   # 获取向量数据
            if index < 0 or index >= len(vectors) or embedding is None:
                continue  # 索引无效或向量缺失时跳过
            vectors[index] = [float(value) for value in embedding]  # 将向量转换为 float 列表并存入对应位置

        # 过滤掉 None 值（未成功解析的向量），返回有效向量列表
        return [vector for vector in vectors if vector is not None]


class Embedder:
    """对 EmbeddingClient 的简单批量包装"""

    def __init__(self, client: EmbeddingClient, batch_size: int = 100) -> None:
        self.client = client          # 嵌入客户端实例
        self.batch_size = batch_size  # 每批处理的文本数量

    def embed(self, text: str) -> List[float]:
        # 嵌入单个文本
        return self.client.embed(text)

    def embed_batch(self, texts: List[str]) -> List[List[float]]:
        # 批量嵌入文本，自动分批处理
        result: List[List[float]] = []  # 存储所有向量
        for start in range(0, len(texts), self.batch_size):
            batch = texts[start : start + self.batch_size]  # 按 batch_size 切片
            result.extend(self.client.embed_multi(batch))   # 调用客户端批量嵌入，并合并结果
        return result


# 模块级单例变量，用于缓存客户端和嵌入器
_client: EmbeddingClient | None = None
_embedder: Embedder | None = None


def get_embedder() -> Embedder:
    # 获取全局 Embedder 单例（懒加载）
    global _client, _embedder
    if _embedder is None:  # 如果尚未创建，则初始化
        settings = get_settings()
        _client = EmbeddingClient.from_env()  # 从环境创建客户端
        _embedder = Embedder(_client, batch_size=settings.embedding.batch_size)  # 创建嵌入器
    return _embedder