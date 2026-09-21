#!/usr/bin/env python3
"""Oracle for the Go port of the guest web pages (ADR-0029, wave W5).

The guest routes of app/api/routes/guests.py render HTML, so the Go port has
to answer with the same bytes Jinja2 produces. The pure pieces are ported to
services/core/internal/web/guest:

    app.web.guest_view                  format_vnd, build_guest_view, constants
    app.web.objection_view              build_not_me_view, build_wrong_amount_view
    app.api.service (pure steps)        guest_view, not_me_view, wrong_amount_view,
                                        record_objection, report_payment
    app.api.routes.guests + templates   the four templates through the real
                                        Jinja2Templates object, the route
                                        handlers over a stub repository
    starlette responses, guest_privacy  TemplateResponse, RedirectResponse(303),
                                        guest_aware_server_error_response

Go must answer exactly as Python answers, so this script calls the real code
inside the parity API image and records what it returned or raised. The image
is built from services/api (unchanged since 7bf58e3d):

    IMAGE=mobile-parity-api:7bf58e3d
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_guest_web_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_guest_web_goldens.py > "$path"
    done

`<module>` holds constants and named edge cases; `<module>-fuzz-0` is a small
seeded sample. `--live MODULE SEED COUNT` draws a larger fuzz for the Go tests
built with `-tags oracle` (oracle_live_test.go); it is never committed.

## Encoding

The encoding of scripts/render_domain_w4_goldens.py: a case is
`{"fn", "name", "args", "result"}`, a result `{"ok": value}` or
`{"raised": {"type", "message", "code"}}`, `"$i:<hex>"` an int of nine or more
digits, `"$sp:<text>"` a str cut into groups of six code points joined by `|`.
A tuple is a list.

A rendered body is too long for one line, and most of it is the same template
text in every case, so the files of the `pages` module carry a line table in
`constants.lines`: a body is the list of indices of its pieces, and the pieces
joined are the body, byte for byte. A piece is one line of the body, cut into
at most PIECE code points. The table is written one entry per line.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import platform
import random
import re
import string
import sys
import uuid
from datetime import UTC, datetime

sys.path.insert(0, "/srv")

from starlette.requests import Request  # noqa: E402
from starlette.responses import RedirectResponse  # noqa: E402

from app.api import limits  # noqa: E402
from app.api import service as api_service  # noqa: E402
from app.api import guest_privacy  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402
from app.api.repository import (  # noqa: E402
    GuestEnvelopeRecord,
    PaymentReportRecord,
    PaymentReportTarget,
    RepositoryConflict,
)
from app.api.routes import guests  # noqa: E402
from app.api.schemas import PaymentReportRequest  # noqa: E402
from app.web import guest_view, objection_view  # noqa: E402

SEED = 45
TARGET = "services/core/internal/web/guest/testdata/python_{mode}.json"
SMALL_INT = 10**8
GROUP = 6
PIECE = 120
LONG_CASE = 3000

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
# Encoding
# ---------------------------------------------------------------------------


def enc(value: object) -> object:
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, int):
        return value if -SMALL_INT < value < SMALL_INT else f"$i:{value:#_x}"
    if isinstance(value, uuid.UUID):
        return enc_str(str(value))
    if isinstance(value, (list, tuple)):
        return [enc(item) for item in value]
    if isinstance(value, (set, frozenset)):
        return [enc(item) for item in sorted(value)]
    if isinstance(value, dict):
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"unencodable {type(value).__name__}")


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


class Lines:
    """The line table of one pages file."""

    def __init__(self):
        self.index: dict[str, int] = {}
        self.table: list[str] = []

    def body(self, text: str) -> list[int]:
        out = []
        for line in re.findall(r"[^\n]*\n|[^\n]+", text):
            for i in range(0, len(line), PIECE):
                piece = line[i : i + PIECE]
                if piece not in self.index:
                    self.index[piece] = len(self.table)
                    self.table.append(piece)
                out.append(self.index[piece])
        return out


LINES = Lines()


def case(fn: str, name: str, kwargs: dict) -> dict:
    args = {key: enc(value) for key, value in kwargs.items()}
    try:
        value = FUNCTIONS[fn](**kwargs)
    except Exception as exc:  # every refusal is data for the oracle
        result = {
            "raised": {
                "type": type(exc).__name__,
                "message": enc_str(str(exc)),
                "code": enc(getattr(exc, "code", None)),
            }
        }
    else:
        result = {"ok": enc(value)}
    return {"fn": fn, "name": name, "args": args, "result": result}


# ---------------------------------------------------------------------------
# Inputs
# ---------------------------------------------------------------------------

O1 = "0b1b1b1b-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
O2 = "0b2b2b2b-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
O3 = "0b3b3b3b-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
TOKEN = "tok_" + "Ab-" * 10 + "cd"

HOSTILE = [
    "<script>alert('x')</script>",
    'Tom & "Jerry"',
    "O'Brien",
    "a&amp;b &#39; &lt;",
    "{{ view.claimed_person_display_name }} {% if %} {# x #}",
    "H" + chr(0x0E0) + " Nam",
    "Ha" + chr(0x0300) + chr(0x0323) + "i",
    chr(0x1F600) + " " + chr(0x1F1FB) + chr(0x1F1F3),
    chr(0x202E)
    + "abc"
    + chr(0x202C)
    + " "
    + chr(0x0645)
    + chr(0x0631)
    + chr(0x062D)
    + chr(0x0628)
    + chr(0x0627),
    "".join(chr(c) for c in (1, 8, 11, 12, 27, 31, 127)),
    "tab\tnew\nline\rcr" + chr(0x85) + chr(0xA0) + chr(0x2028),
    "",
    "   ",
    "None",
    "Nguy" + chr(0x1EC5) + "n " * 1,
    ("Nguy" + chr(0x1EC5) + "n V" + chr(0x0103) + "n ") * 60,
]


def obligation(i=1, **overrides) -> dict:
    data = {
        "obligation_id": [O1, O2, O3][(i - 1) % 3],
        "occasion_label": "b"
        + chr(0x1EEF)
        + "a l"
        + chr(0x1EA9)
        + "u t"
        + chr(0x1ED1)
        + "i",
        "amount_vnd": 82000,
        "recipient_display_name": "Nam",
        "already_reported": False,
        "evidence_requested": False,
        "disputed": False,
        "objections_used": 0,
        "objections_allowed": 3,
        "receiver_confirmed": False,
    }
    data.update(overrides)
    return data


def envelope(n=1, obligations=None, **overrides) -> dict:
    data = {
        "recorded_by_display_name": "Nam",
        "claimed_person_display_name": "H" + chr(0xE0),
        "link_state": "active",
        "obligations": [obligation(i + 1) for i in range(n)]
        if obligations is None
        else obligations,
        "reports_used": 0,
        "reports_allowed": 3,
        "objections_used": 0,
        "objections_allowed": 3,
    }
    data.update(overrides)
    return data


def without(mapping: dict, *keys) -> dict:
    return {k: v for k, v in mapping.items() if k not in keys}


# ---------------------------------------------------------------------------
# views: app.web.guest_view and app.web.objection_view
# ---------------------------------------------------------------------------


def format_vnd_digits(digit: str, count: int) -> dict:
    """format_vnd over int(digit * count), answered as length and digest."""
    text = guest_view.format_vnd(int(digit * count))
    return {"len": len(text), "sha256": hashlib.sha256(text.encode()).hexdigest()}


def views_constants() -> dict:
    return {
        "allowed_top_level": sorted(guest_view.ALLOWED_TOP_LEVEL),
        "allowed_block": sorted(guest_view.ALLOWED_BLOCK),
        "forbidden_input_keys": sorted(guest_view.FORBIDDEN_INPUT_KEYS),
        "allowed_not_me": sorted(objection_view.ALLOWED_NOT_ME),
        "allowed_wrong_amount": sorted(objection_view.ALLOWED_WRONG_AMOUNT),
        "objection_reasons": [list(pair) for pair in objection_view.OBJECTION_REASONS],
        "neutral_preview": dict(guest_view.NEUTRAL_PREVIEW),
        "objection_kinds": sorted(limits.OBJECTION_KINDS),
        "quota_consuming_objections": sorted(limits.QUOTA_CONSUMING_OBJECTIONS),
        "isspace": [c for c in range(0x110000) if chr(c).isspace()],
        "int_max_str_digits": sys.get_int_max_str_digits(),
    }


def views_edges() -> list[dict]:
    cases = []
    for n in (
        0,
        1,
        12,
        123,
        1234,
        12345,
        123456,
        1234567,
        999999,
        10**8 - 1,
        10**8,
        2**63 - 1,
        2**63,
        10**30 + 7,
    ):
        cases.append(
            case("format_vnd", f"int/{len(str(n))}/{n % 1000}", {"amount_vnd": n})
        )
    for name, bad in [
        ("negative", -1),
        ("int64_min", -(2**63)),
        ("past_int64_negative", -(10**30)),
        ("true", True),
        ("false", False),
        ("none", None),
        ("str", "82000"),
        ("list", [82000]),
        ("dict", {}),
    ]:
        cases.append(case("format_vnd", f"refused/{name}", {"amount_vnd": bad}))
    for digit, count in [("9", 4299), ("9", 4300), ("1", 4301), ("5", 1)]:
        cases.append(
            case(
                "format_vnd_digits",
                f"digits/{digit}x{count}",
                {"digit": digit, "count": count},
            )
        )

    g = "build_guest_view"
    cases.append(case(g, "baseline", {"envelope": envelope()}))
    cases.append(
        case(
            g,
            "extra_input_keys_are_dropped",
            {"envelope": envelope(bank_name="x", not_me_reported=True)},
        )
    )
    cases.append(case(g, "three_blocks", {"envelope": envelope(3)}))
    cases.append(case(g, "no_obligations", {"envelope": envelope(0)}))
    for key in sorted(guest_view.FORBIDDEN_INPUT_KEYS):
        cases.append(case(g, f"forbidden/{key}", {"envelope": envelope(**{key: None})}))
    for state in [
        "revoked",
        "expired",
        "rotated",
        "probably_fine",
        "ACTIVE",
        None,
        1,
        True,
        [],
        {"a": 1},
    ]:
        cases.append(
            case(g, f"link_state/{state!r}", {"envelope": envelope(link_state=state)})
        )
    cases.append(
        case(g, "link_state/missing", {"envelope": without(envelope(), "link_state")})
    )
    for key in ("recorded_by_display_name", "claimed_person_display_name"):
        cases.append(
            case(
                g,
                f"revoked_missing/{key}",
                {"envelope": without(envelope(link_state="revoked"), key)},
            )
        )
        cases.append(
            case(g, f"active_missing/{key}", {"envelope": without(envelope(), key)})
        )
    cases.append(
        case(
            g,
            "revoked_needs_no_obligations",
            {"envelope": without(envelope(link_state="expired"), "obligations")},
        )
    )
    for name, value in [
        ("none", None),
        ("empty_str", ""),
        ("str", "ab"),
        ("empty_dict", {}),
        ("dict", {"k": 1}),
        ("int", 5),
        ("list_of_none", [None]),
        ("list_of_str", ["x"]),
    ]:
        cases.append(
            case(g, f"obligations/{name}", {"envelope": envelope(obligations=value)})
        )
    cases.append(
        case(g, "obligations/missing", {"envelope": without(envelope(), "obligations")})
    )
    for key in (
        "obligation_id",
        "occasion_label",
        "amount_vnd",
        "recipient_display_name",
    ):
        cases.append(
            case(
                g,
                f"obligation_missing/{key}",
                {"envelope": envelope(obligations=[without(obligation(), key)])},
            )
        )
    cases.append(
        case(
            g,
            "order/label_before_amount",
            {
                "envelope": envelope(
                    obligations=[without(obligation(amount_vnd=-1), "occasion_label")]
                )
            },
        )
    )
    cases.append(
        case(
            g,
            "order/amount_before_recipient",
            {
                "envelope": envelope(
                    obligations=[
                        without(obligation(amount_vnd=-1), "recipient_display_name")
                    ]
                )
            },
        )
    )
    cases.append(
        case(
            g,
            "order/blocks_before_names",
            {
                "envelope": without(
                    envelope(obligations=[obligation(amount_vnd=-5)]),
                    "recorded_by_display_name",
                )
            },
        )
    )
    cases.append(
        case(
            g,
            "order/second_block_refused",
            {
                "envelope": envelope(
                    obligations=[obligation(1), obligation(2, amount_vnd=True)]
                )
            },
        )
    )
    for name, amount in [
        ("zero", 0),
        ("negative", -1),
        ("bool", False),
        ("str", "1"),
        ("none", None),
        ("past_int64", 2**64 + 1),
    ]:
        cases.append(
            case(
                g,
                f"amount/{name}",
                {"envelope": envelope(obligations=[obligation(amount_vnd=amount)])},
            )
        )
    for name, value in [
        ("none", None),
        ("zero", 0),
        ("one", 1),
        ("empty", ""),
        ("str", "no"),
        ("empty_list", []),
        ("list", [0]),
        ("empty_dict", {}),
        ("true", True),
    ]:
        cases.append(
            case(
                g,
                f"flags/{name}",
                {
                    "envelope": envelope(
                        obligations=[
                            obligation(
                                already_reported=value,
                                receiver_confirmed=value,
                                disputed=value,
                            )
                        ]
                    )
                },
            )
        )
    cases.append(
        case(
            g,
            "flags/missing",
            {
                "envelope": envelope(
                    obligations=[
                        without(
                            obligation(),
                            "already_reported",
                            "receiver_confirmed",
                            "disputed",
                        )
                    ]
                )
            },
        )
    )
    for name, used, allowed in [
        ("spent", 3, 3),
        ("over", 4, 3),
        ("one_left", 2, 3),
        ("bool", True, 2),
        ("big", 2**70, 2**70 + 1),
        ("str_str", "2", "10"),
        ("none", None, 3),
        ("str_int", "1", 3),
        ("int_none", 0, None),
    ]:
        cases.append(
            case(
                g,
                f"objections/{name}",
                {
                    "envelope": envelope(
                        obligations=[
                            obligation(objections_used=used, objections_allowed=allowed)
                        ]
                    )
                },
            )
        )
    cases.append(
        case(
            g,
            "objections/defaults",
            {
                "envelope": envelope(
                    obligations=[
                        without(obligation(), "objections_used", "objections_allowed")
                    ]
                )
            },
        )
    )
    cases.append(
        case(
            g,
            "objections/any_block",
            {
                "envelope": envelope(
                    obligations=[obligation(1, objections_used=3), obligation(2)]
                )
            },
        )
    )
    cases.append(
        case(
            g,
            "objections/no_block",
            {
                "envelope": envelope(
                    obligations=[
                        obligation(1, objections_used=3),
                        obligation(2, objections_used=5),
                    ]
                )
            },
        )
    )
    for name, used, allowed in [
        ("spent", 3, 3),
        ("left", 2, 3),
        ("none", None, 3),
        ("str", "a", "b"),
        ("big", 10**40, 3),
    ]:
        cases.append(
            case(
                g,
                f"reports/{name}",
                {"envelope": envelope(reports_used=used, reports_allowed=allowed)},
            )
        )
    cases.append(
        case(
            g,
            "reports/defaults",
            {"envelope": without(envelope(), "reports_used", "reports_allowed")},
        )
    )
    for i, text in enumerate(HOSTILE):
        cases.append(
            case(
                g,
                f"hostile/{i}",
                {
                    "envelope": envelope(
                        recorded_by_display_name=text,
                        claimed_person_display_name=text,
                        obligations=[
                            obligation(
                                occasion_label=text,
                                recipient_display_name=text,
                                obligation_id=text,
                            )
                        ],
                    )
                },
            )
        )
    cases.append(
        case(
            g,
            "odd_names",
            {
                "envelope": envelope(
                    recorded_by_display_name=None,
                    claimed_person_display_name=7,
                    obligations=[
                        obligation(occasion_label=None, recipient_display_name=True)
                    ],
                )
            },
        )
    )

    n = "build_not_me_view"
    cases.append(case(n, "baseline", {"envelope": envelope()}))
    cases.append(case(n, "reported", {"envelope": envelope(not_me_reported=True)}))
    for name, value in [("none", None), ("zero", 0), ("str", "x"), ("empty", "")]:
        cases.append(
            case(n, f"reported/{name}", {"envelope": envelope(not_me_reported=value)})
        )
    for key in sorted(guest_view.FORBIDDEN_INPUT_KEYS):
        cases.append(
            case(
                n,
                f"forbidden/{key}",
                {"envelope": envelope(link_state="revoked", **{key: 1})},
            )
        )
    for state in ["revoked", "expired", None, [], "active "]:
        cases.append(
            case(n, f"link_state/{state!r}", {"envelope": envelope(link_state=state)})
        )
    cases.append(
        case(n, "link_state/missing", {"envelope": without(envelope(), "link_state")})
    )
    for key in (
        "claimed_person_display_name",
        "recorded_by_display_name",
        "objections_used",
        "objections_allowed",
    ):
        cases.append(case(n, f"missing/{key}", {"envelope": without(envelope(), key)}))
    cases.append(
        case(
            n,
            "order/recorded_before_used",
            {
                "envelope": without(
                    envelope(), "recorded_by_display_name", "objections_used"
                )
            },
        )
    )
    for name, used, allowed in [
        ("spent", 3, 3),
        ("left", 2, 3),
        ("none", None, 3),
        ("str", "1", 3),
        ("strs", "b", "a"),
    ]:
        cases.append(
            case(
                n,
                f"objections/{name}",
                {
                    "envelope": envelope(
                        objections_used=used, objections_allowed=allowed
                    )
                },
            )
        )
    cases.append(
        case(
            n, "needs_no_obligations", {"envelope": without(envelope(), "obligations")}
        )
    )
    for i, text in enumerate(HOSTILE[:6]):
        cases.append(
            case(
                n,
                f"hostile/{i}",
                {
                    "envelope": envelope(
                        recorded_by_display_name=text, claimed_person_display_name=text
                    )
                },
            )
        )

    w = "build_wrong_amount_view"
    cases.append(case(w, "baseline", {"envelope": envelope(), "obligation_id": O1}))
    cases.append(case(w, "second", {"envelope": envelope(2), "obligation_id": O2}))
    cases.append(case(w, "unknown", {"envelope": envelope(2), "obligation_id": O3}))
    cases.append(
        case(
            w,
            "empty_id",
            {
                "envelope": envelope(obligations=[obligation(obligation_id="")]),
                "obligation_id": "",
            },
        )
    )
    cases.append(
        case(
            w,
            "duplicate_takes_first",
            {
                "envelope": envelope(
                    obligations=[
                        obligation(1, occasion_label="first"),
                        obligation(1, occasion_label="second"),
                    ]
                ),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "non_str_id_never_matches",
            {
                "envelope": envelope(obligations=[obligation(obligation_id=1)]),
                "obligation_id": "1",
            },
        )
    )
    for key in ("group_balance", "member_list"):
        cases.append(
            case(
                w,
                f"forbidden/{key}",
                {"envelope": envelope(**{key: 1}), "obligation_id": O1},
            )
        )
    cases.append(
        case(
            w,
            "revoked",
            {"envelope": envelope(link_state="revoked"), "obligation_id": O1},
        )
    )
    cases.append(
        case(
            w,
            "obligations/missing",
            {"envelope": without(envelope(), "obligations"), "obligation_id": O1},
        )
    )
    cases.append(
        case(
            w,
            "obligations/none",
            {"envelope": envelope(obligations=None), "obligation_id": O1},
        )
    )
    cases.append(
        case(
            w,
            "obligations/empty_str",
            {"envelope": envelope(obligations=""), "obligation_id": O1},
        )
    )
    cases.append(
        case(
            w,
            "every_entry_is_read",
            {
                "envelope": envelope(
                    obligations=[obligation(1), without(obligation(2), "obligation_id")]
                ),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "entry_not_a_dict",
            {
                "envelope": envelope(obligations=[obligation(1), "x"]),
                "obligation_id": O1,
            },
        )
    )
    for name, display in [
        ("str", "99.000"),
        ("empty", ""),
        ("zero", 0),
        ("int", 5),
        ("none", None),
        ("list", [1]),
    ]:
        cases.append(
            case(
                w,
                f"amount_display/{name}",
                {
                    "envelope": envelope(
                        obligations=[obligation(amount_display=display)]
                    ),
                    "obligation_id": O1,
                },
            )
        )
    cases.append(
        case(
            w,
            "amount_display_skips_amount",
            {
                "envelope": envelope(
                    obligations=[without(obligation(amount_display="1"), "amount_vnd")]
                ),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "amount/missing",
            {
                "envelope": envelope(obligations=[without(obligation(), "amount_vnd")]),
                "obligation_id": O1,
            },
        )
    )
    for name, amount in [("negative", -3), ("bool", True), ("big", 10**25)]:
        cases.append(
            case(
                w,
                f"amount/{name}",
                {
                    "envelope": envelope(obligations=[obligation(amount_vnd=amount)]),
                    "obligation_id": O1,
                },
            )
        )
    cases.append(
        case(
            w,
            "order/label_before_amount",
            {
                "envelope": envelope(
                    obligations=[without(obligation(amount_vnd=-1), "occasion_label")]
                ),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "order/names_before_label",
            {
                "envelope": without(
                    envelope(obligations=[without(obligation(), "occasion_label")]),
                    "recorded_by_display_name",
                ),
                "obligation_id": O1,
            },
        )
    )
    for key in ("objections_used", "objections_allowed"):
        cases.append(
            case(
                w,
                f"fallback_read_first/{key}",
                {"envelope": without(envelope(), key), "obligation_id": O1},
            )
        )
    cases.append(
        case(
            w,
            "envelope_counts_when_block_has_none",
            {
                "envelope": envelope(
                    obligations=[
                        without(obligation(), "objections_used", "objections_allowed")
                    ],
                    objections_used=3,
                ),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "block_counts_win",
            {"envelope": envelope(objections_used=3), "obligation_id": O1},
        )
    )
    for name, used, allowed in [("spent", 3, 3), ("none", None, 3), ("strs", "a", "b")]:
        cases.append(
            case(
                w,
                f"objections/{name}",
                {
                    "envelope": envelope(
                        obligations=[
                            obligation(objections_used=used, objections_allowed=allowed)
                        ]
                    ),
                    "obligation_id": O1,
                },
            )
        )
    for name, value in [("true", True), ("none", None), ("str", "x"), ("zero", 0)]:
        cases.append(
            case(
                w,
                f"evidence/{name}",
                {
                    "envelope": envelope(
                        obligations=[obligation(evidence_requested=value)]
                    ),
                    "obligation_id": O1,
                },
            )
        )
    for i, text in enumerate(HOSTILE[:6]):
        cases.append(
            case(
                w,
                f"hostile/{i}",
                {
                    "envelope": envelope(
                        obligations=[
                            obligation(obligation_id=text, occasion_label=text)
                        ]
                    ),
                    "obligation_id": text,
                },
            )
        )
    return cases


# ---------------------------------------------------------------------------
# steps: ApiService methods over a recording stub repository
# ---------------------------------------------------------------------------

NOW = datetime(2026, 9, 16, 9, 30, 12, 345678, tzinfo=UTC)
# The service reads the clock through _now; the oracle pins it.
api_service._now = lambda: NOW


class Stub:
    """A repository that answers from its facts and records what was called."""

    def __init__(self, envelope=None, target=None, conflict=None, receipts=()):
        self.envelope = envelope
        self.target = target
        self.conflict = conflict
        self.receipts = tuple(receipts)
        self.calls: list = []

    def get_guest_envelope(self, token_digest, now):
        self.calls.append("get_guest_envelope")
        if self.envelope is None:
            return None
        return GuestEnvelopeRecord(link_id=uuid.UUID(int=1), envelope=self.envelope)

    def save_guest_objection(self, *, token_digest, kind, obligation_id, reason, now):
        oid = None if obligation_id is None else str(obligation_id)
        self.calls.append(["save_guest_objection", kind, oid, reason])

    def get_payment_report_target(self, token_digest, obligation_id, now):
        self.calls.append("get_payment_report_target")
        if self.target is None:
            return None
        return PaymentReportTarget(
            link_id=uuid.UUID(int=1),
            obligation_id=obligation_id,
            amount_vnd=self.target["amount_vnd"],
            active_capability=self.target["active_capability"],
            reports_used=self.target["reports_used"],
        )

    def save_payment_report(self, *, target, idempotency_key, now):
        self.calls.append("save_payment_report")
        if self.conflict is not None:
            raise RepositoryConflict(self.conflict)
        return PaymentReportRecord(
            id=uuid.UUID(int=2),
            obligation_id=target.obligation_id,
            amount_vnd=target.amount_vnd,
            receipt_amounts_vnd=self.receipts,
        )


def problem_of(exc: ApiProblem) -> dict:
    return {"status": exc.status_code, "code": exc.code, "detail": exc.detail}


def new_service(stub: Stub) -> api_service.ApiService:
    return api_service.ApiService(stub, photo_storage=object())


def run_view(method: str, token: str, envelope, obligation_id=None) -> dict:
    stub = Stub(envelope=envelope)
    service = new_service(stub)
    try:
        if method == "wrong_amount_view":
            view = service.wrong_amount_view(token, obligation_id)
        else:
            view = getattr(service, method)(token)
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "view": None}
    return {"calls": stub.calls, "problem": None, "view": view}


def run_record_objection(token, kind, obligation_id, reason, envelope) -> dict:
    stub = Stub(envelope=envelope)
    oid = None if obligation_id is None else uuid.UUID(obligation_id)
    try:
        new_service(stub).record_objection(token, kind, oid, reason)
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc)}
    return {"calls": stub.calls, "problem": None}


def run_report_payment(token, target, conflict, receipts) -> dict:
    stub = Stub(target=target, conflict=conflict, receipts=receipts)
    request = PaymentReportRequest.model_construct(
        obligation_id=uuid.UUID(O1), idempotency_key=None
    )
    try:
        response = new_service(stub).report_payment(token, request)
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "status": None}
    return {"calls": stub.calls, "problem": None, "status": response.obligation_status}


def target(active=True, used=0, amount=82000) -> dict:
    return {"active_capability": active, "reports_used": used, "amount_vnd": amount}


def steps_edges() -> list[dict]:
    cases = []
    for method in ("guest_view", "not_me_view"):
        cases.append(case(method, "ok", {"token": TOKEN, "envelope": envelope(2)}))
        cases.append(case(method, "no_record", {"token": TOKEN, "envelope": None}))
        cases.append(
            case(
                method,
                "revoked",
                {"token": TOKEN, "envelope": envelope(link_state="revoked")},
            )
        )
        cases.append(
            case(
                method,
                "forbidden",
                {"token": TOKEN, "envelope": envelope(member_list=[])},
            )
        )
        cases.append(
            case(
                method,
                "key_error",
                {
                    "token": TOKEN,
                    "envelope": without(envelope(), "claimed_person_display_name"),
                },
            )
        )
        cases.append(case(method, "empty_envelope", {"token": "x", "envelope": {}}))
    cases.append(
        case(
            "guest_view",
            "unknown_state",
            {"token": TOKEN, "envelope": envelope(link_state="gone")},
        )
    )
    cases.append(
        case(
            "guest_view",
            "negative_amount",
            {
                "token": TOKEN,
                "envelope": envelope(obligations=[obligation(amount_vnd=-1)]),
            },
        )
    )
    w = "wrong_amount_view"
    cases.append(
        case(w, "ok", {"token": TOKEN, "envelope": envelope(2), "obligation_id": O2})
    )
    cases.append(
        case(w, "no_record", {"token": TOKEN, "envelope": None, "obligation_id": O1})
    )
    cases.append(
        case(
            w, "unknown", {"token": TOKEN, "envelope": envelope(), "obligation_id": O3}
        )
    )
    cases.append(
        case(
            w,
            "revoked",
            {
                "token": TOKEN,
                "envelope": envelope(link_state="expired"),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "negative_amount_escapes",
            {
                "token": TOKEN,
                "envelope": envelope(obligations=[obligation(amount_vnd=-1)]),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            w,
            "key_error",
            {
                "token": TOKEN,
                "envelope": without(envelope(), "objections_used"),
                "obligation_id": O1,
            },
        )
    )

    r = "record_objection"
    base = {
        "token": TOKEN,
        "kind": "wrong_amount",
        "obligation_id": O1,
        "reason": "amount_too_high",
        "envelope": envelope(2),
    }
    cases.append(case(r, "ok", base))
    for value, _label in objection_view.OBJECTION_REASONS:
        cases.append(case(r, f"reason/{value}", {**base, "reason": value}))
    for name, reason in [
        ("empty", ""),
        ("upper", "AMOUNT_TOO_HIGH"),
        ("space", "other "),
        ("label", "Chia sai ng"),
    ]:
        cases.append(
            case(
                r,
                f"unknown_reason/{name}",
                {**base, "reason": reason, "envelope": None},
            )
        )
    cases.append(
        case(
            r,
            "unknown_kind",
            {**base, "kind": "spam", "reason": "nope", "envelope": None},
        )
    )
    cases.append(case(r, "no_reason", {**base, "reason": None}))
    cases.append(case(r, "no_record", {**base, "envelope": None}))
    cases.append(
        case(r, "revoked", {**base, "envelope": envelope(link_state="revoked")})
    )
    cases.append(
        case(
            r,
            "link_state_missing",
            {**base, "envelope": without(envelope(), "link_state")},
        )
    )
    cases.append(
        case(
            r,
            "forbidden_keys_are_not_checked",
            {**base, "envelope": envelope(group_balance=1)},
        )
    )
    cases.append(case(r, "not_in_scope", {**base, "obligation_id": O3}))
    cases.append(
        case(
            r,
            "obligations_missing",
            {**base, "envelope": without(envelope(), "obligations")},
        )
    )
    cases.append(
        case(
            r,
            "scope_stops_at_first_match",
            {**base, "envelope": envelope(obligations=[obligation(1), "junk"])},
        )
    )
    cases.append(
        case(
            r,
            "malformed_before_match",
            {
                **base,
                "obligation_id": O2,
                "envelope": envelope(
                    obligations=[without(obligation(1), "obligation_id"), obligation(2)]
                ),
            },
        )
    )
    cases.append(
        case(
            r,
            "quota_spent",
            {
                **base,
                "envelope": envelope(obligations=[obligation(1, objections_used=3)]),
            },
        )
    )
    cases.append(
        case(
            r,
            "quota_other_block_spent",
            {
                **base,
                "envelope": envelope(
                    obligations=[obligation(1), obligation(2, objections_used=3)]
                ),
            },
        )
    )
    cases.append(
        case(
            r,
            "quota_block_missing_count",
            {
                **base,
                "envelope": envelope(
                    obligations=[without(obligation(1), "objections_used")]
                ),
            },
        )
    )
    cases.append(
        case(
            r,
            "quota_type_error",
            {
                **base,
                "envelope": envelope(obligations=[obligation(1, objections_used="3")]),
            },
        )
    )
    cases.append(
        case(
            r,
            "quota_strings",
            {
                **base,
                "envelope": envelope(
                    obligations=[
                        obligation(1, objections_used="a", objections_allowed="b")
                    ]
                ),
            },
        )
    )
    cases.append(
        case(
            r,
            "evidence_ignores_quota",
            {
                **base,
                "kind": "evidence_request",
                "reason": None,
                "envelope": envelope(obligations=[obligation(1, objections_used=9)]),
            },
        )
    )
    nm = {"token": TOKEN, "kind": "not_me", "obligation_id": None, "reason": None}
    cases.append(case(r, "not_me/ok", {**nm, "envelope": envelope()}))
    cases.append(
        case(
            r, "not_me/envelope_quota", {**nm, "envelope": envelope(objections_used=3)}
        )
    )
    cases.append(
        case(
            r,
            "not_me/block_quota_ignored",
            {
                **nm,
                "envelope": envelope(obligations=[obligation(1, objections_used=3)]),
            },
        )
    )
    cases.append(
        case(
            r,
            "not_me/id_none_matches_str_none",
            {
                **nm,
                "envelope": envelope(
                    obligations=[obligation(1, obligation_id="None", objections_used=3)]
                ),
            },
        )
    )
    cases.append(
        case(
            r,
            "not_me/envelope_count_missing",
            {**nm, "envelope": without(envelope(), "objections_allowed")},
        )
    )
    cases.append(
        case(r, "not_me/revoked", {**nm, "envelope": envelope(link_state="revoked")})
    )
    cases.append(
        case(
            r,
            "not_me/obligations_unread",
            {**nm, "envelope": without(envelope(), "obligations")},
        )
    )

    p = "report_payment"
    rp = {"token": TOKEN, "target": target(), "conflict": None, "receipts": []}
    cases.append(case(p, "ok", rp))
    cases.append(case(p, "no_target", {**rp, "target": None}))
    for name, t in [
        ("inactive", target(active=False)),
        ("budget_spent", target(used=3)),
        ("both", target(active=False, used=7)),
        ("two_used", target(used=2)),
    ]:
        cases.append(case(p, name, {**rp, "target": t}))
    for code in ("IDEMPOTENCY_KEY_REUSED", "Some_Code"):
        cases.append(case(p, f"conflict/{code}", {**rp, "conflict": code}))
    for name, receipts in [
        ("partial", [1000]),
        ("confirmed", [82000]),
        ("over", [50000, 40000]),
    ]:
        cases.append(case(p, f"receipts/{name}", {**rp, "receipts": receipts}))
    cases.append(case(p, "zero_amount", {**rp, "target": target(amount=0)}))
    cases.append(case(p, "negative_receipt", {**rp, "receipts": [-1]}))
    return cases


# ---------------------------------------------------------------------------
# pages: templates, responses and the route handlers
# ---------------------------------------------------------------------------


def request_for(path: str = "/g/x", headers=()) -> Request:
    return Request(
        {
            "type": "http",
            "method": "GET",
            "path": path,
            "headers": [(k.encode("latin-1"), v.encode("latin-1")) for k, v in headers],
            "query_string": b"",
            "scheme": "http",
            "server": ("testserver", 80),
        }
    )


def response_shape(response) -> dict:
    return {
        "status": response.status_code,
        "headers": [
            [k.decode("latin-1"), v.decode("latin-1")] for k, v in response.raw_headers
        ],
        "body": LINES.body(bytes(response.body).decode("utf-8")),
    }


def template_response(name, context, status_code) -> dict:
    response = guests.templates.TemplateResponse(
        request=request_for(), name=name, context=dict(context), status_code=status_code
    )
    return response_shape(response)


def link_broken_page() -> dict:
    return response_shape(guests.guest_link_broken_page(request_for()))


def see_other(url) -> dict:
    return response_shape(RedirectResponse(url=url, status_code=303))


def server_error(path) -> dict:
    handler = guest_privacy.guest_aware_server_error_response
    return response_shape(asyncio.run(handler(request_for(path), RuntimeError("boom"))))


def accepts_html(values) -> bool:
    request = request_for(headers=[("accept", value) for value in values])
    return "text/html" in request.headers.get("accept", "")


def link_broken_applies(code, url_path) -> bool:
    # The condition of api_problem_handler in app/api/main.py, over the same
    # constant and predicate it reads.
    from app.api.errors import GUEST_LINK_NOT_FOUND

    return code == GUEST_LINK_NOT_FOUND and guest_privacy.is_guest_path(url_path)


def run_route(
    route,
    token,
    envelope=None,
    obligation_id=None,
    reason=None,
    accept=None,
    target=None,
    conflict=None,
    receipts=(),
) -> dict:
    stub = Stub(envelope=envelope, target=target, conflict=conflict, receipts=receipts)
    request = request_for(f"/g/{token}", [] if accept is None else [("accept", accept)])
    try:
        if route == "guest_page":
            response = guests.guest_page(request, token, stub)
        elif route == "not_me_page":
            response = guests.not_me_page(request, token, stub)
        elif route == "not_me_submit":
            response = guests.not_me_submit(request, token, stub)
        elif route == "wrong_amount_page":
            response = guests.wrong_amount_page(request, token, stub, obligation_id)
        elif route == "wrong_amount_submit":
            response = guests.wrong_amount_submit(token, stub, obligation_id, reason)
        elif route == "request_evidence":
            response = guests.request_evidence(token, stub, obligation_id)
        else:
            body = PaymentReportRequest.model_construct(
                obligation_id=uuid.UUID(obligation_id), idempotency_key=None
            )
            response = guests.report_payment(request, token, body, stub)
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "response": None}
    if not hasattr(response, "raw_headers"):
        return {
            "calls": stub.calls,
            "problem": None,
            "response": {"model_status": response.obligation_status},
        }
    return {"calls": stub.calls, "problem": None, "response": response_shape(response)}


def context(view, token=TOKEN, preview=None) -> dict:
    out = {
        "preview": dict(guest_view.NEUTRAL_PREVIEW) if preview is None else preview,
        "token": token,
    }
    if view is not None:
        out["view"] = view
    return out


def guest_block(i=1, **overrides) -> dict:
    view = guest_view.build_guest_view(envelope(obligations=[obligation(i)]))
    block = view["blocks"][0]
    block.update(overrides)
    return block


def guest_page_view(blocks, **overrides) -> dict:
    view = guest_view.build_guest_view(envelope(0))
    view["blocks"] = blocks
    view.update(overrides)
    return view


def pages_edges() -> list[dict]:
    cases = []
    t = "template_response"
    G, N, W, B = (
        "guest.html",
        "guest_not_me.html",
        "guest_wrong_amount.html",
        "guest_link_broken.html",
    )
    cases.append(
        case(
            t,
            "guest/one",
            {
                "name": G,
                "context": context(guest_view.build_guest_view(envelope())),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/three",
            {
                "name": G,
                "context": context(
                    guest_page_view(
                        [
                            guest_block(1, disputed=True),
                            guest_block(2, already_reported=True, can_object=False),
                            guest_block(3, receiver_confirmed=True),
                        ]
                    )
                ),
                "status_code": 200,
            },
        )
    )
    for state in ("revoked", "expired", "rotated"):
        cases.append(
            case(
                t,
                f"guest/{state}",
                {
                    "name": G,
                    "context": context(
                        guest_view.build_guest_view(envelope(link_state=state))
                    ),
                    "status_code": 200,
                },
            )
        )
    for name, state in [("unknown", "gone"), ("none", None), ("int", 3)]:
        cases.append(
            case(
                t,
                f"guest/state_{name}",
                {
                    "name": G,
                    "context": context(guest_page_view([], link_state=state)),
                    "status_code": 200,
                },
            )
        )
    missing = guest_page_view([])
    del missing["link_state"]
    cases.append(
        case(
            t,
            "guest/state_missing",
            {"name": G, "context": context(missing), "status_code": 200},
        )
    )
    cases.append(
        case(
            t,
            "guest/active_no_blocks",
            {"name": G, "context": context(guest_page_view([])), "status_code": 200},
        )
    )
    cases.append(
        case(
            t,
            "guest/cannot_report",
            {
                "name": G,
                "context": context(
                    guest_page_view([guest_block()], can_report_payment=False)
                ),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/reported_and_confirmed",
            {
                "name": G,
                "context": context(
                    guest_page_view(
                        [guest_block(already_reported=True, receiver_confirmed=True)]
                    )
                ),
                "status_code": 200,
            },
        )
    )
    for name, value in [
        ("str_false", "False"),
        ("zero", 0),
        ("empty", ""),
        ("none", None),
        ("list", [0]),
    ]:
        cases.append(
            case(
                t,
                f"guest/truthy_{name}",
                {
                    "name": G,
                    "context": context(
                        guest_page_view(
                            [
                                guest_block(
                                    disputed=value,
                                    already_reported=value,
                                    can_object=value,
                                )
                            ],
                            can_report_payment=value,
                        )
                    ),
                    "status_code": 200,
                },
            )
        )
    cases.append(
        case(
            t,
            "guest/block_keys_missing",
            {"name": G, "context": context(guest_page_view([{}])), "status_code": 200},
        )
    )
    cases.append(
        case(
            t,
            "guest/block_none",
            {
                "name": G,
                "context": context(guest_page_view([None])),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/odd_values",
            {
                "name": G,
                "context": context(
                    guest_page_view(
                        [
                            guest_block(
                                amount_vnd=2**80,
                                amount_display=None,
                                recipient_display_name=True,
                                occasion_label=7,
                            )
                        ],
                        recorded_by_display_name=None,
                        claimed_person_display_name=-1,
                    )
                ),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/blocks_missing",
            {
                "name": G,
                "context": context(without(guest_page_view([]), "blocks")),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/blocks_none",
            {"name": G, "context": context(guest_page_view(None)), "status_code": 200},
        )
    )
    cases.append(
        case(
            t,
            "guest/view_missing",
            {"name": G, "context": context(None), "status_code": 200},
        )
    )
    cases.append(
        case(
            t,
            "guest/preview_missing",
            {
                "name": G,
                "context": without(context(guest_page_view([])), "preview"),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/token_missing",
            {
                "name": G,
                "context": without(context(guest_page_view([guest_block()])), "token"),
                "status_code": 200,
            },
        )
    )
    cases.append(
        case(
            t,
            "guest/status_404",
            {"name": G, "context": context(guest_page_view([])), "status_code": 404},
        )
    )
    for i, text in enumerate(HOSTILE):
        view = guest_page_view(
            [
                guest_block(
                    obligation_id=text,
                    occasion_label=text,
                    amount_display=text,
                    recipient_display_name=text,
                    amount_vnd=text,
                )
            ],
            recorded_by_display_name=text,
            claimed_person_display_name=text,
        )
        cases.append(
            case(
                t,
                f"guest/hostile_{i}",
                {
                    "name": G,
                    "context": context(
                        view, token=text, preview={"title": text, "description": text}
                    ),
                    "status_code": 200,
                },
            )
        )
        if i >= 6:
            continue
        cases.append(
            case(
                t,
                f"guest/hostile_revoked_{i}",
                {
                    "name": G,
                    "context": context(
                        guest_page_view(
                            [], link_state="rotated", recorded_by_display_name=text
                        )
                    ),
                    "status_code": 200,
                },
            )
        )

    for reported in (False, True):
        for can in (False, True):
            view = {
                "claimed_person_display_name": "H" + chr(0xE0),
                "recorded_by_display_name": "Nam",
                "already_reported": reported,
                "can_object": can,
            }
            cases.append(
                case(
                    t,
                    f"not_me/{reported}_{can}",
                    {"name": N, "context": context(view), "status_code": 200},
                )
            )
    for i, text in enumerate(HOSTILE[:10]):
        view = {
            "claimed_person_display_name": text,
            "recorded_by_display_name": text,
            "already_reported": i % 2 == 0,
            "can_object": i % 3 == 0,
        }
        cases.append(
            case(
                t,
                f"not_me/hostile_{i}",
                {"name": N, "context": context(view, token=text), "status_code": 200},
            )
        )
    cases.append(
        case(
            t,
            "not_me/view_empty",
            {"name": N, "context": context({}), "status_code": 200},
        )
    )

    wbase = objection_view.build_wrong_amount_view(envelope(), O1)
    for can in (False, True):
        for asked, can_ask in [
            (False, True),
            (True, False),
            (False, False),
            (True, True),
        ]:
            view = {
                **wbase,
                "can_object": can,
                "evidence_requested": asked,
                "can_request_evidence": can_ask,
            }
            cases.append(
                case(
                    t,
                    f"wrong_amount/{can}_{asked}_{can_ask}",
                    {"name": W, "context": context(view), "status_code": 200},
                )
            )
    for name, reasons in [
        ("empty", []),
        ("one", [["a", "b"]]),
        ("hostile", [[text, text] for text in HOSTILE[:5]]),
        ("tuples_missing", None),
    ]:
        view = (
            {**wbase, "reasons": reasons}
            if reasons is not None
            else without(wbase, "reasons")
        )
        cases.append(
            case(
                t,
                f"wrong_amount/reasons_{name}",
                {"name": W, "context": context(view), "status_code": 200},
            )
        )
    for name, reasons in [
        ("arity_one", [["a"]]),
        ("arity_three", [["a", "b", "c"]]),
        ("int_item", [5]),
        ("none", None),
        ("int", 3),
        ("none_item", [None]),
    ]:
        cases.append(
            case(
                t,
                f"wrong_amount/reasons_bad_{name}",
                {
                    "name": W,
                    "context": context({**wbase, "reasons": reasons}),
                    "status_code": 200,
                },
            )
        )
    for i, text in enumerate(HOSTILE[:10]):
        view = {
            **wbase,
            "claimed_person_display_name": text,
            "recorded_by_display_name": text,
            "occasion_label": text,
            "amount_display": text,
            "obligation_id": text,
        }
        cases.append(
            case(
                t,
                f"wrong_amount/hostile_{i}",
                {"name": W, "context": context(view, token=text), "status_code": 200},
            )
        )

    cases.append(
        case(
            t,
            "link_broken/plain",
            {
                "name": B,
                "context": {"preview": dict(guest_view.NEUTRAL_PREVIEW)},
                "status_code": 404,
            },
        )
    )
    cases.append(
        case(
            t,
            "link_broken/with_view",
            {
                "name": B,
                "context": context(
                    wbase, preview={"title": HOSTILE[0], "description": HOSTILE[1]}
                ),
                "status_code": 200,
            },
        )
    )

    cases.append(case("link_broken_page", "page", {}))
    urls = [
        "/g/" + TOKEN,
        "/g/" + TOKEN + "/doi-so-tien?obligation_id=" + O1,
        "/g/t/doi-so-tien?obligation_id={" + O1.upper() + "}",
        "/g/t?obligation_id=a b&c=%41#frag",
        "/g/" + chr(0xE9) + chr(0x4E2D) + chr(0x1F600),
        "/g/t?x=" + "".join(chr(c) for c in (1, 9, 10, 127, 128, 255)),
        "/g/t?q=\"'<>`^{|}\\",
        "",
    ]
    for i, url in enumerate(urls):
        cases.append(case("see_other", f"url/{i}", {"url": url}))
    for path in [
        "/g",
        "/g/",
        "/g/abc",
        "/g/abc/khong-phai-toi",
        "/goals",
        "/",
        "",
        "/G/abc",
        "/api/g/x",
        "//g/x",
        "/g%2Fx",
    ]:
        cases.append(case("server_error", f"path/{path!r}", {"path": path}))
    for name, values in [
        ("none", []),
        ("html", ["text/html"]),
        ("json", ["application/json"]),
        ("browser", ["text/html,application/xhtml+xml;q=0.9"]),
        ("second_value", ["application/json", "text/html"]),
        ("upper", ["TEXT/HTML"]),
        ("prefix", ["text/htm"]),
        ("star", ["*/*"]),
    ]:
        cases.append(case("accepts_html", name, {"values": values}))
    for code in ("guest_link_not_found", "link_not_active", "GUEST_LINK_NOT_FOUND"):
        for path in ("/g/abc", "/g", "/goals", "/y/g/abc", ""):
            cases.append(
                case(
                    "link_broken_applies",
                    f"{code}{path}",
                    {"code": code, "url_path": path},
                )
            )

    rt = "route"
    cases.append(
        case(
            rt,
            "guest_page/ok",
            {"route": "guest_page", "token": TOKEN, "envelope": envelope(2)},
        )
    )
    cases.append(
        case(
            rt,
            "guest_page/no_record",
            {"route": "guest_page", "token": TOKEN, "envelope": None},
        )
    )
    cases.append(
        case(
            rt,
            "guest_page/revoked",
            {
                "route": "guest_page",
                "token": TOKEN,
                "envelope": envelope(link_state="revoked"),
            },
        )
    )
    cases.append(
        case(
            rt,
            "guest_page/unknown_state",
            {
                "route": "guest_page",
                "token": TOKEN,
                "envelope": envelope(link_state="gone"),
            },
        )
    )
    cases.append(
        case(
            rt,
            "not_me_page/ok",
            {"route": "not_me_page", "token": TOKEN, "envelope": envelope()},
        )
    )
    cases.append(
        case(
            rt,
            "not_me_page/revoked",
            {
                "route": "not_me_page",
                "token": TOKEN,
                "envelope": envelope(link_state="revoked"),
            },
        )
    )
    cases.append(
        case(
            rt,
            "not_me_submit/ok",
            {"route": "not_me_submit", "token": TOKEN, "envelope": envelope()},
        )
    )
    cases.append(
        case(
            rt,
            "not_me_submit/quota",
            {
                "route": "not_me_submit",
                "token": TOKEN,
                "envelope": envelope(objections_used=3),
            },
        )
    )
    cases.append(
        case(
            rt,
            "not_me_submit/no_record",
            {"route": "not_me_submit", "token": TOKEN, "envelope": None},
        )
    )
    cases.append(
        case(
            rt,
            "not_me_submit/hostile",
            {
                "route": "not_me_submit",
                "token": TOKEN,
                "envelope": envelope(
                    claimed_person_display_name=HOSTILE[0],
                    recorded_by_display_name=HOSTILE[2],
                ),
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/given",
            {
                "route": "wrong_amount_page",
                "token": TOKEN,
                "envelope": envelope(2),
                "obligation_id": O2,
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/first",
            {"route": "wrong_amount_page", "token": TOKEN, "envelope": envelope(2)},
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/no_blocks",
            {"route": "wrong_amount_page", "token": TOKEN, "envelope": envelope(0)},
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/revoked_default",
            {
                "route": "wrong_amount_page",
                "token": TOKEN,
                "envelope": envelope(link_state="revoked"),
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/revoked_given",
            {
                "route": "wrong_amount_page",
                "token": TOKEN,
                "envelope": envelope(link_state="revoked"),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/unknown",
            {
                "route": "wrong_amount_page",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": O3,
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/no_record",
            {"route": "wrong_amount_page", "token": TOKEN, "envelope": None},
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_page/negative",
            {
                "route": "wrong_amount_page",
                "token": TOKEN,
                "envelope": envelope(obligations=[obligation(amount_vnd=-2)]),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_submit/ok",
            {
                "route": "wrong_amount_submit",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": O1,
                "reason": "other",
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_submit/reason",
            {
                "route": "wrong_amount_submit",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": O1,
                "reason": "x",
            },
        )
    )
    cases.append(
        case(
            rt,
            "wrong_amount_submit/quota",
            {
                "route": "wrong_amount_submit",
                "token": TOKEN,
                "envelope": envelope(obligations=[obligation(objections_used=3)]),
                "obligation_id": O1,
                "reason": "other",
            },
        )
    )
    cases.append(
        case(
            rt,
            "request_evidence/ok",
            {
                "route": "request_evidence",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": O1,
            },
        )
    )
    cases.append(
        case(
            rt,
            "request_evidence/raw_id_echoed",
            {
                "route": "request_evidence",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": "{" + O1.upper() + "}",
            },
        )
    )
    cases.append(
        case(
            rt,
            "request_evidence/scope",
            {
                "route": "request_evidence",
                "token": TOKEN,
                "envelope": envelope(),
                "obligation_id": O2,
            },
        )
    )
    for accept in ("text/html", "application/json", None):
        cases.append(
            case(
                rt,
                f"report_payment/{accept}",
                {
                    "route": "report_payment",
                    "token": TOKEN,
                    "obligation_id": O1,
                    "accept": accept,
                    "target": target(),
                },
            )
        )
    cases.append(
        case(
            rt,
            "report_payment/no_target",
            {
                "route": "report_payment",
                "token": TOKEN,
                "obligation_id": O1,
                "accept": "text/html",
                "target": None,
            },
        )
    )
    cases.append(
        case(
            rt,
            "report_payment/spent",
            {
                "route": "report_payment",
                "token": TOKEN,
                "obligation_id": O1,
                "accept": "text/html",
                "target": target(used=3),
            },
        )
    )
    cases.append(
        case(
            rt,
            "report_payment/conflict",
            {
                "route": "report_payment",
                "token": TOKEN,
                "obligation_id": O1,
                "accept": "text/html",
                "target": target(),
                "conflict": "IDEMPOTENCY_KEY_REUSED",
            },
        )
    )
    return cases


# ---------------------------------------------------------------------------
# Seeded fuzz
# ---------------------------------------------------------------------------

# Built, not spelled: ten digits in a row trip the repository guard.
DIGITS = "".join(str(d) for d in range(10))
POOLS = [
    "abcdefghijklmnopqrstuvwxyz ",
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "<>&'\"",
    "{}%#-+/=?",
    "".join(
        chr(c)
        for c in (0xE0, 0xE1, 0x1EA1, 0x1EA3, 0x1EC5, 0x0111, 0x01B0, 0x1EEF, 0x0103)
    ),
    "".join(chr(c) for c in (0x0300, 0x0301, 0x0323, 0x0309)),
    "".join(chr(c) for c in (0x1F600, 0x1F44D, 0x2764, 0xFE0F)),
    "".join(chr(c) for c in (0x202E, 0x202C, 0x200F, 0x0627, 0x05D0)),
    "".join(chr(c) for c in (1, 7, 9, 10, 13, 27, 31, 127, 0x85, 0xA0, 0x2028)),
    DIGITS,
]
TOKEN_ALPHABET = string.ascii_letters + DIGITS + "_-"


def rand_text(rng: random.Random) -> str:
    roll = rng.random()
    if roll < 0.08:
        return ""
    if roll < 0.14:
        return rng.choice(HOSTILE)
    length = rng.randint(1, 24) if rng.random() < 0.95 else rng.randint(100, 300)
    alphabet = "".join(rng.sample(POOLS, rng.randint(1, 3)))
    return "".join(rng.choice(alphabet) for _ in range(length))


def rand_int(rng: random.Random) -> int:
    band = rng.randrange(6)
    if band == 0:
        return rng.randint(0, 999)
    if band == 1:
        return rng.randint(1000, 10**8)
    if band == 2:
        return rng.randint(10**8, 2**63 - 1)
    if band == 3:
        return rng.randint(2**63, 10**40)
    if band == 4:
        return -rng.randint(1, 10**20)
    return rng.choice([0, 1, 999, 1000, 999999, 1000000, 2**63 - 1, 2**63])


def rand_count(rng: random.Random) -> object:
    roll = rng.random()
    if roll < 0.9:
        return rng.randint(0, 4)
    return rng.choice([None, True, False, "2", 10**30, -1])


def rand_flag(rng: random.Random) -> object:
    if rng.random() < 0.9:
        return rng.random() < 0.3
    return rng.choice([None, 0, 1, "", "no", [], [0], {}])


def rand_envelope(rng: random.Random, corrupt: float = 0.3) -> dict:
    obligations = []
    for i in range(rng.choice([0, 1, 1, 1, 2, 2, 3, 4])):
        o = obligation(
            i + 1,
            obligation_id=rng.choice([O1, O2, O3])
            if rng.random() < 0.9
            else rand_text(rng),
            occasion_label=rand_text(rng),
            recipient_display_name=rand_text(rng),
            amount_vnd=rng.randint(0, 10**9) if rng.random() < 0.8 else rand_int(rng),
            already_reported=rand_flag(rng),
            receiver_confirmed=rand_flag(rng),
            disputed=rand_flag(rng),
            evidence_requested=rand_flag(rng),
            objections_used=rand_count(rng),
            objections_allowed=3 if rng.random() < 0.9 else rand_count(rng),
        )
        if rng.random() < 0.1:
            o["amount_display"] = rng.choice([None, "", rand_text(rng), 0, 7])
        obligations.append(o)
    env = envelope(
        obligations=obligations,
        recorded_by_display_name=rand_text(rng),
        claimed_person_display_name=rand_text(rng),
        link_state=rng.choice(["active"] * 6 + ["revoked", "expired", "rotated"]),
        reports_used=rand_count(rng),
        objections_used=rand_count(rng),
    )
    if rng.random() < 0.2:
        env["not_me_reported"] = rand_flag(rng)
    if rng.random() < corrupt:
        roll = rng.randrange(6)
        if roll == 0:
            env.pop(rng.choice(sorted(env)))
        elif roll == 1 and obligations:
            target_obligation = rng.choice(obligations)
            target_obligation.pop(rng.choice(sorted(target_obligation)))
        elif roll == 2:
            env["link_state"] = rng.choice([None, 1, "gone", "Active", True])
        elif roll == 3 and obligations:
            rng.choice(obligations)["amount_vnd"] = rng.choice(
                [True, None, "82000", -5, -(10**30)]
            )
        elif roll == 4:
            env[rng.choice(sorted(guest_view.FORBIDDEN_INPUT_KEYS))] = rand_text(rng)
        else:
            env["obligations"] = rng.choice([None, "", "ab", 5, {}, {"k": 1}, [None]])
    return env


def views_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed)
    cases = []
    for i in range(count):
        roll = rng.random()
        if roll < 0.1:
            amount = (
                rand_int(rng) if rng.random() < 0.9 else rng.choice([True, None, "1"])
            )
            cases.append(case("format_vnd", f"fuzz/{i}", {"amount_vnd": amount}))
            continue
        env = rand_envelope(rng)
        if roll < 0.6:
            cases.append(case("build_guest_view", f"fuzz/{i}", {"envelope": env}))
        elif roll < 0.8:
            cases.append(case("build_not_me_view", f"fuzz/{i}", {"envelope": env}))
        else:
            rows = env.get("obligations")
            ids = (
                [o.get("obligation_id") for o in rows if isinstance(o, dict)]
                if isinstance(rows, list)
                else []
            )
            ids = [x for x in ids if isinstance(x, str)]
            oid = (
                rng.choice(ids) if ids and rng.random() < 0.85 else rng.choice([O3, ""])
            )
            cases.append(
                case(
                    "build_wrong_amount_view",
                    f"fuzz/{i}",
                    {"envelope": env, "obligation_id": oid},
                )
            )
    return cases


def rand_token(rng: random.Random) -> str:
    if rng.random() < 0.15:
        return rand_text(rng)
    return "".join(rng.choice(TOKEN_ALPHABET) for _ in range(rng.randint(32, 64)))


def mutate_view(rng: random.Random, view: dict) -> None:
    """Swap one printed or tested value for another Python type, or drop it."""
    holders = [view] + [b for b in view.get("blocks", []) if isinstance(b, dict)]
    holder = rng.choice(holders)
    keys = sorted(k for k in holder if k not in ("blocks", "reasons"))
    if not keys:
        return
    key = rng.choice(keys)
    if rng.random() < 0.2:
        del holder[key]
    else:
        holder[key] = rng.choice(
            [
                None,
                True,
                False,
                rng.randint(-5, 10**12),
                rand_text(rng),
                "active",
                "revoked",
            ]
        )


def pages_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed)
    cases = []
    for i in range(count):
        env = rand_envelope(rng, corrupt=0)
        token = rand_token(rng)
        preview = (
            None
            if rng.random() < 0.9
            else {"title": rand_text(rng), "description": rand_text(rng)}
        )
        roll = rng.random()
        try:
            if roll < 0.5:
                name, view = "guest.html", guest_view.build_guest_view(env)
            elif roll < 0.7:
                name, view = "guest_not_me.html", objection_view.build_not_me_view(env)
            elif roll < 0.95:
                name = "guest_wrong_amount.html"
                view = objection_view.build_wrong_amount_view(
                    env, env["obligations"][0]["obligation_id"]
                )
            else:
                name, view = "guest_link_broken.html", None
        except Exception:
            name, view = (
                "guest.html",
                guest_view.build_guest_view(
                    envelope(link_state=rng.choice(["active", "expired"]))
                ),
            )
        if view is not None and rng.random() < 0.2:
            mutate_view(rng, view)
        status = rng.choice([200, 200, 200, 404])
        cases.append(
            case(
                "template_response",
                f"fuzz/{i}",
                {
                    "name": name,
                    "context": context(view, token, preview),
                    "status_code": status,
                },
            )
        )
    return cases


# ---------------------------------------------------------------------------
# Modes
# ---------------------------------------------------------------------------

FUNCTIONS = {
    "format_vnd": guest_view.format_vnd,
    "format_vnd_digits": format_vnd_digits,
    "build_guest_view": guest_view.build_guest_view,
    "build_not_me_view": objection_view.build_not_me_view,
    "build_wrong_amount_view": objection_view.build_wrong_amount_view,
    "guest_view": lambda token, envelope: run_view("guest_view", token, envelope),
    "not_me_view": lambda token, envelope: run_view("not_me_view", token, envelope),
    "wrong_amount_view": lambda token, envelope, obligation_id: run_view(
        "wrong_amount_view", token, envelope, obligation_id
    ),
    "record_objection": run_record_objection,
    "report_payment": run_report_payment,
    "template_response": template_response,
    "link_broken_page": link_broken_page,
    "see_other": see_other,
    "server_error": server_error,
    "accepts_html": accepts_html,
    "link_broken_applies": link_broken_applies,
    "route": run_route,
}

#: module -> (constants, edges, fuzz, cases in the committed sample, cases a
#: live oracle run draws)
MODULES = {
    "views": (views_constants, views_edges, views_fuzz, 100, 6000),
    "steps": (dict, steps_edges, None, 0, 0),
    "pages": (None, pages_edges, pages_fuzz, 30, 1500),
}


def modes() -> dict[str, tuple[str, bool]]:
    table: dict[str, tuple[str, bool]] = {}
    for module, spec in MODULES.items():
        table[module] = (module, False)
        if spec[2] is not None:
            table[f"{module}-fuzz-0"] = (module, True)
    return table


def document_for(module: str, mode: str, seed: int, cases_of) -> dict:
    LINES.__init__()
    document = {
        "generator": "scripts/render_guest_web_goldens.py",
        "module": module,
        "mode": mode,
        "python": platform.python_version(),
        "seed": seed,
    }
    cases, fuzz = cases_of()
    if fuzz:
        document["fuzz"] = {"shard": 0, "shards": 1, "total": len(cases)}
    if module == "pages":
        document["constants"] = {"lines": LINES.table}
    elif not fuzz:
        document["constants"] = enc(MODULES[module][0]())
    document["cases"] = cases
    return document


def render(mode: str) -> dict:
    module, sample = modes()[mode]
    _constants, edges, fuzz, sample_count, _live = MODULES[module]
    if sample:
        return document_for(
            module, mode, SEED, lambda: (fuzz(SEED, sample_count), True)
        )
    return document_for(module, mode, SEED, lambda: (edges(), False))


def render_live(module: str, seed: int, count: int) -> dict:
    """A differential fuzz drawn for one oracle run; never committed."""
    fuzz = MODULES[module][2]
    return document_for(
        module, f"{module}-fuzz-live", seed, lambda: (fuzz(seed, count), True)
    )


def case_text(item: dict) -> str:
    """One line, or one JSON element per line when that line would be long."""
    line = json.dumps(item, ensure_ascii=True, separators=(",", ":"))
    if len(line) <= LONG_CASE:
        return line
    return json.dumps(item, ensure_ascii=True, indent=1, separators=(",", ":"))


def serialize(document: dict) -> str:
    """One header key per line, a line table entry per line, a case per line."""
    lines = ["{"]
    for key, value in document.items():
        if key == "cases":
            continue
        if key == "constants" and "lines" in value:
            lines.append(' "constants": {"lines": [')
            lines.append(
                ",\n".join(
                    json.dumps(enc_str(piece), ensure_ascii=True)
                    for piece in value["lines"]
                )
            )
            lines.append(" ]},")
            continue
        lines.append(
            f" {json.dumps(key)}: {json.dumps(value, ensure_ascii=True, separators=(',', ':'))},"
        )
    cases = [case_text(item) for item in document["cases"]]
    lines.append(' "cases": [')
    if cases:
        lines.append(",\n".join(cases))
    lines.append(" ]")
    lines.append("}")
    return "\n".join(lines) + "\n"


USAGE = "usage: - MODE | --list | --live MODULE SEED COUNT"


def main(argv: list[str]) -> int:
    table = modes()
    live_modules = [m for m, spec in MODULES.items() if spec[2] is not None]
    if argv == ["--list"]:
        for mode in table:
            print(f"{mode}\t{TARGET.format(mode=mode.replace('-', '_'))}")
        return 0
    if (
        len(argv) == 4
        and argv[0] == "--live"
        and argv[1] in live_modules
        and argv[2].isdigit()
        and argv[3].isdigit()
    ):
        sys.stdout.write(serialize(render_live(argv[1], int(argv[2]), int(argv[3]))))
        return 0
    if len(argv) != 1 or argv[0] not in table:
        print(
            f"{USAGE}; modes: {' '.join(table)}; live modules: {' '.join(live_modules)}",
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
