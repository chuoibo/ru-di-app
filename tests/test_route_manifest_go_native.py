"""Go-only routes remain in the ownership source without Python stubs."""

import importlib.util
from pathlib import Path


def _generator():
    path = Path(__file__).resolve().parents[1] / "scripts/render_route_manifest.py"
    spec = importlib.util.spec_from_file_location("render_route_manifest", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_go_native_route_survives_python_manifest_regeneration(monkeypatch):
    generator = _generator()
    python_route = {
        "id": "GET /healthz",
        "order": 0,
        "kind": "route",
        "method": "GET",
        "path": "/healthz",
        "group": "main",
        "class": "core",
    }
    native = {
        "id": "GET /me/profile-videos/credits",
        "order": 1,
        "kind": "route",
        "method": "GET",
        "path": "/me/profile-videos/credits",
        "group": "profile-media",
        "class": "core",
        "owner": "go",
        "python": "absent",
        "state": "GO-NATIVE",
        "evidence": "docs/migration/go-native-profile.md",
    }
    monkeypatch.setattr(generator, "_app_rows", lambda: [python_route.copy()])
    output = generator.build({"schema": 1, "routes": [native]}, prune=False)
    assert output["routes"][1] == native
