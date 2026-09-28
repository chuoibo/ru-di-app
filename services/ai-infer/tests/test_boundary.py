"""Import boundaries of the sidecar, checked in fresh interpreters.

The sparse encoder serves the places index, which the group assistant's
retrieval reads; memory is Nếp's, per person. Nothing on the sparse path may
load mem0 or the memory store, and an app built with memory off must not load
mem0 at all (no telemetry module, no SQLite manager, nothing to configure).
The canary shows the check sees mem0 when the memory path is imported."""

from __future__ import annotations

import json
import os
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

PROBE = """
import json, sys
mode = sys.argv[1]
if mode == "sparse":
    from ai_infer.sparse import encoders, common
    encoders.StubEncoder().encode(["a b"], "doc")
elif mode == "app-memory-off":
    from ai_infer import config
    from ai_infer.app import create_app
    from ai_infer.sparse.encoders import StubEncoder
    create_app(config.Settings(token="t" * 40, sparse_mode="stub"), encoder=StubEncoder())
else:
    import ai_infer.mem.bootstrap
mods = sorted(m for m in sys.modules if m == "mem0" or m.startswith(("mem0.", "ai_infer.mem")))
print(json.dumps(mods))
"""


def loaded(mode: str) -> list[str]:
    out = subprocess.run(
        [sys.executable, "-c", PROBE, mode],
        capture_output=True,
        text=True,
        cwd=ROOT,
        env={**os.environ, "PYTHONPATH": str(ROOT)},
        timeout=120,
    )
    assert out.returncode == 0, out.stderr[-2000:]
    return json.loads(out.stdout.strip().splitlines()[-1])


def test_sparse_path_never_loads_memory():
    assert loaded("sparse") == []


def test_app_with_memory_off_never_loads_mem0():
    assert not [
        m for m in loaded("app-memory-off") if m == "mem0" or m.startswith("mem0.")
    ]


def test_boundary_canary_is_seen():
    mods = loaded("memory")
    assert "mem0" in mods and "ai_infer.mem.bootstrap" in mods
