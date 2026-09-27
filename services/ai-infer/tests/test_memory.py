"""/v1/memory/*: mem0 as a library over pure fakes (Milvus client, model).

Sentinels: test_cross_user_isolation, test_purge_leaves_zero_rows,
test_delete_all_leaves_zero_rows (scripts/ai_infer_tier.sh)."""

from __future__ import annotations

import uuid

import pytest
from conftest import build_service
from fakes import ScriptedLlm, own
from fastapi.testclient import TestClient

from ai_infer.app import create_app
from ai_infer.mem.store import OWNER_FILTER, NepMilvus, OwnerError


def app_with(settings, svc):
    return TestClient(create_app(settings, memory=svc))


def seed(c, auth, user, *texts):
    for t in texts:
        r = c.post(
            "/v1/memory/add",
            json={"user_id": user, "messages": [{"role": "user", "content": t}]},
            headers=auth,
        )
        assert r.status_code == 200, r.text


def test_add_keeps_only_what_the_model_classified_as_own(settings, auth, fake_store):
    llm = ScriptedLlm(
        [
            own("Người dùng thích quán cà phê yên tĩnh"),
            {
                "text": "Bạn của người dùng ghét phở",
                "ve_ai": "nguoi_khac",
                "noi_dung": "so_thich_rang_buoc_di_choi",
                "loai": "thich_danh_muc",
            },
            {
                "text": "Người dùng có ngân sách 200k",
                "ve_ai": "ban_than",
                "noi_dung": "tien_bac",
                "loai": "thich_danh_muc",
            },
            {
                "text": "Người dùng bị hen suyễn",
                "ve_ai": "ban_than",
                "noi_dung": "dac_diem_nhay_cam",
                "loai": "ne_dia_diem",
            },
            {
                "text": "Người dùng thích đi xe máy",
                "ve_ai": "ban_than",
                "noi_dung": "so_thich_rang_buoc_di_choi",
                "loai": "khong_co_trong_danh_sach",
            },
        ]
    )
    c = app_with(settings, build_service(llm))
    r = c.post(
        "/v1/memory/add",
        json={
            "user_id": "u1",
            "messages": [{"role": "user", "content": "một lời nói bất kỳ"}],
        },
        headers=auth,
    )
    assert r.status_code == 200, r.text
    body = r.json()
    assert [a["text"] for a in body["added"]] == [
        "Người dùng thích quán cà phê yên tĩnh"
    ]
    assert body["added"][0]["loai"] == "thich_danh_muc"
    assert body["refused"] == 4
    # The model was asked for its classification: the schema requires it.
    _, _, schema = llm.calls[0]
    assert set(schema["properties"]["memory"]["items"]["required"]) >= {
        "ve_ai",
        "noi_dung",
        "loai",
    }
    rows = list(fake_store.instances[0].collections["memories_v1"].values())
    assert (
        len(rows) == 1
        and rows[0]["owner"] == "u1"
        and rows[0]["metadata"]["loai"] == "thich_danh_muc"
    )


def test_only_the_persons_own_messages_are_accepted(settings, auth, fake_store):
    c = app_with(settings, build_service(ScriptedLlm()))
    r = c.post(
        "/v1/memory/add",
        json={
            "user_id": "u1",
            "messages": [{"role": "assistant", "content": "Nếp gợi ý quán X"}],
        },
        headers=auth,
    )
    assert r.status_code == 422


