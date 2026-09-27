"""The real Gemini path (google-genai) against a loopback fake of the API:
what goes on the wire, and what comes back is checked. No real call is made
(ADR-0034 §2.6): the base URL is a 127.0.0.1 port this test owns.

Research gemini-embedding-2.md §Kiểm chứng 2 and 6: on the Developer API the
task is written into the text, taskType/title are never sent, and the
dimensionality is checked on the answer (a server that ignores
outputDimensionality answers 3072, which Milvus would refuse later)."""

from __future__ import annotations

import http.server
import json
import threading

import pytest
from conftest import build_service
from fakes import own

from ai_infer.config import EMBED_DIMS, EMBED_MODEL, LLM_MODEL
from ai_infer.mem import gemini


class FakeGemini:
    def __init__(self):
        self.requests: list[tuple[str, dict, dict]] = []
        self.dims = EMBED_DIMS
        self.llm_items: list[dict] = []
        fake = self

        class H(http.server.BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_POST(self):
                body = json.loads(
                    self.rfile.read(int(self.headers["Content-Length"])) or b"{}"
                )
                fake.requests.append((self.path, dict(self.headers), body))
                if self.path.endswith(":batchEmbedContents"):
                    n = len(body.get("requests", []))
                    out = {
                        "embeddings": [
                            {
                                "values": [
                                    float((i + j) % 5 + 1) for j in range(fake.dims)
                                ]
                            }
                            for i in range(n)
                        ]
                    }
                elif self.path.endswith(":generateContent"):
                    text = json.dumps({"memory": fake.llm_items}, ensure_ascii=False)
                    out = {
                        "candidates": [
                            {
                                "content": {"role": "model", "parts": [{"text": text}]},
                                "finishReason": "STOP",
                            }
                        ]
                    }
                else:
                    self.send_response(404)
                    self.end_headers()
                    return
                data = json.dumps(out).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), H)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def close(self):
        self.server.shutdown()


@pytest.fixture
def fake_gemini():
    f = FakeGemini()
    yield f
    f.close()


def test_embed_wire_contract(fake_gemini):
    e = gemini.GeminiEmbed("test-key-not-real", fake_gemini.url, 5)
    from ai_infer.mem.bootstrap import NepEmbed

    ne = NepEmbed.__new__(NepEmbed)
    ne.backend = e
    q = ne.embed_batch(["  Quán cà phê yên tĩnh  "], "search")
    d = ne.embed_batch(["Người dùng thích núi", "Người dùng thích biển"], "add")
    assert len(q) == 1 and len(d) == 2 and all(len(v) == EMBED_DIMS for v in q + d)
    assert abs(sum(x * x for x in q[0]) - 1.0) < 1e-9
    (p1, h1, b1), (p2, _, b2) = fake_gemini.requests
    assert p1.endswith(f"/models/{EMBED_MODEL}:batchEmbedContents")
    assert h1.get("x-goog-api-key") == "test-key-not-real"
    # One request (one content, one part) per text: never N parts in one.
    assert len(b1["requests"]) == 1 and len(b2["requests"]) == 2
    assert all(len(r["content"]["parts"]) == 1 for r in b1["requests"] + b2["requests"])
    texts = [r["content"]["parts"][0]["text"] for r in b1["requests"] + b2["requests"]]
    assert texts == [
        "task: search result | query: Quán cà phê yên tĩnh",
        "title: none | text: Người dùng thích núi",
        "title: none | text: Người dùng thích biển",
    ]
    for r in b1["requests"] + b2["requests"]:
        assert (
            r.get("outputDimensionality") == EMBED_DIMS
            or r.get("embedContentConfig", {}).get("outputDimensionality") == EMBED_DIMS
        ), r
        flat = json.dumps(r)
        assert "taskType" not in flat and '"title"' not in flat


