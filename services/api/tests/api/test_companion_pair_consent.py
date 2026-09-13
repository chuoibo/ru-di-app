"""Nếp và cuộc trò chuyện của hai người (khoản bổ sung ADR-0019, ADR-0027 §4).

Một nhóm sáu người và một cặp hai người là hai thứ khác nhau đối với cùng một
phép cộng. «Nhóm này thích gì» giấu được câu trả lời của từng người khi có sáu
người; với hai người, ai cũng trừ được câu của chính mình ra và cầm trong tay
câu của người kia. Nên gu của một cặp là «chưa biết» cho tới khi **cả hai** nói
Nếp được đọc họ, và Nếp không mở cuộc trò chuyện của một cặp trước khi hỏi.

Ca cuối cùng là ca hồi quy: đường của hội bạn không đổi một byte.
"""

from __future__ import annotations

from types import SimpleNamespace

import anyio
import pytest

from app.api.deps import get_companion, get_repository
from app.api.main import create_app
from app.api.service import ApiService
from app.places.taste import UNKNOWN

from .conftest import ASGITestClient
from .pair_helpers import (
    CAP,
    HOI,
    NGUOI_KIA,
    NGUOI_LA,
    T0,
    TOI,
    dong_thuan,
    head,
    lap_so,
    seed_pair,
)

CARD = {
    "kind": "suy_nghi",
    "body": "Nếp nghĩ hai bạn hợp quán này.",
    "place_ids": [],
}


class CountingCompanion:
    """Ghi lại mọi lời gọi, để «từ chối TRƯỚC mô hình» nhìn thấy được."""

    def __init__(self) -> None:
        self.calls: list[dict] = []

    def reply(self, **kwargs) -> dict:
        self.calls.append(kwargs)
        return CARD


@pytest.fixture
def companion():
    return CountingCompanion()


@pytest.fixture
def client(repository, monkeypatch, companion):
    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    monkeypatch.setattr("app.api.service._now", lambda: T0)
    app = create_app(auth_mode="dev")
    app.dependency_overrides[get_repository] = lambda: repository
    app.dependency_overrides[get_companion] = lambda: companion
    return ASGITestClient(app)


@pytest.fixture(autouse=True)
def seeded(repository, monkeypatch):
    monkeypatch.setattr("app.api.service._now", lambda: T0)
    # `FakeRepository` has no conversation reader -- the companion suite uses a
    # repository of its own for that. This file is about what happens BEFORE
    # the conversation is read, so a benign empty page is enough, and the case
    # that matters replaces it with one that refuses to be called at all.
    monkeypatch.setattr(
        repository,
        "list_messages",
        lambda *args, **kwargs: SimpleNamespace(messages=[], next_cursor=None),
        raising=False,
    )
    seed_pair(repository)
    # Hai người đã trả lời phần cá nhân hoá: có gì để cộng, nên «chưa biết»
    # dưới kia là một lời từ chối chứ không phải một bảng rỗng.
    repository.person_interests[TOI] = {"cafe", "outdoor"}
    repository.person_interests[NGUOI_KIA] = {"cafe", "an-uong"}
    repository.person_interests[NGUOI_LA] = {"cafe"}


def _luot_nep(client, context_id, actor):
    return client.post(
        f"/contexts/{context_id}/ai-turn",
        headers={**head(actor), "Content-Type": "application/json"},
    )


def test_nep_khong_mo_cuoc_tro_chuyen_cua_mot_cap_khi_chua_ai_dong_y(
    client, repository, companion, monkeypatch
):
    """403 với mã riêng, và trước khi một tin nhắn nào được đọc."""
    lap_so(client)

    def khong_duoc_doc(*args, **kwargs):
        raise AssertionError(
            "Nếp đã đọc tin nhắn trước khi hỏi — một lời từ chối viết sau khi "
            "tin nhắn đã nằm trong bộ nhớ là lời từ chối đã làm đúng cái nó từ chối"
        )

    monkeypatch.setattr(repository, "list_messages", khong_duoc_doc)
    refused = _luot_nep(client, CAP, TOI)
    assert refused.status_code == 403, refused.text
    assert refused.json()["code"] == "pair_chat_consent_required"
    assert companion.calls == [], "và cũng không tới mô hình"


def test_bac_hai_khong_keo_theo_bac_bon(client):
    """Lập sổ là lập sổ. Nó không cho Nếp đọc tin nhắn."""
    lap_so(client)
    assert _luot_nep(client, CAP, TOI).status_code == 403


def test_mot_nguoi_dong_y_van_chua_du(client):
    """Im lặng của người kia không phải một lời đồng ý."""
    lap_so(client)
    offered = client.post(
        f"/contexts/{CAP}/notebook/proposals",
        json={"purpose": "doc_chat"},
        headers=head(TOI),
    )
    assert offered.status_code == 201, offered.text
    assert _luot_nep(client, CAP, TOI).status_code == 403


def test_ca_hai_dong_y_thi_nep_noi_duoc(client):
    lap_so(client)
    dong_thuan(client, "doc_chat")
    answer = _luot_nep(client, CAP, TOI)
    assert answer.status_code != 403, answer.text
    assert answer.status_code == 200, answer.text


def test_thu_hoi_co_hieu_luc_ngay_lan_doc_sau(client):
    """Consent được hỏi lại ở MỌI biên đọc, không cache.

    Một lời thu hồi chỉ có tác dụng sau lần khởi động lại kế tiếp thì không
    phải là một lời thu hồi.
    """
    lap_so(client)
    dong_thuan(client, "doc_chat")
    assert _luot_nep(client, CAP, TOI).status_code == 200

    gone = client.delete(
        f"/contexts/{CAP}/notebook/consents/doc_chat", headers=head(NGUOI_KIA)
    )
    assert gone.status_code == 204, gone.text
    assert _luot_nep(client, CAP, TOI).status_code == 403


def test_gu_cua_mot_cap_la_chua_biet_cho_toi_khi_ca_hai_dong_y(client, repository):
    """Không phải một tổng một nửa — nửa tổng ở đây đúng là phép đọc bị cấm."""
    lap_so(client)
    service = ApiService(repository)
    assert service.group_taste(CAP) is UNKNOWN

    dong_thuan(client, "doc_chat")
    sau_khi = service.group_taste(CAP)
    assert sau_khi is not UNKNOWN
    assert sau_khi.people_answered == 2
    assert "cafe" in sau_khi.interests


def test_duong_cua_hoi_ban_khong_doi_mot_byte(client, repository, companion):
    """Ca hồi quy: cùng hai phép, trên một context `group`, không cửa nào mới.

    Cặp bị từ chối ở trên và hội bạn dưới đây có CÙNG một người trong đó, nên
    nếu khoản bổ sung rò ra ngoài phạm vi `pair` thì ca này đỏ.
    """
    truoc = ApiService(repository).group_taste(HOI)
    assert truoc is not UNKNOWN
    assert truoc.people_answered == 2

    answer = _luot_nep(client, HOI, TOI)
    assert answer.status_code == 200, answer.text

    # Và mở một cuốn sổ ở chỗ khác không đụng gì tới hội bạn.
    lap_so(client)
    sau = ApiService(repository).group_taste(HOI)
    assert sau == truoc, "gu của hội bạn phải giống hệt, từng trường"
    assert _luot_nep(client, HOI, TOI).status_code == 200
