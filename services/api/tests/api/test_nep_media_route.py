"""Route media của Nếp: xác thực, sở hữu, và không bao giờ lộ token của proxy."""

from __future__ import annotations

import pytest

from app.api import nep_media
from app.api.routes import nep as nep_route

KHOA = "k" * 48
TOI = "a1b2c3d4-e5f6-4a0b-8c1d-2e3f4a5b6c7d"
NGUOI_KHAC = "b2c3d4e5-f6a7-4b1c-9d2e-3f4a5b6c7d8e"


class TraLoiGia:
    def __init__(self, ma=200, than=None, noi_dung=b"", kieu="application/json"):
        self.status_code = ma
        self._than = than if than is not None else {}
        self.content = noi_dung
        self.headers = {"content-type": kieu}

    def json(self):
        return self._than


@pytest.fixture(autouse=True)
def _cau_hinh(monkeypatch):
    monkeypatch.setenv("NEP_PROXY_URL", "http://proxy.invalid")
    monkeypatch.setenv("NEP_PROXY_TOKEN", "token-proxy-bi-mat")
    monkeypatch.setenv("MOBILE_PERSON_ID_KEY", KHOA)


def _bat_goi(monkeypatch, tra=None):
    """Chặn lời gọi ra proxy và ghi lại nó."""
    daGoi = []

    def gia(method, duong, **kw):
        daGoi.append((method, duong, kw))
        return tra or TraLoiGia(200, {"trang_thai": "dang-cho"})

    monkeypatch.setattr(nep_route, "_goi_proxy", gia)
    return daGoi


def _headers(person_id=TOI):
    return {"X-Actor-ID": person_id, "X-Actor-Roles": "member"}


def test_xin_anh_tra_ve_job_id_mang_the_cua_chinh_minh(client, monkeypatch) -> None:
    _bat_goi(monkeypatch, TraLoiGia(202, {"job_id": "x"}))
    r = client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu tím"}, headers=_headers()
    )
    assert r.status_code == 202
    jid = r.json()["job_id"]
    assert nep_media.la_chu_job(jid, TOI, KHOA)
    assert not nep_media.la_chu_job(jid, NGUOI_KHAC, KHOA)


def test_tenant_gui_sang_proxy_la_the_mo_chu_khong_phai_person_id(
    client, monkeypatch
) -> None:
    daGoi = _bat_goi(monkeypatch, TraLoiGia(202, {}))
    client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu"}, headers=_headers()
    )
    _, _, kw = daGoi[0]
    goi = kw["json"]
    assert goi["tenant"] == nep_media.the_nguoi(TOI, KHOA)
    # Proxy phục vụ nhiều thứ khác trong nhà; nó không cần biết người dùng là ai.
    assert TOI not in str(goi)


def test_loai_la_bi_tu_choi(client, monkeypatch) -> None:
    _bat_goi(monkeypatch)
    r = client.post("/me/nep/media", json={"loai": "nhac"}, headers=_headers())
    assert r.status_code == 422


def test_429_va_422_cua_proxy_di_nguyen_ve_nguoi_dung(client, monkeypatch) -> None:
    _bat_goi(
        monkeypatch,
        TraLoiGia(429, {"detail": "quá hạn mức anh: đã dùng 20/20 hôm nay"}),
    )
    r = client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu"}, headers=_headers()
    )
    assert r.status_code == 429 and "hạn mức" in r.json()["detail"]


def test_su_co_giua_hai_may_thanh_502_chu_khong_do_len_nguoi_dung(
    client, monkeypatch
) -> None:
    _bat_goi(monkeypatch, TraLoiGia(500, {"detail": "sập"}))
    r = client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu"}, headers=_headers()
    )
    assert r.status_code == 502


def test_khong_doc_duoc_job_cua_nguoi_khac_va_tra_404_chu_khong_403(
    client, monkeypatch
) -> None:
    daGoi = _bat_goi(monkeypatch)
    cua_nguoi_khac = nep_media.job_id_moi(NGUOI_KHAC, KHOA)
    r = client.get(f"/me/nep/media/{cua_nguoi_khac}", headers=_headers())
    # 403 là một câu trả lời: nó xác nhận job đó tồn tại. 404 thì không nói gì.
    assert r.status_code == 404
    assert daGoi == [], "không được hỏi proxy về job không phải của mình"


