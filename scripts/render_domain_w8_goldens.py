#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W8 pair routes (ADR-0029).

The nineteen W8 routes (the two-person notebook and its sheets of paper) reach
these pure Python pieces, each ported to one Go package:

    app.domain.pair_paper            services/core/internal/domain/pairpaper
    app.api.service (pair methods)   services/core/internal/domain/pairsteps

app.domain.pair_notebook was ported earlier (internal/domain/pairnotebook,
scripts/render_pair_notebook_goldens.py); pairsteps calls that package.

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each returned or raised. The service methods run as real `ApiService`
methods over a recording stub repository with the clock pinned to the case's
`now`: the golden holds the answer (or the ApiProblem, or the exception that
would be a 500) and every repository call the method made, with its
arguments, in order. Each package's oracle_test.go replays every case. The
image is the one scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w8_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w8_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases;
`<module>-fuzz-0` is a small sample of that module's seeded fuzz. Both are
committed and replayed by plain `go test`. `--live MODULE SEED COUNT` draws
COUNT cases with SEED and prints them without the guard's checks; the Go tests
built with `-tags oracle` run it inside the image at test time (see
internal/oracletest/live.go). The committed sample is the first draw of the
same generator.

## Encoding

The encoding of scripts/render_domain_w4_goldens.py (`{"fn", "name", "args",
"result"}`, `"$i:<hex>"` for a large int, `"$sp:<text>"` for a str cut into
groups of six code points), with three additions:

* a uuid.UUID is its name in ALIASES (`"TOI"`, `"PP1"`), which keeps a case
  on one short line; the table is in the pair_steps constants. Ids in the
  arguments are those names too;
* a datetime is its isoformat(), a date its isoformat();
* a paper record (the dict `paper()` builds) is the list of its values in
  PAPER_KEYS order, and a notebook record the list in NOTEBOOK_KEYS order;
* a stored JSONB value (a sheet's content) is its json.dumps() text, which
  keeps what json.loads hands the service: key order, and 1 apart from 1.0.
  The Go test reads it back with internal/pyjson.

A service case's result is `{"ok": {"calls", "problem", "raised",
"response"}}`: `calls` is `[method, argument, ...]` per repository call with
arguments in the Protocol's order, `problem` the ApiProblem, `raised` the
class and code of an exception Python answers with a 500, `response` the
response model's model_dump().
"""

from __future__ import annotations

import json
import platform
import random
import re
import sys
import uuid
from datetime import UTC, date, datetime, timedelta, timezone
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import schemas  # noqa: E402
from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem, RepositoryConflict  # noqa: E402
from app.api.repository import (  # noqa: E402
    PairConsentRecord,
    PairConstraintRecord,
    PairKeepRecord,
    PairNotebookRecord,
    PairPaperRecord,
    PairProposalRecord,
    PairResponseRecord,
    PairVersionRecord,
    PairViewRecord,
)
from app.domain import pair_notebook, pair_paper  # noqa: E402
from app.domain.permissions import PermissionError_  # noqa: E402

SEED = 28
TARGET = "services/core/{path}/testdata/python_{mode}.json"
SMALL_INT = 10**8
GROUP = 6

# ---------------------------------------------------------------------------
# Copies of the content rules in scripts/repo_guard.py. A rendered file that
# matches one could not be committed, so rendering stops instead.
# ---------------------------------------------------------------------------

LINE_RULES = (
    re.compile(r"(?<![A-Za-z0-9])\d(?:[ .-]?\d){8,63}(?![A-Za-z0-9])"),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:3|5|7|8|9)"
        r"(?:[ ().-]*\d){8}(?!\d)"
    ),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*2"
        r"(?:[ ().-]*\d){8,9}(?!\d)"
    ),
    re.compile(
        r"(?<![A-Za-z0-9.!#$%&'*+/=?^_`{|}~-])"
        r"[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@"
        r"(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+"
        r"[A-Za-z]{2,63}(?![A-Za-z0-9-])"
    ),
    re.compile(
        r"(?<![A-Za-z0-9_])(?:gh[pousr]_[A-Za-z0-9]{36,255}|"
        r"github_pat_[A-Za-z0-9_]{20,255})(?![A-Za-z0-9_])"
    ),
    re.compile(r"(?<![A-Z0-9])(?:AKIA|ASIA)[A-Z0-9]{16}(?![A-Z0-9])"),
    re.compile(r"(?<![A-Za-z0-9_-])AIza[0-9A-Za-z_-]{35}(?![A-Za-z0-9_-])"),
    re.compile(r"[A-Za-z0-9+/_-]{2049,}={0,2}"),
)
FRAGMENT = re.compile(r"[A-Za-z0-9+/_-]{6,}={0,2}")
MIN_FRAGMENT_BYTES = 8
MAX_AGGREGATE_FRAGMENT_BYTES = 16 * 1024
MAX_LINE_BYTES = 4 * 1024
MAX_FILE_BYTES = 2 * 1024 * 1024
#: A case longer than this is redrawn, so every line stays under the guard's.
MAX_CASE_BYTES = 3900


def looks_encoded(fragment: str) -> bool:
    has_lower = any(character.islower() for character in fragment)
    has_upper = any(character.isupper() for character in fragment)
    has_other = any(c.isdigit() or c in "+/_-=" for c in fragment)
    return has_lower and has_upper and has_other


def encoded_bytes(line: str) -> int:
    return sum(
        len(match.group(0))
        for match in FRAGMENT.finditer(line)
        if len(match.group(0)) >= MIN_FRAGMENT_BYTES and looks_encoded(match.group(0))
    )


def guard_problem(rendered: str) -> str | None:
    if len(rendered.encode("utf-8")) > MAX_FILE_BYTES:
        return "the file is larger than the guard's text limit"
    aggregate = 0
    for number, line in enumerate(rendered.splitlines(), 1):
        if len(line.encode("utf-8")) > MAX_LINE_BYTES:
            return f"line {number} is too long"
        if any(rule.search(line) for rule in LINE_RULES):
            return f"line {number} would trip the repository guard"
        aggregate += encoded_bytes(line)
    if aggregate > MAX_AGGREGATE_FRAGMENT_BYTES:
        return f"base64-looking fragments add up to {aggregate} bytes"
    return None


# ---------------------------------------------------------------------------
# Ids
# ---------------------------------------------------------------------------


def _uuid_for(letter: str, digit: str) -> uuid.UUID:
    """A uuid whose digits never run: the guard reads nine as an account."""
    c, d = letter, digit
    return uuid.UUID(
        f"{c}{c}{d}{d}{c}{c}{d}{d}-0{c}0{c}-4{c}0{c}-8{c}0{c}-0{c}0{c}0{c}0{c}0{c}{d}{d}"
    )


#: Every uuid a case names. People, then contexts, notebooks and cycles,
#: proposals, papers, outings, keeps and a place. The ones ending in N are
#: what the stub hands back from a create.
ALIASES = {
    "TOI": _uuid_for("a", "1"),
    "KIA": _uuid_for("b", "2"),
    "LA": _uuid_for("c", "3"),
    "CAP": _uuid_for("d", "4"),
    "HOI": _uuid_for("e", "5"),
    "NB1": _uuid_for("f", "6"),
    "CY1": _uuid_for("a", "7"),
    "CY2": _uuid_for("b", "8"),
    "CYN": _uuid_for("c", "9"),
    "PR1": _uuid_for("d", "1"),
    "PR2": _uuid_for("e", "2"),
    "PR9": _uuid_for("f", "3"),
    "PRN": _uuid_for("a", "4"),
    "PP1": _uuid_for("b", "5"),
    "PP2": _uuid_for("c", "6"),
    "PP3": _uuid_for("d", "7"),
    "PPN": _uuid_for("e", "8"),
    "OU1": _uuid_for("f", "9"),
    "OUN": _uuid_for("a", "2"),
    "KP1": _uuid_for("b", "3"),
    "KPN": _uuid_for("c", "4"),
    "PL1": _uuid_for("d", "5"),
    # Older agreed sheets a draft looks back over (2026-09-24).
    "PP4": _uuid_for("e", "6"),
    "PP5": _uuid_for("f", "7"),
    "PP6": _uuid_for("a", "8"),
    "PP7": _uuid_for("b", "9"),
    # ADR-0034: the couple's proposal and each person's own taste switch.
    "PRD": _uuid_for("a", "3"),
    "PGK": _uuid_for("a", "5"),
    "PGT": _uuid_for("a", "6"),
}
ALIAS_OF = {value: name for name, value in ALIASES.items()}
assert len(ALIAS_OF) == len(ALIASES)


def U(name: str) -> uuid.UUID:
    return ALIASES[name]


def UN(name: str | None) -> uuid.UUID | None:
    return None if name is None else ALIASES[name]


# ---------------------------------------------------------------------------
# Encoding
# ---------------------------------------------------------------------------


class J:
    """A stored JSONB value, encoded as its JSON text."""

    def __init__(self, value: object):
        self.value = value


def enc_str(text: str) -> str:
    if any(0xD800 <= ord(char) <= 0xDFFF for char in text):
        raise ValueError("a surrogate cannot cross into Go strings")
    escaped = json.dumps(text, ensure_ascii=True)
    plain = (
        not text.startswith("$")
        and not any(rule.search(escaped) for rule in LINE_RULES)
        and encoded_bytes(escaped) == 0
    )
    if plain:
        return text
    groups = [text[i : i + GROUP] for i in range(0, len(text), GROUP)]
    return "$sp:" + "|".join(groups)


def enc(value: object) -> object:
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, J):
        return enc_str(
            json.dumps(value.value, ensure_ascii=False, separators=(",", ":"))
        )
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, int):
        return value if -SMALL_INT < value < SMALL_INT else f"$i:{value:#_x}"
    if isinstance(value, uuid.UUID):
        return ALIAS_OF.get(value) or enc_str(str(value))
    if isinstance(value, datetime):
        return enc_str(value.isoformat())
    if isinstance(value, date):
        return value.isoformat()
    if isinstance(value, list | tuple):
        return [enc(item) for item in value]
    if isinstance(value, dict):
        for record_keys in (PAPER_KEYS, NOTEBOOK_KEYS):
            if set(value) == set(record_keys):
                return [enc(value[key]) for key in record_keys]
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"unencodable {type(value).__name__}")


#: The fields of the world's paper and notebook dicts, in their encoded order.
PAPER_KEYS = (
    "id",
    "context",
    "owner",
    "state",
    "version",
    "tuan",
    "expires",
    "outing",
    "versions",
    "views",
    "responses",
    "keeps",
    "cycle",
)
NOTEBOOK_KEYS = (
    "id",
    "cycle",
    "state",
    "participants",
    "consents",
    "proposals",
    "constraints",
)


def case(fn: str, name: str, args: dict) -> dict:
    encoded = {key: enc(value) for key, value in args.items()}
    try:
        value = FUNCTIONS[fn](**args)
    except pair_paper.PaperError as exc:
        result = {
            "raised": {
                "type": type(exc).__name__,
                "message": enc_str(str(exc)),
                "code": enc(exc.code),
            }
        }
    else:
        result = {"ok": enc(value)}
    return {"fn": fn, "name": name, "args": encoded, "result": result}


def fits(built: dict) -> bool:
    return (
        len(json.dumps(built, ensure_ascii=True, separators=(",", ":")))
        <= MAX_CASE_BYTES
    )


# ---------------------------------------------------------------------------
# Shared pieces
# ---------------------------------------------------------------------------

MICRO = timedelta(microseconds=1)
SECOND = timedelta(seconds=1)
HOUR = timedelta(hours=1)
DAY = timedelta(days=1)

#: A Wednesday, noon in Vietnam: the week's Saturday is still ahead.
T = datetime(2030, 9, 18, 5, 0, tzinfo=UTC)
TUAN = date(2030, 9, 16)
SAT = date(2030, 9, 21)
#: han_tuan(T): midnight ending Sunday, local.
WEEK_END = datetime(2030, 9, 22, 17, 0, tzinfo=UTC)
#: Saturday 00:00 in Vietnam.
SAT_START = datetime(2030, 9, 20, 17, 0, tzinfo=UTC)

VIETNAM = timezone(timedelta(hours=7))
HONOLULU = timezone(timedelta(hours=-10))
KIRITIMATI = timezone(timedelta(hours=14))
BAKER = timezone(timedelta(hours=-12))
KATHMANDU = timezone(timedelta(hours=5, minutes=45))
SAIGON_LMT = timezone(timedelta(hours=7, minutes=6, seconds=30))
ONE_SECOND_WEST = timezone(timedelta(seconds=-1))
OFFSETS = (
    UTC,
    VIETNAM,
    HONOLULU,
    KIRITIMATI,
    BAKER,
    KATHMANDU,
    SAIGON_LMT,
    ONE_SECOND_WEST,
)

E_DOT_CIRCUMFLEX = chr(0x1EC7)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
NO_BREAK_SPACE = chr(0xA0)
IDEOGRAPHIC_SPACE = chr(0x3000)
ZERO_WIDTH_SPACE = chr(0x200B)
NEXT_LINE = chr(0x85)
FILE_SEPARATOR = "\x1c"
COMBINING_ACUTE = chr(0x301)
FULLWIDTH_TWO = chr(0xFF12)
ARABIC_INDIC_TWO = chr(0x662)
RIGHT_TO_LEFT_OVERRIDE = chr(0x202E)

#: Texts a person could send, and texts nobody should.
TEXTS = (
    "Hải sản",
    "  Đậu phộng  ",
    "",
    " ",
    "\t\n",
    NO_BREAK_SPACE + "x" + IDEOGRAPHIC_SPACE,
    ZERO_WIDTH_SPACE + "a" + ZERO_WIDTH_SPACE,
    NEXT_LINE + "b" + FILE_SEPARATOR,
    "e" + COMBINING_ACUTE,
    CJK_MIDDLE + GRINNING_FACE,
    RIGHT_TO_LEFT_OVERRIDE + "abc",
    "<script>alert(1)</script>",
    "'; DROP TABLE pair_papers; --",
    "a" * 200,
    " " + "b" * 200 + " ",
    "Ng" + E_DOT_CIRCUMFLEX + "p " * 60,
    "$sp:not|an|encoding",
    "line break",
)

# ---------------------------------------------------------------------------
# pair_paper
# ---------------------------------------------------------------------------


def paper_dict(
    state: str,
    expires: datetime | None = T + DAY,
    version: int = 1,
    paper_id: str = "p",
) -> dict:
    return {
        "id": paper_id,
        "state": state,
        "current_version": version,
        "expires_at": expires,
    }


def routine_of(spec: dict) -> dict:
    kind, *rest = spec["ngay"]
    routine = {
        "ngay": date.fromisoformat(rest[0])
        if kind == "date"
        else (rest[0] if kind == "str" else None),
        "gio": spec["gio"],
        "viec": spec["viec"],
        "di_tiep": spec["di_tiep"],
    }
    if "ly_do" in spec:
        routine["ly_do"] = spec["ly_do"]
    return routine


def pp_chuyen(paper, su_kien, now, facts):
    return pair_paper.chuyen(paper, su_kien, now=now, **facts)


def pp_co_the_rut(paper, versions, views, responses, actor_id):
    return pair_paper.co_the_rut(
        paper,
        [{"version": v, "author_type": a, "sent_by": s} for v, a, s in versions],
        [{"version": v, "person_id": p} for v, p in views],
        [{"version": v, "person_id": p, "kind": k} for v, p, k in responses],
        actor_id=actor_id,
    )


def pp_da_du_dong_y(responses, version):
    return pair_paper.da_du_dong_y(
        [{"version": v, "person_id": p, "kind": k} for v, p, k in responses], version
    )


def pp_phac(routine, rang_buoc, now):
    return pair_paper.phac_to_giay(
        routine_of(routine), [{"kind": "dung", "content": "x"}] * rang_buoc, now=now
    )


def row_of(spec):
    """A catalogue row from [id, name, category, kinds, traits, rating*10, count]."""
    if spec is None:
        return None
    pid, name, category, kinds, traits, rating, count = spec
    return {
        "id": pid,
        "name": name,
        "category": category,
        "kinds": kinds,
        "traits": traits,
        "rating": None if rating is None else rating / 10,
        "rating_count": count,
    }


def pp_lam_giau(routine, rang_buoc, now, lich_su, cho_cu, ung_vien):
    boxes = [{"content": c} for c in rang_buoc]
    return pair_paper.lam_giau_phac(
        pair_paper.phac_to_giay(routine_of(routine), boxes, now=now),
        lich_su=[
            {"ngay": ngay, "chang": [{"gio": g, "viec": v, "place_id": p} for g, v, p in chang]}
            for ngay, chang in lich_su
        ],
        cho_cu=row_of(cho_cu),
        ung_vien=[row_of(r) for r in ung_vien],
        rang_buoc=boxes,
    )


def gu_of(spec):
    """Tastes from [[tag, chung, ten, [nguoi, ...]], ...]."""
    return [{"tag": t, "chung": c, "ten": n, "nguoi": list(ps)} for t, c, n, ps in spec]


def pp_gu_cho_nep(nguoi_chia, gu, ten, ca_hai):
    return pair_paper.gu_cho_nep(nguoi_chia, gu, ten, ca_hai=ca_hai)


def pp_theo_gu(routine, rang_buoc, now, lich_su, cho_cu, ung_vien, gu, ung_vien_gu, da_di):
    # ADR-0034: the draft after lam_giau_phac, told the shared tastes.
    return pair_paper.lam_giau_theo_gu(
        pp_lam_giau(routine, rang_buoc, now, lich_su, cho_cu, ung_vien),
        gu=gu_of(gu),
        ung_vien=[row_of(r) for r in ung_vien_gu],
        da_di=list(da_di),
        rang_buoc=[{"content": c} for c in rang_buoc],
    )


def pair_paper_constants() -> dict:
    return {
        "all": sorted(pair_paper.__all__),
        "paper_states": list(pair_paper.PAPER_STATES),
        "open_states": list(pair_paper.OPEN_STATES),
        "plan_states": list(pair_paper.PLAN_STATES),
        "terminal": list(pair_paper.TERMINAL),
        "author_types": list(pair_paper.AUTHOR_TYPES),
        "response_kinds": list(pair_paper.RESPONSE_KINDS),
        "mui_gio": pair_paper.MUI_GIO.key,
        "thu_bay": pair_paper._THU_BAY,
        "events": [[name, list(states)] for name, states in pair_paper._TU.items()],
        "paper_error_code": pair_paper.PaperError("paper_frozen").code,
    }


