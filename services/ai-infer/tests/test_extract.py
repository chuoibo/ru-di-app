"""The structural check on the model's answer: it keeps what the model
classified as keepable and nothing it did not, and an answer outside the
schema keeps nothing."""

from __future__ import annotations

import json

from fakes import own

from ai_infer.mem import extract


def run(raw: str) -> tuple[list[dict], int]:
    extract.begin()
    return json.loads(extract.filter_answer(raw))["memory"], extract.refused()


def test_keeps_only_own_outing_preferences_of_a_closed_kind():
    items = [
        own("Người dùng thích bảo tàng", "thich_danh_muc"),
        {**own("x"), "ve_ai": "nguoi_khac"},
        {**own("y"), "noi_dung": "tien_bac"},
        {**own("z"), "noi_dung": "dac_diem_nhay_cam"},
        {**own("w"), "noi_dung": "khong_lien_quan"},
        {**own("v"), "loai": "so_thich_bi_mat"},
        {**own("u"), "ve_ai": None},
        {"text": "thiếu phân loại"},
        {**own("a" * 161)},
        {**own("   ")},
        "not an object",
    ]
    kept, refused = run(json.dumps({"memory": items}))
    assert [k["text"] for k in kept] == ["Người dùng thích bảo tàng"]
    assert refused == len(items) - 1
    assert extract.kept_kind("Người dùng thích bảo tàng") == "thich_danh_muc"


def test_answer_outside_the_schema_keeps_nothing():
    for raw in ("", "not json", "[]", '{"memories": []}', '{"memory": {"text": "x"}}'):
        kept, _ = run(raw)
        assert kept == []


def test_schema_and_closed_kinds_match_the_go_contract():
    # services/core/internal/aiharness/trinho: LoaiSuThats, MaxNoiDung = 160.
    assert extract.LOAI == (
        "thich_danh_muc",
        "thich_dia_diem",
        "ne_dia_diem",
        "diem_den_quen",
        "khung_gio_hay_di",
        "phuong_tien",
        "thoi_luong_chang",
        "nhip_len_keo",
        "dieu_da_dan",
    )
    assert extract.MAX_TEXT == 160
    props = extract.SCHEMA["properties"]["memory"]["items"]["properties"]
    assert props["loai"]["enum"] == list(extract.LOAI)