def test_cross_user_isolation(settings, auth, fake_store):
    llm = ScriptedLlm(
        [own("Người dùng thích trà sữa ít đường")],
        [own("Người dùng thích trà đá vỉa hè")],
    )
    svc = build_service(llm)
    c = app_with(settings, svc)
    seed(c, auth, "alice", "a")
    seed(c, auth, "bob", "b")
    for user, mine in (("alice", "trà sữa"), ("bob", "trà đá")):
        s = c.post(
            "/v1/memory/search",
            json={"user_id": user, "query": "trà", "threshold": 0.0},
            headers=auth,
        ).json()
        lst = c.post("/v1/memory/list", json={"user_id": user}, headers=auth).json()
        assert s["items"] and all(mine in i["text"] for i in s["items"]), s
        assert lst["count"] == 1 and mine in lst["items"][0]["text"]
    # alice cannot delete bob's memory by id: same 404 as a missing id.
    bob_id = c.post("/v1/memory/list", json={"user_id": "bob"}, headers=auth).json()[
        "items"
    ][0]["id"]
    r = c.post(
        "/v1/memory/delete",
        json={"user_id": "alice", "memory_id": bob_id},
        headers=auth,
    )
    missing = c.post(
        "/v1/memory/delete",
        json={"user_id": "alice", "memory_id": str(uuid.uuid4())},
        headers=auth,
    )
    assert r.status_code == missing.status_code == 404 and r.json() == missing.json()
    assert (
        c.post("/v1/memory/list", json={"user_id": "bob"}, headers=auth).json()["count"]
        == 1
    )
    # Every filter the store sent was the owner template with the value as a
    # parameter: no owner id was ever spliced into an expression.
    fake = fake_store.instances[0]
    assert fake.filters_seen
    for flt, _ in fake.filters_seen:
        assert "alice" not in flt and "bob" not in flt


def test_store_refuses_filters_without_exactly_one_owner(fake_store):
    s = NepMilvus("fake://", "", "memories_v1", 1536, "COSINE", "nep_memory")
    for bad in (
        None,
        {},
        {"agent_id": "a"},
        {"user_id": "a", "run_id": "r"},
        {"user_id": ""},
        {"user_id": "a b"},
        {"user_id": "x\n"},
    ):
        with pytest.raises(OwnerError):
            s.list(filters=bad)
    with pytest.raises(PermissionError):
        s.reset()
    with pytest.raises(PermissionError):
        s.delete_col()


def test_owner_leak_fails_closed(settings, auth, fake_store):
    svc = build_service(ScriptedLlm([own("Người dùng thích ăn cay")]))
    c = app_with(settings, svc)
    seed(c, auth, "alice", "x")
    # Corrupt the store so that bob's filter matches alice's row.
    fake = fake_store.instances[0]
    orig = fake._match

    def leaky(flt, params):
        if flt == OWNER_FILTER:
            return lambda r: True
        return orig(flt, params)

    fake._match = leaky
    r = c.post("/v1/memory/list", json={"user_id": "bob"}, headers=auth)
    assert r.status_code == 500 and "Người dùng" not in r.text


def test_delete_one_counts_zero(settings, auth, fake_store):
    svc = build_service(ScriptedLlm([own("Người dùng thích đi bộ buổi sáng")]))
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    mid = c.post("/v1/memory/list", json={"user_id": "u1"}, headers=auth).json()[
        "items"
    ][0]["id"]
    r = c.post(
        "/v1/memory/delete", json={"user_id": "u1", "memory_id": mid}, headers=auth
    )
    assert r.status_code == 200 and r.json() == {"deleted": 1, "remaining": 0}
    assert svc.store.count_ids([mid]) == 0


def test_delete_all_leaves_zero_rows(settings, auth, fake_store):
    llm = ScriptedLlm(
        [own("Người dùng thích núi"), own("Người dùng thích biển")],
        [own("Người dùng thích sông")],
    )
    svc = build_service(llm)
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    seed(c, auth, "u2", "y")
    r = c.post("/v1/memory/delete_all", json={"user_id": "u1"}, headers=auth)
    assert r.status_code == 200 and r.json() == {"deleted": 2, "remaining": 0}
    assert svc.store.count_owner("u1") == 0
    assert svc.store.count_owner("u2") == 1


