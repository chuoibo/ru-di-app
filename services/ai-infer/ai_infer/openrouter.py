"""OpenRouter calls of the sidecar (ADR-0049 §2.3-2.4): the reranker
qwen/qwen3-reranker-8b (Fireworks only). Python keeps these calls; Go reaches
them through this sidecar on loopback.

Only the model ids listed here may be called: the sidecar spends the owner's
key, so a caller cannot name another model. Bodies and keys are never logged.
"""

from __future__ import annotations

import math
from typing import Optional

import httpx

RERANK_MODEL = "qwen/qwen3-reranker-8b"
RERANK_PROVIDER = {"only": ["fireworks"], "allow_fallbacks": False}


class OpenRouterError(RuntimeError):
    """The provider failed or answered outside the contract (no body kept)."""


class OpenRouter:
    def __init__(
        self,
        api_key: str,
        base_url: str,
        timeout_s: float,
        client: Optional[httpx.Client] = None,
    ) -> None:
        self._key = api_key
        self._base = base_url.rstrip("/")
        self._client = client or httpx.Client(timeout=timeout_s)

    def rerank(self, query: str, documents: list[str], top_n: int) -> list[dict]:
        """One score per document, as [{index, relevance_score}] in the
        provider's order. Every index is checked: in range, unique, one per
        returned item, finite score."""
        try:
            r = self._client.post(
                self._base + "/rerank",
                headers={"Authorization": "Bearer " + self._key},
                json={
                    "model": RERANK_MODEL,
                    "query": query,
                    "documents": documents,
                    "top_n": top_n,
                    "provider": RERANK_PROVIDER,
                },
            )
        except httpx.HTTPError as exc:
            raise OpenRouterError(type(exc).__name__) from None
        if r.status_code != 200:
            raise OpenRouterError(f"HTTP {r.status_code}")
        try:
            results = r.json()["results"]
        except (ValueError, KeyError, TypeError):
            raise OpenRouterError("unreadable answer") from None
        out, seen = [], set()
        for item in results:
            try:
                i, s = int(item["index"]), float(item["relevance_score"])
            except (KeyError, TypeError, ValueError):
                raise OpenRouterError("malformed result") from None
            if not (0 <= i < len(documents)) or i in seen or not math.isfinite(s):
                raise OpenRouterError("result index or score out of contract")
            seen.add(i)
            out.append({"index": i, "relevance_score": s})
        if len(out) > top_n:
            raise OpenRouterError("more results than asked")
        return out
