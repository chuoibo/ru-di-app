"""Settings of the inference sidecar, read once from the environment.

Every switch that could make a real outside call (Gemini, MILCO weights) is a
configuration value with a safe default: nothing real runs unless the
environment says so. Invalid combinations fail at start-up, never at the
first request.
"""

from __future__ import annotations

import dataclasses
import os
import re
from typing import Mapping

# Sparse encoder modes.
SPARSE_OFF = "off"  # /v1/sparse answers 503: Go uses Milvus BM25 instead
SPARSE_STUB = "stub"  # deterministic hashing, no weights (CI, local dev)
SPARSE_MILCO = "milco"  # the real MILCO model, behind the licence gate
SPARSE_MODES = (SPARSE_OFF, SPARSE_STUB, SPARSE_MILCO)

# Model back-end of the memory endpoints.
GEMINI_OFF = "off"  # /v1/memory/* answers 503
GEMINI_STUB = "stub"  # in-process stub embedder, extractor that stores nothing
GEMINI_REAL = "real"  # the Gemini API through google-genai
GEMINI_MODES = (GEMINI_OFF, GEMINI_STUB, GEMINI_REAL)

# Pinned MILCO weights (research milco.md §7.3): a commit hash, never a branch.
MILCO_REPO = "omai-research/milco-650m"
_COMMIT = re.compile(r"^[0-9a-f]{40}$")

# The one embedding model and dimensionality for every collection (research
# gemini-embedding-2.md §6.1); must match services/core aiharness/nhung.
EMBED_MODEL = "gemini-embedding-2"
EMBED_DIMS = 1536
LLM_MODEL = "gemini-3.5-flash-lite"

MIN_TOKEN_LEN = 32


class ConfigError(ValueError):
    """A configuration the sidecar refuses to start with."""


@dataclasses.dataclass(frozen=True)
class Settings:
    token: str
    sparse_mode: str = SPARSE_OFF
    milco_path: str = ""
    milco_revision: str = ""
    milco_license_review: str = ""
    gemini_mode: str = GEMINI_OFF
    gemini_api_key: str = ""
    gemini_base_url: str = ""
    gemini_timeout_s: float = 20.0
    milvus_uri: str = ""
    milvus_token: str = ""
    milvus_db: str = "nep_memory"
    memory_collection: str = "memories_v1"
    mem0_dir: str = ""

    def __repr__(self) -> str:  # never print secrets
        return (
            f"Settings(sparse_mode={self.sparse_mode!r}, gemini_mode={self.gemini_mode!r}, "
            f"milvus_db={self.milvus_db!r}, memory_collection={self.memory_collection!r})"
        )


_COLLECTION = re.compile(r"^memories_v[1-9][0-9]*$")
_DB = re.compile(r"^[A-Za-z_][A-Za-z0-9_]{0,63}$")


def load(env: Mapping[str, str] | None = None) -> Settings:
    """Build Settings from env (os.environ by default) and validate them."""
    e = os.environ if env is None else env
    g = lambda k, d="": (e.get(k) or d).strip()  # noqa: E731
    try:
        timeout = float(g("AI_INFER_GEMINI_TIMEOUT_S", "20"))
    except ValueError as exc:
        raise ConfigError("AI_INFER_GEMINI_TIMEOUT_S is not a number") from exc
    s = Settings(
        token=g("AI_INFER_TOKEN"),
        sparse_mode=g("AI_INFER_SPARSE_MODE", SPARSE_OFF),
        milco_path=g("AI_INFER_MILCO_PATH"),
        milco_revision=g("AI_INFER_MILCO_REVISION"),
        milco_license_review=g("AI_INFER_MILCO_LICENSE_REVIEW"),
        gemini_mode=g("AI_INFER_GEMINI_MODE", GEMINI_OFF),
        gemini_api_key=g("GEMINI_API_KEY"),
        gemini_base_url=g("AI_INFER_GEMINI_BASE_URL"),
        gemini_timeout_s=timeout,
        milvus_uri=g("AI_INFER_MILVUS_URI"),
        milvus_token=g("AI_INFER_MILVUS_TOKEN"),
        milvus_db=g("AI_INFER_MILVUS_DB", "nep_memory"),
        memory_collection=g("AI_INFER_MEMORY_COLLECTION", "memories_v1"),
        mem0_dir=g("MEM0_DIR"),
    )
    validate(s)
    return s


def validate(s: Settings) -> None:
    if len(s.token) < MIN_TOKEN_LEN:
        raise ConfigError(
            f"AI_INFER_TOKEN must be set and at least {MIN_TOKEN_LEN} characters"
        )
    if s.sparse_mode not in SPARSE_MODES:
        raise ConfigError(f"AI_INFER_SPARSE_MODE must be one of {SPARSE_MODES}")
    if s.sparse_mode == SPARSE_MILCO:
        # The licence gate (research milco.md §Kiểm chứng 11, sdlc §Kiểm chứng
        # 14): the weights embed naver/splade-v3's head (CC BY-NC-SA 4.0 on its
        # HF card) and the MILCO card was never read. A human must read both
        # cards and record where (ADR or ticket) before real mode can start.
        if not s.milco_license_review:
            raise ConfigError(
                "AI_INFER_SPARSE_MODE=milco needs AI_INFER_MILCO_LICENSE_REVIEW: the reference of "
                "a recorded human review of the omai-research/milco-650m and naver/splade-v3 "
                "licences. Until then use Milvus BM25 (mode off)."
            )
        if not _COMMIT.match(s.milco_revision):
            raise ConfigError(
                "AI_INFER_MILCO_REVISION must be the 40-hex commit of the pinned weights"
            )
        if not s.milco_path:
            raise ConfigError(
                "AI_INFER_MILCO_PATH must name the local weights directory (no download)"
            )
    if s.gemini_mode not in GEMINI_MODES:
        raise ConfigError(f"AI_INFER_GEMINI_MODE must be one of {GEMINI_MODES}")
    if s.gemini_mode == GEMINI_REAL and not s.gemini_api_key:
        raise ConfigError("AI_INFER_GEMINI_MODE=real needs GEMINI_API_KEY")
    if s.gemini_mode != GEMINI_OFF:
        if not s.milvus_uri:
            raise ConfigError("memory endpoints need AI_INFER_MILVUS_URI")
        if not _DB.match(s.milvus_db):
            raise ConfigError("AI_INFER_MILVUS_DB is not a valid database name")
        if not _COLLECTION.match(s.memory_collection):
            raise ConfigError("AI_INFER_MEMORY_COLLECTION must look like memories_v<N>")
    if not (0 < s.gemini_timeout_s <= 120):
        raise ConfigError("AI_INFER_GEMINI_TIMEOUT_S must be in (0, 120]")