def test_id_bia_khong_qua_duoc_cong_so_huu(client, monkeypatch) -> None:
    daGoi = _bat_goi(monkeypatch)
    for xau in ["abc", "../../etc/passwd", "z" * 16 + "-aaaaaaaa"]:
        assert client.get(f"/me/nep/media/{xau}", headers=_headers()).status_code == 404
    assert daGoi == []


def test_video_chi_ghep_duoc_tu_anh_cua_chinh_minh(client, monkeypatch) -> None:
    _bat_goi(monkeypatch, TraLoiGia(202, {}))
    cua_toi = nep_media.job_id_moi(TOI, KHOA)
    cua_khac = nep_media.job_id_moi(NGUOI_KHAC, KHOA)
    ok = client.post(
        "/me/nep/media",
        json={"loai": "video", "anh_job_ids": [cua_toi]},
        headers=_headers(),
    )
    assert ok.status_code == 202
    trom = client.post(
        "/me/nep/media",
        json={"loai": "video", "anh_job_ids": [cua_toi, cua_khac]},
        headers=_headers(),
    )
    assert trom.status_code == 404


def test_thieu_cau_hinh_thi_503_chu_khong_500(client, monkeypatch) -> None:
    monkeypatch.delenv("NEP_PROXY_URL", raising=False)
    r = client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu"}, headers=_headers()
    )
    assert r.status_code == 503 and r.json()["detail"] == "nep_media_chua_cau_hinh"


def test_token_proxy_khong_bao_gio_ra_khoi_may_chu(client, monkeypatch) -> None:
    _bat_goi(monkeypatch, TraLoiGia(202, {}))
    r = client.post(
        "/me/nep/media", json={"loai": "anh", "mo_ta": "gấu"}, headers=_headers()
    )
    assert "token-proxy-bi-mat" not in r.text
    assert "token-proxy-bi-mat" not in str(dict(r.headers))


def test_ve_tu_man_tien_bi_tu_choi_truoc_khi_goi_proxy(client, monkeypatch) -> None:
    # ADR-0036 §2.9: the silence on money screens covers drawing too, and it
    # must cost nothing -- no proxy call, no quota seat.
    daGoi = _bat_goi(monkeypatch, TraLoiGia(202, {}))
    for man in ("finance", "/settlements/7", "batches/x", "smart-split/moi/review"):
        r = client.post(
            "/me/nep/media",
            json={"loai": "anh", "mo_ta": "gấu", "man": man},
            headers=_headers(),
        )
        assert r.status_code == 403, man
        assert r.json()["detail"] == "nep_lui_man_tien"
    assert daGoi == []


def test_man_tien_la_ca_doan_dau_khong_phai_tien_to(client, monkeypatch) -> None:
    daGoi = _bat_goi(monkeypatch, TraLoiGia(202, {}))
    for man in ("financial-report", "explore", "/outings/7"):
        r = client.post(
            "/me/nep/media",
            json={"loai": "anh", "mo_ta": "gấu", "man": man},
            headers=_headers(),
        )
        assert r.status_code == 202, man
    assert len(daGoi) == 3
    # The screen is a check, never a field forwarded to the proxy.
    assert all("man" not in kw["json"] for _, _, kw in daGoi)


def test_danh_sach_man_tien_khop_phieu_ts() -> None:
    import pathlib
    import re

    goc = pathlib.Path(__file__).resolve().parents[4]
    ts = (goc / "apps/mobile/src/rudi/nep/phieu.ts").read_text(encoding="utf-8")
    khop = re.search(r"export const MAN_NEP_LUI = \[([^\]]*)\]", ts)
    assert khop, "phieu.ts không còn MAN_NEP_LUI"
    tu_ts = tuple(re.findall(r'"([^"]+)"', khop.group(1)))
    assert tu_ts == nep_route.MAN_NEP_LUI