#: Zone transitions of Asia/Ho_Chi_Minh in the image's tzdata.
TRANSITIONS = (
    datetime(1906, 6, 30, 16, 53, 30, tzinfo=UTC),
    datetime(1911, 4, 30, 16, 53, 30, tzinfo=UTC),
    datetime(1942, 12, 31, 16, 0, tzinfo=UTC),
    datetime(1945, 3, 14, 15, 0, tzinfo=UTC),
    datetime(1945, 9, 1, 15, 0, tzinfo=UTC),
    datetime(1947, 3, 31, 17, 0, tzinfo=UTC),
    datetime(1955, 6, 30, 17, 0, tzinfo=UTC),
    datetime(1959, 12, 31, 16, 0, tzinfo=UTC),
    datetime(1975, 6, 12, 16, 0, tzinfo=UTC),
)


def now_edges() -> list[tuple[str, datetime]]:
    out = [("base", T)]
    out += [(f"base_in_{k}", T.astimezone(tz)) for k, tz in enumerate(OFFSETS)]
    for name, instant in (
        ("sunday_last_micro", WEEK_END - MICRO),
        ("monday_first_micro", WEEK_END),
        ("monday_second_micro", WEEK_END + MICRO),
        ("friday_last_micro", SAT_START - MICRO),
        ("saturday_first_micro", SAT_START),
        ("saturday_last_micro", SAT_START + DAY - MICRO),
        ("sunday_first_micro", SAT_START + DAY),
        ("leap_day", datetime(2028, 2, 29, 12, tzinfo=UTC)),
        ("leap_day_ends_locally", datetime(2028, 2, 29, 17, tzinfo=UTC)),
        ("not_a_leap_century", datetime(2100, 2, 28, 20, tzinfo=UTC)),
        ("leap_century", datetime(2000, 2, 29, 16, 59, 59, tzinfo=UTC)),
        ("new_year_locally", datetime(2030, 12, 31, 17, tzinfo=UTC)),
        ("old_year_locally", datetime(2030, 12, 31, 16, 59, 59, 999999, tzinfo=UTC)),
        ("year_one", datetime(1, 1, 1, tzinfo=UTC)),
        ("year_one_week_two", datetime(1, 1, 7, 12, tzinfo=UTC)),
        ("year_thousand", datetime(1000, 6, 15, 3, 4, 5, 6, tzinfo=UTC)),
        ("year_ninety_nine_ninety_nine", datetime(9999, 12, 20, tzinfo=UTC)),
        ("epoch", datetime(1970, 1, 1, tzinfo=UTC)),
        ("micro_after", T + MICRO),
        ("micro_before_second", T + timedelta(microseconds=999999)),
    ):
        out.append((name, instant))
    for k, edge in enumerate(TRANSITIONS):
        out += [
            (f"transition_{k}/before", edge - SECOND),
            (f"transition_{k}/at", edge),
            (f"transition_{k}/micro_before", edge - MICRO),
            (f"transition_{k}/days_later", edge + 3 * DAY),
            (f"transition_{k}/days_earlier", edge - 4 * DAY),
        ]
    return out


def pair_paper_edges() -> list[dict]:
    out = []
    for name, now in now_edges():
        for fn in ("tuan_cua", "han_tuan", "ngay_de_xuat", "astimezone"):
            out.append(case(fn, name, {"now": now}))

    states = list(pair_paper.PAPER_STATES) + ["", "xoa"]
    for state in states:
        for label, expires in (
            ("none", None),
            ("micro_before", T - MICRO),
            ("at", T),
            ("micro_after", T + MICRO),
            ("at_other_offset", T.astimezone(HONOLULU)),
        ):
            out.append(
                case(
                    "hieu_luc",
                    f"{state or 'empty'}/{label}",
                    {"paper": paper_dict(state, expires), "now": T},
                )
            )

    A, B, C = "TOI", "KIA", "LA"
    for name, responses, version in (
        ("empty", [], 1),
        ("one", [[1, A, "dong_y"]], 1),
        ("same_person_twice", [[1, A, "dong_y"], [1, A, "dong_y"]], 1),
        ("both", [[1, A, "dong_y"], [1, B, "dong_y"]], 1),
        ("both_other_version", [[1, A, "dong_y"], [1, B, "dong_y"]], 2),
        ("counter_is_not_agreement", [[1, A, "dong_y"], [1, B, "de_nghi_sua"]], 1),
        ("split_versions", [[1, A, "dong_y"], [2, B, "dong_y"]], 2),
        ("three_people", [[3, A, "dong_y"], [3, B, "dong_y"], [3, C, "dong_y"]], 3),
        ("kind_case", [[1, A, "DONG_Y"], [1, B, "dong_y"]], 1),
        ("empty_person_counts", [[1, "", "dong_y"], [1, B, "dong_y"]], 1),
        ("version_zero", [[0, A, "dong_y"], [0, B, "dong_y"]], 0),
    ):
        out.append(
            case("da_du_dong_y", name, {"responses": responses, "version": version})
        )

    sent = paper_dict("da_gui")
    mine = [[1, A, "dong_y"]]
    for name, paper, versions, views, responses, actor in (
        ("sender_untouched", sent, [[1, "human", A]], [], mine, A),
        ("other_person", sent, [[1, "human", A]], [], mine, B),
        ("seen", sent, [[1, "human", A]], [[1, B]], mine, A),
        ("answered", sent, [[1, "human", A]], [], mine + [[1, B, "de_nghi_sua"]], A),
        ("seen_other_version", sent, [[1, "human", A]], [[2, B]], mine, A),
        ("own_view", sent, [[1, "human", A]], [[1, A]], mine, A),
        ("nep_sheet", sent, [[1, "nep", None]], [], [], A),
        ("no_current_version", sent, [[2, "human", A]], [], [], A),
        ("empty_sender_empty_actor", sent, [[1, "human", None]], [], [], ""),
        ("blank_sender_empty_actor", sent, [[1, "human", ""]], [], [], ""),
        ("first_match_wins", sent, [[1, "nep", None], [1, "human", A]], [], [], A),
        (
            "expired_still_da_gui",
            paper_dict("da_gui", T - DAY),
            [[1, "human", A]],
            [],
            [],
            A,
        ),
    ):
        args = {
            "paper": paper,
            "versions": versions,
            "views": views,
            "responses": responses,
            "actor_id": actor,
        }
        out.append(case("co_the_rut", name, args))
    for state in pair_paper.PAPER_STATES:
        args = {
            "paper": paper_dict(state),
            "versions": [[1, "human", A]],
            "views": [],
            "responses": [],
            "actor_id": A,
        }
        out.append(case("co_the_rut", f"state/{state}", args))

    everything = {"du_dong_y": True, "co_the_rut": True, "nguoi_ghi": True}
    events = list(pair_paper._TU) + ["xoa_het", "GUI"]
    for event in events:
        for state in states:
            # A deadline only reaches an undecided sheet; hieu_luc covers the rest.
            for expired in (
                (False, True) if state in pair_paper.OPEN_STATES else (False,)
            ):
                paper = paper_dict(state, T - MICRO if expired else T + DAY, version=3)
                facts = everything if event in ("dong_y", "rut", "da_di") else {}
                args = {"paper": paper, "su_kien": event, "now": T, "facts": facts}
                out.append(
                    case(
                        "chuyen",
                        f"{event or 'empty'}/{state or 'empty'}/{'expired' if expired else 'live'}",
                        args,
                    )
                )
    truthy = (
        ("none", {}),
        ("false", {"du_dong_y": False, "co_the_rut": False, "nguoi_ghi": False}),
        ("null", {"du_dong_y": None, "co_the_rut": None, "nguoi_ghi": None}),
        ("empty_str", {"du_dong_y": "", "co_the_rut": "", "nguoi_ghi": ""}),
        ("zero", {"du_dong_y": 0, "co_the_rut": 0, "nguoi_ghi": 0}),
        ("person", {"du_dong_y": A, "co_the_rut": A, "nguoi_ghi": A}),
        ("one", {"du_dong_y": 1, "co_the_rut": 1, "nguoi_ghi": 1}),
    )
    for event, state in (
        ("dong_y", "da_xem"),
        ("dong_y", "dong_y"),
        ("rut", "da_gui"),
        ("da_di", "chot"),
    ):
        for label, facts in truthy:
            args = {
                "paper": paper_dict(state),
                "su_kien": event,
                "now": T,
                "facts": facts,
            }
            out.append(case("chuyen", f"facts/{event}/{state}/{label}", args))
    out.append(
        case(
            "chuyen",
            "no_deadline",
            {
                "paper": paper_dict("dong_y", None),
                "su_kien": "dong_y",
                "now": T,
                "facts": {},
            },
        )
    )
    out.append(
        case(
            "chuyen",
            "deadline_at_now",
            {"paper": paper_dict("da_xem", T), "su_kien": "xem", "now": T, "facts": {}},
        )
    )

    good = {
        "ngay": ["date", "2030-09-21"],
        "gio": "18:30",
        "viec": "Ăn tối",
        "di_tiep": None,
        "ly_do": "",
    }
    for name, routine, rang_buoc, now in (
        ("template", good, 0, T),
        ("with_constraints", good, 1, T),
        ("three_constraints", good, 3, T),
        (
            "next_stop",
            dict(good, di_tiep={"gio": "20:00", "viec": "Đi bộ, rồi chè"}),
            0,
            T,
        ),
        ("empty_next_stop_is_none", dict(good, di_tiep={}), 0, T),
        ("reason", dict(good, ly_do="Ba tuần liền hai bạn ăn ở cùng một khu."), 0, T),
        ("reason_none", dict(good, ly_do=None), 0, T),
        ("reason_zero_text", dict(good, ly_do="0"), 0, T),
        ("reason_missing", {k: v for k, v in good.items() if k != "ly_do"}, 0, T),
        ("date_as_str", dict(good, ngay=["str", "2030-09-21"]), 0, T),
        ("date_none", dict(good, ngay=["none"]), 1, T),
        ("year_one", dict(good, ngay=["date", "0001-01-01"]), 0, T),
        ("clock_offset", good, 0, T.astimezone(KATHMANDU)),
        ("clock_seconds_offset", good, 0, (T + MICRO).astimezone(SAIGON_LMT)),
        ("clock_west", good, 0, T.astimezone(ONE_SECOND_WEST)),
        (
            "hostile_text",
            dict(good, gio=TEXTS[11], viec=TEXTS[10], ly_do=TEXTS[12]),
            2,
            T,
        ),
        (
            "unicode",
            dict(
                good,
                gio=CJK_MIDDLE,
                viec=GRINNING_FACE,
                di_tiep={"gio": "", "viec": " "},
            ),
            0,
            T,
        ),
    ):
        out.append(
            case(
                "phac_to_giay",
                name,
                {"routine": routine, "rang_buoc": rang_buoc, "now": now},
            )
        )
    # lam_giau_phac: the template told this notebook's agreed history.
    lau = ["p-lau", "Lẩu gà lá é", "quan-an-local", ["lẩu", "gà"], [], 46, 120]
    nuong = ["p-nuong", "Tiệm nướng", "quan-an-local", ["nướng"], ["khói"], 48, 90]
    oc = ["p-oc", "Ốc đêm", "quan-an-local", ["HẢI SẢN", 7, None], [], 49, 10]
    bun = ["p-bun", "Bún bò", "quan-an-local", [], [], 48, 200]
    cafe = ["p-cafe", "Lưng chừng", "cafe", [], ["yên tĩnh"], 45, 5]
    khong = ["p-khong", "Không điểm", "quan-an-local", [], [], None, None]
    da_di = ["2030-09-14", [["19:30", "Ăn lẩu", "p-lau"], ["21:00", "Dạo hồ", None]]]
    tu_do = ["2030-09-07", [["20:15", "Ăn ở nhà bạn", None]]]
    long_name = ["p-dai", "Quán " + "rất " * 60, "quan-an-local", [], [], 50, 1]
    for name, rang_buoc, lich_su, cho_cu, ung_vien in (
        ("no_history", [], [], None, []),
        ("history_without_places", [], [tu_do], None, []),
        ("history_empty_stops_skipped", [], [["2030-09-14", []], tu_do], None, []),
        ("history_place_gone", [], [da_di], None, []),
        ("history_place_no_candidates", [], [da_di], lau, [lau]),
        ("best_rated_new_place", [], [da_di], lau, [bun, lau, nuong, oc]),
        ("tie_on_rating_takes_more_ratings", [], [da_di], lau, [nuong, bun]),
        ("exact_tie_takes_the_earlier_row", [], [da_di], lau, [bun, ["p-bun2", "Bún bò 2", "quan-an-local", [], [], 48, 200]]),
        ("none_rating_ranks_last", [], [da_di], lau, [khong, nuong]),
        ("only_unrated", [], [da_di], lau, [khong]),
        ("constraint_blocks_by_kind_folded", ["Hải sản"], [da_di], lau, [oc]),
        ("constraint_blocks_by_name", ["x; TIỆM NƯỚNG"], [da_di], lau, [nuong, bun]),
        ("constraint_blocks_by_trait", ["khói/y"], [da_di], lau, [nuong]),
        ("constraint_one_letter_ignored", ["b, ,  "], [da_di], lau, [bun]),
        ("constraint_unrelated_keeps_place", ["Đừng hát karaoke"], [da_di], lau, [bun]),
        ("constraint_non_str_kind_skipped", ["7"], [da_di], lau, [oc]),
        ("constraint_dotted_capital_i", ["istanbul"], [da_di], lau, [["p-i", "İstanbul", "quan-an-local", [], [], 40, 1]]),
        ("cafe_names_the_stop", [], [da_di], lau, [cafe]),
        ("older_place_is_the_reference", [], [tu_do, da_di], lau, [bun]),
        ("visited_elsewhere_in_history", [], [["2030-09-21", [["19:00", "Bún", "p-bun"]]], da_di], lau, [bun, nuong]),
        ("long_name_shortens_both_sentences", [], [da_di], lau, [["p-dai2", "Quán " + "x" * 80, "quan-an-local", [], [], 50, 1]]),
        ("longer_name_drops_history_sentence", [], [da_di], lau, [["p-dai3", "Quán " + "x" * 115, "quan-an-local", [], [], 50, 1]]),
        ("too_long_name_not_proposed", [], [da_di], lau, [long_name]),
    ):
        out.append(
            case(
                "lam_giau_phac",
                name,
                {
                    "routine": good,
                    "rang_buoc": rang_buoc,
                    "now": T,
                    "lich_su": lich_su,
                    "cho_cu": cho_cu,
                    "ung_vien": ung_vien,
                },
            )
        )
    out.append(
        case(
            "lam_giau_phac",
            "reason_and_next_stop_kept",
            {
                "routine": dict(good, ly_do="Nếp nhớ.", di_tiep={"gio": "21:00", "viec": "Chè"}),
                "rang_buoc": ["Hải sản"],
                "now": T,
                "lich_su": [da_di],
                "cho_cu": lau,
                "ung_vien": [oc, bun],
            },
        )
    )
    out.append(
        case(
            "lam_giau_phac",
            "hostile_texts",
            {
                "routine": good,
                "rang_buoc": [TEXTS[7], TEXTS[10]],
                "now": T,
                "lich_su": [["2030-09-14", [[TEXTS[11], TEXTS[12], "p-lau"]]]],
                "cho_cu": [ "p-lau", TEXTS[9], "quan-an-local", [TEXTS[5]], [], 10, 1],
                "ung_vien": [["p-x", TEXTS[8], "quan-an-local", [TEXTS[6]], [TEXTS[13]], 30, 2]],
            },
        )
    )
    # ADR-0034 §2.2: tastes Nếp may use, and the draft told them.
    for name, chia, gu, ten, ca_hai in (
        ("both_share_common_first", ["TOI-ID", "KIA-ID"], {"TOI-ID": ["game", "cafe"], "KIA-ID": ["cafe", "outdoor", "tag-bo"]}, {"TOI-ID": "Linh", "KIA-ID": "Minh"}, True),
        ("one_shares", ["KIA-ID"], {"KIA-ID": ["nightlife", "an-uong"]}, {"KIA-ID": "Minh"}, False),
        ("sharer_without_tags", ["KIA-ID"], {}, {"KIA-ID": "Minh"}, False),
        ("sharer_without_a_name", ["KIA-ID"], {"KIA-ID": ["cafe"]}, {}, False),
        ("two_share_but_not_both_participants", ["TOI-ID", "KIA-ID"], {"TOI-ID": ["cafe"], "KIA-ID": ["cafe"]}, {"TOI-ID": "Linh", "KIA-ID": "Minh"}, False),
        ("hostile_name", ["KIA-ID"], {"KIA-ID": ["cafe"]}, {"KIA-ID": TEXTS[7]}, False),
    ):
        out.append(case("gu_cho_nep", name, {"nguoi_chia": chia, "gu": gu, "ten": ten, "ca_hai": ca_hai}))
    chung_cafe = [["cafe", True, None, ["TOI-ID", "KIA-ID"]]]
    minh_nightlife = [["outdoor", False, "Minh", ["KIA-ID"]], ["nightlife", False, "Minh", ["KIA-ID"]]]
    cafe2 = ["p-cafe2", "Cafe Hải Sản", "cafe", [], [], 49, 3]
    cafe3 = ["p-cafe3", "Tiệm Chiều", "cafe", [], ["yên tĩnh"], 45, 9]
    for name, rang_buoc, lich_su, cho_cu, ung_vien, gu, ung_vien_gu, da_di_ids in (
        ("no_taste", [], [], None, [], [], [], []),
        ("no_usable_taste", [], [], None, [], [["outdoor", True, None, ["TOI-ID", "KIA-ID"]]], [], []),
        ("no_history_names_the_stop", [], [], None, [], chung_cafe, [], []),
        ("one_sharer_names_the_stop", ["Hải sản"], [], None, [], minh_nightlife, [], []),
        ("history_place_wins", [], [da_di], lau, [["p-lau2", "Lẩu Hai", "quan-an-local", [], [], 45, 3]], chung_cafe, [cafe, cafe3], ["p-lau"]),
        ("history_without_new_place_then_taste_picks", [], [da_di], lau, [], chung_cafe, [cafe, cafe3, cafe2], ["p-lau"]),
        ("taste_avoids_a_box", ["hải sản"], [da_di], lau, [], chung_cafe, [cafe2, cafe3], ["p-lau"]),
        ("taste_skips_visited_and_other_kinds", [], [da_di], lau, [], chung_cafe, [cafe, ["p-x", "Khu Vui", "vui-choi", [], [], 50, 9]], ["p-lau", "p-cafe"]),
        ("long_name_falls_back", [], [da_di], lau, [], chung_cafe, [["p-dai", "Quán " + "x" * 190, "cafe", [], [], 50, 1]], ["p-lau"]),
    ):
        out.append(
            case(
                "lam_giau_theo_gu",
                name,
                {
                    "routine": good,
                    "rang_buoc": rang_buoc,
                    "now": T,
                    "lich_su": lich_su,
                    "cho_cu": cho_cu,
                    "ung_vien": ung_vien,
                    "gu": gu,
                    "ung_vien_gu": ung_vien_gu,
                    "da_di": da_di_ids,
                },
            )
        )
    return out


