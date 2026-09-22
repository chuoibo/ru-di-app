"""Quyền sở hữu job media nằm trong chính cái id, nên phải chứng minh được."""

from __future__ import annotations

import pytest

from app.api.nep_media import DAI_THE, job_id_moi, la_chu_job, the_nguoi

KHOA = "k" * 48
AI_DO = "a1b2c3d4-e5f6-4a0b-8c1d-2e3f4a5b6c7d"
AI_KHAC = "b2c3d4e5-f6a7-4b1c-9d2e-3f4a5b6c7d8e"


def test_cung_nguoi_cung_khoa_thi_cung_the() -> None:
    assert the_nguoi(AI_DO, KHOA) == the_nguoi(AI_DO, KHOA)
    assert len(the_nguoi(AI_DO, KHOA)) == DAI_THE


def test_nguoi_khac_the_khac_va_khoa_khac_the_khac() -> None:
    assert the_nguoi(AI_DO, KHOA) != the_nguoi(AI_KHAC, KHOA)
    assert the_nguoi(AI_DO, KHOA) != the_nguoi(AI_DO, "j" * 48)


def test_the_khong_lo_ra_person_id() -> None:
    the = the_nguoi(AI_DO, KHOA)
    assert AI_DO not in the and AI_DO.replace("-", "") not in the


def test_khoa_ngan_bi_tu_choi_vi_the_do_duoc_thi_khong_con_la_bang_chung() -> None:
    with pytest.raises(ValueError, match="32"):
        the_nguoi(AI_DO, "ngan")
    with pytest.raises(ValueError):
        the_nguoi("", KHOA)


def test_moi_job_mot_id_khac_nhau() -> None:
    assert job_id_moi(AI_DO, KHOA) != job_id_moi(AI_DO, KHOA)


def test_chu_job_doc_duoc_job_cua_minh() -> None:
    jid = job_id_moi(AI_DO, KHOA)
    assert la_chu_job(jid, AI_DO, KHOA) is True


def test_nguoi_khac_khong_doc_duoc_job_khong_phai_cua_minh() -> None:
    jid = job_id_moi(AI_DO, KHOA)
    assert la_chu_job(jid, AI_KHAC, KHOA) is False


def test_id_sai_dang_la_khong_phai_chu_chu_khong_phai_loi() -> None:
    for xau in [
        "",
        "x",
        None,
        123,
        "khong-co-gach",
        "../../etc/passwd",
        "z" * 16 + "-abcdefgh",
    ]:
        assert la_chu_job(xau, AI_DO, KHOA) is False, xau


def test_tu_che_id_bang_the_cua_nguoi_khac_can_biet_khoa() -> None:
    # Kẻ tấn công biết person_id của nạn nhân nhưng không biết khoá máy chủ.
    the_gia = the_nguoi(AI_KHAC, "khoa-doan-sai-nhung-du-dai-32-ky-tu-roi-nhe")
    assert la_chu_job(f"{the_gia}-aaaaaaaa", AI_KHAC, KHOA) is False


def test_doi_khoa_may_chu_thi_moi_job_cu_deu_thanh_khong_phai_cua_ai() -> None:
    jid = job_id_moi(AI_DO, KHOA)
    assert la_chu_job(jid, AI_DO, "m" * 48) is False
