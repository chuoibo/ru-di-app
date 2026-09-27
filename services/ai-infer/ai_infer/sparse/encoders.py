"""Sparse encoders behind POST /v1/sparse.

StubEncoder: deterministic feature hashing of the text's word tokens. It has
no weights, needs no network and gives the same vector on every machine and
every run, so CI exercises the whole wire contract. It is a test double for
the MILCO leg, not a retrieval model: nothing decides anything from it.

MilcoEncoder: the real omai-research/milco-650m, loaded from a local
directory pinned by commit, fully offline. It only starts past the licence
gate in config.validate (research milco.md §Kiểm chứng 11): the weights carry
naver/splade-v3's MLM head, whose HF card says CC BY-NC-SA 4.0.
"""

from __future__ import annotations

import hashlib
import math
import os
import pathlib
import re
import unicodedata
from collections import Counter
from typing import Protocol

from ai_infer.config import MILCO_REPO, Settings
from ai_infer.sparse import common

STUB_REVISION = "stub-hash-v1"
STUB_BUCKETS = 1 << 20

_WORD = re.compile(r"\w+", re.UNICODE)


class Encoder(Protocol):
    model: str
    model_revision: str

    def encode(
        self, texts: list[str], kind: str
    ) -> list[tuple[list[int], list[float]]]: ...


def nfc(text: str) -> str:
    return unicodedata.normalize("NFC", text).strip()


class StubEncoder:
    model = "stub-sparse"
    model_revision = STUB_REVISION

    def encode(
        self, texts: list[str], kind: str
    ) -> list[tuple[list[int], list[float]]]:
        out = []
        for text in texts:
            tokens = _WORD.findall(nfc(text).casefold())[: common.MAX_LENGTH[kind]]
            tf = Counter(tokens)
            pairs = [(_bucket(tok), 1.0 + math.log(n)) for tok, n in tf.items()]
            idx, val = common.prune(common.coalesce(pairs), kind)
            out.append((idx, val))
        return out


def _bucket(token: str) -> int:
    # blake2b, not hash(): Python's str hash is salted per process.
    digest = hashlib.blake2b(token.encode("utf-8"), digest_size=8).digest()
    return int.from_bytes(digest, "big") % STUB_BUCKETS


class MilcoUnavailable(RuntimeError):
    """The real encoder cannot start (missing runtime, weights or pin)."""


# Special token ids dropped from both views. BERT-base-uncased (the pivot
# view's vocabulary, splade-v3's head): [PAD]=0 [UNK]=100 [CLS]=101 [SEP]=102
# [MASK]=103. XLM-R (the source view, offset by the pivot vocabulary size):
# <s>=0 <pad>=1 </s>=2 <unk>=3 <mask>=250001. To be re-read from the pinned
# tokenizers when the weights are available (NOT verified: no HF access).
_PIVOT_SPECIAL = (0, 100, 101, 102, 103)
_SOURCE_SPECIAL = (0, 1, 2, 3, 250001)


class MilcoEncoder:
    model = MILCO_REPO

    def __init__(self, settings: Settings):
        self.model_revision = settings.milco_revision
        path = pathlib.Path(settings.milco_path)
        if not path.is_dir():
            raise MilcoUnavailable(f"MILCO weights directory {path} does not exist")
        # The directory must be the pinned snapshot: an HF cache snapshot is
        # named by its commit, a copied one carries a REVISION file.
        rev_file = path / "REVISION"
        pinned = path.name == settings.milco_revision or (
            rev_file.is_file()
            and rev_file.read_text().strip() == settings.milco_revision
        )
        if not pinned:
            raise MilcoUnavailable("MILCO weights directory is not the pinned revision")
        # Never reach the Hub: trust_remote_code runs code from the snapshot,
        # which must be the reviewed, pinned one.
        os.environ["HF_HUB_OFFLINE"] = "1"
        os.environ["TRANSFORMERS_OFFLINE"] = "1"
        try:
            import torch  # noqa: F401
            from transformers import AutoModel
        except ImportError as exc:
            raise MilcoUnavailable(
                "MILCO mode needs torch and transformers (requirements-milco.txt), not installed"
            ) from exc
        self._model = AutoModel.from_pretrained(
            str(path), trust_remote_code=True, local_files_only=True
        )
        self._model.eval()
        en_vocab = int(getattr(self._model.config, "en_vocab_size", 30522))
        self._drop = frozenset(_PIVOT_SPECIAL) | frozenset(
            en_vocab + i for i in _SOURCE_SPECIAL
        )

    def encode(
        self, texts: list[str], kind: str
    ) -> list[tuple[list[int], list[float]]]:
        import torch

        with torch.inference_mode():
            sp = self._model.encode_text(
                [nfc(t) for t in texts],
                batch_size=len(texts),
                max_length=common.MAX_LENGTH[kind],
                return_dict=False,
            )
        # A single batch comes back uncoalesced (research milco.md §3 trap 3).
        sp = sp.coalesce()
        rows, cols = sp.indices().tolist()
        vals = sp.values().float().tolist()
        per_row: list[list[tuple[int, float]]] = [[] for _ in texts]
        for r, c, v in zip(rows, cols, vals):
            per_row[r].append((c, v))
        return [common.prune(common.coalesce(p, self._drop), kind) for p in per_row]


def build(settings: Settings) -> Encoder | None:
    from ai_infer.config import SPARSE_MILCO, SPARSE_STUB

    if settings.sparse_mode == SPARSE_STUB:
        return StubEncoder()
    if settings.sparse_mode == SPARSE_MILCO:
        return MilcoEncoder(settings)
    return None
