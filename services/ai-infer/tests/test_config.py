"""Start-up refusals: nothing real starts unless configuration says so, and
MILCO does not start at all without a recorded licence review."""

from __future__ import annotations

import pytest

from ai_infer import config
from ai_infer.sparse.encoders import MilcoEncoder, MilcoUnavailable

TOK = "k" * 40
REV = "0123456789abcdef0123456789abcdef01234567"


def env(**kw):
    return {"AI_INFER_TOKEN": TOK, **kw}


def test_defaults_are_all_off():
    s = config.load(env())
    assert (s.sparse_mode, s.gemini_mode) == (config.SPARSE_OFF, config.GEMINI_OFF)
    assert TOK not in repr(s)


@pytest.mark.parametrize(
    "extra,needle",
    [
        ({"AI_INFER_TOKEN": "short"}, "AI_INFER_TOKEN"),
        (
            {
                "AI_INFER_SPARSE_MODE": "milco",
                "AI_INFER_MILCO_REVISION": REV,
                "AI_INFER_MILCO_PATH": "/x",
            },
            "LICENSE_REVIEW",
        ),
        (
            {
                "AI_INFER_SPARSE_MODE": "milco",
                "AI_INFER_MILCO_LICENSE_REVIEW": "ADR-9",
                "AI_INFER_MILCO_PATH": "/x",
                "AI_INFER_MILCO_REVISION": "main",
            },
            "40-hex",
        ),
        (
            {
                "AI_INFER_SPARSE_MODE": "milco",
                "AI_INFER_MILCO_LICENSE_REVIEW": "ADR-9",
                "AI_INFER_MILCO_REVISION": REV,
            },
            "MILCO_PATH",
        ),
        (
            {
                "AI_INFER_GEMINI_MODE": "real",
                "AI_INFER_MILVUS_URI": "http://127.0.0.1:19530",
            },
            "GEMINI_API_KEY",
        ),
        ({"AI_INFER_GEMINI_MODE": "stub"}, "MILVUS_URI"),
        (
            {
                "AI_INFER_GEMINI_MODE": "stub",
                "AI_INFER_MILVUS_URI": "u",
                "AI_INFER_MEMORY_COLLECTION": "nep_memories",
            },
            "memories_v",
        ),
        ({"AI_INFER_SPARSE_MODE": "maybe"}, "SPARSE_MODE"),
    ],
)
def test_refusals(extra, needle):
    with pytest.raises(config.ConfigError, match=needle):
        config.load(env(**extra))


def test_milco_real_mode_needs_pinned_local_weights(tmp_path):
    s = config.load(
        env(
            AI_INFER_SPARSE_MODE="milco",
            AI_INFER_MILCO_LICENSE_REVIEW="ADR-9",
            AI_INFER_MILCO_REVISION=REV,
            AI_INFER_MILCO_PATH=str(tmp_path / "missing"),
        )
    )
    with pytest.raises(MilcoUnavailable, match="does not exist"):
        MilcoEncoder(s)
    wrong = tmp_path / "snap"
    wrong.mkdir()
    (wrong / "REVISION").write_text("f" * 40)
    s2 = config.load(
        env(
            AI_INFER_SPARSE_MODE="milco",
            AI_INFER_MILCO_LICENSE_REVIEW="ADR-9",
            AI_INFER_MILCO_REVISION=REV,
            AI_INFER_MILCO_PATH=str(wrong),
        )
    )
    with pytest.raises(MilcoUnavailable, match="pinned revision"):
        MilcoEncoder(s2)


def test_milco_real_mode_without_runtime_says_so(tmp_path, monkeypatch):
    """This environment has no torch/transformers (they are only in
    requirements-milco.txt): the pinned directory passes, the import fails by
    name, and the Hub is forced offline before any import is tried."""
    snap = tmp_path / REV
    snap.mkdir()
    monkeypatch.delenv("HF_HUB_OFFLINE", raising=False)
    s = config.load(
        env(
            AI_INFER_SPARSE_MODE="milco",
            AI_INFER_MILCO_LICENSE_REVIEW="ADR-9",
            AI_INFER_MILCO_REVISION=REV,
            AI_INFER_MILCO_PATH=str(snap),
        )
    )
    import importlib.util
    import os

    if importlib.util.find_spec("torch") is not None:
        pytest.fail(
            "torch is installed in the offline test environment; this test expects it absent"
        )
    with pytest.raises(MilcoUnavailable, match="torch and transformers"):
        MilcoEncoder(s)
    assert os.environ["HF_HUB_OFFLINE"] == "1"
