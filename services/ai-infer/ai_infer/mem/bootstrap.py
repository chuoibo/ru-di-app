"""Build a mem0 Memory the sidecar can live with (research mem0.md §7.3 and
its §Kiểm chứng; stm-personalization.md §C1):

- telemetry off before mem0 is imported (package __init__);
- SQLiteManager replaced by a no-op BEFORE Memory() is built: mem0 2.2.1
  otherwise keeps the last 10 raw messages per person and the text of every
  deleted memory in SQLite, which «forget» never clears;
- mem0's loggers silenced: they log memory text at INFO/DEBUG/WARNING;
- the LLM, the embedder and the vector store are ours (Gemini with our text
  format and schema, NepMilvus), registered in place of mem0's classes;
- spaCy is never installed, so no entity store is created (a fresh process
  would not clean one on delete: mem0.md §Kiểm chứng 10).
"""

from __future__ import annotations

import logging
import threading
from typing import Any, Optional

import ai_infer.mem  # noqa: F401  -- telemetry off before mem0 is imported
from ai_infer.config import EMBED_DIMS, EMBED_MODEL, LLM_MODEL
from ai_infer.mem import extract, gemini

import mem0.memory.main as _m0main  # noqa: E402
from mem0 import Memory  # noqa: E402
from mem0.configs.base import MemoryConfig  # noqa: E402
from mem0.configs.llms.gemini import GeminiConfig  # noqa: E402
from mem0.embeddings.base import EmbeddingBase  # noqa: E402
from mem0.llms.base import LLMBase  # noqa: E402
from mem0.utils.factory import EmbedderFactory, LlmFactory, VectorStoreFactory  # noqa: E402


class NoHistory:
    """Stands where mem0's SQLiteManager was: remembers nothing, writes
    nothing. The raw conversation and the text of old memories never reach
    a disk or outlive a call."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        pass

    def get_last_messages(self, *args: Any, **kwargs: Any) -> list:
        return []

    def save_messages(self, *args: Any, **kwargs: Any) -> None:
        return None

    def add_history(self, *args: Any, **kwargs: Any) -> None:
        return None

    def batch_add_history(self, *args: Any, **kwargs: Any) -> None:
        return None

    def get_history(self, *args: Any, **kwargs: Any) -> list:
        return []

    def reset(self) -> None:
        return None

    def close(self) -> None:
        return None


def silence_mem0_logs() -> None:
    """mem0 logs memory text (e.g. «Updating memory with data=...»): nothing
    below CRITICAL leaves its loggers, and CRITICAL is not used by it."""
    lg = logging.getLogger("mem0")
    lg.setLevel(logging.CRITICAL + 1)
    lg.propagate = False


# Back-ends handed to the next Memory() built by build(); mem0 constructs its
# components itself from configuration, so they are passed out of band.
_next: dict[str, Any] = {}
_lock = threading.Lock()


class NepLlm(LLMBase):
    def __init__(self, config: Optional[GeminiConfig] = None):
        super().__init__(config)
        self.backend: gemini.LlmBackend = _next["llm"]

    def generate_response(
        self, messages, response_format=None, tools=None, tool_choice="auto", **kwargs
    ):
        system = "\n".join(m["content"] for m in messages if m.get("role") == "system")
        user = "\n".join(m["content"] for m in messages if m.get("role") != "system")
        raw = self.backend.complete(system, user, extract.SCHEMA)
        return extract.filter_answer(raw)


class NepEmbed(EmbeddingBase):
    def __init__(self, config=None):
        super().__init__(config)
        self.backend: gemini.EmbedBackend = _next["embed"]

    def embed(self, text, memory_action=None):
        return self.embed_batch([text], memory_action or "add")[0]

    def embed_batch(self, texts, memory_action="add"):
        if not texts:
            return []
        inputs = [gemini.format_for(t, memory_action) for t in texts]
        vecs = self.backend.embed(inputs, list(texts))
        if len(vecs) != len(texts):
            raise gemini.EmbedError(f"{len(vecs)} embeddings for {len(texts)} texts")
        return [gemini.normalise(v) for v in vecs]


def _register() -> None:
    # The provider names must stay mem0's own ("gemini", "milvus") to pass
    # its config validators; the classes behind them are ours.
    LlmFactory.provider_to_class["gemini"] = (
        "ai_infer.mem.bootstrap.NepLlm",
        GeminiConfig,
    )
    EmbedderFactory.provider_to_class["gemini"] = "ai_infer.mem.bootstrap.NepEmbed"
    VectorStoreFactory.provider_to_class["milvus"] = "ai_infer.mem.store.NepMilvus"
    _m0main.SQLiteManager = NoHistory


def build(
    *,
    llm: gemini.LlmBackend,
    embed: gemini.EmbedBackend,
    milvus_uri: str,
    milvus_token: str,
    db_name: str,
    collection: str,
) -> Memory:
    silence_mem0_logs()
    config = MemoryConfig(
        llm={
            "provider": "gemini",
            "config": {"model": LLM_MODEL, "temperature": 0.1, "max_tokens": 1024},
        },
        embedder={
            "provider": "gemini",
            "config": {"model": EMBED_MODEL, "embedding_dims": EMBED_DIMS},
        },
        vector_store={
            "provider": "milvus",
            "config": {
                "url": milvus_uri,
                "token": milvus_token,
                "collection_name": collection,
                "embedding_model_dims": EMBED_DIMS,
                "metric_type": "COSINE",
                "db_name": db_name,
            },
        },
        history_db_path=":memory:",
        custom_instructions=extract.INSTRUCTIONS,
    )
    with _lock:
        _register()
        _next.update(llm=llm, embed=embed)
        try:
            m = Memory(config)
        finally:
            _next.clear()
    if not isinstance(m.db, NoHistory):
        raise RuntimeError("mem0 built a history store; refusing to run")
    return m