def random_instant(rng: random.Random) -> datetime:
    roll = rng.random()
    if roll < 0.6:
        start, span = datetime(2020, 1, 1, tzinfo=UTC), 20 * 366 * 86400
    elif roll < 0.8:
        start, span = datetime(1900, 1, 1, tzinfo=UTC), 90 * 366 * 86400
    else:
        start, span = datetime(1, 1, 3, tzinfo=UTC), 9998 * 365 * 86400
    instant = start + timedelta(
        seconds=rng.randrange(span), microseconds=rng.randrange(10**6)
    )
    if instant > datetime(9999, 12, 20, tzinfo=UTC):
        instant = datetime(9999, 12, 20, tzinfo=UTC)
    if rng.random() < 0.2:
        instant = rng.choice(TRANSITIONS) + rng.choice(
            (-DAY, -SECOND, -MICRO, timedelta(0), MICRO, SECOND, 4 * DAY)
        )
    return instant.astimezone(rng.choice(OFFSETS)) if rng.random() < 0.3 else instant


def pair_paper_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 7)
    people = ("TOI", "KIA", "LA", "")
    states = list(pair_paper.PAPER_STATES) + ["", "xoa"]
    events = list(pair_paper._TU) + ["xoa_het"]
    out = []
    for i in range(count):
        kind = rng.choice(
            (
                "tuan_cua",
                "han_tuan",
                "ngay_de_xuat",
                "astimezone",
                "hieu_luc",
                "da_du_dong_y",
                "co_the_rut",
                "chuyen",
                "chuyen",
                "chuyen",
                "phac_to_giay",
            )
        )
        now = random_instant(rng)
        name = f"fuzz/{i}"
        if kind in ("tuan_cua", "han_tuan", "ngay_de_xuat", "astimezone"):
            out.append(case(kind, name, {"now": now}))
            continue
        base = now.astimezone(UTC)
        expires = rng.choice(
            (
                None,
                base,
                base - MICRO,
                base + MICRO,
                base + timedelta(hours=rng.randint(-200, 200)),
            )
        )
        paper = paper_dict(rng.choice(states), expires, rng.randint(1, 4))
        responses = [
            [
                rng.randint(1, 4),
                rng.choice(people),
                rng.choice(("dong_y", "de_nghi_sua")),
            ]
            for _ in range(rng.randint(0, 4))
        ]
        if kind == "hieu_luc":
            out.append(case(kind, name, {"paper": paper, "now": base}))
        elif kind == "da_du_dong_y":
            out.append(
                case(kind, name, {"responses": responses, "version": rng.randint(0, 4)})
            )
        elif kind == "co_the_rut":
            versions = [
                [
                    rng.randint(1, 4),
                    rng.choice(("human", "human", "nep")),
                    rng.choice(people + (None,)),
                ]
                for _ in range(rng.randint(0, 3))
            ]
            views = [
                [rng.randint(1, 4), rng.choice(people)]
                for _ in range(rng.randint(0, 2))
            ]
            args = {
                "paper": dict(
                    paper, state=rng.choice(("da_gui", "da_gui", "da_xem", "nhap"))
                ),
                "versions": versions,
                "views": views,
                "responses": responses,
                "actor_id": rng.choice(people),
            }
            out.append(case(kind, name, args))
        elif kind == "chuyen":
            facts = {
                key: rng.choice((True, False, None, "", "TOI", 0, 1))
                for key in ("du_dong_y", "co_the_rut", "nguoi_ghi")
                if rng.random() < 0.8
            }
            out.append(
                case(
                    kind,
                    name,
                    {
                        "paper": paper,
                        "su_kien": rng.choice(events),
                        "now": base,
                        "facts": facts,
                    },
                )
            )
        else:
            ngay = rng.choice(
                (
                    [
                        "date",
                        (
                            date(2030, 9, 21) + timedelta(days=rng.randint(-400, 400))
                        ).isoformat(),
                    ],
                    ["date", "2030-09-21"],
                    ["str", "2030-09-21"],
                    ["none"],
                )
            )
            routine = {
                "ngay": ngay,
                "gio": rng.choice(("18:30", "07:05", rng.choice(TEXTS))),
                "viec": rng.choice(("Ăn tối", rng.choice(TEXTS))),
                "di_tiep": rng.choice(
                    (None, {}, {"gio": "20:00", "viec": rng.choice(TEXTS)})
                ),
            }
            if rng.random() < 0.8:
                routine["ly_do"] = rng.choice((None, "", "0", rng.choice(TEXTS)))
            out.append(
                case(
                    kind,
                    name,
                    {
                        "routine": routine,
                        "rang_buoc": rng.choice((0, 0, 1, 2)),
                        "now": now,
                    },
                )
            )
    # lam_giau_phac draws from its own stream, so adding it left every case
    # above exactly as it was.
    rng = random.Random(seed * 1000 + 11)
    words = ("Hải sản", "hải sản", "HẢI SẢN", "lẩu", "Lẩu gà", "nướng", "khói", "cay", "bún", "İ", "i")
    ids = ("p-a", "p-b", "p-c", "p-d", "p-e")

    def row() -> list:
        return [
            rng.choice(ids),
            rng.choice(words + TEXTS),
            rng.choice(("quan-an-local", "cafe", "vui-choi", "di-choi-dem", "", "khac")),
            [rng.choice(words + TEXTS + (7, None)) for _ in range(rng.randint(0, 3))],
            [rng.choice(words + TEXTS) for _ in range(rng.randint(0, 2))],
            rng.choice((None, 0, 10, 45, 46, 48, 50)),
            rng.choice((None, 0, 1, 90, 200)),
        ]

    for i in range(max(1, count // 5)):
        lich_su = [
            [
                "2030-09-14",
                [
                    [rng.choice(("19:30", "07:05", rng.choice(TEXTS))), rng.choice(words + TEXTS), rng.choice(ids + (None, ""))]
                    for _ in range(rng.randint(0, 2))
                ],
            ]
            for _ in range(rng.randint(0, 3))
        ]
        routine = {
            "ngay": rng.choice((["date", "2030-09-21"], ["none"])),
            "gio": "18:30",
            "viec": rng.choice(("Ăn tối", rng.choice(TEXTS))),
            "di_tiep": rng.choice((None, {"gio": "21:00", "viec": "Chè"})),
            "ly_do": rng.choice(("", "Nếp nhớ.", rng.choice(TEXTS))),
        }
        out.append(
            case(
                "lam_giau_phac",
                f"fuzz-lam-giau/{i}",
                {
                    "routine": routine,
                    "rang_buoc": [
                        ", ".join(rng.choice(words + TEXTS) for _ in range(rng.randint(1, 3)))
                        for _ in range(rng.randint(0, 2))
                    ],
                    "now": random_instant(rng),
                    "lich_su": lich_su,
                    "cho_cu": rng.choice((None, row())),
                    "ung_vien": [row() for _ in range(rng.randint(0, 5))],
                },
            )
        )
    # ADR-0034: its own stream, so every case above stays as it was.
    rng = random.Random(seed * 1000 + 13)
    tags = ("an-uong", "cafe", "nightlife", "mon-local", "outdoor", "shopping", "karaoke", "game", "tag-bo", "")
    people = ("TOI-ID", "KIA-ID")
    for i in range(max(1, count // 10)):
        chia = rng.sample(people, rng.randint(0, 2))
        gu = {p: [rng.choice(tags) for _ in range(rng.randint(0, 4))] for p in people if rng.random() < 0.9}
        out.append(
            case(
                "gu_cho_nep",
                f"fuzz-gu/{i}",
                {"nguoi_chia": chia, "gu": gu, "ten": {p: rng.choice(("Minh", "", TEXTS[3])) for p in people}, "ca_hai": rng.random() < 0.7},
            )
        )
        spec = [
            [rng.choice(tags), rng.random() < 0.5, rng.choice((None, "Minh", rng.choice(TEXTS))), list(rng.sample(people, rng.randint(1, 2)))]
            for _ in range(rng.randint(0, 3))
        ]
        spec = [[t, c, (n if not c and n is not None else (None if c else "Minh")), ps] for t, c, n, ps in spec]
        out.append(
            case(
                "lam_giau_theo_gu",
                f"fuzz-theo-gu/{i}",
                {
                    "routine": {"ngay": ["date", "2030-09-21"], "gio": "18:30", "viec": "Ăn tối", "di_tiep": None, "ly_do": rng.choice(("", "Nếp nhớ."))},
                    "rang_buoc": [rng.choice(words + TEXTS) for _ in range(rng.randint(0, 2))],
                    "now": random_instant(rng),
                    "lich_su": rng.choice(([], [["2030-09-14", [["19:30", "Ăn", rng.choice(ids)]]]])),
                    "cho_cu": rng.choice((None, row())),
                    "ung_vien": [row() for _ in range(rng.randint(0, 3))],
                    "gu": spec,
                    "ung_vien_gu": [row() for _ in range(rng.randint(0, 5))],
                    "da_di": [rng.choice(ids) for _ in range(rng.randint(0, 2))],
                },
            )
        )
    return out


# ---------------------------------------------------------------------------
# pair steps: real ApiService methods over a recording stub repository
# ---------------------------------------------------------------------------


class Unscripted(BaseException):
    """The stub was asked for a value the case did not script: a generator bug."""


WORLD_DEFAULTS = {
    "context": "pair",
    "member": True,
    "roster": [["TOI", "active"], ["KIA", "active"]],
    "locks": [],
    "notebooks": [],
    "proposal": None,
    "papers": [],
    "reads": [],
    "outings": [],
    "conflicts": {},
    "constraint_version": 1,
    # [[catalogue id, name], ...]: the rows get_place finds.
    "places": [],
    # {person name: [tag, ...]}: interests_by_person (ADR-0034 taste).
    "interests": {},
    # None: no week choice stored; [name or None]: the chosen «Người lo».
    "rhythm": None,
}


def notebook_record(n: dict | None) -> PairNotebookRecord | None:
    if n is None:
        return None
    return PairNotebookRecord(
        id=U(n["id"]),
        context_id=U("CAP"),
        cycle_id=UN(n["cycle"]),
        cycle_state=n["state"],
        terms_version=1,
        participants=tuple(U(p) for p in n["participants"]),
        consents=tuple(
            PairConsentRecord(
                proposal_id=U(proposal),
                person_id=U(person),
                purpose=purpose,
                granted_at=granted_at,
                revoked_at=revoked_at,
                proposal_expires_at=expires_at,
                terms_version=1,
            )
            for person, purpose, granted_at, revoked_at, expires_at, proposal in n["consents"]
        ),
        proposals=tuple(proposal_record(row) for row in n["proposals"]),
        constraints=tuple(
            PairConstraintRecord(
                owner_id=U(owner),
                kind=kind,
                content=text,
                version=version,
                updated_at=T - DAY,
            )
            for owner, kind, text, version in n["constraints"]
        ),
    )


def proposal_record(row: list | None) -> PairProposalRecord | None:
    if row is None:
        return None
    proposal_id, cycle, purpose, by, completed_at, expires_at = row
    return PairProposalRecord(
        id=U(proposal_id),
        cycle_id=U(cycle),
        purpose=purpose,
        proposed_by_id=U(by),
        terms_version=1,
        completed_at=completed_at,
        created_at=T - DAY,
        expires_at=expires_at,
    )


def paper_record(p: dict | None) -> PairPaperRecord | None:
    if p is None:
        return None
    return PairPaperRecord(
        id=U(p["id"]),
        context_id=U(p["context"]),
        # A sheet of a kept notebook names its cycle; one before it is the
        # temporary invitation (ADR-0027 §4).
        cycle_id=UN(p["cycle"]),
        is_temporary=p["cycle"] is None,
        draft_owner_id=U(p["owner"]),
        state=p["state"],
        current_version=p["version"],
        tuan=p["tuan"],
        expires_at=p["expires"],
        created_at=T - 7 * DAY,
        done_recorded_by_id=None,
        done_recorded_at=None,
        outing_id=UN(p["outing"]),
        versions=tuple(
            PairVersionRecord(
                version=v,
                content=body.value,
                ly_do=ly_do,
                nguon={},
                author_type=author,
                sent_at=sent_at,
                sent_by=UN(sent_by),
            )
            for v, body, ly_do, author, sent_at, sent_by in p["versions"]
        ),
        views=tuple(
            PairViewRecord(version=v, person_id=U(who), seen_at=seen_at)
            for v, who, seen_at in p["views"]
        ),
        responses=tuple(
            PairResponseRecord(
                version=v, person_id=U(who), kind=kind, created_at=T - HOUR
            )
            for v, who, kind in p["responses"]
        ),
        keeps=tuple(
            PairKeepRecord(id=U(keep_id), person_id=U("TOI"), line=line, created_at=at)
            for keep_id, line, at in p["keeps"]
        ),
    )


class Stub:
    """A repository that answers from the case's world and records every call."""

    def __init__(self, world: dict):
        self.world = {**WORLD_DEFAULTS, **world}
        # Without its own entry, the locked row is the row the first read found.
        self.world.setdefault("locked", self.world["reads"][:1])
        self.calls: list = []
        self.queues = {
            key: list(self.world[key])
            for key in ("locks", "notebooks", "reads", "locked", "outings")
        }
        self.conflicts = {
            key: list(codes) for key, codes in self.world["conflicts"].items()
        }

    def rec(self, name: str, *args: object) -> None:
        self.calls.append([name, *args])

    def take(self, key: str):
        if not self.queues[key]:
            raise Unscripted(key)
        return self.queues[key].pop(0)

    def maybe_conflict(self, name: str) -> None:
        codes = self.conflicts.get(name)
        if codes:
            code = codes.pop(0)
            if code is not None:
                raise RepositoryConflict(code)

    def get_context(self, context_id):
        self.rec("get_context", context_id)
        kind = self.world["context"]
        return None if kind is None else SimpleNamespace(kind=kind)

    def is_member(self, context_id, person_id):
        self.rec("is_member", context_id, person_id)
        return self.world["member"]

    def list_members(self, context_id):
        self.rec("list_members", context_id)
        return [
            SimpleNamespace(person_id=U(person), state=state, display_name=f"Tên {person}")
            for person, state in self.world["roster"]
        ]

    def get_pair_notebook(self, context_id):
        self.rec("get_pair_notebook", context_id)
        return notebook_record(self.take("notebooks"))

    def create_pair_notebook(self, context_id, *, now):
        self.rec("create_pair_notebook", context_id, now)

    def lock_pair_notebook(self, context_id):
        self.rec("lock_pair_notebook", context_id)
        return notebook_record(self.take("locks"))

    def open_pair_cycle(self, notebook_id, *, participants, terms_version, now):
        self.rec("open_pair_cycle", notebook_id, list(participants), terms_version, now)
        return U("CYN")

    def activate_pair_cycle(self, cycle_id, *, now):
        self.rec("activate_pair_cycle", cycle_id, now)

    def close_pair_cycle(self, cycle_id, *, now):
        self.rec("close_pair_cycle", cycle_id, now)

    def create_consent_proposal(
        self, *, cycle_id, purpose, proposed_by_id, terms_version, expires_at, now
    ):
        self.rec(
            "create_consent_proposal",
            cycle_id,
            purpose,
            proposed_by_id,
            terms_version,
            expires_at,
            now,
        )
        return PairProposalRecord(
            id=U("PRN"),
            cycle_id=cycle_id,
            purpose=purpose,
            proposed_by_id=proposed_by_id,
            terms_version=terms_version,
            completed_at=None,
            created_at=now,
            expires_at=expires_at,
        )

    def get_consent_proposal(self, proposal_id):
        self.rec("get_consent_proposal", proposal_id)
        return proposal_record(self.world["proposal"])

    def grant_consent(self, proposal_id, person_id, *, now):
        self.rec("grant_consent", proposal_id, person_id, now)

    def complete_consent_proposal(self, proposal_id, *, now):
        self.rec("complete_consent_proposal", proposal_id, now)

    def revoke_consents(self, cycle_id, purpose, person_id, *, now):
        self.rec("revoke_consents", cycle_id, purpose, person_id, now)
        return 1

    def set_couple_member(self, person_id, cycle_id, *, now):
        self.rec("set_couple_member", person_id, cycle_id, now)
        self.maybe_conflict("set_couple_member")

    def clear_couple_member(self, person_id):
        self.rec("clear_couple_member", person_id)

    def set_pair_constraint(self, *, cycle_id, owner_id, kind, content, now):
        self.rec("set_pair_constraint", cycle_id, owner_id, kind, content, now)
        return PairConstraintRecord(
            owner_id=owner_id,
            kind=kind,
            content=content,
            version=self.world["constraint_version"],
            updated_at=now,
        )

    def delete_pair_constraint(self, cycle_id, owner_id, kind):
        self.rec("delete_pair_constraint", cycle_id, owner_id, kind)
        return True

    def get_pair_rhythm(self, cycle_id, tuan):
        self.rec("get_pair_rhythm", cycle_id, tuan)
        chon = self.world["rhythm"]
        if chon is None:
            return None
        return SimpleNamespace(nguoi_lo_id=UN(chon[0]))

    def set_pair_rhythm(self, *, cycle_id, tuan, nguoi_lo_id, chon_boi_id, now):
        self.rec("set_pair_rhythm", cycle_id, tuan, nguoi_lo_id, chon_boi_id, now)
        # What the next read of this week finds.
        self.world["rhythm"] = [None if nguoi_lo_id is None else ALIAS_OF[nguoi_lo_id]]
        return SimpleNamespace(nguoi_lo_id=nguoi_lo_id)

    def create_pair_paper(
        self,
        *,
        context_id,
        cycle_id,
        draft_owner_id,
        tuan,
        expires_at,
        content,
        ly_do,
        nguon,
        author_type,
        now,
    ):
        self.rec(
            "create_pair_paper",
            context_id,
            cycle_id,
            draft_owner_id,
            tuan,
            expires_at,
            content,
            ly_do,
            nguon,
            author_type,
            now,
        )
        return PairPaperRecord(
            id=U("PPN"),
            context_id=context_id,
            cycle_id=cycle_id,
            is_temporary=cycle_id is None,
            draft_owner_id=draft_owner_id,
            state="nhap",
            current_version=1,
            tuan=tuan,
            expires_at=expires_at,
            created_at=now,
            done_recorded_by_id=None,
            done_recorded_at=None,
            outing_id=None,
            versions=(),
            views=(),
            responses=(),
            keeps=(),
        )

    def get_pair_paper(self, paper_id):
        self.rec("get_pair_paper", paper_id)
        return paper_record(self.take("reads"))

    def lock_pair_paper(self, paper_id):
        self.rec("lock_pair_paper", paper_id)
        return paper_record(self.take("locked"))

    def list_pair_papers(self, context_id):
        self.rec("list_pair_papers", context_id)
        return tuple(paper_record(p) for p in self.world["papers"])

    def update_pair_draft(self, paper_id, *, content, ly_do):
        self.rec("update_pair_draft", paper_id, content, ly_do)

    def add_paper_version(
        self,
        *,
        paper_id,
        version,
        content,
        ly_do,
        nguon,
        author_type,
        sent_at,
        sent_by,
        now,
    ):
        self.rec(
            "add_paper_version",
            paper_id,
            version,
            content,
            ly_do,
            nguon,
            author_type,
            sent_at,
            sent_by,
            now,
        )

    def mark_version_sent(self, paper_id, version, *, sent_by, now):
        self.rec("mark_version_sent", paper_id, version, sent_by, now)

    def set_paper_state(
        self, paper_id, state, *, now, current_version=None, recorded_by_id=None
    ):
        self.rec(
            "set_paper_state", paper_id, state, now, current_version, recorded_by_id
        )

    def mark_paper_viewed(self, paper_id, version, person_id, *, now):
        self.rec("mark_paper_viewed", paper_id, version, person_id, now)
        return now

    def add_paper_response(self, *, paper_id, version, person_id, kind, now):
        self.rec("add_paper_response", paper_id, version, person_id, kind, now)
        self.maybe_conflict("add_paper_response")

    def link_paper_outing(self, *, paper_id, version, outing_id, now):
        self.rec("link_paper_outing", paper_id, version, outing_id, now)
        self.maybe_conflict("link_paper_outing")

    def get_paper_outing(self, paper_id):
        self.rec("get_paper_outing", paper_id)
        return UN(self.take("outings"))

    def add_paper_keep(self, *, paper_id, person_id, line, now):
        self.rec("add_paper_keep", paper_id, person_id, line, now)
        return PairKeepRecord(
            id=U("KPN"), person_id=person_id, line=line, created_at=now
        )

    def close_open_pair_papers(self, context_id, *, now):
        self.rec("close_open_pair_papers", context_id, now)
        return {}

    def create_outing(
        self,
        *,
        context_id,
        created_by_id,
        title,
        starts_on,
        ends_on,
        headcount,
        budget_per_person_vnd,
        now,
    ):
        self.rec(
            "create_outing",
            context_id,
            created_by_id,
            title,
            starts_on,
            ends_on,
            headcount,
            budget_per_person_vnd,
            now,
        )
        return SimpleNamespace(id=U("OUN"))

    def place_rows(self) -> list[dict]:
        """The world's catalogue: [id, name] or [id, name, destination,
        category, kinds, traits, rating*10, count]."""
        out = []
        for entry in self.world["places"]:
            pid, name, dest, cat, kinds, traits, rating, count = (
                list(entry) + ["d-mau", "quan-an-local", [], [], None, None][len(entry) - 2 :]
            )
            out.append(
                {
                    "id": pid,
                    "name": name,
                    "destination_id": dest,
                    "category": cat,
                    "kinds": kinds,
                    "traits": traits,
                    "rating": None if rating is None else rating / 10,
                    "rating_count": count,
                }
            )
        return out

    @staticmethod
    def place_record(row: dict):
        return SimpleNamespace(
            destination_id=row["destination_id"],
            category=row["category"],
            to_row=lambda row=row: dict(row),
        )

    def interests_by_person(self, person_ids):
        self.rec("interests_by_person", list(person_ids))
        by_id = {U(name): list(tags) for name, tags in self.world["interests"].items()}
        return {pid: by_id[pid] for pid in person_ids if by_id.get(pid)}

    def get_place(self, place_id):
        self.rec("get_place", place_id)
        for row in self.place_rows():
            if row["id"] == place_id:
                return self.place_record(row)
        return None

    def list_places(self, *, destination_id=None, category=None):
        self.rec("list_places", destination_id, category)
        return [
            self.place_record(row)
            for row in self.place_rows()
            if row["destination_id"] == destination_id and row["category"] == category
        ]

    def replace_outing_stops(self, *, outing_id, stops, expected_revision=None):
        assert expected_revision is None
        self.rec("replace_outing_stops", outing_id, [dict(stop) for stop in stops])


def content_input(spec: dict):
    return schemas.PaperContentInput.model_construct(
        ngay=date.fromisoformat(spec["ngay"]),
        chang=[
            schemas.PaperStopInput.model_construct(
                gio=gio,
                viec=viec,
                place_id=place,
                can_kiem=can_kiem,
            )
            for gio, viec, place, can_kiem in spec["chang"]
        ],
    )


def reply_request(spec: dict):
    if spec["kind"] == "dong_y":
        return schemas.PaperAgreeRequest.model_construct(kind="dong_y")
    return schemas.PaperReviseRequest.model_construct(
        kind=spec["kind"], content=content_input(spec["content"]), ly_do=spec["ly_do"]
    )


CALLERS = {
    "pair_notebook": lambda s, a, r: s.pair_notebook(U(r["context_id"]), a),
    "propose_pair_consent": lambda s, a, r: s.propose_pair_consent(
        U(r["context_id"]),
        schemas.PairProposalCreateRequest.model_construct(purpose=r["purpose"]),
        a,
    ),
    "grant_pair_consent": lambda s, a, r: s.grant_pair_consent(
        U(r["context_id"]), U(r["proposal_id"]), a
    ),
    "revoke_pair_consent": lambda s, a, r: s.revoke_pair_consent(
        U(r["context_id"]), r["purpose"], a
    ),
    "put_pair_constraint": lambda s, a, r: s.put_pair_constraint(
        U(r["context_id"]),
        r["kind"],
        schemas.PairConstraintPutRequest.model_construct(content=r["content"]),
        a,
    ),
    "delete_pair_constraint": lambda s, a, r: s.delete_pair_constraint(
        U(r["context_id"]), r["kind"], a
    ),
    "preview_close_pair_notebook": lambda s, a, r: s.preview_close_pair_notebook(
        U(r["context_id"]), a
    ),
    "close_pair_notebook": lambda s, a, r: s.close_pair_notebook(
        U(r["context_id"]),
        schemas.CloseNotebookRequest.model_construct(revision=r["revision"]),
        a,
    ),
    "set_pair_week_role": lambda s, a, r: s.set_pair_week_role(
        U(r["context_id"]), schemas.PairWeekRoleRequest.model_construct(lo=r["lo"]), a
    ),
    "list_pair_papers": lambda s, a, r: s.list_pair_papers(U(r["context_id"]), a),
    "draft_pair_paper": lambda s, a, r: s.draft_pair_paper(U(r["context_id"]), a),
    "pair_paper": lambda s, a, r: s.pair_paper(U(r["paper_id"]), a),
    "edit_pair_draft": lambda s, a, r: s.edit_pair_draft(
        U(r["paper_id"]),
        schemas.PaperDraftEditRequest.model_construct(
            content=content_input(r["content"]), ly_do=r["ly_do"]
        ),
        a,
    ),
    "send_pair_paper": lambda s, a, r: s.send_pair_paper(
        U(r["paper_id"]),
        schemas.PaperSendRequest.model_construct(version=r["version"]),
        a,
    ),
    "mark_pair_paper_viewed": lambda s, a, r: s.mark_pair_paper_viewed(
        U(r["paper_id"]), r["version"], a
    ),
    "respond_pair_paper": lambda s, a, r: s.respond_pair_paper(
        U(r["paper_id"]), r["version"], reply_request(r["reply"]), a
    ),
    "withdraw_pair_paper": lambda s, a, r: s.withdraw_pair_paper(
        U(r["paper_id"]),
        schemas.PaperWithdrawRequest.model_construct(version=r["version"]),
        a,
    ),
    "skip_pair_week": lambda s, a, r: s.skip_pair_week(U(r["paper_id"]), a),
    "record_pair_outing_done": lambda s, a, r: s.record_pair_outing_done(
        U(r["paper_id"]), a
    ),
    "keep_pair_paper_line": lambda s, a, r: s.keep_pair_paper_line(
        U(r["paper_id"]), schemas.PaperKeepRequest.model_construct(line=r["line"]), a
    ),
}
CONTEXT_METHODS = tuple(CALLERS)[:11]
PAPER_METHODS = tuple(CALLERS)[11:]


def run_step(fn: str, now: datetime, actor: list, req: dict, world: dict) -> dict:
    api_service._now = lambda: now
    stub = Stub(world)
    who = Actor(id=U(actor[0]), roles=frozenset(actor[1]), context_ids=frozenset())
    out = {"calls": stub.calls, "problem": None, "raised": None, "response": None}
    try:
        response = CALLERS[fn](
            api_service.ApiService(stub, photo_storage=object()), who, req
        )
    except ApiProblem as exc:
        out["problem"] = {
            "status": exc.status_code,
            "code": exc.code,
            "detail": exc.detail,
        }
    except (
        RepositoryConflict,
        AssertionError,
        PermissionError_,
        pair_paper.PaperError,
    ) as exc:
        out["raised"] = {"type": type(exc).__name__, "code": getattr(exc, "code", None)}
    else:
        out["response"] = None if response is None else response.model_dump()
    return out


def step_case(
    fn: str,
    name: str,
    req: dict,
    world: dict,
    now: datetime = T,
    actor=("TOI", ("member",)),
) -> dict:
    return case(
        fn,
        name,
        {"now": now, "actor": [actor[0], list(actor[1])], "req": req, "world": world},
    )


# --- worlds ------------------------------------------------------------------


def nb(
    cycle="CY1",
    state="active",
    participants=("TOI", "KIA"),
    consents=(),
    proposals=(),
    constraints=(),
    nb_id="NB1",
) -> dict:
    return {
        "id": nb_id,
        "cycle": cycle,
        "state": state,
        "participants": list(participants),
        "consents": [list(row) for row in consents],
        "proposals": [list(row) for row in proposals],
        "constraints": [list(row) for row in constraints],
    }


NB_NONE = nb(cycle=None, state=None, participants=())


def grant(person, purpose, granted=T - HOUR, revoked=None, expires=T + 6 * DAY, proposal="PR1") -> list:
    # The sixth field names the proposal the answer belongs to (2026-09-23:
    # agreement and «does this offer still stand» are per proposal).
    return [person, purpose, granted, revoked, expires, proposal]


def both(purpose, **kw) -> list:
    return [grant("TOI", purpose, **kw), grant("KIA", purpose, **kw)]


def prop(
    proposal_id, purpose, by="KIA", cycle="CY1", completed=None, expires=T + 6 * DAY
) -> list:
    return [proposal_id, cycle, purpose, by, completed, expires]


NB_PENDING = nb(
    state="pending",
    consents=[grant("KIA", "lap_so")],
    proposals=[prop("PR1", "lap_so")],
)
NB_ACTIVE = nb(
    consents=both("lap_so"), proposals=[prop("PR1", "lap_so", completed=T - HOUR)]
)


def body(
    ngay="2030-09-21", stops=(("19:00", "Ăn tối"),), place=None, can_kiem=True
) -> dict:
    return {
        "ngay": ngay,
        "chang": [
            {"gio": gio, "viec": viec, "place_id": place, "can_kiem": can_kiem}
            for gio, viec in stops
        ],
    }


def ver(
    v=1, content=None, ly_do=None, author="human", sent_at=T - HOUR, sent_by="TOI"
) -> list:
    return [
        v,
        J(body() if content is None else content),
        ly_do,
        author,
        sent_at,
        sent_by,
    ]


def paper(
    paper_id="PP1",
    owner="TOI",
    state="da_gui",
    version=1,
    versions=None,
    views=(),
    responses=(),
    keeps=(),
    expires=WEEK_END,
    tuan=TUAN,
    outing=None,
    context="CAP",
    cycle=None,
) -> dict:
    if versions is None:
        draft = state == "nhap"
        versions = [
            ver(
                n, sent_at=None if draft else T - HOUR, sent_by=None if draft else owner
            )
            for n in range(1, version + 1)
        ]
    return {
        "id": paper_id,
        "context": context,
        "owner": owner,
        "state": state,
        "version": version,
        "tuan": tuan,
        "expires": expires,
        "outing": outing,
        "versions": versions,
        "views": [list(row) for row in views],
        "responses": [list(row) for row in responses],
        "keeps": [list(row) for row in keeps],
        "cycle": cycle,
    }


def content_in(ngay="2030-09-21", stops=(("19:00", "Ăn tối", None, True),)) -> dict:
    return {"ngay": ngay, "chang": [list(stop) for stop in stops]}


PLACE = str(U("PL1"))

DEFAULT_REQ = {
    "pair_notebook": {"context_id": "CAP"},
    "propose_pair_consent": {"context_id": "CAP", "purpose": "lap_so"},
    "grant_pair_consent": {"context_id": "CAP", "proposal_id": "PR1"},
    "revoke_pair_consent": {"context_id": "CAP", "purpose": "bat_doi"},
    "put_pair_constraint": {"context_id": "CAP", "kind": "dung", "content": "Hải sản"},
    "delete_pair_constraint": {"context_id": "CAP", "kind": "dung"},
    "preview_close_pair_notebook": {"context_id": "CAP"},
    "close_pair_notebook": {"context_id": "CAP", "revision": "stale"},
    "set_pair_week_role": {"context_id": "CAP", "lo": "toi"},
    "list_pair_papers": {"context_id": "CAP"},
    "draft_pair_paper": {"context_id": "CAP"},
    "pair_paper": {"paper_id": "PP1"},
    "edit_pair_draft": {"paper_id": "PP1", "content": content_in(), "ly_do": None},
    "send_pair_paper": {"paper_id": "PP1", "version": 1},
    "mark_pair_paper_viewed": {"paper_id": "PP1", "version": 1},
    "respond_pair_paper": {
        "paper_id": "PP1",
        "version": 1,
        "reply": {"kind": "dong_y"},
    },
    "withdraw_pair_paper": {"paper_id": "PP1", "version": 1},
    "skip_pair_week": {"paper_id": "PP1"},
    "record_pair_outing_done": {"paper_id": "PP1"},
    "keep_pair_paper_line": {"paper_id": "PP1", "line": "Quán ngon"},
}


def req(fn: str, **over) -> dict:
    return {**DEFAULT_REQ[fn], **over}


def revision_of(world: dict, now: datetime) -> str:
    """The revision the service will compute from this world's first
    get_pair_notebook and its papers, by the real functions."""
    full = {**WORLD_DEFAULTS, **world}
    notebook = notebook_record(full["notebooks"][0]) if full["notebooks"] else None
    papers = [api_service._paper_dict(paper_record(p)) for p in full["papers"]]
    proposals = [] if notebook is None else api_service._proposals_as_dicts(notebook)
    return pair_notebook.xem_truoc_dong_so(papers, proposals, now=now)["revision"]


def door_world(fn: str) -> dict:
    """Just enough world to reach each route's doors."""
    if fn in PAPER_METHODS:
        return {"reads": [paper(owner="KIA")]}
    return {"locks": [nb()], "proposal": prop("PR1", "bat_doi")}


def pair_steps_constants() -> dict:
    return {
        "aliases": [[name, str(value)] for name, value in ALIASES.items()],
        "methods": list(CALLERS),
        "permission_refusals": [
            [name, list(answer)]
            for name, answer in api_service._TU_CHOI_TO_GIAY.items()
        ],
        "paper_error_details": [
            [code, text] for code, text in api_service._LOI_TO_GIAY.items()
        ],
        "dieu_khoan_hien_tai": api_service.DIEU_KHOAN_HIEN_TAI,
        "khung_mac_dinh": dict(api_service._KHUNG_MAC_DINH),
    }


def pair_steps_edges() -> list[dict]:
    out = []
    S = step_case

    # --- the doors every route shares -----------------------------------------
    for fn in CONTEXT_METHODS:
        w = door_world(fn)
        r = req(fn)
        out += [
            S(fn, "door/no_context", r, dict(w, context=None)),
            S(fn, "door/group", r, dict(w, context="group")),
            S(fn, "door/stranger", r, dict(w, member=False)),
            S(fn, "door/no_role", r, w, actor=("TOI", ())),
            S(fn, "door/unknown_role", r, w, actor=("TOI", ("member", "BOGUS"))),
        ]
    for fn in PAPER_METHODS:
        w = door_world(fn)
        r = req(fn)
        theirs = paper(owner="KIA", state="nhap")
        out += [
            S(fn, "door/no_paper", r, dict(w, reads=[None])),
            S(fn, "door/paper_in_group", r, dict(w, context="group")),
            S(fn, "door/stranger", r, dict(w, member=False)),
            S(fn, "door/others_draft", r, dict(w, reads=[theirs])),
            S(fn, "door/no_role", r, w, actor=("TOI", ())),
            S(fn, "door/unknown_role", r, w, actor=("TOI", ("BOGUS",))),
        ]
        if fn != "pair_paper":
            out.append(S(fn, "door/lock_lost", r, dict(w, locked=[None])))

    # --- pair_notebook -------------------------------------------------------------
    fn = "pair_notebook"
    for name, notebook, papers, roster, actor in (
        ("no_notebook", None, [], None, "TOI"),
        ("no_cycle", NB_NONE, [], None, "TOI"),
        ("pending_one_side", NB_PENDING, [], None, "TOI"),
        ("pending_seen_by_proposer", NB_PENDING, [], None, "KIA"),
        ("active", NB_ACTIVE, [], None, "TOI"),
        (
            "proposals_at_their_deadlines",
            nb(
                consents=both("lap_so")
                + [
                    grant("KIA", "bat_doi"),
                    grant("TOI", "doc_chat", revoked=T - MICRO),
                ],
                proposals=[
                    prop("PR1", "bat_doi", expires=T + MICRO),
                    prop("PR2", "doc_chat", expires=T),
                    prop("PR9", "bat_doi", expires=T - MICRO),
                    prop("PRN", "lap_so", completed=T - DAY),
                ],
                constraints=[
                    ["KIA", "dung", "Hải sản", 3],
                    ["TOI", "khong_an_duoc", "Đậu", 1],
                ],
            ),
            [],
            None,
            "TOI",
        ),
        (
            "grant_after_its_proposal_lapsed",
            nb(
                consents=[
                    grant("TOI", "bat_doi", expires=T),
                    grant("KIA", "bat_doi", expires=T + DAY),
                ]
            ),
            [],
            None,
            "TOI",
        ),
        (
            "cycle_list_without_me",
            nb(
                participants=("KIA", "LA"),
                consents=both("lap_so") + [grant("LA", "lap_so")],
            ),
            [],
            None,
            "TOI",
        ),
        (
            "cycle_list_only_me",
            nb(participants=("TOI",), consents=both("lap_so")),
            [],
            None,
            "TOI",
        ),
        (
            "empty_cycle_list",
            nb(participants=(), consents=both("lap_so")),
            [],
            None,
            "TOI",
        ),
        (
            "members_without_cycle",
            None,
            [],
            [["LA", "active"], ["TOI", "active"], ["KIA", "left"]],
            "TOI",
        ),
        ("members_only_me", None, [], [["TOI", "active"]], "TOI"),
        (
            "others_draft_skipped",
            NB_ACTIVE,
            [paper(owner="KIA", state="nhap"), paper("PP2", state="da_xem")],
            None,
            "TOI",
        ),
        (
            "own_draft_found",
            NB_ACTIVE,
            [paper(owner="TOI", state="nhap"), paper("PP2", state="da_xem")],
            None,
            "TOI",
        ),
        (
            "expired_skipped",
            NB_ACTIVE,
            [
                paper(state="dong_y", expires=T),
                paper("PP2", state="chot"),
                paper("PP3", state="da_gui", expires=T + MICRO),
            ],
            None,
            "TOI",
        ),
        (
            "nothing_open",
            NB_ACTIVE,
            [paper(state="chot"), paper("PP2", state="huy")],
            None,
            "TOI",
        ),
    ):
        w = {"notebooks": [notebook], "papers": papers}
        if roster is not None:
            w["roster"] = roster
        out.append(S(fn, name, req(fn), w, actor=(actor, ("member",))))

    # ADR-0034: taste is read only inside «Một đôi», per person.
    doi = both("bat_doi", proposal="PRD")
    doi_props = [prop("PRD", "bat_doi", completed=T - DAY)]
    gu = {"TOI": ["cafe", "an-uong"], "KIA": ["outdoor", "cafe", "tag-da-bo"]}
    for name, consents, proposals in (
        ("taste_friends_only", both("lap_so") + [grant("KIA", "chia_gu", proposal="PGK")], [prop("PGK", "chia_gu", completed=T - DAY)]),
        ("taste_couple_nobody_shares", doi, doi_props),
        ("taste_couple_they_share", doi + [grant("KIA", "chia_gu", proposal="PGK")], doi_props + [prop("PGK", "chia_gu", completed=T - DAY)]),
        ("taste_couple_i_share", doi + [grant("TOI", "chia_gu", proposal="PGT")], doi_props + [prop("PGT", "chia_gu", by="TOI", completed=T - DAY)]),
        (
            "taste_couple_both_share",
            doi + [grant("KIA", "chia_gu", proposal="PGK"), grant("TOI", "chia_gu", proposal="PGT")],
            doi_props + [prop("PGK", "chia_gu", completed=T - DAY), prop("PGT", "chia_gu", by="TOI", completed=T - DAY)],
        ),
        (
            "taste_they_took_it_back",
            doi + [grant("KIA", "chia_gu", proposal="PGK", revoked=T - MICRO), grant("TOI", "chia_gu", proposal="PGT")],
            doi_props + [prop("PGK", "chia_gu", completed=T - DAY), prop("PGT", "chia_gu", by="TOI", completed=T - DAY)],
        ),
    ):
        out.append(S(fn, name, req(fn), {"notebooks": [nb(consents=consents, proposals=proposals)], "papers": [], "interests": gu}, actor=("TOI", ("member",))))
    # ADR-0034 §2.4: «Người lo» of the week, inferred or chosen.
    doi_lap = doi + both("lap_so")
    doi_props_lap = doi_props + [prop("PR1", "lap_so", completed=T - DAY)]
    def gui(pid, ai, cycle="CY1"):
        return paper(pid, owner=ai, state="da_gui", cycle=cycle, versions=[ver(1, sent_by=ai)])
    for name, state, papers, rhythm in (
        ("role_inferred_opener", "active", [], None),
        ("role_sent_first_wins", "active", [gui("PP2", "TOI"), gui("PP3", "TOI"), gui("PP4", "KIA", cycle="CY2")], None),
        ("role_chosen_share", "active", [], [None]),
        ("role_chosen_me", "active", [gui("PP2", "KIA")], ["TOI"]),
        ("role_pending_cycle", "pending", [], ["TOI"]),
    ):
        out.append(
            S(fn, name, req(fn), {"notebooks": [nb(state=state, consents=doi_lap, proposals=doi_props_lap)], "papers": papers, "rhythm": rhythm}, actor=("TOI", ("member",)))
        )
    fn = "set_pair_week_role"
    for name, lo, locks, rhythm in (
        ("outside_a_couple", "toi", [nb(consents=both("lap_so"), proposals=[prop("PR1", "lap_so", completed=T - DAY)])], None),
        ("no_cycle", "toi", [NB_NONE], None),
        ("pending_cycle", "toi", [nb(state="pending", consents=doi_lap, proposals=doi_props_lap)], None),
        ("me", "toi", [nb(consents=doi_lap, proposals=doi_props_lap)], None),
        ("the_other", "nguoi_kia", [nb(consents=doi_lap, proposals=doi_props_lap)], None),
        ("both", "ca_hai", [nb(consents=doi_lap, proposals=doi_props_lap)], ["KIA"]),
        ("notebook_created", "toi", [None, NB_NONE], None),
    ):
        out.append(S(fn, name, req(fn, lo=lo), {"locks": locks, "papers": [], "rhythm": rhythm}))
    fn = "pair_notebook"

    # --- propose_pair_consent ------------------------------------------------------
    fn = "propose_pair_consent"
    for name, purpose, locks, roster in (
        ("first_offer_opens_a_cycle", "lap_so", [NB_NONE], None),
        ("first_offer_creates_the_notebook", "lap_so", [None, NB_NONE], None),
        ("notebook_never_appears", "lap_so", [None, None], None),
        ("one_member", "lap_so", [NB_NONE], [["TOI", "active"], ["KIA", "left"]]),
        (
            "three_members",
            "lap_so",
            [NB_NONE],
            [["TOI", "active"], ["KIA", "active"], ["LA", "active"]],
        ),
        ("again_in_a_pending_cycle", "lap_so", [NB_PENDING], None),
        ("couple_before_the_notebook", "bat_doi", [NB_NONE], None),
        ("couple_while_pending", "bat_doi", [NB_PENDING], [["TOI", "active"]]),
        ("chat_while_pending", "doc_chat", [NB_PENDING], None),
        ("couple_when_active", "bat_doi", [NB_ACTIVE], None),
        ("chat_when_active", "doc_chat", [NB_ACTIVE], [["TOI", "active"]]),
        ("lap_so_when_active", "lap_so", [NB_ACTIVE], None),
        # QA 23/09: one offer per rung. The proposer asking again gets the offer
        # already standing; the other person asking gets 409 and must answer it.
        (
            "again_by_the_proposer",
            "lap_so",
            [nb(state="pending", consents=[grant("TOI", "lap_so")], proposals=[prop("PR1", "lap_so", by="TOI")])],
            None,
        ),
        (
            "again_after_taking_ones_own_yes_back",
            "lap_so",
            [nb(state="pending", consents=[grant("TOI", "lap_so", revoked=T - HOUR)], proposals=[prop("PR1", "lap_so", by="TOI")])],
            None,
        ),
        (
            "couple_while_the_other_offers_it",
            "bat_doi",
            [nb(consents=both("lap_so") + [grant("KIA", "bat_doi", proposal="PR2")], proposals=[prop("PR1", "lap_so", completed=T - HOUR), prop("PR2", "bat_doi", by="KIA")])],
            None,
        ),
        (
            "couple_after_the_other_offer_lapsed",
            "bat_doi",
            [nb(consents=both("lap_so") + [grant("KIA", "bat_doi", proposal="PR2")], proposals=[prop("PR1", "lap_so", completed=T - HOUR), prop("PR2", "bat_doi", by="KIA", expires=T - HOUR)])],
            None,
        ),
    ):
        w = {"locks": locks}
        if roster is not None:
            w["roster"] = roster
        out.append(S(fn, name, req(fn, purpose=purpose), w))
    doi = both("lap_so") + both("bat_doi", proposal="PRD")
    doi_props = [prop("PR1", "lap_so", completed=T - DAY), prop("PRD", "bat_doi", completed=T - DAY)]
    for name, notebook in (
        ("taste_outside_a_couple", nb(consents=both("lap_so"), proposals=[prop("PR1", "lap_so", completed=T - DAY)])),
        ("taste_in_a_couple", nb(consents=doi, proposals=doi_props)),
        ("taste_while_the_other_shares", nb(consents=doi + [grant("KIA", "chia_gu", proposal="PGK")], proposals=doi_props + [prop("PGK", "chia_gu", completed=T - DAY)])),
        ("taste_again_while_on", nb(consents=doi + [grant("TOI", "chia_gu", proposal="PGT")], proposals=doi_props + [prop("PGT", "chia_gu", by="TOI", completed=T - DAY)])),
        (
            "taste_again_after_taking_it_back",
            nb(consents=doi + [grant("TOI", "chia_gu", proposal="PGT", revoked=T - HOUR)], proposals=doi_props + [prop("PGT", "chia_gu", by="TOI", completed=T - DAY)]),
        ),
    ):
        out.append(S(fn, name, req(fn, purpose="chia_gu"), {"locks": [notebook]}))
    out.append(
        S(
            fn,
            "offer_at_other_offset",
            req(fn),
            {"locks": [NB_NONE]},
            now=T.astimezone(KATHMANDU),
        )
    )

    # --- grant_pair_consent --------------------------------------------------------
    fn = "grant_pair_consent"
    lap_offer = prop("PR1", "lap_so", by="KIA")
    after_lap = nb(state="pending", consents=both("lap_so"), proposals=[lap_offer])
    after_bat = nb(
        consents=both("lap_so") + both("bat_doi"), proposals=[prop("PR1", "bat_doi")]
    )
    after_chat = nb(
        consents=both("lap_so") + both("doc_chat"), proposals=[prop("PR1", "doc_chat")]
    )
    for name, locks, proposal, after, conflicts, actor, extra in (
        ("second_yes_opens", [NB_PENDING], lap_offer, after_lap, {}, "TOI", {}),
        (
            "second_yes_not_yet_both",
            [NB_PENDING],
            lap_offer,
            nb(
                state="pending",
                consents=[grant("TOI", "lap_so")],
                proposals=[lap_offer],
            ),
            {},
            "TOI",
            {},
        ),
        ("couple", [NB_ACTIVE], prop("PR1", "bat_doi"), after_bat, {}, "TOI", {}),
        (
            "couple_first_slot_taken",
            [NB_ACTIVE],
            prop("PR1", "bat_doi"),
            after_bat,
            {"set_couple_member": ["couple_slot_taken"]},
            "TOI",
            {},
        ),
        (
            "couple_second_slot_taken",
            [NB_ACTIVE],
            prop("PR1", "bat_doi"),
            after_bat,
            {"set_couple_member": [None, "COUPLE_SLOT_TAKEN"]},
            "TOI",
            {},
        ),
        ("chat", [NB_ACTIVE], prop("PR1", "doc_chat"), after_chat, {}, "TOI", {}),
        ("no_proposal", [NB_ACTIVE], None, None, {}, "TOI", {}),
        ("no_cycle", [NB_NONE], lap_offer, None, {}, "TOI", {}),
        (
            "other_cycle",
            [NB_ACTIVE],
            prop("PR1", "bat_doi", cycle="CY2"),
            None,
            {},
            "TOI",
            {},
        ),
        ("own_offer", [NB_PENDING], lap_offer, None, {}, "KIA", {}),
        (
            "stranger_to_the_cycle",
            [nb(participants=("KIA", "LA"))],
            prop("PR1", "bat_doi"),
            None,
            {},
            "TOI",
            {},
        ),
        (
            "at_deadline",
            [NB_ACTIVE],
            prop("PR1", "bat_doi", expires=T),
            None,
            {},
            "TOI",
            {},
        ),
        (
            "micro_before_deadline",
            [NB_ACTIVE],
            prop("PR1", "bat_doi", expires=T + MICRO),
            after_bat,
            {},
            "TOI",
            {},
        ),
        (
            "completed_reads_as_expired",
            [NB_ACTIVE],
            prop("PR1", "bat_doi", completed=T - HOUR),
            None,
            {},
            "TOI",
            {},
        ),
        (
            "own_and_expired",
            [NB_ACTIVE],
            prop("PR1", "bat_doi", by="TOI", expires=T - DAY),
            None,
            {},
            "TOI",
            {},
        ),
        (
            "notebook_gone_after_grant",
            [NB_ACTIVE],
            prop("PR1", "bat_doi"),
            None,
            {},
            "TOI",
            {},
        ),
        (
            "created_then_locked",
            [None, NB_PENDING],
            lap_offer,
            after_lap,
            {},
            "TOI",
            {},
        ),
        (
            "members_when_no_cycle_list",
            [nb(participants=())],
            prop("PR1", "bat_doi"),
            nb(participants=(), consents=both("bat_doi")),
            {},
            "TOI",
            {},
        ),
        (
            "revoked_in_between",
            [NB_ACTIVE],
            prop("PR1", "bat_doi"),
            nb(
                consents=[
                    grant("TOI", "bat_doi"),
                    grant("KIA", "bat_doi", revoked=T - MICRO),
                ]
            ),
            {},
            "TOI",
            {},
        ),
    ):
        w = {
            "locks": locks,
            "proposal": proposal,
            "notebooks": [after],
            "conflicts": conflicts,
            **extra,
        }
        out.append(S(fn, name, req(fn), w, actor=(actor, ("member",))))

    # --- revoke_pair_consent -------------------------------------------------------
    fn = "revoke_pair_consent"
    for text in (
        "",
        "LAP_SO",
        " lap_so",
        "lap_so ",
        "doc-chat",
        "bat_doi\n",
        CJK_MIDDLE,
        TEXTS[12],
        "a" * 300,
    ):
        out.append(
            S(
                fn,
                f"unknown_purpose/{len(text)}/{text[:6]!r}",
                req(fn, purpose=text),
                {"context": None},
            )
        )
    nep_draft = paper(
        "PP1",
        state="nhap",
        versions=[ver(1, author="nep", sent_by=None, sent_at=T - HOUR)],
    )
    nep_later = paper(
        "PP2",
        state="nhap",
        version=2,
        versions=[
            ver(1, sent_by=None, sent_at=None),
            ver(2, author="nep", sent_by=None),
        ],
    )
    expired_nep = paper(
        "PP3",
        state="nhap",
        expires=T - DAY,
        versions=[ver(1, author="nep", sent_by=None)],
    )
    sent_nep = paper(
        "PP3", state="da_gui", versions=[ver(1, author="nep", sent_by=None)]
    )
    human_draft = paper("PP2", state="nhap")
    for name, purpose, locks, papers in (
        ("no_cycle", "bat_doi", [NB_NONE], []),
        ("notebook_created", "lap_so", [None, NB_NONE], []),
        ("notebook", "lap_so", [NB_ACTIVE], []),
        ("couple_ends_for_both", "bat_doi", [NB_ACTIVE], []),
        (
            "couple_uses_the_cycle_list",
            "bat_doi",
            [nb(participants=("KIA", "LA", "KIA"))],
            [],
        ),
        (
            "chat_drops_nep_drafts",
            "doc_chat",
            [NB_ACTIVE],
            [nep_draft, human_draft, nep_later, expired_nep, sent_nep],
        ),
        ("chat_without_papers", "doc_chat", [NB_ACTIVE], []),
        ("taste_is_ones_own", "chia_gu", [NB_ACTIVE], []),
    ):
        out.append(
            S(fn, name, req(fn, purpose=purpose), {"locks": locks, "papers": papers})
        )

    # --- put and delete a constraint -----------------------------------------------
    for fn in ("put_pair_constraint", "delete_pair_constraint"):
        for text in ("", "DUNG", "dung ", "khong-an-duoc", GRINNING_FACE):
            out.append(
                S(fn, f"unknown_kind/{text!r}", req(fn, kind=text), {"context": None})
            )
        out += [
            S(fn, "no_cycle", req(fn), {"locks": [NB_NONE]}),
            S(fn, "created_without_cycle", req(fn), {"locks": [None, NB_NONE]}),
            S(fn, "pending", req(fn, kind="khong_an_duoc"), {"locks": [NB_PENDING]}),
            S(fn, "active", req(fn), {"locks": [NB_ACTIVE], "constraint_version": 7}),
        ]
    fn = "put_pair_constraint"
    for k, text in enumerate(TEXTS):
        out.append(S(fn, f"content/{k}", req(fn, content=text), {"locks": [NB_ACTIVE]}))

    # --- preview and close -----------------------------------------------------------
    mixed = [
        paper("PP1", owner="KIA", state="nhap"),
        paper("PP2", state="da_xem"),
        paper("PP3", state="chot"),
        paper("PP1", state="dong_y", expires=T),
        paper("PP2", state="da_di"),
        paper("PP3", state="huy"),
    ]
    offers = nb(
        proposals=[
            prop("PR1", "bat_doi"),
            prop("PR2", "doc_chat", expires=T),
            prop("PR9", "bat_doi", completed=T - HOUR),
        ]
    )
    fn = "preview_close_pair_notebook"
    for name, notebook, papers in (
        ("empty", NB_ACTIVE, []),
        ("mixed", offers, mixed),
        ("no_notebook", None, mixed[:2]),
        ("no_cycle", NB_NONE, mixed[3:]),
    ):
        out.append(S(fn, name, req(fn), {"notebooks": [notebook], "papers": papers}))
    fn = "close_pair_notebook"
    for name, locks, notebook, papers in (
        ("active", [NB_ACTIVE], offers, mixed),
        ("no_cycle", [NB_NONE], None, mixed[:3]),
        ("created", [None, NB_NONE], NB_NONE, []),
        ("cycle_list", [nb(participants=("LA", "TOI"))], NB_ACTIVE, []),
    ):
        w = {"locks": locks, "notebooks": [notebook], "papers": papers}
        right = revision_of(w, T)
        out += [
            S(fn, f"{name}/current", req(fn, revision=right), w),
            S(
                fn,
                f"{name}/stale",
                req(fn, revision=right[:-1] + ("0" if right[-1] != "0" else "1")),
                w,
            ),
            S(fn, f"{name}/uppercase", req(fn, revision=right.upper()), w),
            S(fn, f"{name}/padded", req(fn, revision=" " + right), w),
        ]

    # --- list_pair_papers --------------------------------------------------------------
    fn = "list_pair_papers"
    unreadable = [
        ("no_current_version", paper(version=2, versions=[ver(1)])),
        ("list_content", paper(versions=[ver(1, content=["2030-09-21"])])),
        ("str_content", paper(versions=[ver(1, content="2030-09-21")])),
        ("null_content", paper(versions=[ver(1, content=None)])),
        (
            "no_day",
            paper(
                versions=[ver(1, content={"chang": [{"gio": "19:00", "viec": "x"}]})]
            ),
        ),
        ("int_day", paper(versions=[ver(1, content={"ngay": 20300921, "chang": []})])),
        (
            "week_day",
            paper(versions=[ver(1, content={"ngay": "2030-W38-6", "chang": "x"})]),
        ),
        (
            "basic_day_and_tail",
            paper(versions=[ver(1, content={"ngay": "20300921xy", "chang": [5]})]),
        ),
        (
            "bad_day",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-31",
                            "chang": [{"gio": 1830, "viec": None}],
                        },
                    )
                ]
            ),
        ),
        (
            "stop_without_viec",
            paper(
                versions=[
                    ver(1, content={"ngay": "2030-09-21", "chang": [{"gio": "19:00"}]})
                ]
            ),
        ),
        (
            "stop_as_list",
            paper(
                versions=[
                    ver(1, content={"ngay": "2030-09-21", "chang": [["19:00", "x"]]})
                ]
            ),
        ),
        (
            "stops_as_dict",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": {"gio": "19:00", "viec": "x"},
                        },
                    )
                ]
            ),
        ),
        (
            "empty_stops",
            paper(versions=[ver(1, content={"ngay": "2030-09-21", "chang": []})]),
        ),
        (
            "odd_stop_values",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [
                                {
                                    "gio": [1.5, None, True],
                                    "viec": {"a": "b'\"", "c": 1e16},
                                }
                            ],
                        },
                    )
                ]
            ),
        ),
    ]
    for name, sheet in unreadable:
        out.append(S(fn, f"read/{name}", req(fn), {"papers": [sheet]}))
    listing = [
        paper("PP1", owner="KIA", state="nhap"),
        paper("PP2", owner="TOI", state="nhap", expires=T),
        paper(
            "PP3",
            state="da_giu",
            keeps=[["KP1", "Lần sau quay lại", T - DAY], ["KPN", "Thứ hai", T]],
        ),
        paper("PP1", state="dong_y", expires=T - MICRO),
    ]
    out += [
        S(fn, "mixed", req(fn), {"papers": listing}),
        S(
            fn,
            "as_the_other_person",
            req(fn),
            {"papers": listing},
            actor=("KIA", ("member",)),
        ),
        S(fn, "empty", req(fn), {}),
    ]
    # A sheet nobody sent is its owner's draft whatever its state (24/09).
    chua_gui = [ver(1, sent_at=None, sent_by=None)]
    unsent = [
        paper("PP4", state="nghi_tuan", versions=chua_gui),
        paper("PP5", state="bo", versions=chua_gui),
        paper("PP6", state="het_han", versions=chua_gui),
        paper("PP7", state="nghi_tuan"),
    ]
    for who in ("TOI", "KIA"):
        out.append(S(fn, f"unsent_closed_as_{who.lower()}", req(fn), {"papers": unsent}, actor=(who, ("member",))))

    # --- draft_pair_paper --------------------------------------------------------------
    fn = "draft_pair_paper"
    with_lines = nb(constraints=[["KIA", "dung", "Hải sản", 1]])
    for name, locks, papers, now in (
        ("first_invitation", [NB_NONE], [], T),
        ("notebook_created", [None, NB_NONE], [], T),
        ("notebook_never_appears", [None, None], [], T),
        ("pending_refused", [NB_PENDING], [], T),
        ("active", [NB_ACTIVE], [], T),
        ("constraints_named", [with_lines], [], T),
        (
            "one_open_sheet",
            [NB_ACTIVE],
            [paper(state="chot"), paper("PP2", state="da_xem")],
            T,
        ),
        ("others_draft_blocks", [NB_ACTIVE], [paper(owner="KIA", state="nhap")], T),
        (
            "expired_sheet_does_not_block",
            [NB_ACTIVE],
            [paper(state="dong_y", expires=T)],
            T,
        ),
        (
            "terminal_does_not_block",
            [NB_ACTIVE],
            [paper(state="het_han"), paper("PP2", state="rut")],
            T,
        ),
        ("sunday_proposes_today", [NB_ACTIVE], [], SAT_START + DAY + HOUR),
        ("saturday_proposes_today", [NB_ACTIVE], [], SAT_START),
        ("monday_starts_a_week", [NB_ACTIVE], [], WEEK_END),
        ("sunday_last_micro", [NB_ACTIVE], [], WEEK_END - MICRO),
        (
            "clock_in_vietnam",
            [NB_ACTIVE],
            [],
            (T + timedelta(microseconds=123456)).astimezone(VIETNAM),
        ),
        ("pre_zone_history", [NB_ACTIVE], [], datetime(1945, 9, 1, 15, tzinfo=UTC)),
    ):
        out.append(S(fn, name, req(fn), {"locks": locks, "papers": papers}, now=now))
    out.append(
        S(
            fn,
            "no_role_after_lock",
            req(fn),
            {"locks": [None, NB_PENDING]},
            actor=("TOI", ()),
        )
    )
    # The draft reads this cycle's agreed sheets and the catalogue around the
    # place they chose (2026-09-24).
    def agreed(pid="PP3", state="chot", cycle="CY1", stops=(("19:30", "Ăn lẩu"),), place="p-lau", content=None):
        return paper(
            pid,
            owner="KIA",
            state=state,
            cycle=cycle,
            versions=[ver(1, content=content or body(stops=stops, place=place), sent_by="KIA")],
        )

    catalogue = [
        ["p-lau", "Lẩu gà lá é", "d-da-lat", "quan-an-local", ["lẩu"], [], 46, 120],
        ["p-bun", "Bún bò", "d-da-lat", "quan-an-local", [], [], 48, 200],
        ["p-oc", "Ốc đêm", "d-da-lat", "quan-an-local", ["HẢI SẢN"], [], 49, 10],
        ["p-cafe", "Lưng chừng", "d-da-lat", "cafe", [], [], 50, 9],
        ["p-sg", "Bún bò Sài Gòn", "d-tphcm", "quan-an-local", [], [], 50, 999],
    ]
    for name, locks, papers, places in (
        ("history_keeps_hour_and_proposes", [NB_ACTIVE], [agreed()], catalogue),
        ("history_constraint_blocks", [with_lines], [agreed()], catalogue),
        ("history_other_cycle_ignored", [NB_ACTIVE], [agreed(cycle="CY2")], catalogue),
        ("history_invitation_ignored", [NB_ACTIVE], [agreed(cycle=None)], catalogue),
        ("history_pending_notebook_reads_nothing", [NB_PENDING], [agreed()], catalogue),
        ("history_cancelled_ignored", [NB_ACTIVE], [agreed(state="huy")], catalogue),
        ("history_done_and_kept_count", [NB_ACTIVE], [agreed("PP3", "da_giu"), agreed("PP4", "da_di", place="p-bun")], catalogue),
        ("history_place_unknown", [NB_ACTIVE], [agreed(place="p-khong-con")], catalogue),
        ("history_without_place", [NB_ACTIVE], [agreed(place=None)], catalogue),
        (
            "history_unreadable_skipped",
            [NB_ACTIVE],
            [agreed("PP3", content={"ngay": None}), agreed("PP4", stops=(("20:00", "Ăn bún"),))],
            catalogue,
        ),
        (
            "history_limited_to_four",
            [NB_ACTIVE],
            [agreed(f"PP{i}", place=None, stops=((f"2{i}:00", "Ăn"),)) for i in range(3, 7)] + [agreed("PP7")],
            catalogue,
        ),
    ):
        out.append(S(fn, name, req(fn), {"locks": locks, "papers": papers, "places": places}))
    # ADR-0034 §2.2: the tastes of whoever shared theirs, and nobody else's.
    doi = both("lap_so") + both("bat_doi", proposal="PRD")
    doi_props = [prop("PR1", "lap_so", completed=T - DAY), prop("PRD", "bat_doi", completed=T - DAY)]
    kia_chia = [grant("KIA", "chia_gu", proposal="PGK")], [prop("PGK", "chia_gu", completed=T - DAY)]
    toi_chia = [grant("TOI", "chia_gu", proposal="PGT")], [prop("PGT", "chia_gu", by="TOI", completed=T - DAY)]
    gu = {"TOI": ["cafe", "game"], "KIA": ["nightlife", "cafe", "tag-da-bo"]}
    chi_lau = [catalogue[0], catalogue[3], ["p-cafe2", "Cafe Hải Sản", "d-da-lat", "cafe", [], [], 49, 3]]
    for name, consents, proposals, papers, places in (
        ("taste_nobody_shared", doi, doi_props, [], catalogue),
        ("taste_friends_only", both("lap_so") + kia_chia[0], [prop("PR1", "lap_so", completed=T - DAY)] + kia_chia[1], [], catalogue),
        ("taste_they_shared_no_history", doi + kia_chia[0], doi_props + kia_chia[1], [], catalogue),
        ("taste_both_shared_no_history", doi + kia_chia[0] + toi_chia[0], doi_props + kia_chia[1] + toi_chia[1], [], catalogue),
        ("taste_history_place_wins", doi + kia_chia[0] + toi_chia[0], doi_props + kia_chia[1] + toi_chia[1], [agreed()], catalogue),
        ("taste_after_history_ran_out", doi + kia_chia[0] + toi_chia[0], doi_props + kia_chia[1] + toi_chia[1], [agreed()], chi_lau),
        ("taste_only_i_shared", doi + toi_chia[0], doi_props + toi_chia[1], [agreed()], chi_lau),
    ):
        out.append(
            S(fn, name, req(fn), {"locks": [nb(consents=consents, proposals=proposals)], "papers": papers, "places": places, "interests": gu})
        )

    # --- pair_paper ----------------------------------------------------------------------
    fn = "pair_paper"
    revised = paper(
        state="da_gui",
        version=2,
        versions=[
            ver(1, sent_by="TOI", ly_do="vì mưa"),
            ver(
                2,
                content=body(stops=(("20:00", "Chè"), ("21:00", "Đi bộ")), place=PLACE),
                sent_by="KIA",
            ),
        ],
        views=[[1, "KIA", T - 2 * HOUR], [1, "TOI", T - HOUR], [2, "TOI", T - MICRO]],
        responses=[
            [1, "TOI", "dong_y"],
            [1, "KIA", "de_nghi_sua"],
            [2, "KIA", "dong_y"],
            [1, "TOI", "de_nghi_sua"],
        ],
        keeps=[["KP1", "Một dòng", T]],
        outing="OU1",
    )
    for name, sheet, actor, now in (
        ("revised_as_toi", revised, "TOI", T),
        ("revised_as_kia", revised, "KIA", T),
        ("draft_as_owner", paper(state="nhap"), "TOI", T),
        ("unsent_skipped_as_owner", paper(state="nghi_tuan", versions=[ver(1, sent_at=None, sent_by=None)]), "TOI", T),
        ("unsent_skipped_as_kia", paper(state="nghi_tuan", versions=[ver(1, sent_at=None, sent_by=None)]), "KIA", T),
        ("unsent_discarded_as_kia", paper(state="bo", versions=[ver(1, sent_at=None, sent_by=None)]), "KIA", T),
        ("sent_then_skipped_as_kia", paper(state="nghi_tuan"), "KIA", T),
        (
            "nep_sheet",
            paper(state="da_gui", versions=[ver(1, author="nep", sent_by=None)]),
            "KIA",
            T,
        ),
        (
            "current_version_missing",
            paper(version=3, versions=[ver(1), ver(2)]),
            "KIA",
            T,
        ),
        ("expired_reads_het_han", paper(state="dong_y", expires=T), "KIA", T),
        ("chot_on_the_day", paper(state="chot", expires=T - DAY), "KIA", SAT_START),
        ("chot_before_the_day", paper(state="chot"), "KIA", SAT_START - MICRO),
        ("da_di_is_not_recordable", paper(state="da_di"), "KIA", SAT_START + DAY),
        (
            "chot_unreadable_day",
            paper(
                state="chot",
                versions=[ver(1, content={"ngay": "2030-9-21", "chang": []})],
            ),
            "KIA",
            SAT_START,
        ),
        (
            "old_version_unreadable",
            paper(version=2, versions=[ver(1, content=[]), ver(2)]),
            "KIA",
            T,
        ),
        (
            "place_variants",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [
                                {"gio": "a", "viec": "b", "place_id": ""},
                                {"gio": "c", "viec": "d", "place_id": None},
                                {"gio": "e", "viec": "f"},
                                {
                                    "gio": "g",
                                    "viec": "h",
                                    "place_id": "{" + PLACE.upper() + "}",
                                    "can_kiem": 0,
                                },
                                {
                                    "gio": "i",
                                    "viec": "j",
                                    "place_id": "urn:uuid:" + PLACE.replace("-", ""),
                                    "can_kiem": "no",
                                },
                                {"gio": 7, "viec": 1.5, "can_kiem": None},
                            ],
                        },
                    )
                ]
            ),
            "KIA",
            T,
        ),
        (
            "place_not_a_uuid",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [{"gio": "a", "viec": "b", "place_id": "PL1"}],
                        },
                    )
                ]
            ),
            "KIA",
            T,
        ),
        (
            "place_as_int",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [{"gio": "a", "viec": "b", "place_id": 12}],
                        },
                    )
                ]
            ),
            "KIA",
            T,
        ),
        (
            "place_false",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [{"gio": "a", "viec": "b", "place_id": False}],
                        },
                    )
                ]
            ),
            "KIA",
            T,
        ),
        (
            "stops_missing",
            paper(versions=[ver(1, content={"ngay": "2030-09-21"})]),
            "KIA",
            T,
        ),
        (
            "stops_empty_str",
            paper(versions=[ver(1, content={"ngay": "2030-09-21", "chang": ""})]),
            "KIA",
            T,
        ),
        (
            "stops_empty_dict",
            paper(versions=[ver(1, content={"ngay": "2030-09-21", "chang": {}})]),
            "KIA",
            T,
        ),
        (
            "stops_null",
            paper(versions=[ver(1, content={"ngay": "2030-09-21", "chang": None})]),
            "KIA",
            T,
        ),
        (
            "repr_texts",
            paper(
                versions=[
                    ver(
                        1,
                        content={
                            "ngay": "2030-09-21",
                            "chang": [
                                {
                                    "gio": ["it's", 'say "hi"', "both '\""],
                                    "viec": {
                                        "\t": "\x7f",
                                        NO_BREAK_SPACE: GRINNING_FACE,
                                        "k": [False, 10**20, -0.0, 1e-05],
                                    },
                                }
                            ],
                        },
                    )
                ]
            ),
            "KIA",
            T,
        ),
    ):
        out.append(
            S(
                fn,
                name,
                req(fn),
                {"reads": [sheet]},
                now=now,
                actor=(actor, ("member",)),
            )
        )
    for k, ngay in enumerate(
        (
            "20300921",
            "2030W386",
            "2030-W38",
            "2030W38",
            "2030-W38-6",
            "2030-W53-1",
            "2026-W53-5",
            "2020-W53-1",
            "2030-W00-1",
            "2030-W38-8",
            "0000-01-01",
            "9999-W52-5",
            "9999-W52-6",
            "0001-W01-1",
            "2030-02-29",
            "2028-02-29",
            "2030-13-01",
            "2030-09-2x",
            "2030-0921",
            "203009-21",
            "2030-09-21T",
            "+030-09-21",
            " 030-09-21",
            "2030-" + FULLWIDTH_TWO + "1-01",
            "2030-09-" + ARABIC_INDIC_TWO + "1",
            E_DOT_CIRCUMFLEX + "030-09",
            "2030W381xy",
            "2030W38-6",
            "2030-W386",
            2030921,
            20300921,
            -2030921,
            True,
            None,
            2030.5,
            [20300921],
            {"y": 2030},
        )
    ):
        sheet = paper(
            state="chot", versions=[ver(1, content={"ngay": ngay, "chang": []})]
        )
        if k < 6:
            out.append(
                S(
                    fn,
                    f"iso_day/{k}",
                    req(fn),
                    {"reads": [sheet]},
                    now=SAT_START,
                    actor=("KIA", ("member",)),
                )
            )
        else:
            listed = "list_pair_papers"
            out.append(
                S(
                    listed,
                    f"iso_day/{k}",
                    req(listed),
                    {"papers": [sheet]},
                    now=SAT_START,
                    actor=("KIA", ("member",)),
                )
            )

    # --- edit_pair_draft -----------------------------------------------------------------
    fn = "edit_pair_draft"
    draft = paper(state="nhap")
    for name, reads, locked, r, now in (
        ("ok", [draft], [draft], req(fn), T),
        ("reason_stripped", [draft], [draft], req(fn, ly_do="  vì mưa  "), T),
        ("reason_blank", [draft], [draft], req(fn, ly_do=IDEOGRAPHIC_SPACE + "\t"), T),
        ("reason_empty", [draft], [draft], req(fn, ly_do=""), T),
        ("reason_hostile", [draft], [draft], req(fn, ly_do=TEXTS[14]), T),
        (
            "two_stops_with_place",
            [draft],
            [draft],
            req(
                fn,
                content=content_in(
                    "2031-01-01",
                    (("07:05", TEXTS[9], PLACE, False), ("23:59", " ", None, True)),
                ),
            ),
            T,
        ),
        ("expired_draft", [draft], [paper(state="nhap", expires=T)], req(fn), T),
        ("sent_meanwhile", [draft], [paper(state="da_gui")], req(fn), T),
        (
            "owner_changed_meanwhile",
            [draft],
            [paper(owner="KIA", state="nhap")],
            req(fn),
            T,
        ),
        ("sent_sheet", [paper(state="da_gui")], [paper(state="da_gui")], req(fn), T),
        (
            "other_persons_sent_sheet",
            [paper(owner="KIA")],
            [paper(owner="KIA")],
            req(fn),
            T,
        ),
    ):
        out.append(S(fn, name, r, {"reads": reads, "locked": locked}, now=now))

    # --- send_pair_paper -----------------------------------------------------------------
    fn = "send_pair_paper"
    for name, sheet, version, actor, conflicts in (
        ("ok", draft, 1, "TOI", {}),
        ("stale_version", draft, 2, "TOI", {}),
        ("not_owner_and_stale", paper(owner="KIA", state="da_gui"), 2, "TOI", {}),
        ("already_sent", paper(state="da_gui"), 1, "TOI", {}),
        ("expired", paper(state="nhap", expires=T), 1, "TOI", {}),
        (
            "second_version",
            paper(
                state="nhap",
                version=2,
                versions=[ver(1), ver(2, sent_by=None, sent_at=None)],
            ),
            2,
            "TOI",
            {},
        ),
        ("version_past_int64", draft, 2**63, "TOI", {}),
        ("version_far_past_int64", draft, 2**70, "TOI", {}),
        ("version_zero", draft, 0, "TOI", {}),
        (
            "agreement_conflict",
            draft,
            1,
            "TOI",
            {"add_paper_response": ["paper_already_agreed"]},
        ),
        ("frozen", paper(state="chot"), 1, "TOI", {}),
    ):
        out.append(
            S(
                fn,
                name,
                req(fn, version=version),
                {"reads": [sheet], "conflicts": conflicts},
                actor=(actor, ("member",)),
            )
        )

    # --- mark_pair_paper_viewed ----------------------------------------------------------
    fn = "mark_pair_paper_viewed"
    sent_by_toi = paper(
        state="da_gui",
        version=2,
        versions=[ver(1, sent_by="KIA"), ver(2, sent_by="TOI")],
    )
    for name, sheet, version, actor in (
        ("recipient_current", sent_by_toi, 2, "KIA"),
        ("recipient_old_version", sent_by_toi, 1, "TOI"),
        ("sender_current", sent_by_toi, 2, "TOI"),
        ("sender_old_version", sent_by_toi, 1, "KIA"),
        ("missing_version", sent_by_toi, 3, "KIA"),
        ("version_zero", sent_by_toi, 0, "KIA"),
        ("version_negative", sent_by_toi, -1, "KIA"),
        ("version_past_int64", sent_by_toi, 2**64, "KIA"),
        ("already_seen", paper(state="da_xem", owner="TOI"), 1, "KIA"),
        ("expired_sent", paper(state="da_gui", expires=T), 1, "KIA"),
        (
            "expired_old_version",
            paper(
                state="da_gui",
                version=2,
                versions=[ver(1, sent_by="TOI"), ver(2, sent_by="TOI")],
                expires=T,
            ),
            1,
            "KIA",
        ),
        ("unsent_draft_by_owner", draft, 1, "TOI"),
        ("chot", paper(state="chot"), 1, "KIA"),
        (
            "nep_version",
            paper(state="da_gui", versions=[ver(1, author="nep", sent_by=None)]),
            1,
            "TOI",
        ),
    ):
        out.append(
            S(
                fn,
                name,
                req(fn, version=version),
                {"reads": [sheet]},
                actor=(actor, ("member",)),
            )
        )

    # --- respond_pair_paper --------------------------------------------------------------
    fn = "respond_pair_paper"
    kia_sent = paper(owner="KIA", state="da_xem", responses=[[1, "KIA", "dong_y"]])
    agreed = dict(kia_sent, responses=[[1, "KIA", "dong_y"], [1, "TOI", "dong_y"]])
    two_places = {
        "ngay": "2030-09-21",
        "chang": [
            {"gio": "19:00", "viec": "Ăn tối", "place_id": "p-lau-ga", "can_kiem": True},
            {"gio": "21:30", "viec": "Chè", "place_id": "p-da-dong-cua", "can_kiem": False},
        ],
    }
    placed_agreed = dict(agreed, versions=[ver(1, content=two_places, sent_by="KIA")])
    unknown_agreed = dict(
        agreed,
        versions=[
            ver(1, content=body(place="p-da-dong-cua"), sent_by="KIA"),
        ],
    )
    long_agreed = dict(
        agreed, versions=[ver(1, content=body(place="p-dai"), sent_by="KIA")]
    )
    places = [["p-lau-ga", "Lẩu gà lá é"], ["p-dai", "Lẩu gà " * 28]]
    counter = {
        "kind": "de_nghi_sua",
        "content": content_in("2030-09-20", (("18:00", "Phở", PLACE, True),)),
        "ly_do": "  sớm hơn  ",
    }
    for name, reads, sheet, r, outings, conflicts, now in (
        ("agree_first", [kia_sent, kia_sent], kia_sent, req(fn), [], {}, T),
        (
            "agree_second_makes_outing",
            [kia_sent, agreed],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "outing_already_linked",
            [kia_sent, agreed],
            kia_sent,
            req(fn),
            ["OU1"],
            {},
            T,
        ),
        (
            "link_raced_and_found",
            [kia_sent, agreed],
            kia_sent,
            req(fn),
            [None, "OU1"],
            {"link_paper_outing": ["paper_outing_exists"]},
            T,
        ),
        (
            "link_raced_and_lost",
            [kia_sent, agreed],
            kia_sent,
            req(fn),
            [None, None],
            {"link_paper_outing": ["PAPER_OUTING_EXISTS"]},
            T,
        ),
        (
            "agreed_version_missing_after",
            [kia_sent, dict(agreed, versions=[])],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "agreed_content_unreadable",
            [
                kia_sent,
                dict(agreed, versions=[ver(1, content={"ngay": None}, sent_by="KIA")]),
            ],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "agreed_on_first_of_january",
            [
                kia_sent,
                dict(
                    agreed, versions=[ver(1, content=body("2031-01-01"), sent_by="KIA")]
                ),
            ],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        # The agreed stops become the outing's timeline, named after the place.
        (
            "agreed_place_names_outing",
            [kia_sent, placed_agreed],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "agreed_place_unknown_keeps_line",
            [kia_sent, unknown_agreed],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "agreed_place_name_cut_at_190",
            [kia_sent, long_agreed],
            kia_sent,
            req(fn),
            [None],
            {},
            T,
        ),
        (
            "agreed_place_link_raced_writes_no_stops",
            [kia_sent, placed_agreed],
            kia_sent,
            req(fn),
            [None, "OU1"],
            {"link_paper_outing": ["paper_outing_exists"]},
            T,
        ),
        (
            "repeat_yes",
            [kia_sent, agreed],
            kia_sent,
            req(fn),
            [],
            {"add_paper_response": ["paper_already_agreed"]},
            T,
        ),
        (
            "repeat_yes_sheet_gone",
            [kia_sent, None],
            kia_sent,
            req(fn),
            [],
            {"add_paper_response": ["paper_already_agreed"]},
            T,
        ),
        (
            "repeat_yes_expired",
            [kia_sent, dict(agreed, state="dong_y", expires=T)],
            kia_sent,
            req(fn),
            [],
            {"add_paper_response": ["paper_already_agreed"]},
            T,
        ),
        (
            "other_conflict",
            [kia_sent],
            kia_sent,
            req(fn),
            [],
            {"add_paper_response": ["something_else"]},
            T,
        ),
        ("sheet_gone_after_yes", [kia_sent, None], kia_sent, req(fn), [], {}, T),
        (
            "expired_after_write",
            [kia_sent, agreed],
            dict(kia_sent, expires=T),
            req(fn),
            [],
            {},
            T,
        ),
        (
            "agree_to_a_plan",
            [kia_sent, agreed],
            dict(kia_sent, state="chot"),
            req(fn),
            [],
            {},
            T,
        ),
        (
            "agree_to_a_draft",
            [kia_sent, agreed],
            dict(kia_sent, state="nhap"),
            req(fn),
            [],
            {},
            T,
        ),
        ("self_response", [kia_sent], kia_sent, req(fn), [], {}, T),
        (
            "stale_and_self",
            [kia_sent],
            dict(
                kia_sent,
                version=2,
                versions=[ver(1, sent_by="TOI"), ver(2, sent_by="KIA")],
            ),
            req(fn),
            [],
            {},
            T,
        ),
        ("missing_version", [kia_sent], kia_sent, req(fn, version=5), [], {}, T),
        (
            "version_past_int64",
            [kia_sent],
            kia_sent,
            req(fn, version=2**63 + 1),
            [],
            {},
            T,
        ),
        ("counter", [kia_sent], kia_sent, req(fn, reply=counter), [], {}, T),
        (
            "counter_blank_reason",
            [kia_sent],
            kia_sent,
            req(fn, reply=dict(counter, ly_do=" ")),
            [],
            {},
            T,
        ),
        (
            "counter_no_reason",
            [kia_sent],
            kia_sent,
            req(fn, reply=dict(counter, ly_do=None)),
            [],
            {},
            T,
        ),
        (
            "counter_to_a_plan",
            [kia_sent],
            dict(kia_sent, state="chot"),
            req(fn, reply=counter),
            [],
            {},
            T,
        ),
        (
            "counter_expired",
            [kia_sent],
            dict(kia_sent, expires=T),
            req(fn, reply=counter),
            [],
            {},
            T,
        ),
        (
            "counter_to_a_draft",
            [kia_sent],
            dict(kia_sent, state="nhap"),
            req(fn, reply=counter),
            [],
            {},
            T,
        ),
        (
            "counter_conflict",
            [kia_sent],
            kia_sent,
            req(fn, reply=counter),
            [],
            {"add_paper_response": ["x"]},
            T,
        ),
        (
            "counter_keeps_outing",
            [kia_sent],
            dict(kia_sent, state="dong_y", outing="OU1"),
            req(fn, reply=counter),
            [],
            {},
            T.astimezone(HONOLULU),
        ),
    ):
        actor = "KIA" if name in ("self_response",) else "TOI"
        w = {
            "reads": reads,
            "locked": [sheet],
            "outings": outings,
            "conflicts": conflicts,
        }
        if name.startswith("agreed_place"):
            w["places"] = places
        out.append(S(fn, name, r, w, now=now, actor=(actor, ("member",))))

    # --- withdraw_pair_paper -------------------------------------------------------------
    fn = "withdraw_pair_paper"
    fresh = paper(state="da_gui", responses=[[1, "TOI", "dong_y"]])
    for name, sheet, version, actor in (
        ("untouched", fresh, 1, "TOI"),
        ("stale", fresh, 2, "TOI"),
        ("seen", dict(fresh, views=[[1, "KIA", T - MICRO]]), 1, "TOI"),
        ("seen_and_stale", dict(fresh, views=[[1, "KIA", T]]), 9, "TOI"),
        ("answered", dict(fresh, responses=[[1, "KIA", "de_nghi_sua"]]), 1, "TOI"),
        ("not_the_sender", fresh, 1, "KIA"),
        (
            "nep",
            paper(state="da_gui", versions=[ver(1, author="nep", sent_by=None)]),
            1,
            "TOI",
        ),
        ("expired", dict(fresh, expires=T), 1, "TOI"),
        ("already_seen_state", dict(fresh, state="da_xem"), 1, "TOI"),
        ("version_past_int64", fresh, -(2**63) - 1, "TOI"),
    ):
        out.append(
            S(
                fn,
                name,
                req(fn, version=version),
                {"reads": [sheet]},
                actor=(actor, ("member",)),
            )
        )

    # --- skip, done, keep ----------------------------------------------------------------
    for fn, states_tried in (
        ("skip_pair_week", ("nhap", "da_gui", "da_xem", "dong_y", "chot", "huy")),
        ("record_pair_outing_done", ("chot", "da_di", "dong_y", "nhap", "da_giu")),
        ("keep_pair_paper_line", ("da_di", "da_giu", "chot", "rut", "nhap")),
    ):
        for state in states_tried:
            sheet = paper(state=state, owner="TOI")
            out.append(
                S(fn, f"state/{state}", req(fn), {"reads": [sheet]}, now=SAT_START)
            )
        expired = paper(state="dong_y", expires=T)
        out.append(S(fn, "expired", req(fn), {"reads": [expired]}))
    fn = "record_pair_outing_done"
    plan = paper(state="chot", expires=T - DAY)
    for name, sheet, now in (
        ("friday_last_micro", plan, SAT_START - MICRO),
        ("saturday_first_micro", plan, SAT_START),
        ("a_week_later", plan, SAT_START + 7 * DAY),
        (
            "unreadable_day",
            paper(state="chot", versions=[ver(1, content={"ngay": 5})]),
            SAT_START,
        ),
        (
            "no_current_version",
            paper(state="chot", version=2, versions=[ver(1)]),
            SAT_START,
        ),
        (
            "day_in_a_week_format",
            paper(state="chot", versions=[ver(1, content={"ngay": "2030-W38-6"})]),
            SAT_START,
        ),
        ("clock_west_of_utc", plan, SAT_START.astimezone(HONOLULU)),
    ):
        out.append(
            S(
                fn,
                name,
                req(fn),
                {"reads": [sheet]},
                now=now,
                actor=("KIA", ("member",)),
            )
        )
    fn = "keep_pair_paper_line"
    went = paper(state="da_di")
    for k, text in enumerate(TEXTS[:5] + TEXTS[9:12]):
        out.append(S(fn, f"line/{k}", req(fn, line=text), {"reads": [went]}))
    out.append(
        S(
            fn,
            "second_line",
            req(fn),
            {
                "reads": [paper(state="da_giu")],
                "locked": [paper(state="da_giu", keeps=[["KP1", "x", T]])],
            },
        )
    )
    return out


# --- fuzz ------------------------------------------------------------------------------

PEOPLE = ("TOI", "KIA", "LA")
PAPER_IDS = ("PP1", "PP2", "PP3")


def fuzz_time(rng: random.Random) -> datetime:
    return rng.choice(
        (
            T,
            T,
            T - MICRO,
            T + MICRO,
            WEEK_END - MICRO,
            WEEK_END,
            SAT_START,
            SAT_START - MICRO,
            T
            + timedelta(
                days=rng.randint(-9, 9),
                seconds=rng.randint(0, 86399),
                microseconds=rng.randint(0, 999999),
            ),
        )
    )


def fuzz_content(rng: random.Random, near: date) -> object:
    roll = rng.random()
    if roll < 0.8:
        stops = []
        for _ in range(rng.choice((1, 1, 2, 0))):
            stop = {
                "gio": rng.choice(("19:00", "07:30", "23:59")),
                "viec": rng.choice(("Ăn tối", "Chè", rng.choice(TEXTS))),
            }
            placed = rng.random()
            if placed < 0.6:
                stop["place_id"] = None
            elif placed < 0.8:
                stop["place_id"] = PLACE
            elif placed < 0.9:
                stop["place_id"] = rng.choice(
                    ("", "PL1", PLACE.upper(), "{" + PLACE + "}", 7)
                )
            if rng.random() < 0.8:
                stop["can_kiem"] = rng.choice((True, True, False, None, 0, "x"))
            stops.append(stop)
        return {
            "ngay": (near + timedelta(days=rng.randint(-3, 3))).isoformat(),
            "chang": stops,
        }
    if roll < 0.9:
        return {
            "ngay": rng.choice(
                (
                    "2030-W38-6",
                    "20300921",
                    20300921,
                    "2030-09-31",
                    None,
                    "",
                    [1],
                    {"a": 1.5},
                )
            ),
            "chang": rng.choice(
                ([], "", {}, None, [{"gio": 1, "viec": [True, None]}], [{"gio": "x"}])
            ),
        }
    return rng.choice(([], "2030-09-21", None, 5, {"chang": []}))


def fuzz_paper(rng: random.Random, paper_id: str, now: datetime) -> dict:
    state = rng.choice(
        pair_paper.PAPER_STATES
        + ("da_gui", "da_xem", "dong_y", "chot", "nhap", "da_di")
    )
    owner = rng.choice(("TOI", "TOI", "KIA", "LA"))
    count = rng.choice((1, 1, 1, 2, 3))
    versions = []
    for v in range(1, count + 1):
        author = "nep" if rng.random() < 0.15 else "human"
        unsent = author == "human" and (
            state == "nhap" and v == count or rng.random() < 0.1
        )
        sent_by = (
            None
            if unsent or (author == "nep" and rng.random() < 0.7)
            else rng.choice(PEOPLE)
        )
        sent_at = None if unsent else now - timedelta(hours=rng.randint(1, 30))
        versions.append(
            ver(
                v,
                content=fuzz_content(rng, SAT),
                ly_do=rng.choice((None, None, "vì mưa", "")),
                author=author,
                sent_at=sent_at,
                sent_by=sent_by,
            )
        )
    current = count if rng.random() < 0.9 else rng.randint(1, count + 1)
    if rng.random() < 0.05:
        versions.pop(rng.randrange(len(versions)))
    views = [
        [
            rng.randint(1, count),
            rng.choice(PEOPLE),
            now - timedelta(minutes=rng.randint(0, 90)),
        ]
        for _ in range(rng.choice((0, 0, 1, 2)))
    ]
    responses = [
        [
            rng.randint(1, count),
            rng.choice(PEOPLE),
            rng.choice(("dong_y", "dong_y", "de_nghi_sua")),
        ]
        for _ in range(rng.choice((0, 1, 1, 2, 3)))
    ]
    keeps = [
        [rng.choice(("KP1", "KPN")), rng.choice(TEXTS[:3] + ("Nhớ mãi",)), now - HOUR]
        for _ in range(rng.choice((0, 0, 0, 1, 2)))
    ]
    expires = rng.choice(
        (
            WEEK_END,
            WEEK_END,
            now,
            now - MICRO,
            now + MICRO,
            now + timedelta(days=rng.randint(-5, 5)),
        )
    )
    return paper(
        paper_id,
        owner=owner,
        state=state,
        version=current,
        versions=versions,
        views=views,
        responses=responses,
        keeps=keeps,
        expires=expires,
        outing=rng.choice((None, None, "OU1")),
    )


def fuzz_notebook(rng: random.Random, now: datetime) -> dict | None:
    roll = rng.random()
    if roll < 0.05:
        return None
    if roll < 0.2:
        return NB_NONE
    state = rng.choice(("pending", "active", "active", "active"))
    participants = rng.choice(
        (("TOI", "KIA"), ("TOI", "KIA"), ("KIA", "TOI"), ("KIA", "LA"), ("TOI",), ())
    )
    consents = []
    for person in PEOPLE:
        for purpose in pair_notebook.CONSENT_PURPOSES:
            if rng.random() < (0.7 if person != "LA" else 0.15):
                granted = rng.choice((now - HOUR, now - HOUR, None))
                revoked = (
                    rng.choice((None, None, None, now - MICRO)) if granted else None
                )
                expires = rng.choice(
                    (now + DAY, now + DAY, now, now + MICRO, now - MICRO)
                )
                consents.append(
                    grant(
                        person,
                        purpose,
                        granted=granted,
                        revoked=revoked,
                        expires=expires,
                    )
                )
    proposals = [
        prop(
            rng.choice(("PR1", "PR2", "PR9")),
            rng.choice(pair_notebook.CONSENT_PURPOSES),
            by=rng.choice(PEOPLE),
            cycle=rng.choice(("CY1", "CY1", "CY2")),
            completed=rng.choice((None, None, now - HOUR)),
            expires=rng.choice((now + DAY, now, now + MICRO, now - DAY)),
        )
        for _ in range(rng.choice((0, 1, 1, 2, 3)))
    ]
    constraints = [
        [
            rng.choice(PEOPLE),
            rng.choice(pair_notebook.CONSTRAINT_KINDS),
            rng.choice(TEXTS[:4]),
            rng.randint(1, 4),
        ]
        for _ in range(rng.choice((0, 0, 1, 2)))
    ]
    return nb(
        state=state,
        participants=participants,
        consents=consents,
        proposals=proposals,
        constraints=constraints,
    )


def other_of(actor: str) -> str:
    return "KIA" if actor == "TOI" else "TOI"


def coherent_paper(rng: random.Random, fn: str, actor: str, now: datetime) -> dict:
    """A sheet shaped to get past fn's doors, so the fuzz reaches the writes
    behind them; the caller perturbs it."""
    other = other_of(actor)
    readable = body((SAT + timedelta(days=rng.randint(-3, 3))).isoformat())
    content = readable if rng.random() < 0.9 else fuzz_content(rng, SAT)
    expires = rng.choice((WEEK_END, WEEK_END, now + DAY, now, now - MICRO, now + MICRO))
    if fn in ("edit_pair_draft", "send_pair_paper"):
        count = rng.choice((1, 1, 2))
        versions = [
            ver(
                v,
                content=content,
                sent_by=actor if v < count else None,
                sent_at=T - HOUR if v < count else None,
            )
            for v in range(1, count + 1)
        ]
        return paper(
            owner=actor, state="nhap", version=count, versions=versions, expires=expires
        )
    if fn in ("mark_pair_paper_viewed", "respond_pair_paper"):
        count = rng.choice((1, 1, 2))
        versions = [
            ver(
                v,
                content=content,
                sent_by=other if v == count else actor,
                author="nep" if rng.random() < 0.1 else "human",
            )
            for v in range(1, count + 1)
        ]
        state = rng.choice(("da_gui", "da_gui", "da_xem", "dong_y"))
        responses = [[count, other, "dong_y"]] if rng.random() < 0.7 else []
        views = (
            [[count, actor, now - HOUR]]
            if state != "da_gui" and rng.random() < 0.5
            else []
        )
        return paper(
            owner=other,
            state=state,
            version=count,
            versions=versions,
            views=views,
            responses=responses,
            expires=expires,
        )
    if fn == "withdraw_pair_paper":
        views = [[1, other, now - MICRO]] if rng.random() < 0.3 else []
        responses = [[1, actor, "dong_y"]] + (
            [[1, other, rng.choice(("dong_y", "de_nghi_sua"))]]
            if rng.random() < 0.3
            else []
        )
        return paper(
            owner=actor,
            state="da_gui",
            versions=[ver(1, content=content, sent_by=actor)],
            views=views,
            responses=responses,
            expires=expires,
        )
    if fn == "record_pair_outing_done":
        return paper(
            owner=rng.choice(("TOI", "KIA")),
            state="chot",
            versions=[ver(1, content=content, sent_by=other)],
            expires=now - DAY,
            outing="OU1",
        )
    if fn == "keep_pair_paper_line":
        state = rng.choice(("da_di", "da_di", "da_giu", "chot"))
        return paper(
            owner=actor,
            state=state,
            versions=[ver(1, content=content, sent_by=other)],
            expires=now - DAY,
            outing="OU1",
        )
    if fn == "skip_pair_week":
        state = rng.choice(("nhap", "da_gui", "da_xem", "dong_y", "chot"))
        sent_by = None if state == "nhap" else actor
        return paper(
            owner=actor,
            state=state,
            versions=[
                ver(
                    1,
                    content=content,
                    sent_by=sent_by,
                    sent_at=None if sent_by is None else T - HOUR,
                )
            ],
            expires=expires,
        )
    sheet = fuzz_paper(rng, "PP1", now)
    if rng.random() < 0.7:
        sheet = dict(sheet, owner=actor)
    return sheet


def coherent_grant(
    rng: random.Random, w: dict, r: dict, actor: str, now: datetime
) -> None:
    """A live offer from the other person, and the notebook as it reads after
    this grant: completes, activates the cycle or writes the couple rows."""
    other = other_of(actor)
    purpose = rng.choice(pair_notebook.CONSENT_PURPOSES)
    live = {"expires": now + DAY}
    offer = prop(
        "PR1",
        purpose,
        by=other,
        completed=None if rng.random() < 0.9 else now - HOUR,
        expires=rng.choice((now + DAY, now + DAY, now, now + MICRO)),
    )
    if purpose == "lap_so":
        base = nb(
            state="pending",
            consents=[grant(other, "lap_so", **live)],
            proposals=[offer],
        )
    else:
        opened = [grant(person, "lap_so", **live) for person in ("TOI", "KIA")]
        base = nb(consents=opened + [grant(other, purpose, **live)], proposals=[offer])
    after_consents = base["consents"] + [grant(actor, purpose, **live)]
    if rng.random() < 0.15:
        after_consents = [
            row for row in after_consents if not (row[0] == other and row[1] == purpose)
        ]
    w["locks"] = [base] if rng.random() < 0.95 else [None, base]
    w["proposal"] = offer
    w["notebooks"] = [dict(base, consents=after_consents)]
    r["proposal_id"] = "PR1"


def fuzz_step(rng: random.Random, i: int) -> dict:
    fn = rng.choice(tuple(CALLERS))
    now = fuzz_time(rng)
    actor = rng.choice(("TOI", "TOI", "TOI", "KIA", "KIA", "LA"))
    roles = rng.choice((("member",),) * 30 + ((), ("advancer",), ("member", "BOGUS")))
    coherent = rng.random() < 0.65
    if coherent and actor == "LA":
        actor = "KIA"
    w: dict = {}
    roll = rng.random()
    if roll < 0.04:
        w["context"] = rng.choice((None, "group"))
    elif roll < 0.07:
        w["member"] = False
    if rng.random() < 0.2:
        w["roster"] = rng.choice(
            (
                [["TOI", "active"]],
                [["TOI", "active"], ["KIA", "left"]],
                [["KIA", "active"], ["LA", "active"], ["TOI", "active"]],
                [],
            )
        )
    notebook = fuzz_notebook(rng, now)
    lock_roll = rng.random()
    lock_nb = notebook if notebook is not None else NB_NONE
    w["locks"] = (
        [lock_nb]
        if lock_roll < 0.9
        else ([None, lock_nb] if lock_roll < 0.97 else [None, None])
    )
    after = fuzz_notebook(rng, now) if rng.random() < 0.3 else notebook
    w["notebooks"] = [rng.choice((notebook, notebook, after)), after]
    w["papers"] = [
        fuzz_paper(rng, rng.choice(PAPER_IDS), now)
        for _ in range(rng.choice((0, 0, 1, 2)))
    ]
    if lock_nb["proposals"] and rng.random() < 0.85:
        w["proposal"] = list(rng.choice(lock_nb["proposals"]))
    elif rng.random() < 0.5:
        w["proposal"] = prop(
            "PR1", rng.choice(pair_notebook.CONSENT_PURPOSES), by=rng.choice(PEOPLE)
        )
    sheet = (
        coherent_paper(rng, fn, actor, now) if coherent else fuzz_paper(rng, "PP1", now)
    )
    locked = sheet if rng.random() < 0.9 else fuzz_paper(rng, "PP1", now)
    after_sheet = locked
    if fn == "respond_pair_paper" and rng.random() < 0.7:
        extra = [[locked["version"], actor, "dong_y"]]
        if rng.random() < 0.4:
            extra.append([locked["version"], rng.choice(PEOPLE), "dong_y"])
        after_sheet = dict(locked, responses=locked["responses"] + extra)
    w["reads"] = [
        sheet if rng.random() < 0.96 else None,
        after_sheet if rng.random() < 0.97 else None,
        rng.choice((locked, after_sheet, None)),
    ]
    w["locked"] = [locked if rng.random() < 0.97 else None]
    w["outings"] = [rng.choice((None, None, None, "OU1")), rng.choice((None, "OU1"))]
    conflicts = {}
    if rng.random() < 0.1:
        conflicts["add_paper_response"] = [
            rng.choice(("paper_already_agreed", "paper_already_agreed", "other"))
        ]
    if rng.random() < 0.1:
        conflicts["link_paper_outing"] = ["paper_outing_exists"]
    if rng.random() < 0.15:
        conflicts["set_couple_member"] = [
            rng.choice((None, "couple_slot_taken")),
            "couple_slot_taken",
        ]
    if conflicts:
        w["conflicts"] = conflicts
    if rng.random() < 0.3:
        w["constraint_version"] = rng.randint(1, 9)

    r = dict(DEFAULT_REQ[fn])
    if "proposal_id" in r:
        r["proposal_id"] = rng.choice(("PR1", "PR1", "PR2", "PR9"))
    if fn == "grant_pair_consent" and coherent:
        coherent_grant(rng, w, r, actor, now)
    if "purpose" in r:
        r["purpose"] = rng.choice(
            pair_notebook.CONSENT_PURPOSES * 5
            + (("", "LAP_SO", "x") if fn == "revoke_pair_consent" else ())
        )
    if "kind" in r:
        r["kind"] = rng.choice(pair_notebook.CONSTRAINT_KINDS * 5 + ("", "Dung"))
    if "content" in r and fn == "put_pair_constraint":
        r["content"] = rng.choice(TEXTS)
    if "lo" in r:
        r["lo"] = rng.choice(("toi", "nguoi_kia", "ca_hai"))
    if "revision" in r:
        right = revision_of(w, now)
        r["revision"] = rng.choice((right, right, right, right[:-1] + "x", ""))
    if "version" in r:
        current = locked["version"]
        r["version"] = rng.choice(
            (current,) * 8 + (current - 1, current + 1, 0, 2**63, -(2**64))
        )
    if fn == "edit_pair_draft":
        stops = [
            (
                rng.choice(("19:00", "06:00")),
                rng.choice(TEXTS[:2] + ("Chè",)),
                rng.choice((None, PLACE)),
                rng.random() < 0.8,
            )
            for _ in range(rng.choice((1, 2)))
        ]
        r["content"] = content_in(
            (SAT + timedelta(days=rng.randint(-5, 5))).isoformat(), stops
        )
        r["ly_do"] = rng.choice((None, "", " vì mưa ", rng.choice(TEXTS)))
    if fn == "respond_pair_paper" and rng.random() < 0.4:
        stop = ("18:00", "Phở", rng.choice((None, PLACE)), True)
        r["reply"] = {
            "kind": "de_nghi_sua",
            "content": content_in("2030-09-20", (stop,)),
            "ly_do": rng.choice((None, "", " sớm hơn ")),
        }
    if fn == "keep_pair_paper_line":
        r["line"] = rng.choice(TEXTS)
    if rng.random() < 0.03 and "context_id" in r:
        r["context_id"] = "HOI"
    return step_case(fn, f"fuzz/{i}", r, w, now=now, actor=(actor, roles))


def pair_steps_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 8)
    out = []
    for i in range(count):
        while True:
            built = fuzz_step(rng, i)
            if fits(built):
                break
        out.append(built)
    return out


