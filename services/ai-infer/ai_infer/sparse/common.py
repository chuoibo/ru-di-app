"""Shape rules every sparse encoder's output follows before it leaves the
process: coalesced (one entry per index), special ids dropped, pruned to the
per-kind budget, indices strictly increasing, values positive and finite.

The Go client (services/core vectordb.ThuaVec.Kiem) refuses anything else, so
these rules are the wire contract, checked on both sides.
"""

from __future__ import annotations

import math
from typing import Iterable

KIND_QUERY = "query"
KIND_DOC = "doc"
KINDS = (KIND_QUERY, KIND_DOC)

# Token budgets fed to the encoder (research milco.md §7.4: query 64, doc
# 256; MILCO's own default would be BGE-M3's 8192).
MAX_LENGTH = {KIND_QUERY: 64, KIND_DOC: 256}
# Non-zeros kept per vector: a document keeps its top 128 terms (≈ the blog's
# "~120 tokens/doc"), a query keeps all its terms up to 64.
MAX_NNZ = {KIND_QUERY: 64, KIND_DOC: 128}
# Index space bound shared with Go (vectordb.MILCOChiMax = 1 << 30).
INDEX_LIMIT = 1 << 30
# Request limits (vectordb.MaxLoMILCO = 32).
MAX_TEXTS = 32
MAX_TEXT_CHARS = 4000


def coalesce(
    pairs: Iterable[tuple[int, float]], drop: frozenset[int] = frozenset()
) -> dict[int, float]:
    """Sum the weights of repeated indices (MILCO's own coalesce sums, unlike
    BGE-M3's max) and drop the ids in drop."""
    out: dict[int, float] = {}
    for idx, val in pairs:
        if idx in drop:
            continue
        out[idx] = out.get(idx, 0.0) + float(val)
    return out


def prune(weights: dict[int, float], kind: str) -> tuple[list[int], list[float]]:
    """Keep the kind's budget of largest positive finite weights, then return
    them ordered by index. Ties on weight break on the smaller index so the
    result is deterministic."""
    kept = [
        (i, w)
        for i, w in weights.items()
        if w > 0 and math.isfinite(w) and 0 <= i < INDEX_LIMIT
    ]
    kept.sort(key=lambda iw: (-iw[1], iw[0]))
    kept = kept[: MAX_NNZ[kind]]
    kept.sort(key=lambda iw: iw[0])
    return [i for i, _ in kept], [w for _, w in kept]


def check(indices: list[int], values: list[float], kind: str) -> None:
    """Raise ValueError unless (indices, values) meets the wire contract."""
    if len(indices) != len(values):
        raise ValueError("indices and values differ in length")
    if len(indices) > MAX_NNZ[kind]:
        raise ValueError("more non-zeros than the kind allows")
    prev = -1
    for i, v in zip(indices, values):
        if not (0 <= i < INDEX_LIMIT) or i <= prev:
            raise ValueError("indices not strictly increasing within range")
        if not (v > 0 and math.isfinite(v)):
            raise ValueError("value not positive and finite")
        prev = i