def test_delete_all_does_not_trust_mem0(settings, auth, fake_store, monkeypatch):
    """mem0's delete_all can stop early and still report success: the count
    decides, and rows it left are deleted by owner."""
    svc = build_service(ScriptedLlm([own("Người dùng thích phố cổ")]))
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    monkeypatch.setattr(
        svc.m, "delete_all", lambda **kw: {"message": "Memories deleted successfully!"}
    )
    r = c.post("/v1/memory/delete_all", json={"user_id": "u1"}, headers=auth)
    assert r.json() == {"deleted": 1, "remaining": 0}
    assert svc.store.count_owner("u1") == 0


def test_delete_all_repeats_until_zero(settings, auth, fake_store, monkeypatch):
    """mem0 deleting one row per call (stopping early) is called again
    after each non-zero count, until the count reads zero; the owner-filter
    fallback is not needed while each round makes progress."""
    llm = ScriptedLlm(
        [
            own("Người dùng thích núi"),
            own("Người dùng thích biển"),
            own("Người dùng thích sông"),
        ]
    )
    svc = build_service(llm)
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    calls = []

    def one_per_call(**kw):
        calls.append(kw)
        first = svc.store.list({"user_id": kw["user_id"]}, 1)[0][0]
        svc.store.delete(first.id)
        return {"message": "Memories deleted successfully!"}

    monkeypatch.setattr(svc.m, "delete_all", one_per_call)
    monkeypatch.setattr(
        svc.store,
        "delete_owner",
        lambda owner: pytest.fail("fallback used while mem0 made progress"),
    )
    r = c.post("/v1/memory/delete_all", json={"user_id": "u1"}, headers=auth)
    assert r.status_code == 200 and r.json() == {"deleted": 3, "remaining": 0}
    assert len(calls) == 3 and svc.store.count_owner("u1") == 0


def test_a_delete_the_store_ignored_is_an_error(settings, auth, fake_store):
    svc = build_service(ScriptedLlm([own("Người dùng thích chợ đêm")]))
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    fake_store.instances[0].ignore_deletes = True
    for path in ("/v1/memory/delete_all", "/v1/memory/purge_user"):
        r = c.post(path, json={"user_id": "u1"}, headers=auth)
        assert r.status_code == 500 and r.json()["remaining"] == 1


def test_purge_leaves_zero_rows(settings, auth, fake_store):
    llm = ScriptedLlm(
        [
            own("Người dùng thích cà phê muối"),
            own("Người dùng hay đi sau 8 giờ tối", "khung_gio_hay_di"),
        ]
    )
    svc = build_service(llm)
    c = app_with(settings, svc)
    seed(c, auth, "u1", "x")
    r = c.post("/v1/memory/purge_user", json={"user_id": "u1"}, headers=auth)
    assert r.status_code == 200, r.text
    assert r.json()["deleted"] == 2 and r.json()["remaining"] == 0
    assert svc.store.count_owner("u1") == 0
    # Flush, then L0, then mix: a mix compaction alone removes nothing.
    ops = [x for x in fake_store.instances[0].calls if x[0] in ("flush", "compact")]
    assert ops == [
        ("flush", "memories_v1"),
        ("compact", "memories_v1", "l0"),
        ("compact", "memories_v1", "mix"),
    ]


def test_memory_off_answers_503(settings, auth):
    c = TestClient(create_app(settings, memory=None))
    assert (
        c.post("/v1/memory/list", json={"user_id": "u1"}, headers=auth).status_code
        == 503
    )


def test_memory_needs_the_token(settings, fake_store):
    c = app_with(settings, build_service(ScriptedLlm()))
    assert c.post("/v1/memory/list", json={"user_id": "u1"}).status_code == 401


def test_collection_is_strong_and_named_as_configured(fake_store):
    build_service(ScriptedLlm())
    fake = fake_store.instances[0]
    assert ("create_collection", "memories_v1", "Strong") in fake.calls
    assert fake.db_name == "nep_memory"
