"""The memory store on a real Milvus (opt-in: AI_INFER_TEST_MILVUS_URI and
AI_INFER_TEST_MILVUS_TOKEN; a failure rather than a skip when
AI_INFER_REQUIRE_MILVUS=1). The model back-ends are fakes: no Gemini call.

Each run creates its own database and drops it at the end, so the live
instance keeps no test memories.

Sentinel: test_live_memory_lifecycle."""

from __future__ import annotations

import os
import time
import uuid

import pytest
from fakes import ScriptedLlm, own

pytestmark = pytest.mark.milvus


@pytest.fixture
def live():
    from pymilvus import MilvusClient

    uri = os.environ["AI_INFER_TEST_MILVUS_URI"]
    token = os.environ.get("AI_INFER_TEST_MILVUS_TOKEN", "")
    db = "nep_memory_test_" + uuid.uuid4().hex[:10]
    admin = MilvusClient(uri=uri, token=token)
    admin.create_database(db)
    try:
        yield uri, token, db
    finally:
        c = MilvusClient(uri=uri, token=token, db_name=db)
        for name in c.list_collections():
            c.drop_collection(name)
        admin.drop_database(db)


def build(live, llm):
    from ai_infer.mem import bootstrap, gemini
    from ai_infer.mem.service import MemoryService

    uri, token, db = live
    return MemoryService(
        bootstrap.build(
            llm=llm,
            embed=gemini.StubEmbed(),
            milvus_uri=uri,
            milvus_token=token,
            db_name=db,
            collection="memories_v1",
        )
    )


def eventually(fn, ok, timeout=5.0):
    """Vector search can trail a fresh insert by ~200 ms even at Strong
    (retrieval-core finding); counts and queries do not."""
    end = time.monotonic() + timeout
    while True:
        got = fn()
        if ok(got) or time.monotonic() > end:
            return got
        time.sleep(0.1)


def test_live_memory_lifecycle(live):
    llm = ScriptedLlm(
        [
            own("Người dùng thích quán cà phê có sân vườn"),
            own("Người dùng hay đi vào tối thứ Sáu", "khung_gio_hay_di"),
        ],
        [own("Người dùng thích quán cà phê trên sân thượng")],
        [own("Người dùng thích đi bộ ở phố cổ")],
    )
    svc = build(live, llm)
    t0 = time.perf_counter()
    a_added, _ = svc.add("alice", ["x"])
    add_ms = (time.perf_counter() - t0) * 1000
    b_added, _ = svc.add("bob", ["y"])
    svc.add("alice", ["z"])
    assert svc.store.count_owner("alice") == 3 and svc.store.count_owner("bob") == 1

    # Isolation: search and list only ever return the caller's rows.
    def alice_hits():
        return svc.search("alice", "quán cà phê", 10, 0.0)

    # Alice's café memory is found; bob's café memory, the closest other
    # text in the collection, never is.
    cafe = "Người dùng thích quán cà phê có sân vườn"
    t0 = time.perf_counter()
    hits = eventually(alice_hits, lambda h: cafe in {r["memory"] for r in h})
    search_ms = (time.perf_counter() - t0) * 1000
    assert {r["user_id"] for r in hits} == {"alice"}
    assert cafe in {r["memory"] for r in hits}
    assert "Người dùng thích quán cà phê trên sân thượng" not in {
        r["memory"] for r in hits
    }
    bob = [r["memory"] for r in svc.search("bob", "quán cà phê", 10, 0.0)]
    assert bob == ["Người dùng thích quán cà phê trên sân thượng"]
    assert {r["user_id"] for r in svc.list("alice", 100)} == {"alice"}
    # The kind recorded with the memory is the model's.
    kinds = {
        r["memory"]: (r.get("metadata") or {}).get("loai")
        for r in svc.list("alice", 100)
    }
    assert kinds["Người dùng hay đi vào tối thứ Sáu"] == "khung_gio_hay_di"

    # Cross-owner delete by id is refused; bob's row stays.
    from ai_infer.mem.service import NotFound

    with pytest.raises(NotFound):
        svc.delete("alice", b_added[0].id)
    assert svc.store.count_owner("bob") == 1

    # One delete, then delete_all, each followed by a Strong count of 0.
    assert svc.delete("alice", a_added[0].id) == 1
    assert svc.store.count_ids([a_added[0].id]) == 0
    assert svc.delete_all("alice") == (2, 0)
    assert svc.store.count_owner("alice") == 0 and svc.store.count_owner("bob") == 1

    # Purge: delete by owner, flush, L0 then mix compaction, count 0.
    deleted, remaining, seconds = svc.purge_user("bob")
    assert (deleted, remaining) == (1, 0) and seconds > 0
    assert svc.store.count_owner("bob") == 0
    print(
        f"SO-DO add_ms={add_ms:.0f} first_search_ms={search_ms:.0f} purge_compaction_s={seconds:.2f}"
    )

    # No side collection: no entity store, no mem0migrations (telemetry off).
    from pymilvus import MilvusClient

    uri, token, db = live
    assert MilvusClient(uri=uri, token=token, db_name=db).list_collections() == [
        "memories_v1"
    ]


def test_live_owner_value_is_a_parameter_not_code(live):
    """An owner id shaped like an expression matches nothing but itself."""
    svc = build(
        live,
        ScriptedLlm(
            [own("Người dùng thích hồ Tây")], [own("Người dùng thích hồ Gươm")]
        ),
    )
    svc.add("alice", ["x"])
    evil = 'x"||owner!="'
    svc.add(evil, ["y"])
    assert svc.store.count_owner(evil) == 1
    assert [r["user_id"] for r in svc.list(evil, 100)] == [evil]
    assert svc.store.count_owner("alice") == 1


def test_live_analyzer_folds_vietnamese(live):
    from pymilvus import MilvusClient

    from ai_infer.mem.store import ANALYZER

    uri, token, _ = live
    c = MilvusClient(uri=uri, token=token)
    a = c.run_analyzer(["Quán Cà Phê ở Đà Lạt"], analyzer_params=ANALYZER)
    b = c.run_analyzer(["quan ca phe o da lat"], analyzer_params=ANALYZER)
    tok = lambda r: list(r[0].tokens) if hasattr(r[0], "tokens") else list(r[0])  # noqa: E731
    assert tok(a) == tok(b) == ["quan", "ca", "phe", "o", "da", "lat"]