FUNCTIONS = {
    "tuan_cua": lambda now: pair_paper.tuan_cua(now),
    "han_tuan": lambda now: pair_paper.han_tuan(now),
    "ngay_de_xuat": lambda now: pair_paper.ngay_de_xuat(now),
    "astimezone": lambda now: now.astimezone(pair_paper.MUI_GIO).isoformat(),
    "hieu_luc": lambda paper, now: pair_paper.hieu_luc(paper, now=now),
    "da_du_dong_y": pp_da_du_dong_y,
    "co_the_rut": pp_co_the_rut,
    "chuyen": pp_chuyen,
    "phac_to_giay": pp_phac,
    "lam_giau_phac": pp_lam_giau,
    "gu_cho_nep": pp_gu_cho_nep,
    "lam_giau_theo_gu": pp_theo_gu,
    **{
        name: (lambda name: lambda **kw: run_step(name, **kw))(name) for name in CALLERS
    },
}

#: module -> (Go package path, target, constants, edges, fuzz,
#:            cases in the committed sample, cases a live oracle run draws)
MODULES = {
    "pair_paper": (
        "internal/domain/pairpaper",
        pair_paper,
        pair_paper_constants,
        pair_paper_edges,
        pair_paper_fuzz,
        150,
        20000,
    ),
    "pair_steps": (
        "internal/domain/pairsteps",
        api_service,
        pair_steps_constants,
        pair_steps_edges,
        pair_steps_fuzz,
        30,
        6000,
    ),
}