def test_embed_wrong_dimensionality_is_refused(fake_gemini):
    fake_gemini.dims = 3072
    from ai_infer.mem.bootstrap import NepEmbed

    ne = NepEmbed.__new__(NepEmbed)
    ne.backend = gemini.GeminiEmbed("k", fake_gemini.url, 5)
    with pytest.raises(gemini.EmbedError, match="3072"):
        ne.embed_batch(["x"], "add")


def test_llm_wire_contract_and_full_add(fake_gemini, fake_store):
    """The whole add through the real google-genai clients: JSON mode with our
    schema, the extraction instructions in the prompt, the model's own
    classification applied."""
    fake_gemini.llm_items = [
        own("Người dùng thích ngắm hoàng hôn"),
        {
            "text": "Mẹ người dùng thích chùa",
            "ve_ai": "nguoi_khac",
            "noi_dung": "so_thich_rang_buoc_di_choi",
            "loai": "thich_dia_diem",
        },
    ]
    svc = build_service(
        gemini.GeminiLlm("test-key-not-real", fake_gemini.url, 5),
        gemini.GeminiEmbed("test-key-not-real", fake_gemini.url, 5),
    )
    added, refused = svc.add("u1", ["mình mê hoàng hôn, mẹ mình thì thích chùa"])
    assert [a.text for a in added] == [
        "Người dùng thích ngắm hoàng hôn"
    ] and refused == 1
    gen = [b for p, _, b in fake_gemini.requests if p.endswith(":generateContent")]
    paths = [p for p, _, _ in fake_gemini.requests if p.endswith(":generateContent")]
    assert len(gen) == 1 and paths[0].endswith(f"/models/{LLM_MODEL}:generateContent")
    cfg = gen[0]["generationConfig"]
    assert cfg["responseMimeType"] == "application/json"
    assert cfg["responseJsonSchema"]["properties"]["memory"]["items"]["properties"][
        "ve_ai"
    ]["enum"] == [
        "ban_than",
        "nguoi_khac",
    ]
    user_text = gen[0]["contents"][0]["parts"][0]["text"]
    assert "ve_ai" in user_text and "mẹ mình thì thích chùa" in user_text
    assert "systemInstruction" in gen[0]


class RecordingEmbed:
    def __init__(self):
        from ai_infer.mem import gemini

        self.inner = gemini.StubEmbed()
        self.inputs: list[str] = []

    def embed(self, inputs, raw):
        self.inputs += list(inputs)
        return self.inner.embed(inputs, raw)


def test_embedding_is_the_owner_decision_and_search_has_no_hyde(fake_store):
    """Owner decision 2026-09-27: one embedding model, gemini-embedding-2 at
    1536 dims, Developer-API text prefixes, no TaskType; no hypothetical
    document generated for a query. The built mem0 config says so, stored
    memories go in as documents, and a search embeds exactly the query with
    the query prefix and makes no LLM call at all."""
    import pathlib

    from conftest import build_service
    from fakes import ScriptedLlm, own

    from ai_infer.mem import gemini

    llm = ScriptedLlm([own("Người dùng thích núi")])
    emb = RecordingEmbed()
    svc = build_service(llm, emb)
    cfg = svc.m.config.embedder.config
    assert cfg["model"] == "gemini-embedding-2" and cfg["embedding_dims"] == 1536
    vs = svc.m.config.vector_store.config
    assert vs.embedding_model_dims == 1536
    assert (gemini.DOC_PREFIX, gemini.QUERY_PREFIX) == (
        "title: none | text: ",
        "task: search result | query: ",
    )

    svc.add("u1", ["x"])
    assert "title: none | text: Người dùng thích núi" in emb.inputs
    llm_calls, emb.inputs = len(llm.calls), []
    svc.search("u1", "đi núi", 5, 0.0)
    svc.list("u1", 10)
    assert emb.inputs == ["task: search result | query: đi núi"]
    assert len(llm.calls) == llm_calls

    # No other embedding model or width anywhere in the sidecar's code.
    src = pathlib.Path(gemini.__file__).resolve().parents[1]
    for p in src.rglob("*.py"):
        text = p.read_text()
        assert "gemini-embedding-001" not in text and "768" not in text, p
