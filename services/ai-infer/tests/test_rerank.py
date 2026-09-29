"""/rerank: the vLLM/Cohere contract Go's internal/rerank sends, answered by
OpenRouter's qwen/qwen3-reranker-8b (Fireworks only). No test reaches
OpenRouter: the transport is a local fake."""

from __future__ import annotations

import json

import httpx
import pytest
from fastapi.testclient import TestClient

from conftest import TOKEN


def _app(settings, handler):
    from ai_infer.app import create_app
    from ai_infer.openrouter import OpenRouter

    client = httpx.Client(transport=httpx.MockTransport(handler))
    return TestClient(
        create_app(
            settings,
            openrouter=OpenRouter(
                "or-key", "https://openrouter.test/api/v1", 5, client
            ),
        )
    )


def _ok(seen):
    def handler(req: httpx.Request) -> httpx.Response:
        seen.append(req)
        body = json.loads(req.content)
        n = len(body["documents"])
        results = [
            {"index": i, "relevance_score": 1.0 / (i + 1), "document": {"text": "echo"}}
            for i in reversed(range(n))
        ]
        return httpx.Response(200, json={"results": results[: body["top_n"]]})

    return handler


REQ = {
    "model": "qwen/qwen3-reranker-8b",
    "query": "cà phê yên tĩnh",
    "documents": ["a", "b", "c"],
    "top_n": 3,
}
AUTH = {"Authorization": "Bearer " + TOKEN}


def test_rerank_goi_dung_model_va_provider(settings):
    seen = []
    c = _app(settings, _ok(seen))
    r = c.post("/rerank", json=REQ, headers=AUTH)
    assert r.status_code == 200
    assert [x["index"] for x in r.json()["results"]] == [2, 1, 0]
    assert all(
        "document" not in x for x in r.json()["results"]
    )  # the echoed text is never passed on
    sent = json.loads(seen[0].content)
    assert seen[0].url.path == "/api/v1/rerank"
    assert seen[0].headers["authorization"] == "Bearer or-key"
    assert sent["model"] == "qwen/qwen3-reranker-8b"
    assert sent["provider"] == {"only": ["fireworks"], "allow_fallbacks": False}


def test_rerank_can_token_va_chi_mot_model(settings):
    c = _app(settings, _ok([]))
    assert c.post("/rerank", json=REQ).status_code == 401
    other = dict(REQ, model="cohere/rerank-v3.5")
    assert c.post("/rerank", json=other, headers=AUTH).status_code == 400


@pytest.mark.parametrize(
    "answer",
    [
        {"results": [{"index": 5, "relevance_score": 0.3}]},  # out of range
        {
            "results": [
                {"index": 0, "relevance_score": 0.3},
                {"index": 0, "relevance_score": 0.2},
            ]
        },  # repeated
        {"results": [{"index": 0, "relevance_score": "nan"}]},  # not finite
        {"nope": []},
    ],
)
def test_rerank_tra_loi_sai_hop_dong_la_502(settings, answer):
    c = _app(settings, lambda req: httpx.Response(200, json=answer))
    assert c.post("/rerank", json=REQ, headers=AUTH).status_code == 502


def test_rerank_khong_cau_hinh_la_503(settings):
    from ai_infer.app import create_app

    c = TestClient(create_app(settings))
    assert c.post("/rerank", json=REQ, headers=AUTH).status_code == 503


def test_cau_hinh_openrouter():
    from ai_infer import config

    base = {"AI_INFER_TOKEN": TOKEN}
    s = config.load(dict(base, OPEN_ROUTER_API_KEY="k"))
    assert s.openrouter_base_url == "https://openrouter.ai/api/v1" and "k" not in repr(
        s
    )
    with pytest.raises(config.ConfigError):
        config.load(dict(base, AI_INFER_OPENROUTER_BASE_URL="http://evil.example/api"))
    assert config.load(
        dict(base, AI_INFER_OPENROUTER_BASE_URL="http://127.0.0.1:9/api")
    ).openrouter_base_url