def modes() -> dict[str, tuple[str, bool]]:
    """Every committed mode: the edges, and one shard holding the sample."""
    table: dict[str, tuple[str, bool]] = {}
    for module in MODULES:
        table[module] = (module, False)
        table[f"{module}-fuzz-0"] = (module, True)
    return table


def header(module: str, mode: str, seed: int) -> dict:
    return {
        "generator": "scripts/render_domain_w8_goldens.py",
        "module": MODULES[module][1].__name__,
        "mode": mode,
        "python": platform.python_version(),
        "seed": seed,
    }


def render(mode: str) -> dict:
    module, sample = modes()[mode]
    _path, _target, constants, edges, fuzz, sample_count, _live_count = MODULES[module]
    document = header(module, mode, SEED)
    if not sample:
        document["constants"] = enc(constants())
        document["cases"] = edges()
    else:
        cases = fuzz(SEED, sample_count)
        document["fuzz"] = {"shard": 0, "shards": 1, "total": len(cases)}
        document["cases"] = cases
    return document


def render_live(module: str, seed: int, count: int) -> dict:
    """A differential fuzz drawn for one oracle run; never committed."""
    cases = MODULES[module][4](seed, count)
    document = header(module, f"{module}-fuzz-live", seed)
    document["fuzz"] = {"shard": 0, "shards": 1, "total": len(cases)}
    document["cases"] = cases
    return document


