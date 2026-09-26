"""Gu của hai người (khoản bổ sung ADR-0019, ADR-0027 §4).

Một nhóm sáu người và một cặp hai người là hai thứ khác nhau đối với cùng một
phép cộng. «Nhóm này thích gì» giấu được câu trả lời của từng người khi có sáu
người; với hai người, ai cũng trừ được câu của chính mình ra và cầm trong tay
câu của người kia. Nên gu của một cặp là «chưa biết» cho tới khi **cả hai** nói
Nếp được đọc họ.

Các ca «Nếp không mở cuộc trò chuyện của một cặp» đi cùng lượt companion tự
động, đã bị xoá theo ADR-0036 §2.1: AI chỉ chạy khi được gọi tường minh, và
máy chủ không đọc bảng tin nhắn để lấy ngữ cảnh.

Ca cuối cùng là ca hồi quy: đường của hội bạn không đổi một byte.
"""

from __future__ import annotations

import anyio
import pytest

from app.api.deps import get_repository
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


@pytest.fixture
def client(repository, monkeypatch):
    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    monkeypatch.setattr("app.api.service._now", lambda: T0)
    app = create_app(auth_mode="dev")
    app.dependency_overrides[get_repository] = lambda: repository
    return ASGITestClient(app)


@pytest.fixture(autouse=True)
def seeded(repository, monkeypatch):
    monkeypatch.setattr("app.api.service._now", lambda: T0)
    seed_pair(repository)
    # Hai người đã trả lời phần cá nhân hoá: có gì để cộng, nên «chưa biết»
    # dưới kia là một lời từ chối chứ không phải một bảng rỗng.
    repository.person_interests[TOI] = {"cafe", "outdoor"}
    repository.person_interests[NGUOI_KIA] = {"cafe", "an-uong"}
    repository.person_interests[NGUOI_LA] = {"cafe"}


def test_mot_nguoi_dong_y_van_chua_du(client, repository):
    """Im lặng của người kia không phải một lời đồng ý."""
    lap_so(client)
    offered = client.post(
        f"/contexts/{CAP}/notebook/proposals",
        json={"purpose": "doc_chat"},
        headers=head(TOI),
    )
    assert offered.status_code == 201, offered.text
    assert ApiService(repository).group_taste(CAP) is UNKNOWN


def test_thu_hoi_co_hieu_luc_ngay_lan_doc_sau(client, repository):
    """Consent được hỏi lại ở MỌI biên đọc, không cache.

    Một lời thu hồi chỉ có tác dụng sau lần khởi động lại kế tiếp thì không
    phải là một lời thu hồi.
    """
    lap_so(client)
    dong_thuan(client, "doc_chat")
    service = ApiService(repository)
    assert service.group_taste(CAP) is not UNKNOWN

    gone = client.delete(
        f"/contexts/{CAP}/notebook/consents/doc_chat", headers=head(NGUOI_KIA)
    )
    assert gone.status_code == 204, gone.text
    assert service.group_taste(CAP) is UNKNOWN


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


def test_duong_cua_hoi_ban_khong_doi_mot_byte(client, repository):
    """Ca hồi quy: cùng hai phép, trên một context `group`, không cửa nào mới.

    Cặp bị từ chối ở trên và hội bạn dưới đây có CÙNG một người trong đó, nên
    nếu khoản bổ sung rò ra ngoài phạm vi `pair` thì ca này đỏ.
    """
    truoc = ApiService(repository).group_taste(HOI)
    assert truoc is not UNKNOWN
    assert truoc.people_answered == 2

    # Và mở một cuốn sổ ở chỗ khác không đụng gì tới hội bạn.
    lap_so(client)
    sau = ApiService(repository).group_taste(HOI)
    assert sau == truoc, "gu của hội bạn phải giống hệt, từng trường"
