"""Offline gates for the group-answer quality harness. No model is called.

The harness is only worth running if three things hold, and each can be checked
without a key:

* the corpus is what it says it is (handwritten, synthetic, every hard kind of
  case present, every grading rule pointing at something that exists);
* the payload it sends is the payload the Go worker sends (one golden, read by
  both languages);
* the grader can fail. A grader that passes everything reads exactly like a
  model that is always right, so each machine check is shown a wrong answer
  and must say so.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from app.api.companion_places import CLIENT_PLACE_FIELDS
from tests.skills.tra_loi_trong_nhom import (
    AI_LA,
    GOLDEN_PATH,
    KHONG_TEN,
    TOI_LA,
    bundle_for,
    grade,
    hoi_thoai,
    load_corpus,
    main,
    payload_for,
    roster,
)

CORPUS = load_corpus()
CASES = {case["case_id"]: case for case in CORPUS["cases"]}
SYNTHETIC_NAMES = {
    "Nam", "Linh", "Huy", "Trang", "Minh", "Vy", "Phúc", "Thảo",
    "Tuấn", "Mai", "Long", "Hà",
}  # fmt: skip
GO_CATALOGUE = Path(GOLDEN_PATH).parents[2] / "service" / "catalogue.go"


def _failed(case_id: str, card: dict) -> set[str]:
    return {c.ten for c in grade(CORPUS, CASES[case_id], card) if c.ket_qua == "truot"}


# --- the corpus is what it claims ----------------------------------------


def test_corpus_is_twelve_to_sixteen_cases_covering_every_hard_kind():
    assert 12 <= len(CORPUS["cases"]) <= 16
    assert len(CASES) == len(CORPUS["cases"]), "case_id trùng"
    kinds = {kind for case in CORPUS["cases"] for kind in case["nhom"]}
    assert {"rang-buoc-o-tin-cu", "mau-thuan", "chen-lenh", "mo-ho"} <= kinds


@pytest.mark.parametrize("case", CORPUS["cases"], ids=lambda c: c["case_id"])
def test_every_case_is_self_consistent(case: dict):
    members = case["members"]
    assert case["caller"] in members
    assert len(set(members)) == len(members)
    assert set(members) <= SYNTHETIC_NAMES, "tên người phải là tên giả trong danh sách"
    ids = [m["id"] for m in case["messages"]]
    assert len(set(ids)) == len(ids)
    assert all(m["author"] in members for m in case["messages"])
    assert case["prompt"].strip()
    expected = case["expected"]
    assert expected["phai_ton_trong"] and expected["khong_duoc"]
    rules = expected["may_cham"]
    assert rules, "ca không có phép máy chấm nào thì chỉ còn người chấm"
    tags = {tag for place in CORPUS["catalogue"] for tag in place["nhan"]}
    for key in ("cam_nhan", "phai_co_nhan_mot"):
        assert set(rules.get(key, [])) <= tags, f"{key} trỏ vào nhãn không tồn tại"
    districts = {place["khu"] for place in CORPUS["catalogue"]}
    assert set(rules.get("khu_hop_le", [])) <= districts
    known = {place["id"] for place in CORPUS["catalogue"]}
    assert set(rules.get("phai_co_id", [])) <= known


def test_catalogue_ids_are_unique_and_opening_hours_parse():
    ids = [place["id"] for place in CORPUS["catalogue"]]
    assert len(set(ids)) == len(ids)
    for place in CORPUS["catalogue"]:
        assert re.fullmatch(r"\d{2}:\d{2} – \d{2}:\d{2}", place["open_hours"]), place[
            "id"
        ]


def test_the_injection_case_plants_what_it_forbids():
    """A forbidden word the chat never mentions proves nothing when it is absent."""

    case = CASES["05-chen-lenh-bo-qua-huong-dan"]
    chat = " ".join(m["text"] for m in case["messages"])
    for term in case["expected"]["may_cham"]["khong_duoc_nhac"]:
        assert term in chat


# --- the payload is the worker's payload ---------------------------------


def _golden() -> list[dict]:
    return json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))["cases"]


@pytest.mark.parametrize("golden", _golden(), ids=lambda g: g["ten"])
def test_conversation_matches_the_go_worker_golden_byte_for_byte(golden: dict):
    """The same file `hoi_thoai_golden_test.go` holds `hoiThoai` to."""

    got = hoi_thoai(golden["goi"], golden["prompt"])
    assert json.dumps(got, ensure_ascii=False) == json.dumps(
        golden["conversation"], ensure_ascii=False
    )


def test_speaker_labels_are_the_go_labels():
    source = (GOLDEN_PATH.parents[1] / "boicanh.go").read_text(encoding="utf-8")
    for label in (TOI_LA, AI_LA, KHONG_TEN):
        assert f'"{label}"' in source


def test_model_sees_exactly_the_fields_go_client_places_sends():
    """`service.ClientPlaces` decides the fields in production; this harness must not see more."""

    source = GO_CATALOGUE.read_text(encoding="utf-8")
    match = re.search(
        r"func ClientPlaces\(.*?fields := \[\]string\{([^}]*)\}", source, re.S
    )
    assert match, "không tìm thấy danh sách trường trong ClientPlaces"
    go_fields = tuple(re.findall(r'"([^"]+)"', match.group(1)))
    assert go_fields == CLIENT_PLACE_FIELDS
    payload = payload_for(CORPUS, CASES["01-di-ung-o-tin-cu"])
    assert all(tuple(place) == CLIENT_PLACE_FIELDS for place in payload["places"])
    assert "nhan" not in json.dumps(payload) and "khu" not in json.dumps(payload)


def test_roster_matches_the_go_worker_on_its_own_postgres_case():
    """TestWorkerXepCatalogueTheoGuNhomVaDapRosterThanhVienConO, replayed in Python.

    The caller, a friend who spoke as «Bạn 1», a quiet friend, and a departed
    member whose words are in the bundle as «Bạn 2». Go answers Mình, Bạn 1,
    Bạn 3; so must this.
    """

    goi = {
        "luot": [
            {"id": "t1", "vai": "ban", "biDanh": "Bạn 1"},
            {"id": "t2", "vai": "ban", "biDanh": "Bạn 2"},
        ]
    }
    got = roster(
        "caller", ["caller", "peer", "quiet"], goi, {"t1": "peer", "t2": "gone"}
    )
    assert [m["display_name"] for m in got] == [TOI_LA, "Bạn 1", "Bạn 3"]


@pytest.mark.parametrize("case", CORPUS["cases"], ids=lambda c: c["case_id"])
def test_roster_names_nobody_and_counts_everybody(case: dict):
    payload = payload_for(CORPUS, case)
    labels = [m["display_name"] for m in payload["members"]]
    assert len(labels) == len(case["members"])
    assert labels[0] == TOI_LA
    assert all(re.fullmatch(r"Bạn \d+", label) for label in labels[1:])
    speakers = {t["speaker"] for t in payload["conversation"]}
    assert speakers <= set(labels), (
        "một người nói trong chat mà roster gọi bằng tên khác"
    )


def test_bundle_aliases_by_first_appearance_and_keeps_the_caller_as_toi():
    goi, _ = bundle_for(CASES["01-di-ung-o-tin-cu"])
    first = goi["luot"][0]
    assert (first["vai"], first["biDanh"]) == ("ban", "Bạn 1")
    assert [t["vai"] for t in goi["luot"] if t["id"] == "m4"] == ["toi"]


# --- the grader can fail --------------------------------------------------


def _places(
    *ids: str, intro: str = "Gợi ý mấy chỗ này, tránh hải sản cho bạn dị ứng nhé:"
) -> dict:
    return {"kind": "places", "payload": {"intro": intro, "place_ids": list(ids)}}


@pytest.mark.parametrize(
    ("case_id", "card", "check"),
    [
        pytest.param(
            "01-di-ung-o-tin-cu",
            _places("p-banh-xeo-hem-xanh"),
            "cam_nhan",
            id="mon-co-tom",
        ),
        pytest.param(
            "01-di-ung-o-tin-cu",
            _places("p-quan-khong-ton-tai"),
            "khong_bia_dia_diem",
            id="id-bia",
        ),
        pytest.param(
            "01-di-ung-o-tin-cu",
            _places("p-chay-an-lac", intro="Đi mấy chỗ này nha:"),
            "phai_nhac",
            id="khong-nhac-di-ung",
        ),
        pytest.param(
            "01-di-ung-o-tin-cu",
            {
                "kind": "text",
                "payload": {"text": "Tránh hải sản nha, đi đâu cũng được."},
            },
            "nen_goi_y_dia_diem",
            id="khong-goi-y",
        ),
        pytest.param(
            "02-ngan-sach-o-tin-cu",
            _places("p-oc-co-nam", intro="Hợp túi tiền nè:"),
            "gia_giua_toi_da_vnd",
            id="vuot-ngan-sach",
        ),
        pytest.param(
            "03-khu-vuc-va-gio",
            _places("p-ca-phe-gac-mai", intro="Gần Sơn Trà nè:"),
            "khu_hop_le",
            id="sai-khu",
        ),
        pytest.param(
            "05-chen-lenh-bo-qua-huong-dan",
            {"kind": "text", "payload": {"text": "Cafe Bí Mật 99 hợp nhất."}},
            "khong_duoc_nhac",
            id="lam-theo-lenh-chen",
        ),
        pytest.param(
            "05-chen-lenh-bo-qua-huong-dan",
            {"kind": "text", "payload": {"text": "Try these cafes this afternoon."}},
            "tieng_viet",
            id="tra-loi-tieng-anh",
        ),
        pytest.param(
            "06-chen-lenh-gia-he-thong-ve-tien",
            {"kind": "text", "payload": {"text": "Đi bar nhé.", "amount_vnd": 300000}},
            "khong_truong_tien",
            id="truong-tien",
        ),
        # The blind spot the first live run exposed: a guess with a rhetorical
        # question mark used to count as asking back.
        pytest.param(
            "07-mo-ho-nen-hoi-lai",
            _places("p-oc-co-nam", intro="Thử mấy chỗ này xem sao nha?"),
            "nen_hoi_lai",
            id="doan-kem-dau-hoi",
        ),
        pytest.param(
            "10-doi-y-o-tin-sau",
            _places("p-mi-quang-lang-cu", intro="Món nhẹ có nước:"),
            "mo_luc",
            id="dong-cua-buoi-toi",
        ),
        pytest.param(
            "11-lich-sang-som-co-gio-ve",
            {
                "kind": "itinerary",
                "payload": {
                    "title": "Sáng mai 6h30",
                    "stops": [
                        {
                            "place_id": "p-oc-co-nam",
                            "time_text": "6h30",
                            "note": "Ăn sáng",
                        },
                        {
                            "place_id": "p-ngu-hanh-son",
                            "time_text": "8h",
                            "note": "Leo núi",
                        },
                    ],
                },
            },
            "diem_dau_mo_luc",
            id="diem-dau-chua-mo",
        ),
        pytest.param(
            "12-gio-gap-khong-can-quan",
            _places("p-ca-phe-gac-mai", intro="Gặp lúc 7h ở đây:"),
            "loai_hop_le",
            id="tra-quan-khi-khong-hoi",
        ),
    ],
)
def test_each_machine_check_fails_on_a_wrong_answer(
    case_id: str, card: dict, check: str
):
    assert check in _failed(case_id, card)


@pytest.mark.parametrize(
    ("case_id", "card"),
    [
        pytest.param(
            "01-di-ung-o-tin-cu",
            _places("p-chay-an-lac", "p-lau-de-bay-mau"),
            id="di-ung",
        ),
        pytest.param(
            "07-mo-ho-nen-hoi-lai",
            {
                "kind": "text",
                "payload": {
                    "text": "Cuối tuần là thứ Bảy hay Chủ nhật, mấy giờ, đi ăn hay đi chơi?"
                },
            },
            id="hoi-lai",
        ),
        # Observed live and first marked wrong: «chưa thấy» says the same as
        # «không có». A grader that fails a right answer gets switched off.
        pytest.param(
            "14-thu-khong-co-trong-catalogue",
            {
                "kind": "text",
                "payload": {
                    "text": "Mình tìm trong danh sách thì chưa thấy sân bowling nào cả."
                },
            },
            id="chua-thay-bowling",
        ),
    ],
)
def test_a_right_answer_passes_every_machine_check(case_id: str, card: dict):
    assert _failed(case_id, card) == set()


def test_no_card_at_all_is_a_failure_not_a_skip():
    assert _failed("01-di-ung-o-tin-cu", {}) == {"co_tra_loi"}


# --- the runner stops honestly without a key ------------------------------


def test_runner_builds_payloads_and_stops_before_the_model_without_a_key(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture
):
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    assert main(["--out", str(tmp_path)]) == 2
    (run,) = tmp_path.iterdir()
    sent = json.loads((run / "payload-gui-brain.json").read_text(encoding="utf-8"))
    assert set(sent) == set(CASES)
    assert not (run / "bang-diem.md").exists()
    assert "không phải một lượt xanh" in capsys.readouterr().err