def serialize(document: dict) -> str:
    """One header key per line, then one case per line."""
    lines = ["{"]
    for key, value in document.items():
        if key != "cases":
            lines.append(
                f" {json.dumps(key)}: {json.dumps(value, ensure_ascii=True, separators=(',', ':'))},"
            )
    cases = [
        json.dumps(item, ensure_ascii=True, separators=(",", ":"))
        for item in document["cases"]
    ]
    lines.append(' "cases": [')
    if cases:
        lines.append(",\n".join(cases))
    lines.append(" ]")
    lines.append("}")
    return "\n".join(lines) + "\n"


USAGE = "usage: - MODE | --list | --live MODULE SEED COUNT"


def main(argv: list[str]) -> int:
    table = modes()
    if argv == ["--list"]:
        for mode, (module, _sample) in table.items():
            path = TARGET.format(path=MODULES[module][0], mode=mode.replace("-", "_"))
            print(f"{mode}\t{path}")
        return 0
    if (
        len(argv) == 4
        and argv[0] == "--live"
        and argv[1] in MODULES
        and argv[2].isdigit()
        and argv[3].isdigit()
    ):
        sys.stdout.write(serialize(render_live(argv[1], int(argv[2]), int(argv[3]))))
        return 0
    if len(argv) != 1 or argv[0] not in table:
        print(
            f"{USAGE}; modes: {' '.join(table)}; live modules: {' '.join(MODULES)}",
            file=sys.stderr,
        )
        return 2
    rendered = serialize(render(argv[0]))
    problem = guard_problem(rendered)
    if problem is not None:
        print(problem, file=sys.stderr)
        return 1
    sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
