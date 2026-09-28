"""What mem0 does by default and the sidecar must never do: telemetry
(PostHog), SQLite history of raw messages, memory text in logs, an entity
store. Each check comes with a canary that shows the check can see the
thing it forbids.

Sentinels: test_posthog_never_constructed, test_no_sqlite_ever."""

from __future__ import annotations

import importlib.util
import logging
import os
import pathlib
import sqlite3
import subprocess
import sys
import textwrap

import pytest
from conftest import MEM0_DIR, build_service
from fakes import ScriptedLlm, own

ROOT = pathlib.Path(__file__).resolve().parents[1]

# Runs in a fresh interpreter: posthog.Posthog is replaced before anything
# imports mem0, every socket connect is recorded and refused, then the whole
# memory flow runs. Prints the constructions and connects it saw.
PROBE = textwrap.dedent(
    """
    import json, os, socket, sys
    sys.path.insert(0, "tests")
    import posthog
    made = []
    class Boom:
        def __init__(self, *a, **k):
            made.append("posthog")
            raise RuntimeError("posthog constructed")
    posthog.Posthog = Boom
    connects = []
    def no_connect(self, addr):
        connects.append(str(addr))
        raise OSError("egress refused in test")
    socket.socket.connect = no_connect
    MODE = sys.argv[1]
    if MODE == "sidecar":
        from ai_infer.mem import store
        from fakes import FakeMilvusClient, ScriptedLlm, own
        store.client_factory = FakeMilvusClient
        from conftest import build_service
        svc = build_service(ScriptedLlm([own("Người dùng thích hồ")]))
        svc.add("u1", ["x"]); svc.search("u1", "hồ", 5, 0.0); svc.list("u1", 10); svc.delete_all("u1")
        import mem0.memory.telemetry as t
        tel = t.MEM0_TELEMETRY
    else:  # canary: mem0 imported on its own with telemetry on builds a
        # PostHog client at import (mem0/memory/telemetry.py, module level)
        try:
            import mem0.memory.telemetry
        except RuntimeError:
            pass
        tel = os.environ["MEM0_TELEMETRY"].lower() in ("true", "1", "yes")
    print(json.dumps({"posthog": made, "connects": connects, "telemetry": tel}))
    """
)


def run_probe(mode: str) -> dict:
    import json

    env = {**os.environ, "MEM0_TELEMETRY": "true", "PYTHONPATH": str(ROOT)}
    out = subprocess.run(
        [sys.executable, "-c", PROBE, mode],
        capture_output=True,
        text=True,
        cwd=ROOT,
        env=env,
        timeout=120,
    )
    assert out.returncode == 0, out.stderr[-2000:]
    return json.loads(out.stdout.strip().splitlines()[-1])


def test_posthog_never_constructed():
    """With MEM0_TELEMETRY=true in the environment, the sidecar's full memory
    flow constructs no PostHog client and opens no connection."""
    got = run_probe("sidecar")
    assert got == {"posthog": [], "connects": [], "telemetry": False}


def test_posthog_canary_is_seen():
    """The same probe sees PostHog being built when mem0 is used directly
    with telemetry on: the check above is able to fail."""
    got = run_probe("canary")
    assert got["telemetry"] is True and got["posthog"] == ["posthog"]


def _db_files() -> list[str]:
    found = []
    for base in (pathlib.Path(MEM0_DIR), ROOT, pathlib.Path.home() / ".mem0"):
        if base.exists():
            found += [str(p) for p in base.rglob("*.db")]
    return found


def test_no_sqlite_ever(fake_store, monkeypatch):
    """No SQLite connection is opened by the whole flow (add, duplicate add,
    search, list, delete, delete_all, purge) and no .db file appears."""
    before = _db_files()
    opened = []

    def refuse(*a, **k):
        opened.append(a)
        raise AssertionError("sqlite3.connect called")

    monkeypatch.setattr(sqlite3, "connect", refuse)
    svc = build_service(
        ScriptedLlm(
            [own("Người dùng thích công viên")],
            [own("Người dùng thích công viên")],
            [own("Người dùng thích sân khấu")],
        )
    )
    svc.add("u1", ["x"])
    svc.add("u1", ["x"])
    added, _ = svc.add("u1", ["y"])
    svc.search("u1", "công viên", 5, 0.0)
    svc.list("u1", 10)
    svc.delete("u1", added[0].id)
    svc.delete_all("u1")
    svc.purge_user("u1")
    assert opened == []
    assert _db_files() == before


@pytest.mark.filterwarnings("ignore::pytest.PytestUnraisableExceptionWarning")
def test_sqlite_canary_is_seen(monkeypatch):
    """mem0's own history manager does open SQLite: the patch above would see it."""
    import ai_infer.mem.bootstrap  # noqa: F401
    from mem0.memory.storage import SQLiteManager

    opened = []
    monkeypatch.setattr(
        sqlite3,
        "connect",
        lambda *a, **k: opened.append(a) or (_ for _ in ()).throw(RuntimeError()),
    )
    with pytest.raises(RuntimeError):
        SQLiteManager(":memory:")
    assert opened


CANARY = "CHIM-CANH-KY-UC-7"


def _flow_logs(caplog) -> str:
    svc = build_service(
        ScriptedLlm(
            [own(f"Người dùng thích {CANARY}")], [own(f"Người dùng thích {CANARY}")]
        )
    )
    with caplog.at_level(logging.DEBUG):
        svc.add("u1", [CANARY])
        svc.add("u1", [CANARY])  # duplicate: mem0 logs the text at DEBUG
        mid = svc.list("u1", 10)[0]["id"]
        svc.search("u1", CANARY, 5, 0.0)
        svc.delete("u1", mid)
    return "\n".join(r.getMessage() for r in caplog.records)


def test_memory_text_never_logged(fake_store, caplog):
    assert CANARY not in _flow_logs(caplog)


def test_log_canary_is_seen(fake_store, caplog):
    """With mem0's loggers left as mem0 ships them, the same flow logs the text."""
    from ai_infer.mem import bootstrap

    real = bootstrap.silence_mem0_logs
    try:
        bootstrap.silence_mem0_logs = lambda: None
        lg = logging.getLogger("mem0")
        lg.setLevel(logging.NOTSET)
        lg.propagate = True
        assert CANARY in _flow_logs(caplog)
    finally:
        bootstrap.silence_mem0_logs = real
        real()


def test_no_spacy_no_entity_store(fake_store):
    assert importlib.util.find_spec("spacy") is None
    svc = build_service(ScriptedLlm([own("Người dùng thích Hà Giang")]))
    svc.add("u1", ["x"])
    svc.search("u1", "Hà Giang", 5, 0.0)
    assert svc.m._entity_store is None
    assert list(fake_store.instances[0].collections) == ["memories_v1"]
    assert len(fake_store.instances) == 1
