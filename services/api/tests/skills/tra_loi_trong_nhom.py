"""Answer-quality harness for the group AI: one real model call per case.

Every other gate on the group AI measures the plumbing. The Postgres tier proves
what is stored, scrubbed and refused; ``chat-e2e`` runs against a deterministic
stub brain; parity proves Go matches Python. None of them reads an answer. This
module does: it hands the real ``companion-reply`` brain the payload the Go
worker would send for each handwritten case in
``corpus/tra-loi-trong-nhom.json``, keeps the raw card, and grades what a
machine can grade.

What it proves when it runs
---------------------------
* The model saw what production sends. The conversation is built by
  ``hoi_thoai``, which is held to the SAME handwritten golden as the Go worker's
  ``hoiThoai`` (``services/core/internal/chatassist/testdata/``), key order
  included. The roster follows the worker's ``roster``: display names where
  they pass the prompt-safety test (ADR-0036 §5), never an account id.
* Per case: every ``place_id`` is in the catalogue it was handed, no money field
  appears, the answer is Vietnamese, forbidden place tags are absent, prices,
  districts and opening hours fit what the group typed, required words appear,
  planted words do not, and ambiguous requests come back as a question.

What it does not prove
----------------------
That an answer is natural, fair to two people who disagree, or actually useful.
Those rows are printed for a person under ``phai_ton_trong`` / ``khong_duoc``.
Nor is one run a rate: temperature is 0.0 and each case runs ``--lap`` times,
which bounds nothing. "Passed the machine checks" means "no failure observed in
these samples".

Run it from ``services/api``::

    set -a && . /path/to/.env && set +a
    python -m tests.skills.tra_loi_trong_nhom --out /tmp/tra-loi-trong-nhom

Model output is written under ``--out`` only. It is never committed: it is a
model's words about a synthetic conversation, and a checked-in answer would
start being read as the expected one.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import secrets
import sys
import unicodedata
from dataclasses import asdict, dataclass
from datetime import UTC, datetime, timedelta
from pathlib import Path

from app.places.prompt_safety import field_is_safe
from tests.evals import bang_chung, thong_ke

CORPUS_PATH = Path(__file__).parent / "corpus" / "tra-loi-trong-nhom.json"
GOLDEN_PATH = (
    Path(__file__).resolve().parents[3]
    / "core"
    / "internal"
    / "chatassist"
    / "testdata"
    / "hoi_thoai_golden.json"
)
#: The evidence store (tests/evals/bang_chung.py); --out inside a git worktree
#: is refused, because model output does not go into Git.
DEFAULT_OUT = None
REPO_ROOT = Path(__file__).resolve().parents[4]
#: One brain call per run on this path: `companion-reply` is a single call.
LOI_GOI_MOI_LUOT = 1

# The fallback speaker labels of chatassist/boicanh.go nhanNguoiNoi, and the
# caller's fallback label that chatassist/roster.go shares with it.
TOI_LA = "Mình"
AI_LA = "Rủ Đi AI"
KHONG_TEN = "Một người trong nhóm"
# chatassist/roster.go maxTenDoc.
MAX_TEN_DOC = 60

MONEY_KEYS = frozenset({"expense", "amount_vnd", "obligation", "split", "total_vnd"})
_VIETNAMESE = re.compile(
    "[ăâđêôơưạảấầẩẫậắằẳẵặẹẻẽếềểễệỉịọỏốồổỗộớờởỡợụủứừửữựỳỵỷỹáàãéèíìóòõúùýđ]",
    re.IGNORECASE,
)
_BASE_TIME = datetime(2026, 9, 20, 11, 0, tzinfo=UTC)


# --- the payload, built the way the worker builds it ---------------------


def load_corpus(path: Path = CORPUS_PATH) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def ten_doc(name: str | None) -> str:
    """``chatassist.tenDoc``: a display name the model may read, or "".

    The same test ``promptsafety.TextSafe`` applies in Go, which is itself the
    port of ``app.places.prompt_safety.field_is_safe``.
    """

    name = (name or "").strip()
    if not name or not field_is_safe(name, max_chars=MAX_TEN_DOC):
        return ""
    return name


def bundle_for(case: dict) -> tuple[dict, dict[str, str]]:
    """The bundle the client would attach, plus each turn's author.

    Mirrors ``gomBoiCanhChat`` for text messages: the caller's own lines are
    ``toi``, everybody else is ``ban`` labelled by their display name (the
    corpus authors ARE display names), a repeated name deduplicated as
    «Tên (2)» by first appearance. The author map stands in for the
    ``messages.author_id`` column the worker reads to line the roster up with
    those labels.
    """

    alias: dict[str, str] = {}
    used: set[str] = set()
    luot = []
    author_of = {}
    for index, message in enumerate(case["messages"]):
        author = message["author"]
        author_of[message["id"]] = author
        luc = (_BASE_TIME + timedelta(minutes=index)).strftime("%Y-%m-%dT%H:%M:%SZ")
        turn = {"id": message["id"], "vai": "toi", "luc": luc}
        if author != case["caller"]:
            if author not in alias:
                label, k = author, 2
                while label in used:
                    label, k = f"{author} ({k})", k + 1
                used.add(label)
                alias[author] = label
            turn = {
                "id": message["id"],
                "vai": "ban",
                "luc": luc,
                "biDanh": alias[author],
            }
        turn.update({"loai": "chu", "chu": message["text"]})
        luot.append(turn)
    goi = {
        "ban": 1,
        "nguon": "chat-nhom",
        "luot": luot,
        "tongLuot": len(luot),
        "daCat": False,
    }
    return goi, author_of


def _speaker(turn: dict, toi: str) -> str:
    vai = turn.get("vai")
    if vai == "toi":
        return toi
    if vai == "ai":
        return AI_LA
    return ten_doc(turn.get("biDanh")) or KHONG_TEN


def hoi_thoai(goi: dict | None, prompt: str, toi: str) -> list[dict]:
    """``chatassist.hoiThoai``, field for field and in the same key order."""

    out = []
    for turn in (goi or {}).get("luot", []):
        out.append(
            {
                "author_kind": "ai" if turn.get("vai") == "ai" else "human",
                "kind": "text",
                "speaker": _speaker(turn, toi),
                "body": turn.get("chu", ""),
                "created_at": turn.get("luc", ""),
            }
        )
    out.append({"author_kind": "human", "kind": "text", "speaker": toi, "body": prompt})
    return out


def roster(
    caller: str,
    active: list[str],
    goi: dict | None,
    author_of: dict[str, str],
    names: dict[str, str] | None = None,
) -> tuple[list[dict], str]:
    """``chatassist.roster``: every active member once, and the caller's label.

    ``names`` maps a person to their display name; the corpus uses names as
    person ids, so it defaults to the id itself.
    """

    def name_of(person: str) -> str:
        return ten_doc((names or {}).get(person, person))

    turns = (goi or {}).get("luot", [])
    others = {
        ten_doc(t.get("biDanh"))
        for t in turns
        if t.get("vai") == "ban" and ten_doc(t.get("biDanh"))
    }
    toi = TOI_LA
    if caller in active:
        own = name_of(caller)
        if own and own not in others:
            toi = own
    used = {toi}
    alias: dict[str, str] = {}
    for turn in turns:
        label = ten_doc(turn.get("biDanh"))
        if turn.get("vai") != "ban" or not label:
            continue
        taken = label in used
        used.add(label)
        person = author_of.get(turn["id"])
        if person is None or person == caller or taken:
            continue
        alias.setdefault(person, label)
    counter = 0

    def fresh() -> str:
        nonlocal counter
        while True:
            counter += 1
            label = f"Bạn {counter}"
            if label not in used:
                used.add(label)
                return label

    def unique(name: str) -> str:
        label, k = name, 2
        while label in used:
            label, k = f"{name} ({k})", k + 1
        used.add(label)
        return label

    out = [{"display_name": toi}]
    for member in active:
        if member == caller:
            continue
        label = alias.get(member)
        if label is None:
            name = name_of(member)
            label = unique(name) if name else fresh()
        out.append({"display_name": label})
    return out, toi


def model_places(catalogue: list[dict]) -> list[dict]:
    """What ``service.ModelPlaceRows`` hands the model: nine fields, promptsafety first.

    The grading fields (``nhan``, ``khu``) never reach the model.
    """

    from app.api.companion_places import load_place_catalogue

    return load_place_catalogue(catalogue)


def payload_for(corpus: dict, case: dict) -> dict:
    goi, author_of = bundle_for(case)
    members, toi = roster(case["caller"], case["members"], goi, author_of)
    return {
        "conversation": hoi_thoai(goi, case["prompt"], toi),
        "members": members,
        "places": model_places(corpus["catalogue"]),
        "budget_per_person_vnd": case.get("budget_per_person_vnd"),
    }


def call_brain(payload: dict) -> tuple[int, dict]:
    """POST the payload to the real brain route, exactly as the worker does."""

    from fastapi.testclient import TestClient

    from app.api.internal_token import INTERNAL_TOKEN_HEADER
    from app.api.routes.brain import build_brain_app

    token = secrets.token_hex(16)
    with TestClient(build_brain_app(token)) as client:
        response = client.post(
            "/internal/brain/v1/companion-reply",
            json=payload,
            headers={INTERNAL_TOKEN_HEADER: token},
        )
    try:
        body = response.json()
    except ValueError:
        body = {}
    return response.status_code, body if isinstance(body, dict) else {}


# --- grading --------------------------------------------------------------


@dataclass(frozen=True)
class Check:
    ten: str
    ket_qua: str  # "dat" | "truot" | "khong_ap_dung"
    chi_tiet: str = ""


def _fold(text: str) -> str:
    return unicodedata.normalize("NFC", text).casefold()


def _payload(card: dict) -> dict:
    payload = card.get("payload")
    return payload if isinstance(payload, dict) else {}


def chosen_ids(card: dict) -> list[str]:
    payload = _payload(card)
    ids = [pid for pid in payload.get("place_ids") or [] if isinstance(pid, str)]
    for stop in payload.get("stops") or []:
        if isinstance(stop, dict) and isinstance(stop.get("place_id"), str):
            ids.append(stop["place_id"])
    return ids


def answer_text(card: dict) -> str:
    """Every sentence a reader of the card would see."""

    payload = _payload(card)
    parts = [payload.get(key) for key in ("text", "intro", "title")]
    for stop in payload.get("stops") or []:
        if isinstance(stop, dict):
            parts += [stop.get("time_text"), stop.get("note")]
    return "\n".join(part for part in parts if isinstance(part, str) and part)


def _minutes(hhmm: str) -> int:
    hours, minutes = hhmm.split(":")
    return int(hours) * 60 + int(minutes)


def _opening(place: dict) -> tuple[int, int] | None:
    match = re.fullmatch(
        r"\s*(\d{1,2}:\d{2})\s*[–-]\s*(\d{1,2}:\d{2})\s*", place.get("open_hours") or ""
    )
    if not match:
        return None
    start, end = _minutes(match.group(1)), _minutes(match.group(2))
    if end <= start:  # past midnight
        end += 24 * 60
    return start, end


def open_at(place: dict, hhmm: str) -> bool:
    span = _opening(place)
    if span is None:
        return False
    t = _minutes(hhmm)
    start, end = span
    return start <= t < end or start <= t + 24 * 60 < end


def open_within(place: dict, window: list[str]) -> bool:
    span = _opening(place)
    if span is None:
        return False
    lo, hi = _minutes(window[0]), _minutes(window[1])
    start, end = span
    return start < hi and lo < end


def grade(corpus: dict, case: dict, card: dict | None) -> list[Check]:
    """Every machine check for one answer. Never stops at the first failure."""

    if not isinstance(card, dict) or not card:
        return [Check("co_tra_loi", "truot", "brain không trả thẻ nào")]
    rules = case["expected"].get("may_cham", {})
    by_id = {place["id"]: place for place in corpus["catalogue"]}
    ids = chosen_ids(card)
    known = [by_id[pid] for pid in ids if pid in by_id]
    text = answer_text(card)
    folded = _fold(text)
    whole = _fold(json.dumps(card, ensure_ascii=False))
    kind = card.get("kind")
    checks: list[Check] = []

    def verdict(name: str, ok: bool, detail: str = "") -> None:
        checks.append(Check(name, "dat" if ok else "truot", "" if ok else detail))

    def not_applicable(name: str, why: str) -> None:
        checks.append(Check(name, "khong_ap_dung", why))

    invented = [pid for pid in ids if pid not in by_id]
    verdict(
        "khong_bia_dia_diem", not invented, f"id không có trong catalogue: {invented}"
    )
    money = sorted(MONEY_KEYS & set(_payload(card)))
    verdict("khong_truong_tien", not money, f"thẻ mang trường tiền: {money}")
    verdict(
        "tieng_viet", bool(_VIETNAMESE.search(text)), "không thấy chữ tiếng Việt có dấu"
    )

    if "loai_hop_le" in rules:
        verdict(
            "loai_hop_le",
            kind in rules["loai_hop_le"],
            f"loại thẻ {kind!r}, cần một trong {rules['loai_hop_le']}",
        )
    if rules.get("nen_goi_y_dia_diem"):
        verdict(
            "nen_goi_y_dia_diem",
            kind in {"places", "itinerary"} and bool(ids),
            f"loại thẻ {kind!r} không gợi ý địa điểm nào",
        )
    if rules.get("nen_hoi_lai"):
        # A question mark alone is not a question back. Observed live: five
        # places and "xem sao nha?" -- a guess with a rhetorical tail. When the
        # facts are missing, the answer is a text card that asks.
        verdict(
            "nen_hoi_lai",
            kind == "text" and "?" in text,
            f"loại thẻ {kind!r}, {'có' if '?' in text else 'không có'} dấu hỏi: đoán thay vì hỏi lại",
        )

    place_rules = {
        "cam_nhan",
        "phai_co_nhan_mot",
        "gia_giua_toi_da_vnd",
        "khu_hop_le",
        "mo_luc",
        "diem_dau_mo_luc",
        "mo_trong_khung",
    }
    for name in sorted(place_rules & set(rules)):
        if not known:
            not_applicable(name, "thẻ không chọn địa điểm nào trong catalogue")
            continue
        value = rules[name]
        if name == "cam_nhan":
            bad = [(p["id"], tag) for p in known for tag in p["nhan"] if tag in value]
            verdict(name, not bad, f"chọn địa điểm mang nhãn bị cấm: {bad}")
        elif name == "phai_co_nhan_mot":
            verdict(
                name,
                any(set(p["nhan"]) & set(value) for p in known),
                f"không chỗ nào mang một trong {value}",
            )
        elif name == "gia_giua_toi_da_vnd":
            bad = [
                p["id"]
                for p in known
                if (p["price_min_vnd"] + p["price_max_vnd"]) // 2 > value
            ]
            verdict(name, not bad, f"giá giữa vượt {value}: {bad}")
        elif name == "khu_hop_le":
            bad = [f"{p['id']} ({p['khu']})" for p in known if p["khu"] not in value]
            verdict(name, not bad, f"ngoài khu {value}: {bad}")
        elif name == "mo_luc":
            # An itinerary is a sequence of times; only its first stop has to be
            # open when the group arrives. A list of places is a set of
            # alternatives, and every one of them has to be.
            subject = known[:1] if kind == "itinerary" else known
            bad = [
                f"{p['id']} ({p['open_hours']})"
                for p in subject
                if not open_at(p, value)
            ]
            verdict(name, not bad, f"đóng cửa lúc {value}: {bad}")
        elif name == "diem_dau_mo_luc":
            first = known[0]
            verdict(
                name,
                open_at(first, value),
                f"điểm đầu {first['id']} ({first['open_hours']}) đóng lúc {value}",
            )
        elif name == "mo_trong_khung":
            bad = [
                f"{p['id']} ({p['open_hours']})"
                for p in known
                if not open_within(p, value)
            ]
            verdict(name, not bad, f"không mở trong khung {value}: {bad}")

    if "phai_co_id" in rules:
        missing = [pid for pid in rules["phai_co_id"] if pid not in ids]
        verdict("phai_co_id", not missing, f"thiếu {missing}")
    if "phai_nhac" in rules:
        missing = [
            group
            for group in rules["phai_nhac"]
            if not any(_fold(term) in folded for term in group)
        ]
        verdict(
            "phai_nhac", not missing, f"không nhắc tới nhóm chữ nào trong {missing}"
        )
    if "khong_duoc_nhac" in rules:
        found = [term for term in rules["khong_duoc_nhac"] if _fold(term) in whole]
        verdict("khong_duoc_nhac", not found, f"có nhắc {found}")
    return checks


def ground_status(card: dict | None, places: list[dict]) -> str:
    """What the worker would do with this card: publish it, or refuse it."""

    from app.domain.companion import CompanionError, ground_card

    if not isinstance(card, dict):
        return "khong_co_the"
    try:
        ground_card(card, places)
    except CompanionError as exc:
        return f"tu_choi:{exc.code}"
    except Exception as exc:  # noqa: BLE001 -- the harness reports, never raises
        return f"tu_choi:{type(exc).__name__}"
    return "dang_duoc"


# --- the run --------------------------------------------------------------


def _cell(text: str) -> str:
    return text.replace("|", "\\|").replace("\n", " ")


def render_markdown(results: list[dict], meta: dict) -> str:
    lines = [
        "# Bảng điểm «trả lời trong nhóm»",
        "",
        f"- Model: `{meta['model']}` · lượt mỗi ca: {meta['lap']} · lúc chạy: {meta['luc']}",
        f"- Ca qua mọi phép máy chấm: **{meta['qua']}/{meta['tong']}** lượt",
        "- Máy chỉ chấm được phần ghi ở `may_cham`. Cột «Người chấm» là việc còn lại, chưa ai làm.",
        "",
        "| Ca | Lượt | Thẻ | Địa điểm chọn | Worker đăng? | Máy chấm | Trượt ở |",
        "|---|---|---|---|---|---|---|",
    ]
    for row in results:
        failed = [c for c in row["cham"] if c["ket_qua"] == "truot"]
        passed = sum(1 for c in row["cham"] if c["ket_qua"] == "dat")
        total = sum(1 for c in row["cham"] if c["ket_qua"] != "khong_ap_dung")
        lines.append(
            "| {ca} | {lap} | {the} | {dd} | {ground} | {ok}/{total} | {fail} |".format(
                ca=row["case_id"],
                lap=row["lap"],
                the=row["kind"] or f"HTTP {row['http']}",
                dd=_cell(", ".join(row["dia_diem"]) or "(không)"),
                ground=row["ground"],
                ok=passed,
                total=total,
                fail=_cell("; ".join(c["ten"] for c in failed) or "—"),
            )
        )
    lines += ["", "## Từng ca", ""]
    for row in results:
        lines += [
            f"### {row['case_id']} · lượt {row['lap']}",
            "",
            f"- Câu nhờ: {row['prompt']}",
            f"- Thẻ: `{row['kind']}` · địa điểm: {', '.join(row['dia_diem']) or '(không)'}",
            "",
            "```text",
            row["chu"] or "(không có chữ)",
            "```",
            "",
            "Máy chấm:",
            "",
        ]
        for check in row["cham"]:
            mark = {
                "dat": "đạt",
                "truot": "**TRƯỢT**",
                "khong_ap_dung": "không áp dụng",
            }[check["ket_qua"]]
            detail = f" ({check['chi_tiet']})" if check["chi_tiet"] else ""
            lines.append(f"- `{check['ten']}`: {mark}{detail}")
        lines += ["", "Người chấm:", ""]
        lines += [f"- [ ] Tôn trọng: {item}" for item in row["phai_ton_trong"]]
        lines += [f"- [ ] Không: {item}" for item in row["khong_duoc"]]
        lines.append("")
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.split("\n", 1)[0])
    parser.add_argument(
        "--out",
        type=Path,
        default=DEFAULT_OUT,
        help="kho bằng chứng (mặc định ~/.cache/rudi-bang-chung/eval); trong worktree git bị từ chối",
    )
    parser.add_argument(
        "--ca", action="append", default=[], help="chỉ chạy ca có case_id này"
    )
    parser.add_argument("--lap", type=int, default=1, help="số lượt gọi mỗi ca")
    parser.add_argument(
        "--du-toan",
        action="store_true",
        help="in trần số lời gọi model của lượt này rồi dừng, không gọi gì",
    )
    parser.add_argument(
        "--tran-goi",
        type=int,
        help="số lời gọi Lead đã duyệt; bắt buộc khi có GEMINI_API_KEY, dự toán vượt thì từ chối",
    )
    parser.add_argument(
        "--cham-lai",
        type=Path,
        help="chấm lại ket-qua.json của một lượt đã chạy bằng corpus hiện tại, không gọi model",
    )
    args = parser.parse_args(argv)

    corpus = load_corpus()
    if args.cham_lai:
        return _regrade(corpus, args.cham_lai)
    cases = [c for c in corpus["cases"] if not args.ca or c["case_id"] in args.ca]
    if not cases:
        print(f"Không có ca nào khớp {args.ca}", file=sys.stderr)
        return 2
    du_toan = len(cases) * args.lap * LOI_GOI_MOI_LUOT
    if args.du_toan:
        print(
            f"Dự toán: {len(cases)} ca × {args.lap} lượt × {LOI_GOI_MOI_LUOT} = {du_toan} lời gọi model"
        )
        return 0
    stamp = datetime.now(UTC).strftime("%Y%m%d-%H%M%S")
    try:
        sha = bang_chung.git_sha(REPO_ROOT)
    except Exception:  # noqa: BLE001 - a tree without git still runs, marked unknown
        sha = "khong-ro"
    run_id = f"{stamp}-{sha[:8]}"
    try:
        out = bang_chung.thu_muc_ra(args.out, run_id)
    except bang_chung.NgoaiKho as exc:
        print(str(exc), file=sys.stderr)
        return 2
    out.mkdir(parents=True, exist_ok=True)
    payloads = {case["case_id"]: payload_for(corpus, case) for case in cases}
    (out / "payload-gui-brain.json").write_text(
        json.dumps(payloads, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    if not os.environ.get("GEMINI_API_KEY", "").strip():
        print(
            f"Không có GEMINI_API_KEY: đã dựng {len(payloads)} payload đúng hình worker gửi "
            f"ở {out / 'payload-gui-brain.json'}, dừng TRƯỚC bước gọi model. "
            "Không có bảng điểm nào; đây không phải một lượt xanh.",
            file=sys.stderr,
        )
        return 2

    if args.tran_goi is None:
        print(
            f"Lượt thật cần --tran-goi (số lời gọi Lead đã duyệt). Dự toán lượt này: {du_toan}.",
            file=sys.stderr,
        )
        return 2
    if du_toan > args.tran_goi:
        print(
            f"Dự toán {du_toan} lời gọi vượt trần đã duyệt {args.tran_goi}: không chạy.",
            file=sys.stderr,
        )
        return 2

    from app.api.companion_gemini import DEFAULT_MODEL

    model = os.environ.get("MOBILE_GEMINI_MODEL") or DEFAULT_MODEL
    results = []
    da_goi = 0
    for case in cases:
        places = payloads[case["case_id"]]["places"]
        for lap in range(1, args.lap + 1):
            if da_goi + LOI_GOI_MOI_LUOT > args.tran_goi:
                raise SystemExit("chạm trần lời gọi giữa chừng: dự toán đã sai")
            da_goi += LOI_GOI_MOI_LUOT
            status, card = call_brain(payloads[case["case_id"]])
            card = card if status == 200 else None
            checks = grade(corpus, case, card)
            by_id = {p["id"]: p for p in corpus["catalogue"]}
            results.append(
                {
                    "case_id": case["case_id"],
                    "lap": lap,
                    "prompt": case["prompt"],
                    "http": status,
                    "kind": (card or {}).get("kind"),
                    "the_tho": card,
                    "dia_diem": [
                        by_id[pid]["name"] if pid in by_id else f"{pid} (BỊA)"
                        for pid in chosen_ids(card or {})
                    ],
                    "chu": answer_text(card or {}),
                    "ground": ground_status(card, places),
                    "cham": [asdict(check) for check in checks],
                    "phai_ton_trong": case["expected"]["phai_ton_trong"],
                    "khong_duoc": case["expected"]["khong_duoc"],
                }
            )
            print(f"{case['case_id']} lượt {lap}: HTTP {status}", file=sys.stderr)

    passed = sum(
        1 for row in results if not any(c["ket_qua"] == "truot" for c in row["cham"])
    )
    meta = {
        "model": model,
        "lap": args.lap,
        "luc": stamp,
        "qua": passed,
        "tong": len(results),
    }
    (out / "ket-qua.json").write_text(
        json.dumps({"meta": meta, "ket_qua": results}, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    (out / "bang-diem.md").write_text(render_markdown(results, meta), encoding="utf-8")
    print(
        f"{passed}/{len(results)} lượt qua mọi phép máy chấm · bảng điểm: {out / 'bang-diem.md'}"
    )
    tong = tong_hop(results, args.lap)
    bang_chung.ghi_manifest(
        out,
        bang_chung.manifest(
            run_id=run_id,
            git_sha=sha,
            corpus=CORPUS_PATH.name,
            corpus_sha256=bang_chung.sha256_file(CORPUS_PATH),
            model=model,
            lap=args.lap,
            loi_goi={
                "du_toan": du_toan,
                "tran_duyet": args.tran_goi,
                "da_dung": da_goi,
            },
            chi_so={"nhom_plan": tong},
            chua_xong=False,
        ),
    )
    for line in bang_chung.trailer_nhom_plan(
        run_id=run_id,
        lap=args.lap,
        goi_da_dung=da_goi,
        goi_duyet=args.tran_goi,
        model=model,
        so_ca=tong["so_ca"],
        ca_vung=tong["ca_vung"],
        it_nhat=tong["it_nhat"],
        pass_at_1=tong["pass_at_1"],
        ci=(tong["ci95"][0], tong["ci95"][1]),
    ):
        print(line)
    return 0


def tong_hop(results: list[dict], lap: int) -> dict:
    """Per-case aggregation: pass@1 with a case-resampled interval, solid cases.

    A run passes when no machine check failed. A case is solid when it passed
    at least four runs in five (the same fraction for other --lap values).
    """
    cases = thong_ke.gop_theo_ca(
        (row["case_id"], not any(c["ket_qua"] == "truot" for c in row["cham"]))
        for row in results
    )
    it_nhat = max(1, -(-4 * lap // 5))
    lo, hi = thong_ke.bootstrap_ci([c.ty_le for c in cases])
    return {
        "so_ca": len(cases),
        "so_luot": sum(c.tong for c in cases),
        "pass_at_1": round(thong_ke.pass_at_1(cases), 4),
        "ci95": [round(lo, 4), round(hi, 4)],
        "it_nhat": it_nhat,
        "ca_vung": thong_ke.ca_vung(cases, it_nhat=it_nhat),
        "pass_mu_k": thong_ke.pass_mu_k(cases),
    }


def _regrade(corpus: dict, path: Path) -> int:
    """Grade saved cards again: a grader fix must be measurable without a model."""

    saved = json.loads(path.read_text(encoding="utf-8"))
    cases = {case["case_id"]: case for case in corpus["cases"]}
    results = []
    for row in saved["ket_qua"]:
        case = cases[row["case_id"]]
        row = dict(row)
        row["cham"] = [asdict(check) for check in grade(corpus, case, row["the_tho"])]
        row["phai_ton_trong"] = case["expected"]["phai_ton_trong"]
        row["khong_duoc"] = case["expected"]["khong_duoc"]
        results.append(row)
    passed = sum(
        1 for row in results if not any(c["ket_qua"] == "truot" for c in row["cham"])
    )
    meta = dict(saved["meta"], qua=passed, tong=len(results))
    meta["luc"] = f"{saved['meta']['luc']} (chấm lại, không gọi model)"
    out = path.with_name("bang-diem-cham-lai.md")
    out.write_text(render_markdown(results, meta), encoding="utf-8")
    print(f"{passed}/{len(results)} lượt qua mọi phép máy chấm · bảng điểm: {out}")
    tong = tong_hop(results, int(saved["meta"].get("lap", 1)))
    print(
        f"pass@1 {tong['pass_at_1']:.2f} [{tong['ci95'][0]:.2f},{tong['ci95'][1]:.2f}]"
        f" · ca vững {tong['ca_vung']}/{tong['so_ca']} (>={tong['it_nhat']}/{saved['meta'].get('lap', 1)})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
