"""POST /v1/sparse: the wire contract the Go client (vectordb.MILCO) checks,
the stub's determinism, and the encoder being off by default."""

from __future__ import annotations

import json
import pathlib
import subprocess
import sys

import pytest
from fastapi.testclient import TestClient

from ai_infer import config
from ai_infer.app import create_app
from ai_infer.sparse import common
from ai_infer.sparse.encoders import StubEncoder

ROOT = pathlib.Path(__file__).resolve().parents[1]
TEXTS = ["Quán cà phê yên tĩnh ở Đà Lạt", "quan ca phe o da lat", "phở bò phở gà phở"]


def client(settings, encoder):
    return TestClient(create_app(settings, encoder=encoder))


def test_sparse_stub_deterministic(settings, auth):
    """Same vectors in this process, in a fresh process (no salted hash())
    and on a second call, with the revision named."""
    c = client(settings, StubEncoder())
    r1 = c.post("/v1/sparse", json={"kind": "doc", "texts": TEXTS}, headers=auth)
    r2 = c.post("/v1/sparse", json={"kind": "doc", "texts": TEXTS}, headers=auth)
    assert r1.status_code == 200, r1.text
    assert r1.json() == r2.json()
    body = r1.json()
    assert body["model_revision"] == "stub-hash-v1" and body["kind"] == "doc"
    code = (
        "import json,sys; from ai_infer.sparse.encoders import StubEncoder;"
        f"print(json.dumps(StubEncoder().encode({TEXTS!r}, 'doc')))"
    )
    out = subprocess.run(
        [sys.executable, "-c", code],
        capture_output=True,
        text=True,
        check=True,
        cwd=ROOT,
        env={"PYTHONHASHSEED": "123", "PYTHONPATH": str(ROOT)},
    )
    other = json.loads(out.stdout)
    assert [[v["indices"], v["values"]] for v in body["vectors"]] == other


def test_sparse_contract_shape(settings, auth):
    c = client(settings, StubEncoder())
    long_text = " ".join(f"từ{i}" for i in range(400))
    for kind in ("query", "doc"):
        r = c.post(
            "/v1/sparse",
            json={"kind": kind, "texts": TEXTS + [long_text]},
            headers=auth,
        )
        assert r.status_code == 200
        vecs = r.json()["vectors"]
        assert len(vecs) == len(TEXTS) + 1
        for v in vecs:
            common.check(v["indices"], v["values"], kind)
        assert len(vecs[-1]["indices"]) == common.MAX_NNZ[kind]
    # Repeated tokens coalesce into one index with a larger weight.
    pho = c.post(
        "/v1/sparse",
        json={"kind": "query", "texts": ["phở", "phở phở phở"]},
        headers=auth,
    ).json()
    one, three = pho["vectors"]
    assert one["indices"] == three["indices"] and len(one["indices"]) == 1
    assert three["values"][0] > one["values"][0]


def test_prune_keeps_largest_then_orders_by_index():
    w = {i: float(i % 7) + 0.5 for i in range(300)}
    idx, val = common.prune(w, "doc")
    assert len(idx) == common.MAX_NNZ["doc"] and idx == sorted(idx)
    assert min(val) >= sorted(w.values(), reverse=True)[common.MAX_NNZ["doc"] - 1]
    common.check(idx, val, "doc")


def test_coalesce_sums_and_drops_special_ids():
    assert common.coalesce([(5, 1.0), (5, 0.5), (0, 9.0)], frozenset({0})) == {5: 1.5}


@pytest.mark.parametrize(
    "idx,val",
    [
        ([2, 1], [1.0, 1.0]),
        ([1, 1], [1.0, 1.0]),
        ([1], [0.0]),
        ([1], [float("inf")]),
        ([1 << 30], [1.0]),
        ([1], []),
    ],
)
def test_check_refuses_off_contract(idx, val):
    with pytest.raises(ValueError):
        common.check(idx, val, "query")


def test_sparse_off_by_default(auth):
    s = config.load({"AI_INFER_TOKEN": "x" * 40})
    assert s.sparse_mode == config.SPARSE_OFF
    c = TestClient(create_app(s, encoder=None))
    r = c.post(
        "/v1/sparse",
        json={"kind": "query", "texts": ["a"]},
        headers={"Authorization": "Bearer " + "x" * 40},
    )
    assert r.status_code == 503


def test_sparse_needs_the_token(settings):
    c = client(settings, StubEncoder())
    for h in ({}, {"Authorization": "Bearer wrong"}, {"Authorization": settings.token}):
        assert (
            c.post(
                "/v1/sparse", json={"kind": "query", "texts": ["a"]}, headers=h
            ).status_code
            == 401
        )


def test_validation_error_never_echoes_input(settings, auth):
    c = client(settings, StubEncoder())
    secret = "BI-MAT-KHONG-DUOC-LO"
    r = c.post(
        "/v1/sparse", json={"kind": "bogus", "texts": [secret * 300]}, headers=auth
    )
    assert r.status_code == 422
    assert secret not in r.text


def test_request_limits(settings, auth):
    c = client(settings, StubEncoder())
    assert (
        c.post(
            "/v1/sparse", json={"kind": "doc", "texts": []}, headers=auth
        ).status_code
        == 422
    )
    too_many = ["a"] * (common.MAX_TEXTS + 1)
    assert (
        c.post(
            "/v1/sparse", json={"kind": "doc", "texts": too_many}, headers=auth
        ).status_code
        == 422
    )
