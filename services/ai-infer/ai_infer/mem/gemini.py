"""Model back-ends of the memory path: the Gemini API through google-genai
(real, switched on by configuration only) and in-process stubs.

Text format is the one services/core aiharness/nhung uses, so a memory and a
place embed the same way (research gemini-embedding-2.md §6.2, §Kiểm chứng
2): on the Gemini Developer API the task is written into the text, and
neither TaskType nor Title is ever set.
"""

from __future__ import annotations

import hashlib
import math
import re
import unicodedata
from typing import Protocol

from ai_infer.config import EMBED_DIMS, EMBED_MODEL, LLM_MODEL

QUERY_PREFIX = "task: search result | query: "
DOC_PREFIX = "title: none | text: "


def nfc(text: str) -> str:
    return unicodedata.normalize("NFC", text).strip()


def format_for(text: str, action: str | None) -> str:
    """The model input for one text: a search query or a stored document."""
    t = nfc(text)
    if action == "search":
        return QUERY_PREFIX + t
    return DOC_PREFIX + t


class EmbedBackend(Protocol):
    def embed(self, inputs: list[str], raw: list[str]) -> list[list[float]]:
        """Vectors for the formatted inputs (raw: the same texts unformatted)."""


class LlmBackend(Protocol):
    def complete(self, system: str, user: str, schema: dict) -> str:
        """The model's JSON answer to one extraction call."""


class EmbedError(RuntimeError):
    pass


def normalise(values: list[float], dims: int = EMBED_DIMS) -> list[float]:
    """Check the dimensionality and scale to unit length. A truncated MRL
    vector is not documented as normalised (research §Kiểm chứng 4), and a
    silent 3072 would be refused by Milvus anyway: fail here, by name."""
    if len(values) != dims:
        raise EmbedError(f"embedding has {len(values)} values, want {dims}")
    n = math.sqrt(sum(v * v for v in values))
    if not (n > 0 and math.isfinite(n)):
        raise EmbedError("embedding has zero or non-finite norm")
    return [v / n for v in values]


def _client(api_key: str, base_url: str, timeout_s: float):
    from google import genai
    from google.genai import types

    opts = types.HttpOptions(timeout=int(timeout_s * 1000))
    if base_url:
        opts = types.HttpOptions(base_url=base_url, timeout=int(timeout_s * 1000))
    return genai.Client(api_key=api_key, http_options=opts)


class GeminiEmbed:
    """gemini-embedding-2 at 1536 dims, one batch request per call."""

    def __init__(self, api_key: str, base_url: str = "", timeout_s: float = 20.0):
        self._client = _client(api_key, base_url, timeout_s)

    def embed(self, inputs: list[str], raw: list[str]) -> list[list[float]]:
        from google.genai import types

        # One Content per text. A plain list of strings is NOT a batch in
        # google-genai 2.25: it becomes ONE content with N parts, one request,
        # and one (multimodal, aggregated) vector for all N texts -- measured
        # against the loopback fake (tests/test_gemini_wire.py).
        resp = self._client.models.embed_content(
            model=EMBED_MODEL,
            contents=[
                types.Content(role="user", parts=[types.Part(text=t)]) for t in inputs
            ],
            config=types.EmbedContentConfig(output_dimensionality=EMBED_DIMS),
        )
        embs = resp.embeddings or []
        if len(embs) != len(inputs):
            raise EmbedError(f"{len(embs)} embeddings for {len(inputs)} texts")
        return [list(e.values or []) for e in embs]


class GeminiLlm:
    """gemini-3.5-flash-lite in JSON mode with a response schema."""

    def __init__(self, api_key: str, base_url: str = "", timeout_s: float = 20.0):
        self._client = _client(api_key, base_url, timeout_s)

    def complete(self, system: str, user: str, schema: dict) -> str:
        from google.genai import types

        resp = self._client.models.generate_content(
            model=LLM_MODEL,
            contents=[types.Content(role="user", parts=[types.Part(text=user)])],
            config=types.GenerateContentConfig(
                system_instruction=system,
                temperature=0.1,
                max_output_tokens=1024,
                response_mime_type="application/json",
                response_json_schema=schema,
            ),
        )
        return resp.text or ""


_WORD = re.compile(r"\w+", re.UNICODE)


class StubEmbed:
    """Deterministic signed feature hashing of the raw text's words, so texts
    sharing words land close. A test double with no model behind it."""

    def embed(self, inputs: list[str], raw: list[str]) -> list[list[float]]:
        out = []
        for text in raw:
            v = [0.0] * EMBED_DIMS
            for tok in _WORD.findall(nfc(text).casefold()) or ["<empty>"]:
                d = hashlib.blake2b(tok.encode("utf-8"), digest_size=8).digest()
                h = int.from_bytes(d, "big")
                v[h % EMBED_DIMS] += 1.0 if (h >> 32) & 1 else -1.0
            if not any(v):
                v[0] = 1.0
            out.append(v)
        return out


class StubLlm:
    """Extracts nothing: a dev stack without a key stores no memory."""

    def complete(self, system: str, user: str, schema: dict) -> str:
        return '{"memory": []}'
