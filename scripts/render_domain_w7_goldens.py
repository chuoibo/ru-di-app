#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W7 outing routes (ADR-0029).

The eleven W7 routes (a group's trip, its timeline, its itinerary preview, its
check-ins and its invitations) reach these pure Python pieces, each ported to
one Go package:

    app.domain.journey                 services/core/internal/domain/journey
    app.journey.routing (pure half)    services/core/internal/domain/valhalla
    app.journey.preview                services/core/internal/domain/itinerary
    app.api.service (outing methods)   services/core/internal/domain/outingsteps

The Valhalla call itself is not ported and is not rendered here: `_post` opens
a socket, and a coordinate reaches the configured private service and nothing
else. What is rendered is everything around it -- the cost check, the polyline
decoder, the shaping of a body into costs, the ordering search, and the
service methods -- with the provider scripted so the same answers reach Go.

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each returned or raised. The service methods run as real `ApiService`
methods over a recording stub repository with the clock pinned to the case's
`now`: the golden holds the answer (or the ApiProblem, or the exception that
would be a 500) and every repository call the method made, with its arguments,
in order. Each package's oracle_test.go replays every case. The image is the
one scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w7_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w7_goldens.py > "$path"
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
groups of six code points), with these additions:

* a uuid.UUID is its name in ALIASES (`"TOI"`, `"OU1"`), and so is every id in
  the arguments; the table is in each module's constants;
* a datetime is its isoformat(), a date its isoformat();
* a float is `{"float": "<repr>"}`, so a NaN, an infinity and the shortest
  round-trip spelling all survive JSON;
* a bytes is its name in DIGESTS. The only bytes in this corpus are SHA-256
  token digests, which are compared and stored and never read, so a name is
  the whole of their identity -- and a hex digest would trip the repository
  guard's run-of-digits rule about once per digest;
* a decoded JSON body (what a routing answer is once json.loads has read it)
  is tagged so int and float stay apart: None, bool, int and str are
  themselves, a list is a list, and a dict is `{"dict": [[key, value], ...]}`.

An exception's `code` is `getattr(exc, "code", None)` where there is one and
`str(exc)` where there is not, because that is the pair the Go replay compares.

A service case's result is `{"ok": {"calls", "problem", "raised",
"response"}}`: `calls` is `[method, argument, ...]` per repository call with
arguments in the Protocol's order, `problem` the ApiProblem, `raised` the
class and code of an exception Python answers with a 500, `response` the
response model's model_dump().
"""

from __future__ import annotations

import dataclasses
import json
import os
import platform
import random
import re
import sys
import uuid
from datetime import UTC, date, datetime, timedelta
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import schemas  # noqa: E402
from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem, RepositoryConflict  # noqa: E402
from app.api.repository import (  # noqa: E402
    MembershipRecord,
    OutingInviteRecord,
    OutingRecord,
    OutingStopRecord,
    PersonRecord,
    StopCheckinRecord,
)
from app.domain import journey as journey_module  # noqa: E402
from app.domain.permissions import PermissionError_  # noqa: E402
from app.journey import preview as preview_module  # noqa: E402
from app.journey import routing as routing_module  # noqa: E402

SEED = 7
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
# Ids and secrets
# ---------------------------------------------------------------------------


def _uuid_for(letter: str, digit: str) -> uuid.UUID:
    """A uuid whose digits never run: the guard reads nine as an account."""
    c, d = letter, digit
    return uuid.UUID(
        f"{c}{c}{d}{d}{c}{c}{d}{d}-0{c}0{c}-4{c}0{c}-8{c}0{c}-0{c}0{c}0{c}0{c}0{c}{d}{d}"
    )


#: Every uuid a case names: people, the two contexts, outings and their stops,
#: check-ins, invitations and a membership. The ones ending in N are what the
#: stub hands back from a create.
ALIASES = {
    "TOI": _uuid_for("a", "1"),
    "BAN": _uuid_for("b", "2"),
    "LA": _uuid_for("c", "3"),
    "XOA": _uuid_for("d", "4"),
    "HOI": _uuid_for("e", "5"),
    "RIENG": _uuid_for("f", "6"),
    "OU1": _uuid_for("a", "7"),
    "OU2": _uuid_for("b", "8"),
    "OUN": _uuid_for("c", "9"),
    "ST1": _uuid_for("d", "1"),
    "ST2": _uuid_for("e", "2"),
    "ST3": _uuid_for("f", "3"),
    "CI1": _uuid_for("a", "4"),
    "CIN": _uuid_for("b", "5"),
    "IV1": _uuid_for("c", "6"),
    "IV2": _uuid_for("d", "7"),
    "IVN": _uuid_for("e", "8"),
    "MB1": _uuid_for("f", "9"),
}
ALIAS_OF = {value: name for name, value in ALIASES.items()}
assert len(ALIAS_OF) == len(ALIASES)

#: Every raw invitation secret a case uses. They are deliberately lowercase
#: words, not the 43 url-safe characters `secrets.token_urlsafe(32)` really
#: draws: a real one reads as an encoded blob to the repository guard, and the
#: port never looks inside a token anyway.
TOKENS = {
    "TK_MOI": "loi-moi-moi",
    "TK_CU": "loi-moi-cu",
    "TK_LA": "loi-moi-la",
}
DIGESTS = {api_service.token_digest(value): name for name, value in TOKENS.items()}
assert len(DIGESTS) == len(TOKENS)


def U(name: str) -> uuid.UUID:
    return ALIASES[name]


def UN(name: str | None) -> uuid.UUID | None:
    return None if name is None else ALIASES[name]


def SID(name: str | None) -> str | None:
    """A stop id as the client sends it.

    `_itinerary_draft` compares it with `str(stop.id)` of the stored row, so an
    alias in a fixture has to become that canonical spelling before the request
    is built. A draft id (`tmp-...`) and the empty anchor are already what they
    are. The Go replay maps the same names the same way.
    """
    if name is None or name not in ALIASES:
        return name
    return str(ALIASES[name])


# ---------------------------------------------------------------------------
# Encoding
# ---------------------------------------------------------------------------


#: A draft stop and a day config are encoded as the list of their values in
#: these orders. A draft of fifty-one stops has to fit on one line, and the
#: repository guard's line limit is four kibibytes.
DRAFT_STOP_KEYS = (
    "at",
    "label",
    "place_name",
    "place_id",
    "id",
    "day",
    "duration_minutes",
    "time_locked",
    "meeting_point",
    "lat",
    "lng",
    "checked_in",
)
DRAFT_DAY_KEYS = (
    "day",
    "transport_mode",
    "start_at",
    "start_stop_id",
    "end_stop_id",
    "return_to_start",
)


class Body:
    """A decoded JSON body, tagged so int and float stay apart."""

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


def enc_float(value: float) -> dict:
    # repr() through enc_str: a float of nine or more digits reads to the
    # repository guard as an account number, and the grouped `$sp:` spelling
    # is what every other long literal in these corpora uses.
    return {"float": enc_str(repr(value))}


def enc_body(value: object) -> object:
    """A decoded JSON value, keeping Python's int/float distinction."""
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, int):
        return enc(value)
    if isinstance(value, float):
        return enc_float(value)
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, list | tuple):
        return [enc_body(item) for item in value]
    if isinstance(value, dict):
        return {"dict": [[enc_str(str(k)), enc_body(v)] for k, v in value.items()]}
    raise TypeError(f"unencodable body {type(value).__name__}")


def enc(value: object) -> object:
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, Body):
        return enc_body(value.value)
    if isinstance(value, bytes):
        if value not in DIGESTS:
            raise TypeError("an unnamed digest would not survive the guard")
        return DIGESTS[value]
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, int):
        return value if -SMALL_INT < value < SMALL_INT else f"$i:{value:#_x}"
    if isinstance(value, float):
        return enc_float(value)
    if isinstance(value, uuid.UUID):
        return ALIAS_OF.get(value) or enc_str(str(value))
    if isinstance(value, datetime):
        return enc_str(value.isoformat())
    if isinstance(value, date):
        return value.isoformat()
    if isinstance(value, list | tuple):
        return [enc(item) for item in value]
    if isinstance(value, dict):
        for record_keys in (DRAFT_STOP_KEYS, DRAFT_DAY_KEYS):
            if set(value) == set(record_keys):
                return [enc(value[key]) for key in record_keys]
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"unencodable {type(value).__name__}")


def raised(exc: BaseException) -> dict:
    """The class, the message, and the code the Go replay compares."""
    code = getattr(exc, "code", None)
    return {
        "raised": {
            "type": type(exc).__name__,
            "message": enc_str(str(exc)),
            "code": enc(code if code is not None else str(exc)),
        }
    }


def fits(built: dict) -> bool:
    return (
        len(json.dumps(built, ensure_ascii=True, separators=(",", ":")))
        <= MAX_CASE_BYTES
    )


# ---------------------------------------------------------------------------
# journey: app/domain/journey.py
# ---------------------------------------------------------------------------

JOURNEY_FUNCTIONS = {
    "issue": journey_module.issue,
    "minute": journey_module.minute,
    "clock": journey_module.clock,
    "schedule": journey_module.schedule,
    "order_costs": journey_module.order_costs,
    "suggest_order": journey_module.suggest_order,
}
#: What Python raises out of these functions. Anything else is a corpus bug
#: and stops the render rather than being recorded as an answer.
JOURNEY_RAISES = (ValueError, IndexError, TypeError, KeyError, ZeroDivisionError)


def journey_case(fn: str, name: str, args: dict) -> dict:
    encoded = {key: enc(value) for key, value in args.items()}
    try:
        value = JOURNEY_FUNCTIONS[fn](**args)
    except JOURNEY_RAISES as exc:
        result = raised(exc)
    else:
        result = {"ok": enc(value)}
    return {"fn": fn, "name": name, "args": encoded, "result": result}


def stop(
    sid: str,
    at: str = "08:00",
    *,
    dwell: int | None = 30,
    locked: bool = True,
    checked: bool = False,
) -> dict:
    return {
        "id": sid,
        "at": at,
        "duration_minutes": dwell,
        "time_locked": locked,
        "checked_in": checked,
    }


def settings_of(
    start: str = "08:00", *, returning: bool = False, end: str | None = None
) -> dict:
    return {"start_at": start, "return_to_start": returning, "end_stop_id": end}


def journey_constants() -> dict:
    return {
        "max_stops": journey_module.MAX_STOPS,
        "max_evaluations": journey_module.MAX_EVALUATIONS,
        "unreachable_cost": 10**9,
    }


#: Times that exercise int(): unicode decimal digits, the underscore rule, the
#: sign, the whitespace int() strips, and the shapes str.split(":") makes.
MINUTE_EDGES = (
    "00:00",
    "23:59",
    "07:30",
    "7:5",
    "24:00",
    "23:60",
    "-1:30",
    "-0:00",
    "12",
    "",
    ":",
    "1:2:3",
    "1:2:x",
    "1:2:30",
    " 7 : 30",
    "07:30\n",
    "\t07:30\t",
    "+7:+30",
    "1_0:0_0",
    "1_:00",
    "_1:00",
    "1__0:00",
    "10:0_",
    "0x10:00",
    "٧:٣٠",
    "０７:３０",
    "۰۷:۳۰",
    "7.0:30",
    "07:30 ",
    "  :  ",
    "07:３0",
)

CLOCK_EDGES = (-1441, -60, -1, 0, 1, 59, 60, 599, 600, 1439, 1440, 1441, 100000)


def journey_edges() -> list[dict]:
    out = []
    C = journey_case

    for index, value in enumerate(MINUTE_EDGES):
        out.append(C("minute", f"minute/{index}", {"value": value}))
    for index, value in enumerate(CLOCK_EDGES):
        out.append(C("clock", f"clock/{index}", {"value": value}))
    out.append(
        C("issue", "issue/with-stop", {"code": "late_fixed_stop", "message": "Muộn."})
    )
    out.append(
        C(
            "issue",
            "issue/named",
            {"code": "unreachable", "message": "Không tới được.", "stop_id": "ST1"},
        )
    )

    # --- schedule ---------------------------------------------------------
    one = [stop("a", "08:00")]
    two = [stop("a", "08:00"), stop("b", "09:00")]
    three = [stop("a", "08:00"), stop("b", "09:00"), stop("c", "10:00")]
    S = settings_of
    out += [
        C("schedule", "schedule/empty", {"stops": [], "costs": [], "settings": S()}),
        C("schedule", "schedule/one", {"stops": one, "costs": [], "settings": S()}),
        C(
            "schedule",
            "schedule/two-on-time",
            {"stops": two, "costs": [(600, 3000)], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/rounds-seconds-up",
            {"stops": two, "costs": [(61, 3000)], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/unreachable",
            {"stops": two, "costs": [None], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/late-fixed",
            {"stops": two, "costs": [(7200, 3000)], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/unlocked-no-wait",
            {
                "stops": [stop("a", "08:00"), stop("b", "09:00", locked=False)],
                "costs": [(600, 3000)],
                "settings": S(),
            },
        ),
        C(
            "schedule",
            "schedule/checked-in-relocks",
            {
                "stops": [
                    stop("a", "08:00"),
                    stop("b", "09:00", locked=False, checked=True),
                ],
                "costs": [(600, 3000)],
                "settings": S(),
            },
        ),
        C(
            "schedule",
            "schedule/missing-duration",
            {
                "stops": [stop("a", "08:00", dwell=None), stop("b", "09:00")],
                "costs": [(600, 3000)],
                "settings": S(),
            },
        ),
        C(
            "schedule",
            "schedule/day-overflow",
            {
                "stops": [stop("a", "23:00", dwell=120)],
                "costs": [],
                "settings": S("23:00"),
            },
        ),
        C(
            "schedule",
            "schedule/return-unreachable",
            {"stops": two, "costs": [(600, 3000), None], "settings": S(returning=True)},
        ),
        C(
            "schedule",
            "schedule/return-overflows",
            {
                "stops": [stop("a", "22:00"), stop("b", "23:00")],
                "costs": [(600, 3000), (7200, 3000)],
                "settings": S(returning=True),
            },
        ),
        C(
            "schedule",
            "schedule/return-one-stop-ignores-cost",
            {"stops": one, "costs": [], "settings": S(returning=True)},
        ),
        C(
            "schedule",
            "schedule/costs-too-short",
            {"stops": three, "costs": [(600, 3000)], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/return-with-no-costs",
            {"stops": two, "costs": [], "settings": S(returning=True)},
        ),
        C(
            "schedule",
            "schedule/bad-start",
            {"stops": one, "costs": [], "settings": S("aa:bb")},
        ),
        C(
            "schedule",
            "schedule/bad-stop-time",
            {"stops": [stop("a", "25:00")], "costs": [], "settings": S()},
        ),
        C(
            "schedule",
            "schedule/negative-dwell",
            {
                "stops": [stop("a", "08:00", dwell=-120), stop("b", "09:00")],
                "costs": [(60, 10)],
                "settings": S(),
            },
        ),
        C(
            "schedule",
            "schedule/zero-second-road",
            {"stops": two, "costs": [(0, 0)], "settings": S()},
        ),
    ]

    # --- order_costs ------------------------------------------------------
    matrix = [
        [None, (600, 3000), (900, 5000)],
        [(600, 3000), None, (300, 1000)],
        [(900, 5000), (300, 1000), None],
    ]
    out += [
        C(
            "order_costs",
            "order_costs/forward",
            {"order": [0, 1, 2], "matrix": matrix, "returning": False},
        ),
        C(
            "order_costs",
            "order_costs/returning",
            {"order": [0, 1, 2], "matrix": matrix, "returning": True},
        ),
        C(
            "order_costs",
            "order_costs/one-returning",
            {"order": [1], "matrix": matrix, "returning": True},
        ),
        C(
            "order_costs",
            "order_costs/empty",
            {"order": [], "matrix": matrix, "returning": True},
        ),
        C(
            "order_costs",
            "order_costs/negative-index-wraps",
            {"order": [0, -1], "matrix": matrix, "returning": False},
        ),
        C(
            "order_costs",
            "order_costs/out-of-range",
            {"order": [0, 9], "matrix": matrix, "returning": False},
        ),
    ]

    # --- suggest_order ----------------------------------------------------
    free = [
        stop("a", "08:00"),
        stop("b", "09:00", locked=False),
        stop("c", "10:00", locked=False),
        stop("d", "17:00"),
    ]
    square = [
        [None, (600, 3000), (300, 1000), (1200, 9000)],
        [(600, 3000), None, (1800, 9000), (600, 2000)],
        [(300, 1000), (1800, 9000), None, (900, 4000)],
        [(1200, 9000), (600, 2000), (900, 4000), None],
    ]
    out += [
        C(
            "suggest_order",
            "suggest_order/too-few",
            {"stops": free[:2], "matrix": square, "settings": S()},
        ),
        C(
            "suggest_order",
            "suggest_order/all-locked",
            {"stops": free[:3], "matrix": square, "settings": S()},
        ),
        C(
            "suggest_order",
            "suggest_order/two-free",
            {"stops": free, "matrix": square, "settings": S("07:00")},
        ),
        C(
            "suggest_order",
            "suggest_order/end-anchor-pins",
            {"stops": free, "matrix": square, "settings": S("07:00", end="c")},
        ),
        C(
            "suggest_order",
            "suggest_order/returning",
            {"stops": free, "matrix": square, "settings": S("07:00", returning=True)},
        ),
        C(
            "suggest_order",
            "suggest_order/checked-in-is-fixed",
            {
                "stops": [
                    stop("a", "08:00"),
                    stop("b", "09:00", locked=False, checked=True),
                    stop("c", "10:00", locked=False),
                    stop("d", "17:00", locked=False),
                ],
                "matrix": square,
                "settings": S("07:00"),
            },
        ),
        C(
            "suggest_order",
            "suggest_order/unreachable-row",
            {
                "stops": free,
                "matrix": [[None] * 4 for _ in range(4)],
                "settings": S("07:00"),
            },
        ),
        C(
            "suggest_order",
            "suggest_order/matrix-too-small",
            {"stops": free, "matrix": square[:2], "settings": S("07:00")},
        ),
        # Three movable slots all one road away from the start: the tie is
        # broken by the stop's own index, and breaking it the other way ends
        # the search somewhere 2-opt cannot walk back from.
        C(
            "suggest_order",
            "suggest_order/nearest-tie-broken-by-index",
            {
                "stops": [
                    stop("s0", "08:00"),
                    stop("s1", "09:00", locked=False),
                    stop("s2", "09:00", locked=False),
                    stop("s3", "09:00", locked=False),
                    stop("s4", "20:00"),
                ],
                "matrix": [
                    [None, (600, 3000), (600, 3000), (600, 3000), (6140, 300)],
                    [(7960, 4750), None, (3340, 4510), (6050, 8620), (2000, 5310)],
                    [(2390, 6550), (3010, 5110), None, (40, 6780), (870, 4680)],
                    [(6700, 2840), (4160, 5640), (8600, 850), None, (7240, 2600)],
                    [(3220, 7760), (2350, 5250), (2950, 300), (710, 5760), None],
                ],
                "settings": S("08:00"),
            },
        ),
        C(
            "suggest_order",
            "suggest_order/missing-duration-everywhere",
            {
                "stops": [
                    stop("a", "08:00", dwell=None),
                    stop("b", "09:00", dwell=None, locked=False),
                    stop("c", "10:00", dwell=None, locked=False),
                ],
                "matrix": square,
                "settings": S("07:00"),
            },
        ),
    ]
    return out


def fuzz_time(rng: random.Random) -> str:
    """Mostly a valid clock, sometimes one of the shapes int() argues with."""
    if rng.random() < 0.82:
        return f"{rng.randrange(24):02d}:{rng.randrange(60):02d}"
    return rng.choice(MINUTE_EDGES)


def fuzz_cost(rng: random.Random) -> tuple | None:
    if rng.random() < 0.18:
        return None
    seconds = rng.choice(
        (0, rng.randrange(0, 7200), rng.randrange(0, 10**6), 10**9, -60)
    )
    metres = rng.choice((0, rng.randrange(0, 20000), rng.randrange(0, 10**6), 10**9))
    return (seconds, metres)


def fuzz_stop(rng: random.Random, index: int) -> dict:
    dwell = None
    if rng.random() < 0.8:
        dwell = rng.choice(
            (0, rng.randrange(0, 240), rng.randrange(0, 1441), -30, 10000)
        )
    return stop(
        f"s{index}",
        fuzz_time(rng),
        dwell=dwell,
        locked=rng.random() < 0.55,
        checked=rng.random() < 0.18,
    )


def fuzz_settings(rng: random.Random, stops: list[dict]) -> dict:
    end = None
    if stops and rng.random() < 0.3:
        end = rng.choice(stops)["id"]
    return settings_of(fuzz_time(rng), returning=rng.random() < 0.35, end=end)


def journey_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(f"w7-journey/{seed}")
    out = []
    for i in range(count):
        pick = rng.random()
        if pick < 0.14:
            built = journey_case("minute", f"fuzz/{i}", {"value": fuzz_time(rng)})
        elif pick < 0.2:
            built = journey_case(
                "clock", f"fuzz/{i}", {"value": rng.randrange(-2000, 3000)}
            )
        elif pick < 0.55:
            stops = [fuzz_stop(rng, n) for n in range(rng.randrange(0, 7))]
            settings = fuzz_settings(rng, stops)
            wanted = max(0, len(stops) - 1) + (
                1 if settings["return_to_start"] and len(stops) > 1 else 0
            )
            length = wanted if rng.random() < 0.85 else rng.randrange(0, wanted + 2)
            built = journey_case(
                "schedule",
                f"fuzz/{i}",
                {
                    "stops": stops,
                    "costs": [fuzz_cost(rng) for _ in range(length)],
                    "settings": settings,
                },
            )
        elif pick < 0.7:
            size = rng.randrange(1, 6)
            matrix = [[fuzz_cost(rng) for _ in range(size)] for _ in range(size)]
            span = size + 1 if rng.random() < 0.12 else size
            order = [rng.randrange(-1, span) for _ in range(rng.randrange(0, size + 2))]
            built = journey_case(
                "order_costs",
                f"fuzz/{i}",
                {
                    "order": order,
                    "matrix": matrix,
                    "returning": rng.random() < 0.5,
                },
            )
        else:
            size = rng.randrange(0, 7)
            stops = [fuzz_stop(rng, n) for n in range(size)]
            rows = size if rng.random() < 0.9 else max(0, size - 1)
            matrix = [[fuzz_cost(rng) for _ in range(size)] for _ in range(rows)]
            built = journey_case(
                "suggest_order",
                f"fuzz/{i}",
                {
                    "stops": stops,
                    "matrix": matrix,
                    "settings": fuzz_settings(rng, stops),
                },
            )
        if fits(built):
            out.append(built)
    return out


# ---------------------------------------------------------------------------
# valhalla: the pure half of app/journey/routing.py
# ---------------------------------------------------------------------------


def _shaping_provider(body: object) -> routing_module.ValhallaProvider:
    """A provider whose `_post` is already holding the answer.

    Built without __init__ on purpose: the constructor validates a base url and
    stores an opener, and neither is part of what is being measured. Assigning
    `_post` on the instance shadows the method that would have opened a socket.
    """
    provider = routing_module.ValhallaProvider.__new__(routing_module.ValhallaProvider)
    provider.base_url = "http://valhalla.invalid"
    provider.graph_version = "graph-mot"
    provider.timeout = 1
    provider._opener = None
    provider._post = lambda action, payload: body
    return provider


def shape_matrix(body: object, points: int) -> object:
    dummy = [{"lat": 0.0, "lon": 0.0} for _ in range(points)]
    return _shaping_provider(body).matrix(dummy, "car")


def shape_route(body: object, points: int) -> object:
    dummy = [{"lat": 0.0, "lon": 0.0} for _ in range(points)]
    return _shaping_provider(body).route(dummy, "car")


VALHALLA_FUNCTIONS = {
    "number": routing_module._number,
    "decode_shape": routing_module._decode_shape,
    "shape_matrix": shape_matrix,
    "shape_route": shape_route,
}
VALHALLA_RAISES = (ValueError, TypeError, routing_module.RoutingUnavailable)


def valhalla_case(fn: str, name: str, args: dict) -> dict:
    encoded = {key: enc(value) for key, value in args.items()}
    called = {
        key: value.value if isinstance(value, Body) else value
        for key, value in args.items()
    }
    try:
        value = VALHALLA_FUNCTIONS[fn](**called)
    except VALHALLA_RAISES as exc:
        result = raised(exc)
    else:
        result = {"ok": enc_body(value) if fn == "number" else enc(value)}
    return {"fn": fn, "name": name, "args": encoded, "result": result}


def valhalla_constants() -> dict:
    return {
        "modes": dict(routing_module.MODES),
        "max_response_bytes": routing_module.MAX_RESPONSE_BYTES,
        "cost_ceiling": 1_000_000_000,
    }


#: A real polyline6 pair and the shapes that argue with the decoder.
SHAPE_EDGES = (
    "_p~iF~ps|U_ulLnnqC",
    "??",
    "?",
    "",
    "_p~iF~ps|U",
    "_p~iF~ps|U_ulLnnqC_ulLnnqC",
    "~~~~~~~~~~~~",
    "??>",
    "?? ",
    "??~",
    "\x00\x00",
    "??éé",
    "_p~iF~ps|U" * 3,
    "oooooooo??",
    # Seven continuation bytes carrying nothing, then a terminator: the eighth
    # group is one shift past the limit, so the whole string is refused even
    # though every coordinate it spells out is zero.
    "_______?" * 4,
    "_______?" * 2,
    "______?" * 4,
)

NUMBER_EDGES = (
    0,
    1,
    1000,
    1_000_000_000,
    1_000_000_001,
    -1,
    True,
    False,
    0.0,
    0.5,
    3.5,
    4.5,
    0.0035,
    0.0045,
    1e9,
    1.0000000001e9,
    -0.0,
    float("nan"),
    float("inf"),
    float("-inf"),
    "12",
    None,
    10**20,
    -(10**20),
)


def cell(time: object, distance: object) -> dict:
    return {"time": time, "distance": distance}


def trip(legs: list, *, status: object = 0, units: object = "kilometers") -> dict:
    body = {"trip": {"status": status, "legs": legs}}
    if units is not None:
        body["trip"]["units"] = units
    return body


def summary_leg(
    length: object, time: object, shape: str = "_p~iF~ps|U_ulLnnqC"
) -> dict:
    return {"summary": {"length": length, "time": time}, "shape": shape}


def valhalla_edges() -> list[dict]:
    out = []
    C = valhalla_case
    for index, value in enumerate(NUMBER_EDGES):
        out.append(C("number", f"number/{index}", {"value": Body(value)}))
    for index, value in enumerate(SHAPE_EDGES):
        out.append(C("decode_shape", f"shape/{index}", {"encoded": value}))

    square = {
        "sources_to_targets": [
            [cell(0, 0.0), cell(600, 3.0)],
            [cell(601, 3.0005), cell(None, None)],
        ]
    }
    out += [
        C("shape_matrix", "matrix/two", {"body": Body(square), "points": 2}),
        C(
            "shape_matrix",
            "matrix/missing-time-only",
            {
                "body": Body(
                    {"sources_to_targets": [[cell(None, 3.0)], [cell(1, None)]]}
                ),
                "points": 1,
            },
        ),
        C(
            "shape_matrix",
            "matrix/wrong-size",
            {"body": Body(square), "points": 3},
        ),
        C(
            "shape_matrix",
            "matrix/ragged",
            {
                "body": Body(
                    {"sources_to_targets": [[cell(1, 1.0), cell(1, 1.0)], []]}
                ),
                "points": 2,
            },
        ),
        C("shape_matrix", "matrix/no-key", {"body": Body({}), "points": 0}),
        C(
            "shape_matrix",
            "matrix/cost-out-of-range",
            {
                "body": Body({"sources_to_targets": [[cell(-1, 1.0)]]}),
                "points": 1,
            },
        ),
        C(
            "shape_matrix",
            "matrix/cell-is-not-a-dict",
            {"body": Body({"sources_to_targets": [[7]]}), "points": 1},
        ),
        C(
            "shape_matrix",
            "matrix/half-rounds-to-even",
            {
                "body": Body(
                    {"sources_to_targets": [[cell(0, 0.0035), cell(0, 0.0045)]]}
                ),
                "points": 2,
            },
        ),
    ]

    one = summary_leg(3.0, 600)
    out += [
        C("shape_route", "route/one-point", {"body": Body({}), "points": 1}),
        C("shape_route", "route/two-points", {"body": Body(trip([one])), "points": 2}),
        C(
            "shape_route",
            "route/status-not-zero",
            {"body": Body(trip([one], status=1)), "points": 2},
        ),
        C(
            "shape_route",
            "route/status-is-false",
            {"body": Body(trip([one], status=False)), "points": 2},
        ),
        C(
            "shape_route",
            "route/status-is-float-zero",
            {"body": Body(trip([one], status=0.0)), "points": 2},
        ),
        C(
            "shape_route",
            "route/units-absent",
            {"body": Body(trip([one], units=None)), "points": 2},
        ),
        C(
            "shape_route",
            "route/units-miles",
            {"body": Body(trip([one], units="miles")), "points": 2},
        ),
        C(
            "shape_route",
            "route/leg-count-wrong",
            {"body": Body(trip([one])), "points": 3},
        ),
        C(
            "shape_route",
            "route/bad-shape",
            {"body": Body(trip([summary_leg(3.0, 600, "?")])), "points": 2},
        ),
        C(
            "shape_route",
            "route/bad-cost",
            {"body": Body(trip([summary_leg(-3.0, 600)])), "points": 2},
        ),
        C(
            "shape_route",
            "route/no-summary",
            {"body": Body({"trip": {"status": 0, "legs": [{}]}}), "points": 2},
        ),
        C("shape_route", "route/no-trip", {"body": Body({}), "points": 2}),
        C(
            "shape_route",
            "route/int-costs",
            {"body": Body(trip([summary_leg(3, 600)])), "points": 2},
        ),
    ]
    return out


def fuzz_number(rng: random.Random) -> object:
    pick = rng.random()
    if pick < 0.3:
        return rng.randrange(-5, 1_000_000_005)
    if pick < 0.6:
        return rng.uniform(-1, 1_000_000_002)
    if pick < 0.72:
        return rng.choice((0, 0.0, -0.0, 1_000_000_000, 1e9, 1.0e-7, 0.0035, 0.0045))
    if pick < 0.82:
        return rng.choice((True, False))
    if pick < 0.9:
        return rng.choice((float("nan"), float("inf"), float("-inf")))
    if pick < 0.96:
        return rng.choice((None, "3", [1], {"a": 1}))
    return rng.choice((10**20, -(10**20), 10**9 + 1))


SHAPE_ALPHABET = "?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~>=< "


def fuzz_shape(rng: random.Random) -> str:
    if rng.random() < 0.2:
        return rng.choice(SHAPE_EDGES)
    length = rng.randrange(0, 22)
    return "".join(rng.choice(SHAPE_ALPHABET) for _ in range(length))


def fuzz_cell(rng: random.Random) -> object:
    pick = rng.random()
    if pick < 0.2:
        return cell(None, None)
    if pick < 0.3:
        return cell(fuzz_number(rng), None)
    if pick < 0.36:
        return rng.choice((7, "x", [1], None))
    return cell(fuzz_number(rng), fuzz_number(rng))


def valhalla_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(f"w7-valhalla/{seed}")
    out = []
    for i in range(count):
        pick = rng.random()
        if pick < 0.3:
            built = valhalla_case(
                "number", f"fuzz/{i}", {"value": Body(fuzz_number(rng))}
            )
        elif pick < 0.62:
            built = valhalla_case(
                "decode_shape", f"fuzz/{i}", {"encoded": fuzz_shape(rng)}
            )
        elif pick < 0.8:
            size = rng.randrange(0, 4)
            rows = size if rng.random() < 0.85 else rng.randrange(0, 4)
            body: object = {
                "sources_to_targets": [
                    [fuzz_cell(rng) for _ in range(size)] for _ in range(rows)
                ]
            }
            if rng.random() < 0.08:
                body = rng.choice(({}, {"sources_to_targets": 3}, {"other": 1}))
            built = valhalla_case(
                "shape_matrix", f"fuzz/{i}", {"body": Body(body), "points": size}
            )
        else:
            count_legs = rng.randrange(0, 4)
            legs = [
                summary_leg(fuzz_number(rng), fuzz_number(rng), fuzz_shape(rng))
                if rng.random() < 0.85
                else rng.choice(({}, {"summary": 3}, 7))
                for _ in range(count_legs)
            ]
            body = trip(
                legs,
                status=rng.choice((0, 1, 0.0, False, True, "0")),
                units=rng.choice(("kilometers", "miles", None)),
            )
            if rng.random() < 0.08:
                body = rng.choice(({}, {"trip": 3}, {"trip": {"status": 0}}))
            points = count_legs + 1 if rng.random() < 0.85 else rng.randrange(0, 5)
            built = valhalla_case(
                "shape_route", f"fuzz/{i}", {"body": Body(body), "points": points}
            )
        if fits(built):
            out.append(built)
    return out


# ---------------------------------------------------------------------------
# itinerary: app/journey/preview.py
# ---------------------------------------------------------------------------

FAIL = "fail"


class ScriptedProvider:
    """A provider that answers from the case's script and records every call.

    A queue's last answer repeats, so a case never has to know how many times
    the preview will ask; the Go stub repeats the same way.
    """

    def __init__(self, script: dict, calls: list):
        self.graph_version = script.get("graph_version", "graph-mot")
        self.script = {
            "route": list(script.get("route", [])),
            "matrix": list(script.get("matrix", [])),
        }
        self.calls = calls

    def _answer(self, kind: str, points: list, mode: str) -> object:
        self.calls.append(
            [kind, [[point["lat"], point["lon"]] for point in points], mode]
        )
        queue = self.script[kind]
        if not queue:
            raise routing_module.RoutingUnavailable("routing_unavailable")
        answer = queue.pop(0) if len(queue) > 1 else queue[0]
        if answer == FAIL:
            raise routing_module.RoutingUnavailable("routing_unavailable")
        return answer

    def route(self, points: list, mode: str) -> object:
        return self._answer("route", points, mode)

    def matrix(self, points: list, mode: str) -> object:
        return self._answer("matrix", points, mode)


class Capacity:
    """_CAPACITY with the slot count decided by the case."""

    def __init__(self, free: bool):
        self.free = free

    def acquire(self, blocking: bool = True) -> bool:
        return self.free

    def release(self) -> None:
        return None


#: A scripted leg's geometry is empty unless a case asks for one. `_route`
#: never looks inside a leg -- it copies the whole dict into the segment and
#: reads only the two costs -- so one case with a real polyline proves the
#: pass-through and the rest stay inside the guard's line limit.
def leg(seconds: object, metres: object, shape: list | None = None) -> dict:
    return {
        "distance_meters": metres,
        "duration_seconds": seconds,
        "geometry": [] if shape is None else shape,
        "source": "valhalla",
    }


def dstop(
    sid: str,
    at: str = "08:00",
    *,
    day: str | None = "2026-03-02",
    dwell: object = 30,
    locked: bool = True,
    lat: object = 10.77,
    lng: object = 106.7,
    checked: bool = False,
    label: str = "Điểm hẹn",
) -> dict:
    """One stop as _itinerary_draft leaves it: model_dump plus the three keys
    the service injects."""
    return {
        "at": at,
        "label": label,
        "place_name": None,
        "place_id": None,
        "id": sid,
        "day": day,
        "duration_minutes": dwell,
        "time_locked": locked,
        "meeting_point": None,
        "lat": lat,
        "lng": lng,
        "checked_in": checked,
    }


def dday(
    day: str = "2026-03-02",
    *,
    mode: str = "car",
    start: str = "08:00",
    start_stop: str | None = None,
    end_stop: str | None = None,
    returning: bool = False,
) -> dict:
    return {
        "day": day,
        "transport_mode": mode,
        "start_at": start,
        "start_stop_id": start_stop,
        "end_stop_id": end_stop,
        "return_to_start": returning,
    }


def draft_of(
    stops: list,
    days: list,
    *,
    day: str = "2026-03-02",
    revision: int = 0,
    suggest: bool = False,
) -> dict:
    return {
        "expected_revision": revision,
        "stops": stops,
        "days": days,
        "day": day,
        "include_suggestion": suggest,
    }


ENV_DEFAULT = {"provider": True, "capacity": True, "suggestions_enabled": True}


def run_preview(draft: dict, script: dict, env: dict) -> dict:
    calls: list = []
    provider = ScriptedProvider(script, calls) if env["provider"] else None
    os.environ["MOBILE_VALHALLA_URL"] = ""
    os.environ["MOBILE_JOURNEY_SUGGESTIONS_ENABLED"] = (
        "1" if env["suggestions_enabled"] else "0"
    )
    saved = preview_module._CAPACITY
    preview_module._CAPACITY = Capacity(env["capacity"])
    out: dict = {"calls": calls, "result": None, "raised": None}
    try:
        out["result"] = preview_module.preview_itinerary(draft, provider)
    except (ValueError, IndexError, TypeError, KeyError) as exc:
        out["raised"] = {
            "type": type(exc).__name__,
            "message": str(exc),
            "code": getattr(exc, "code", None),
        }
    finally:
        preview_module._CAPACITY = saved
    return out


def preview_case(
    name: str, draft: dict, script: dict | None = None, env: dict | None = None
) -> dict:
    args = {
        "draft": draft,
        "script": script or {},
        "env": {**ENV_DEFAULT, **(env or {})},
    }
    answer = run_preview(
        json.loads(json.dumps(draft, default=str)), args["script"], args["env"]
    )
    return {
        "fn": "preview_itinerary",
        "name": name,
        "args": enc_preview_args(args),
        "result": {"ok": enc(answer)},
    }


def enc_preview_args(args: dict) -> dict:
    return {
        "draft": enc(args["draft"]),
        "script": {
            "graph_version": args["script"].get("graph_version", "graph-mot"),
            "route": enc(args["script"].get("route", [])),
            "matrix": enc(args["script"].get("matrix", [])),
        },
        "env": args["env"],
    }


def itinerary_constants() -> dict:
    return {
        "engine": "valhalla",
        "traffic": "none",
        "max_stops": journey_module.MAX_STOPS,
        "modes": sorted(routing_module.MODES),
        "env_default": ENV_DEFAULT,
    }


#: The four stops and the roads that make an ordering worth suggesting: the
#: last appointment is missed on the client's order and kept on the alternate.
SUGGEST_STOPS = [
    dstop("a", "08:00"),
    dstop("b", "09:00", locked=False),
    dstop("c", "09:30", locked=False),
    dstop("d", "12:00"),
]
SUGGEST_MATRIX = [
    [None, (1800, 1000), (600, 500), (9000, 9000)],
    [(1800, 1000), None, (1800, 900), (600, 300)],
    [(600, 500), (600, 400), None, (7200, 5000)],
    [(9000, 9000), (600, 300), (7200, 5000), None],
]
SUGGEST_CURRENT = [leg(1800, 1000), leg(1800, 900), leg(7200, 5000)]
SUGGEST_BETTER = [leg(600, 500), leg(600, 400), leg(600, 300)]


def itinerary_edges() -> list[dict]:
    out = []
    P = preview_case
    day = dday()
    pair = [dstop("a", "08:00"), dstop("b", "09:00")]
    two_legs = {"route": [[leg(600, 3000)]]}

    out += [
        P(
            "too-many-stops",
            draft_of(
                [
                    dstop(f"s{i}", day=None, dwell=None, lat=1, lng=1, label="x")
                    for i in range(51)
                ],
                [day],
            ),
        ),
        P(
            "duplicate-ids",
            draft_of([dstop("a"), dstop("a", "09:00")], [day]),
        ),
        P("no-settings-for-day", draft_of(pair, [dday("2026-03-03")])),
        P("unknown-transport", draft_of(pair, [dday(mode="teleport")])),
        P("empty-day", draft_of([dstop("a", day="2026-03-03")], [day])),
        P(
            "unassigned-day-is-not-fatal",
            draft_of([dstop("a"), dstop("b", "09:00", day=None)], [day]),
            two_legs,
        ),
        P("bad-start-time", draft_of(pair, [dday(start="aa:bb")]), two_legs),
        P("bad-stop-time", draft_of([dstop("a", "25:00")], [day])),
        P("dwell-is-a-float", draft_of([dstop("a", dwell=30.0)], [day])),
        P("dwell-is-a-bool", draft_of([dstop("a", dwell=True)], [day])),
        P("dwell-too-long", draft_of([dstop("a", dwell=1441)], [day])),
        P("dwell-none-is-allowed", draft_of([dstop("a", dwell=None)], [day]), two_legs),
        P("missing-location", draft_of([dstop("a", lat=None)], [day])),
        P("location-is-a-bool", draft_of([dstop("a", lat=True)], [day])),
        P("location-is-a-string", draft_of([dstop("a", lat="10.0")], [day])),
        P("location-out-of-range", draft_of([dstop("a", lat=91.0)], [day])),
        P("longitude-out-of-range", draft_of([dstop("a", lng=181.0)], [day])),
        P(
            "location-is-an-int",
            draft_of([dstop("a", lat=10, lng=106)], [day]),
            two_legs,
        ),
        P(
            "anchor-mismatch",
            draft_of(pair, [dday(start_stop="b", end_stop="a")]),
            two_legs,
        ),
        P(
            "anchor-empty-string-is-no-anchor",
            draft_of(pair, [dday(start_stop="", end_stop="")]),
            two_legs,
        ),
        P("no-provider", draft_of(pair, [day]), env={"provider": False}),
        P("no-capacity", draft_of(pair, [day]), env={"capacity": False}),
        P("routing-fails", draft_of(pair, [day]), {"route": [FAIL]}),
        P(
            "leg-count-wrong",
            draft_of(pair, [day]),
            {"route": [[leg(600, 3000), leg(600, 3000)]]},
        ),
        P("ready", draft_of(pair, [day]), two_legs),
        P(
            "ready-with-geometry",
            draft_of(pair, [day]),
            {"route": [[leg(600, 3000, [[106.7, 10.77], [106.71, 10.78]])]]},
        ),
        P(
            "one-stop-no-legs",
            draft_of([dstop("a")], [day]),
            {"route": [[]]},
        ),
        P(
            "returning",
            draft_of(pair, [dday(returning=True)]),
            {"route": [[leg(600, 3000), leg(900, 4000)]]},
        ),
        P(
            "late-without-suggestion",
            draft_of(SUGGEST_STOPS, [day]),
            {"route": [SUGGEST_CURRENT]},
        ),
        P(
            "suggestion-improves",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {"route": [SUGGEST_CURRENT, SUGGEST_BETTER], "matrix": [SUGGEST_MATRIX]},
        ),
        P(
            "suggestion-disabled",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {"route": [SUGGEST_CURRENT]},
            {"suggestions_enabled": False},
        ),
        P(
            "suggestion-matrix-fails",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {"route": [SUGGEST_CURRENT], "matrix": [FAIL]},
        ),
        P(
            "suggestion-candidate-fails",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {"route": [SUGGEST_CURRENT, FAIL], "matrix": [SUGGEST_MATRIX]},
        ),
        P(
            "suggestion-no-better-order",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {
                "route": [SUGGEST_CURRENT],
                "matrix": [[[None] * 4 for _ in range(4)]],
            },
        ),
        # Same time, less road. The whole-route comparison is a pair, so the
        # distance decides when the durations tie.
        P(
            "suggestion-improves-on-distance-only",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {
                "route": [[leg(1800, 3000)] * 3, [leg(1800, 1000)] * 3],
                "matrix": [SUGGEST_MATRIX],
            },
        ),
        P(
            "suggestion-does-not-improve",
            draft_of(SUGGEST_STOPS, [day], suggest=True),
            {
                "route": [SUGGEST_CURRENT, [leg(90000, 90000)] * 3],
                "matrix": [SUGGEST_MATRIX],
            },
        ),
        P(
            "suggestion-blocked-by-other-issue",
            draft_of(
                [dstop("a", "08:00", dwell=None), *SUGGEST_STOPS[1:]],
                [day],
                suggest=True,
            ),
            {"route": [SUGGEST_CURRENT]},
        ),
        P(
            "suggestion-wanted-but-feasible",
            draft_of(pair, [day], suggest=True),
            {"route": [[leg(600, 3000)]], "matrix": [[[None, None], [None, None]]]},
        ),
    ]
    return out


def fuzz_draft_stop(rng: random.Random, index: int, days: list[str]) -> dict:
    lat: object = round(rng.uniform(-91, 91), 4)
    lng: object = round(rng.uniform(-181, 181), 4)
    if rng.random() < 0.12:
        lat = rng.choice((None, True, "10", float("nan"), float("inf"), 10, 95.0))
    if rng.random() < 0.08:
        lng = rng.choice((None, False, 200.0, -181.0, 106))
    dwell: object = rng.choice((None, 0, rng.randrange(0, 300), 1440, 1441, -1))
    if rng.random() < 0.06:
        dwell = rng.choice((30.0, True, "30"))
    return dstop(
        f"s{index}",
        fuzz_time(rng),
        day=rng.choice([*days, None, "2026-03-09"]),
        dwell=dwell,
        locked=rng.random() < 0.6,
        lat=lat,
        lng=lng,
        checked=rng.random() < 0.15,
    )


def fuzz_legs(rng: random.Random, count: int) -> object:
    if rng.random() < 0.18:
        return FAIL
    if rng.random() < 0.1:
        count = max(0, count + rng.choice((-1, 1)))
    return [leg(rng.randrange(0, 20000), rng.randrange(0, 90000)) for _ in range(count)]


def itinerary_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(f"w7-itinerary/{seed}")
    days = ["2026-03-02", "2026-03-03"]
    out = []
    for i in range(count):
        size = rng.randrange(0, 6)
        stops = [fuzz_draft_stop(rng, n, days) for n in range(size)]
        chosen = rng.choice(days)
        configs = []
        for day in days:
            if rng.random() < 0.85:
                anchor = rng.choice(
                    [None, "", *[s["id"] for s in stops]] if stops else [None, ""]
                )
                configs.append(
                    dday(
                        day,
                        mode=rng.choice(("car", "motorbike", "walk", "teleport")),
                        start=fuzz_time(rng),
                        start_stop=anchor,
                        end_stop=rng.choice([None, anchor]),
                        returning=rng.random() < 0.35,
                    )
                )
        onDay = [s for s in stops if s["day"] == chosen]
        legs = max(0, len(onDay) - 1)
        script = {
            "route": [fuzz_legs(rng, legs), fuzz_legs(rng, legs)],
            "matrix": [
                FAIL
                if rng.random() < 0.15
                else [
                    [fuzz_cost(rng) for _ in range(len(onDay))]
                    for _ in range(len(onDay))
                ]
            ],
        }
        built = preview_case(
            f"fuzz/{i}",
            draft_of(
                stops,
                configs,
                day=chosen,
                revision=rng.randrange(0, 5),
                suggest=rng.random() < 0.6,
            ),
            script,
            {
                "provider": rng.random() < 0.92,
                "capacity": rng.random() < 0.92,
                "suggestions_enabled": rng.random() < 0.85,
            },
        )
        if fits(built):
            out.append(built)
    return out


# ---------------------------------------------------------------------------
# outing_steps: the twelve outing methods of ApiService
# ---------------------------------------------------------------------------


class Unscripted(BaseException):
    """A repository call the world did not script: the corpus is wrong."""


T = datetime(2026, 3, 1, 9, 0, tzinfo=UTC)
EARLIER = T - timedelta(days=2)
LATER = T + timedelta(days=3)
D1 = "2026-03-02"
D2 = "2026-03-03"

#: [id, position, minute_of_day, label, place_name, place_id, day,
#:  duration_minutes, time_locked, meeting_lat, meeting_lng, meeting_label]
STOP_KEYS = (
    "id",
    "position",
    "minute_of_day",
    "label",
    "place_name",
    "place_id",
    "day",
    "duration_minutes",
    "time_locked",
    "meeting_lat",
    "meeting_lng",
    "meeting_label",
)
#: [id, context, created_by, title, starts_on, ends_on, headcount, budget,
#:  created_at, stops, timeline_revision, itinerary_version, itinerary_days]
OUTING_KEYS = (
    "id",
    "context_id",
    "created_by_id",
    "title",
    "starts_on",
    "ends_on",
    "headcount",
    "budget_per_person_vnd",
    "created_at",
    "stops",
    "timeline_revision",
    "itinerary_version",
    "itinerary_days",
)
#: [id, outing, source, invited_person, invited_by, accepted_at, accepted_by,
#:  created_at, expires_at, revoked_at]
INVITE_KEYS = (
    "id",
    "outing_id",
    "source",
    "invited_person_id",
    "invited_by_id",
    "accepted_at",
    "accepted_by_id",
    "created_at",
    "expires_at",
    "revoked_at",
)


def world_stop(
    sid: str,
    *,
    position: int = 0,
    minute: int = 480,
    label: str = "Điểm hẹn",
    place_name: str | None = None,
    place_id: str | None = None,
    day: str | None = D1,
    dwell: int | None = 30,
    locked: bool = True,
    meeting: list | None = None,
) -> list:
    lat, lng, name = meeting or (None, None, None)
    return [
        sid,
        position,
        minute,
        label,
        place_name,
        place_id,
        day,
        dwell,
        locked,
        lat,
        lng,
        name,
    ]


def world_outing(
    oid: str = "OU1",
    *,
    context: str = "HOI",
    by: str = "TOI",
    title: str = "Đà Lạt cuối tuần",
    starts: str = D1,
    ends: str = D2,
    headcount: int = 4,
    budget: int = 500000,
    stops: list | None = None,
    revision: int = 0,
    version: int = 1,
    days: list | None = None,
) -> list:
    return [
        oid,
        context,
        by,
        title,
        starts,
        ends,
        headcount,
        budget,
        EARLIER,
        stops if stops is not None else [world_stop("ST1")],
        revision,
        version,
        days or [],
    ]


def world_invite(
    iid: str = "IV1",
    *,
    outing: str = "OU1",
    source: str = "link",
    person: str | None = None,
    by: str = "TOI",
    accepted: datetime | None = None,
    accepted_by: str | None = None,
    expires: datetime | None = None,
    revoked: datetime | None = None,
) -> list:
    return [
        iid,
        outing,
        source,
        person,
        by,
        accepted,
        accepted_by,
        EARLIER,
        expires or LATER,
        revoked,
    ]


WORLD_DEFAULTS = {
    "people": {"TOI": "Tôi", "BAN": "Bạn", "LA": "Người lạ"},
    "contexts": {"HOI": "group", "RIENG": "pair"},
    "members": {
        "HOI": [["TOI", "active"], ["BAN", "active"], ["LA", "left"]],
        "RIENG": [["TOI", "active"], ["BAN", "active"]],
    },
    "outings": [world_outing()],
    "places": {"pho-ha-noi": [10.77, 106.7], "khong-toa-do": [None, None]},
    "checkins": [],
    "invites": [],
    "digests": {},
    "membership": ["MB1", "invited"],
    "conflicts": {},
    "stop_owner": {"ST1": "OU1", "ST2": "OU1"},
    "created": "OUN",
}


def stop_record(row: list) -> OutingStopRecord:
    values = dict(zip(STOP_KEYS, row, strict=True))
    return OutingStopRecord(
        id=U(values["id"]),
        position=values["position"],
        minute_of_day=values["minute_of_day"],
        label=values["label"],
        place_name=values["place_name"],
        place_id=values["place_id"],
        day=None if values["day"] is None else date.fromisoformat(values["day"]),
        duration_minutes=values["duration_minutes"],
        time_locked=values["time_locked"],
        meeting_lat=values["meeting_lat"],
        meeting_lng=values["meeting_lng"],
        meeting_label=values["meeting_label"],
    )


def outing_record(row: list) -> OutingRecord:
    values = dict(zip(OUTING_KEYS, row, strict=True))
    return OutingRecord(
        id=U(values["id"]),
        context_id=U(values["context_id"]),
        created_by_id=U(values["created_by_id"]),
        title=values["title"],
        starts_on=date.fromisoformat(values["starts_on"]),
        ends_on=date.fromisoformat(values["ends_on"]),
        headcount=values["headcount"],
        budget_per_person_vnd=values["budget_per_person_vnd"],
        created_at=values["created_at"],
        stops=tuple(stop_record(stop) for stop in values["stops"]),
        timeline_revision=values["timeline_revision"],
        itinerary_version=values["itinerary_version"],
        itinerary_days=tuple(values["itinerary_days"]),
    )


def invite_record(row: list) -> OutingInviteRecord:
    values = dict(zip(INVITE_KEYS, row, strict=True))
    return OutingInviteRecord(
        id=U(values["id"]),
        outing_id=U(values["outing_id"]),
        source=values["source"],
        invited_person_id=UN(values["invited_person_id"]),
        invited_by_id=U(values["invited_by_id"]),
        accepted_at=values["accepted_at"],
        accepted_by_id=UN(values["accepted_by_id"]),
        created_at=values["created_at"],
        expires_at=values["expires_at"],
        revoked_at=values["revoked_at"],
    )


class Stub:
    """A repository that answers from the case's world and records every call."""

    def __init__(self, world: dict, now: datetime):
        self.world = {**WORLD_DEFAULTS, **world}
        self.now = now
        self.calls: list = []
        self.conflicts = {
            key: list(codes) for key, codes in self.world["conflicts"].items()
        }
        # Both are lists in the world, and both are walked in order somewhere,
        # so the index is built here rather than the order thrown away.
        self.outings = {row[0]: row for row in self.world["outings"]}
        self.invites = {row[0]: row for row in self.world["invites"]}

    def __getattr__(self, name):
        raise Unscripted(name)

    def rec(self, name: str, *args: object) -> None:
        self.calls.append([name, *args])

    def maybe_conflict(self, name: str) -> None:
        codes = self.conflicts.get(name)
        if codes:
            code = codes.pop(0)
            if code is not None:
                raise RepositoryConflict(code)

    # --- roster and catalogue --------------------------------------------

    def is_member(self, context_id, person_id):
        self.rec("is_member", context_id, person_id)
        rows = self.world["members"].get(ALIAS_OF[context_id], [])
        return any(U(pid) == person_id and state == "active" for pid, state in rows)

    def get_context(self, context_id):
        self.rec("get_context", context_id)
        kind = self.world["contexts"].get(ALIAS_OF[context_id])
        if kind is None:
            return None
        return SimpleNamespace(id=context_id, kind=kind)

    def list_members(self, context_id):
        self.rec("list_members", context_id)
        rows = self.world["members"].get(ALIAS_OF[context_id], [])
        return tuple(
            MembershipRecord(
                id=U("MB1"),
                context_id=context_id,
                person_id=U(pid),
                display_name=self.world["people"].get(pid, ""),
                state=state,
                role="member",
                origin="link",
                invited_by_id=None,
                joined_at=EARLIER,
                left_at=None,
                created_at=EARLIER,
            )
            for pid, state in rows
        )

    def get_person(self, person_id):
        self.rec("get_person", person_id)
        name = self.world["people"].get(ALIAS_OF[person_id])
        if name is None:
            return None
        return PersonRecord(id=person_id, display_name=name, created_at=EARLIER)

    def get_place(self, place_id):
        self.rec("get_place", place_id)
        found = self.world["places"].get(place_id)
        if found is None:
            return None
        lat, lng = found
        return SimpleNamespace(
            id=place_id, to_row=lambda lat=lat, lng=lng: {"lat": lat, "lng": lng}
        )

    # --- outings -----------------------------------------------------------

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
        self.maybe_conflict("create_outing")
        return outing_record(
            world_outing(
                self.world["created"],
                context=ALIAS_OF[context_id],
                by=ALIAS_OF[created_by_id],
                title=title,
                starts=starts_on.isoformat(),
                ends=ends_on.isoformat(),
                headcount=headcount,
                budget=budget_per_person_vnd,
                stops=[],
            )
        )

    def get_outing(self, outing_id):
        self.rec("get_outing", outing_id)
        row = self.outings.get(ALIAS_OF[outing_id])
        return None if row is None else outing_record(row)

    def list_outings(self, context_id):
        self.rec("list_outings", context_id)
        return tuple(
            outing_record(row)
            for row in self.world["outings"]
            if U(row[1]) == context_id
        )

    def replace_outing_stops(self, *, outing_id, stops, expected_revision=None):
        self.rec("replace_outing_stops", outing_id, stops, expected_revision)
        self.maybe_conflict("replace_outing_stops")
        return self._saved(outing_id)

    def replace_outing_itinerary(
        self, *, outing_id, stops, itinerary_days, expected_revision
    ):
        self.rec(
            "replace_outing_itinerary",
            outing_id,
            stops,
            itinerary_days,
            expected_revision,
        )
        self.maybe_conflict("replace_outing_itinerary")
        return self._saved(outing_id)

    def _saved(self, outing_id) -> OutingRecord:
        row = self.world.get("saved") or self.outings.get(ALIAS_OF[outing_id])
        if row is None:
            raise Unscripted("saved")
        return outing_record(row)

    def get_outing_stop(self, stop_id):
        self.rec("get_outing_stop", stop_id)
        owner = self.world["stop_owner"].get(ALIAS_OF[stop_id])
        if owner is None or owner not in self.outings:
            return None
        outing = outing_record(self.outings[owner])
        found = next((s for s in outing.stops if s.id == stop_id), None)
        return None if found is None else (found, outing)

    def create_stop_checkin(self, *, stop_id, person_id, now):
        self.rec("create_stop_checkin", stop_id, person_id, now)
        self.maybe_conflict("create_stop_checkin")
        return StopCheckinRecord(
            id=U("CIN"), stop_id=stop_id, person_id=person_id, created_at=now
        )

    def list_outing_checkins(self, outing_id):
        self.rec("list_outing_checkins", outing_id)
        return tuple(
            StopCheckinRecord(
                id=U(cid), stop_id=U(sid), person_id=U(pid), created_at=EARLIER
            )
            for cid, sid, pid in self.world["checkins"]
        )

    # --- invitations -------------------------------------------------------

    def create_outing_invite(
        self,
        *,
        outing_id,
        source,
        invited_person_id,
        invited_by_id,
        token_digest,
        expires_at,
        now,
    ):
        self.rec(
            "create_outing_invite",
            outing_id,
            source,
            invited_person_id,
            invited_by_id,
            token_digest,
            expires_at,
            now,
        )
        self.maybe_conflict("create_outing_invite")
        return invite_record(
            world_invite(
                "IVN",
                outing=ALIAS_OF[outing_id],
                source=source,
                person=None
                if invited_person_id is None
                else ALIAS_OF[invited_person_id],
                by=ALIAS_OF[invited_by_id],
                expires=expires_at,
            )
        )

    def find_outing_invite_for_person(self, outing_id, person_id):
        self.rec("find_outing_invite_for_person", outing_id, person_id)
        for row in self.world["invites"]:
            if U(row[1]) == outing_id and row[3] is not None and U(row[3]) == person_id:
                return invite_record(row)
        return None

    def get_outing_invite(self, invite_id):
        self.rec("get_outing_invite", invite_id)
        row = self.invites.get(ALIAS_OF[invite_id])
        return None if row is None else invite_record(row)

    def get_outing_invite_by_digest(self, token_digest):
        self.rec("get_outing_invite_by_digest", token_digest)
        name = self.world["digests"].get(DIGESTS.get(token_digest, ""))
        if name is None or name not in self.invites:
            return None
        return invite_record(self.invites[name])

    def accept_outing_invite(self, *, invite_id, accepted_by_id, now):
        self.rec("accept_outing_invite", invite_id, accepted_by_id, now)
        self.maybe_conflict("accept_outing_invite")
        return invite_record(self.invites[ALIAS_OF[invite_id]])

    def revoke_outing_invite(self, *, invite_id, now):
        self.rec("revoke_outing_invite", invite_id, now)
        self.maybe_conflict("revoke_outing_invite")
        row = list(self.invites[ALIAS_OF[invite_id]])
        row[9] = now
        return invite_record(row)

    def rotate_outing_invite_digest(self, *, invite_id, token_digest, expires_at, now):
        self.rec(
            "rotate_outing_invite_digest", invite_id, token_digest, expires_at, now
        )
        self.maybe_conflict("rotate_outing_invite_digest")
        row = list(self.invites[ALIAS_OF[invite_id]])
        row[8] = expires_at
        return invite_record(row)

    def ensure_invited_membership(
        self, *, context_id, person_id, invited_by_id, origin, now
    ):
        self.rec(
            "ensure_invited_membership",
            context_id,
            person_id,
            invited_by_id,
            origin,
            now,
        )
        self.maybe_conflict("ensure_invited_membership")
        mid, state = self.world["membership"]
        return MembershipRecord(
            id=U(mid),
            context_id=context_id,
            person_id=person_id,
            display_name="",
            state=state,
            role="member",
            origin=origin,
            invited_by_id=invited_by_id,
            joined_at=None,
            left_at=None,
            created_at=now,
        )


def dump(value: object) -> object:
    if dataclasses.is_dataclass(value) and not isinstance(value, type):
        return {
            field.name: dump(getattr(value, field.name))
            for field in dataclasses.fields(value)
        }
    if isinstance(value, list | tuple):
        return [dump(item) for item in value]
    if isinstance(value, dict):
        return {key: dump(item) for key, item in value.items()}
    if hasattr(value, "model_dump"):
        return value.model_dump()
    return value


def meeting_of(row: dict | None):
    if row is None:
        return None
    return schemas.MeetingPoint.model_construct(
        lat=row["lat"], lng=row["lng"], label=row["label"]
    )


def stop_input(row: dict):
    return schemas.OutingStopInput.model_construct(
        at=row["at"],
        label=row["label"],
        place_name=row.get("place_name"),
        place_id=row.get("place_id"),
    )


def itinerary_stop_input(row: dict):
    return schemas.ItineraryStopInput.model_construct(
        at=row["at"],
        label=row["label"],
        place_name=row.get("place_name"),
        place_id=row.get("place_id"),
        id=SID(row["id"]),
        day=None if row.get("day") is None else date.fromisoformat(row["day"]),
        duration_minutes=row.get("duration_minutes"),
        time_locked=row.get("time_locked", True),
        meeting_point=meeting_of(row.get("meeting_point")),
    )


def itinerary_day_input(row: dict):
    return schemas.ItineraryDay.model_construct(
        day=date.fromisoformat(row["day"]),
        transport_mode=row["transport_mode"],
        start_at=row["start_at"],
        start_stop_id=SID(row.get("start_stop_id")),
        end_stop_id=SID(row.get("end_stop_id")),
        return_to_start=row.get("return_to_start", False),
    )


def itinerary_request(row: dict, preview: bool):
    model = schemas.ItineraryPreviewRequest if preview else schemas.ItineraryRequest
    fields = {
        "expected_revision": row["expected_revision"],
        "stops": [itinerary_stop_input(s) for s in row["stops"]],
        "days": [itinerary_day_input(d) for d in row["days"]],
    }
    if preview:
        fields["day"] = date.fromisoformat(row["day"])
        fields["include_suggestion"] = row.get("include_suggestion", False)
    return model.model_construct(**fields)


CALLERS = {
    "create_outing": lambda s, a, r: s.create_outing(
        U(r["context_id"]),
        schemas.OutingCreateRequest.model_construct(
            title=r["title"],
            starts_on=date.fromisoformat(r["starts_on"]),
            ends_on=date.fromisoformat(r["ends_on"]),
            headcount=r["headcount"],
            budget_per_person_vnd=r["budget_per_person_vnd"],
        ),
        a,
    ),
    "list_context_outings": lambda s, a, r: s.list_context_outings(
        U(r["context_id"]), a
    ),
    "replace_outing_timeline": lambda s, a, r: s.replace_outing_timeline(
        U(r["outing_id"]),
        schemas.OutingTimelineRequest.model_construct(
            expected_revision=r.get("expected_revision"),
            stops=[stop_input(row) for row in r["stops"]],
        ),
        a,
    ),
    "authorize_outing_itinerary": lambda s, a, r: s.authorize_outing_itinerary(
        U(r["outing_id"]), a
    ),
    "preview_outing_itinerary": lambda s, a, r: s.preview_outing_itinerary(
        U(r["outing_id"]), itinerary_request(r["request"], True), a
    ),
    "replace_outing_itinerary": lambda s, a, r: s.replace_outing_itinerary(
        U(r["outing_id"]), itinerary_request(r["request"], False), a
    ),
    "check_in_to_stop": lambda s, a, r: s.check_in_to_stop(U(r["stop_id"]), a),
    "list_outing_checkins": lambda s, a, r: s.list_outing_checkins(
        U(r["outing_id"]), a
    ),
    "create_outing_invite": lambda s, a, r: s.create_outing_invite(
        U(r["outing_id"]),
        schemas.OutingInviteCreateRequest.model_construct(
            source=r["source"], person_id=UN(r.get("person_id"))
        ),
        a,
    ),
    "accept_outing_invite": lambda s, a, r: s.accept_outing_invite(
        TOKENS[r["token"]], a
    ),
    "rotate_outing_invite_secret": lambda s, a, r: s.rotate_outing_invite_secret(
        U(r["outing_id"]), U(r["invite_id"]), a
    ),
    "revoke_outing_invite": lambda s, a, r: s.revoke_outing_invite(
        U(r["outing_id"]), U(r["invite_id"]), a
    ),
}

STEP_RAISES = (
    RepositoryConflict,
    AssertionError,
    PermissionError_,
    ValueError,
    TypeError,
    KeyError,
    IndexError,
)


def run_step(fn: str, now: datetime, actor: list, req: dict, world: dict) -> dict:
    api_service._now = lambda: now
    api_service.secrets = SimpleNamespace(token_urlsafe=lambda size: TOKENS["TK_MOI"])
    # preview_outing_itinerary's last line hands the draft to the routing
    # layer. That call is the boundary the Go port stops at, so here it is the
    # identity: the golden records the draft, and internal/domain/itinerary is
    # measured against preview_itinerary on its own.
    preview_module.preview_itinerary = lambda draft, provider=None: draft
    stub = Stub(world, now)
    who = Actor(id=U(actor[0]), roles=frozenset(actor[1]), context_ids=frozenset())
    out: dict = {
        "calls": stub.calls,
        "problem": None,
        "raised": None,
        "response": None,
    }
    try:
        response = CALLERS[fn](api_service.ApiService(stub), who, req)
    except ApiProblem as exc:
        out["problem"] = {
            "status": exc.status_code,
            "code": exc.code,
            "detail": exc.detail,
        }
    except STEP_RAISES as exc:
        out["raised"] = {
            "type": type(exc).__name__,
            "code": getattr(exc, "code", None),
            "message": str(exc),
        }
    else:
        out["response"] = dump(response)
    return out


def step_case(
    fn: str,
    name: str,
    req: dict,
    world: dict | None = None,
    now: datetime = T,
    actor=("TOI", ("member",)),
) -> dict:
    args = {
        "now": now,
        "actor": [actor[0], list(actor[1])],
        "req": req,
        "world": world or {},
    }
    answer = run_step(fn, now, args["actor"], req, args["world"])
    return {
        "fn": fn,
        "name": name,
        "args": enc(args),
        "result": {"ok": enc(answer)},
    }


def outing_steps_constants() -> dict:
    return {
        "aliases": [[name, str(value)] for name, value in ALIASES.items()],
        "tokens": [[name, value] for name, value in TOKENS.items()],
        "methods": list(CALLERS),
        "world_defaults": WORLD_DEFAULTS,
        "invite_ttl_seconds": int(api_service.OUTING_INVITE_TTL.total_seconds()),
        "earlier": EARLIER,
        "later": LATER,
    }


def rstop(
    at: str = "08:00", *, label: str = "Cà phê", place: str | None = None
) -> dict:
    return {"at": at, "label": label, "place_name": None, "place_id": place}


def istop(
    sid: str = "ST1",
    at: str = "08:00",
    *,
    day: str | None = D1,
    dwell: int | None = 30,
    locked: bool = True,
    place: str | None = None,
    meeting: dict | None = None,
) -> dict:
    return {
        "at": at,
        "label": "Cà phê",
        "place_name": None,
        "place_id": place,
        "id": sid,
        "day": day,
        "duration_minutes": dwell,
        "time_locked": locked,
        "meeting_point": meeting,
    }


def iday(
    day: str = D1,
    *,
    mode: str = "car",
    start: str = "08:00",
    start_stop: str | None = None,
    end_stop: str | None = None,
    returning: bool = False,
) -> dict:
    return {
        "day": day,
        "transport_mode": mode,
        "start_at": start,
        "start_stop_id": start_stop,
        "end_stop_id": end_stop,
        "return_to_start": returning,
    }


def ireq(
    stops: list | None = None,
    days: list | None = None,
    *,
    revision: int = 0,
    day: str = D1,
    suggest: bool = False,
) -> dict:
    return {
        "expected_revision": revision,
        "stops": stops if stops is not None else [istop()],
        "days": days if days is not None else [iday()],
        "day": day,
        "include_suggestion": suggest,
    }


DEFAULT_REQ = {
    "create_outing": {
        "context_id": "HOI",
        "title": "Đà Lạt cuối tuần",
        "starts_on": D1,
        "ends_on": D2,
        "headcount": 4,
        "budget_per_person_vnd": 500000,
    },
    "list_context_outings": {"context_id": "HOI"},
    "replace_outing_timeline": {"outing_id": "OU1", "stops": [rstop()]},
    "authorize_outing_itinerary": {"outing_id": "OU1"},
    "preview_outing_itinerary": {"outing_id": "OU1", "request": ireq()},
    "replace_outing_itinerary": {"outing_id": "OU1", "request": ireq()},
    "check_in_to_stop": {"stop_id": "ST1"},
    "list_outing_checkins": {"outing_id": "OU1"},
    "create_outing_invite": {"outing_id": "OU1", "source": "link"},
    "accept_outing_invite": {"token": "TK_CU"},
    "rotate_outing_invite_secret": {"outing_id": "OU1", "invite_id": "IV1"},
    "revoke_outing_invite": {"outing_id": "OU1", "invite_id": "IV1"},
}

LINK_WORLD = {
    "invites": [world_invite()],
    "digests": {"TK_CU": "IV1"},
}
NAMED_WORLD = {
    "invites": [world_invite(source="group", person="BAN")],
    "digests": {"TK_CU": "IV1"},
}
DOOR_ACTORS = (
    ("no_role", ("TOI", ())),
    ("unknown_role", ("TOI", ("member", "BOGUS"))),
    ("stranger", ("LA", ("member",))),
)
DOOR_WORLDS = {
    "accept_outing_invite": LINK_WORLD,
    "rotate_outing_invite_secret": NAMED_WORLD,
    "revoke_outing_invite": LINK_WORLD,
}


def outing_steps_edges() -> list[dict]:
    out = []
    S = step_case

    # --- the permission door every method has ------------------------------
    for fn, req in DEFAULT_REQ.items():
        for label, actor in DOOR_ACTORS:
            out.append(
                S(fn, f"{fn}/door/{label}", req, DOOR_WORLDS.get(fn), actor=actor)
            )
        out.append(S(fn, f"{fn}/happy", req, DOOR_WORLDS.get(fn)))

    # --- create and list ---------------------------------------------------
    out += [
        S(
            "create_outing",
            "create_outing/one-day",
            {**DEFAULT_REQ["create_outing"], "ends_on": D1},
        ),
        S(
            "create_outing",
            "create_outing/pair-context-is-allowed",
            {**DEFAULT_REQ["create_outing"], "context_id": "RIENG"},
        ),
        S(
            "list_context_outings",
            "list/two",
            {"context_id": "HOI"},
            {"outings": [world_outing(), world_outing("OU2", stops=[])]},
        ),
        S("list_context_outings", "list/none", {"context_id": "HOI"}, {"outings": []}),
        S(
            "list_context_outings",
            "list/meeting-point",
            {"context_id": "HOI"},
            {
                "outings": [
                    world_outing(
                        stops=[world_stop("ST1", meeting=[10.77, 106.7, "Cổng chính"])]
                    )
                ]
            },
        ),
    ]

    # --- timeline ----------------------------------------------------------
    out += [
        S(
            "replace_outing_timeline",
            "timeline/no-outing",
            {"outing_id": "OU2", "stops": [rstop()]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/place-unknown",
            {"outing_id": "OU1", "stops": [rstop(place="khong-co")]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/place-known",
            {"outing_id": "OU1", "stops": [rstop(place="pho-ha-noi")]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/expected-revision",
            {"outing_id": "OU1", "expected_revision": 3, "stops": [rstop("23:59")]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/revision-conflict",
            {"outing_id": "OU1", "expected_revision": 3, "stops": [rstop()]},
            {"conflicts": {"replace_outing_stops": ["TIMELINE_REVISION_CONFLICT"]}},
        ),
        S(
            "replace_outing_timeline",
            "timeline/upgrade-required",
            {"outing_id": "OU1", "stops": [rstop()]},
            {"conflicts": {"replace_outing_stops": ["ITINERARY_UPGRADE_REQUIRED"]}},
        ),
        S(
            "replace_outing_timeline",
            "timeline/unknown-conflict-is-a-500",
            {"outing_id": "OU1", "stops": [rstop()]},
            {"conflicts": {"replace_outing_stops": ["SOMETHING_ELSE"]}},
        ),
        S(
            "replace_outing_timeline",
            "timeline/empty",
            {"outing_id": "OU1", "stops": []},
        ),
        # `_minute_of_day` has no range check and no pattern of its own: the
        # field's pattern is what keeps these out, and past it they are a 500.
        S(
            "replace_outing_timeline",
            "timeline/at-has-three-parts",
            {"outing_id": "OU1", "stops": [rstop("1:2:3")]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/at-is-not-a-clock",
            {"outing_id": "OU1", "stops": [rstop("mot gio")]},
        ),
        S(
            "replace_outing_timeline",
            "timeline/at-has-one-part",
            {"outing_id": "OU1", "stops": [rstop("0800")]},
        ),
    ]

    # --- itinerary draft ---------------------------------------------------
    out += [
        S(
            "preview_outing_itinerary",
            "draft/revision-mismatch",
            {"outing_id": "OU1", "request": ireq(revision=2)},
        ),
        S(
            "preview_outing_itinerary",
            "draft/day-outside-trip",
            {"outing_id": "OU1", "request": ireq(days=[iday("2026-04-01")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/anchor-not-on-day",
            {"outing_id": "OU1", "request": ireq(days=[iday(start_stop="ST2")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/anchor-empty-string",
            {"outing_id": "OU1", "request": ireq(days=[iday(start_stop="")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/stop-not-in-outing",
            {"outing_id": "OU1", "request": ireq([istop("ST3")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/temporary-id",
            {"outing_id": "OU1", "request": ireq([istop("tmp-mot")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/temporary-id-too-short",
            {"outing_id": "OU1", "request": ireq([istop("tmp-")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/stop-day-outside-trip",
            {"outing_id": "OU1", "request": ireq([istop(day="2026-04-01")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/stop-day-without-config",
            {
                "outing_id": "OU1",
                "request": ireq([istop(day=D2)], [iday(D1)]),
            },
        ),
        S(
            "preview_outing_itinerary",
            "draft/stop-with-no-day",
            {"outing_id": "OU1", "request": ireq([istop(day=None)])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/place-unknown",
            {"outing_id": "OU1", "request": ireq([istop(place="khong-co")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/place-known",
            {"outing_id": "OU1", "request": ireq([istop(place="pho-ha-noi")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/place-without-coordinates",
            {"outing_id": "OU1", "request": ireq([istop(place="khong-toa-do")])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/meeting-point",
            {
                "outing_id": "OU1",
                "request": ireq(
                    [istop(meeting={"lat": 10.5, "lng": 106.5, "label": "Cổng"})]
                ),
            },
        ),
        S(
            "preview_outing_itinerary",
            "draft/checked-in",
            {"outing_id": "OU1", "request": ireq()},
            {"checkins": [["CI1", "ST1", "BAN"]]},
        ),
        S(
            "preview_outing_itinerary",
            "draft/preview-day-outside-trip",
            {"outing_id": "OU1", "request": ireq(day="2026-04-01", days=[iday(D1)])},
        ),
        S(
            "preview_outing_itinerary",
            "draft/no-outing",
            {"outing_id": "OU2", "request": ireq()},
        ),
    ]

    # --- itinerary write ---------------------------------------------------
    out += [
        S(
            "replace_outing_itinerary",
            "itinerary/two-days",
            {
                "outing_id": "OU1",
                "request": ireq(
                    [istop("ST1"), istop("ST2", "09:00", day=D2)],
                    [iday(D1), iday(D2, mode="walk", returning=True)],
                ),
            },
            {
                "outings": [
                    world_outing(
                        stops=[world_stop("ST1"), world_stop("ST2", position=1)]
                    )
                ]
            },
        ),
        S(
            "replace_outing_itinerary",
            "itinerary/stop-not-found-conflict",
            {"outing_id": "OU1", "request": ireq()},
            {"conflicts": {"replace_outing_itinerary": ["STOP_NOT_FOUND"]}},
        ),
        S(
            "replace_outing_itinerary",
            "itinerary/outing-not-found-conflict",
            {"outing_id": "OU1", "request": ireq()},
            {"conflicts": {"replace_outing_itinerary": ["OUTING_NOT_FOUND"]}},
        ),
        S(
            "replace_outing_itinerary",
            "itinerary/meeting-point-round-trip",
            {
                "outing_id": "OU1",
                "request": ireq(
                    [istop(meeting={"lat": 10.5, "lng": 106.5, "label": "Cổng"})]
                ),
            },
            {
                "saved": world_outing(
                    stops=[world_stop("ST1", meeting=[10.5, 106.5, "Cổng"])],
                    revision=1,
                    version=2,
                    days=[iday()],
                )
            },
        ),
    ]

    # --- check-ins ---------------------------------------------------------
    out += [
        S("check_in_to_stop", "checkin/no-stop", {"stop_id": "ST3"}),
        S(
            "check_in_to_stop",
            "checkin/already",
            {"stop_id": "ST1"},
            {"conflicts": {"create_stop_checkin": ["ALREADY_CHECKED_IN"]}},
        ),
        S(
            "check_in_to_stop",
            "checkin/stop-removed",
            {"stop_id": "ST1"},
            {"conflicts": {"create_stop_checkin": ["STOP_NOT_FOUND"]}},
        ),
        S(
            "check_in_to_stop",
            "checkin/unknown-conflict-is-a-500",
            {"stop_id": "ST1"},
            {"conflicts": {"create_stop_checkin": ["SOMETHING_ELSE"]}},
        ),
        S(
            "list_outing_checkins",
            "checkins/three",
            {"outing_id": "OU1"},
            {
                "checkins": [
                    ["CI1", "ST1", "TOI"],
                    ["CIN", "ST1", "BAN"],
                    ["MB1", "ST2", "XOA"],
                ]
            },
        ),
    ]

    # --- invitations -------------------------------------------------------
    out += [
        S(
            "create_outing_invite",
            "invite/group",
            {"outing_id": "OU1", "source": "group", "person_id": "BAN"},
        ),
        S(
            "create_outing_invite",
            "invite/friend-outside-the-group",
            {"outing_id": "OU1", "source": "friend", "person_id": "LA"},
        ),
        S(
            "create_outing_invite",
            "invite/group-but-not-a-member",
            {"outing_id": "OU1", "source": "group", "person_id": "LA"},
        ),
        S(
            "create_outing_invite",
            "invite/person-not-registered",
            {"outing_id": "OU1", "source": "group", "person_id": "XOA"},
        ),
        S(
            "create_outing_invite",
            "invite/already-invited",
            {"outing_id": "OU1", "source": "group", "person_id": "BAN"},
            {"invites": [world_invite(source="group", person="BAN")]},
        ),
        S(
            "create_outing_invite",
            "invite/pair-context",
            {"outing_id": "OU1", "source": "link"},
            {"outings": [world_outing(context="RIENG")]},
        ),
        S(
            "create_outing_invite",
            "invite/no-outing",
            {"outing_id": "OU2", "source": "link"},
        ),
        S(
            "accept_outing_invite",
            "accept/unknown-digest",
            {"token": "TK_LA"},
            LINK_WORLD,
        ),
        S(
            "accept_outing_invite",
            "accept/named-invite",
            {"token": "TK_CU"},
            NAMED_WORLD,
        ),
        S(
            "accept_outing_invite",
            "accept/already-accepted",
            {"token": "TK_CU"},
            {
                "invites": [world_invite(accepted=EARLIER, accepted_by="BAN")],
                "digests": {"TK_CU": "IV1"},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/revoked",
            {"token": "TK_CU"},
            {
                "invites": [world_invite(revoked=EARLIER)],
                "digests": {"TK_CU": "IV1"},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/expired",
            {"token": "TK_CU"},
            {
                "invites": [world_invite(expires=EARLIER)],
                "digests": {"TK_CU": "IV1"},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/expires-exactly-now",
            {"token": "TK_CU"},
            {
                "invites": [world_invite(expires=T)],
                "digests": {"TK_CU": "IV1"},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/outing-vanished",
            {"token": "TK_CU"},
            {
                "invites": [world_invite(outing="OU2")],
                "digests": {"TK_CU": "IV1"},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/stranger-may-redeem",
            {"token": "TK_CU"},
            LINK_WORLD,
            actor=("LA", ()),
        ),
        S(
            "accept_outing_invite",
            "accept/race-already-accepted",
            {"token": "TK_CU"},
            {
                **LINK_WORLD,
                "conflicts": {
                    "accept_outing_invite": ["OUTING_INVITE_ALREADY_ACCEPTED"]
                },
            },
        ),
        S(
            "accept_outing_invite",
            "accept/race-not-redeemable",
            {"token": "TK_CU"},
            {
                **LINK_WORLD,
                "conflicts": {"accept_outing_invite": ["OUTING_INVITE_NOT_REDEEMABLE"]},
            },
        ),
        S(
            "accept_outing_invite",
            "accept/race-unknown-conflict",
            {"token": "TK_CU"},
            {**LINK_WORLD, "conflicts": {"accept_outing_invite": ["SOMETHING_ELSE"]}},
        ),
        S(
            "accept_outing_invite",
            "accept/membership-already-active",
            {"token": "TK_CU"},
            {**LINK_WORLD, "membership": ["MB1", "active"]},
        ),
        S(
            "rotate_outing_invite_secret",
            "rotate/no-invite",
            {"outing_id": "OU1", "invite_id": "IV2"},
            NAMED_WORLD,
        ),
        S(
            "rotate_outing_invite_secret",
            "rotate/other-outing",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {"invites": [world_invite(outing="OU2", source="group", person="BAN")]},
        ),
        S(
            "rotate_outing_invite_secret",
            "rotate/link-is-not-named",
            {"outing_id": "OU1", "invite_id": "IV1"},
            LINK_WORLD,
        ),
        S(
            "rotate_outing_invite_secret",
            "rotate/conflict-is-a-404",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {
                **NAMED_WORLD,
                "conflicts": {
                    "rotate_outing_invite_digest": ["OUTING_INVITE_NOT_FOUND"]
                },
            },
        ),
        S(
            "revoke_outing_invite",
            "revoke/already-accepted",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {"invites": [world_invite(accepted=EARLIER, accepted_by="BAN")]},
        ),
        S(
            "revoke_outing_invite",
            "revoke/race-accepted",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {
                **LINK_WORLD,
                "conflicts": {
                    "revoke_outing_invite": ["OUTING_INVITE_ALREADY_ACCEPTED"]
                },
            },
        ),
        S(
            "revoke_outing_invite",
            "revoke/race-not-found",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {
                **LINK_WORLD,
                "conflicts": {"revoke_outing_invite": ["OUTING_INVITE_NOT_FOUND"]},
            },
        ),
        S(
            "revoke_outing_invite",
            "revoke/race-unknown-conflict",
            {"outing_id": "OU1", "invite_id": "IV1"},
            {**LINK_WORLD, "conflicts": {"revoke_outing_invite": ["SOMETHING_ELSE"]}},
        ),
        S(
            "revoke_outing_invite",
            "revoke/named",
            {"outing_id": "OU1", "invite_id": "IV1"},
            NAMED_WORLD,
        ),
    ]
    return out


FUZZ_CONFLICTS = {
    "replace_outing_stops": (
        "TIMELINE_REVISION_CONFLICT",
        "ITINERARY_UPGRADE_REQUIRED",
        "STOP_NOT_FOUND",
        "OUTING_NOT_FOUND",
        "SOMETHING_ELSE",
    ),
    "replace_outing_itinerary": (
        "TIMELINE_REVISION_CONFLICT",
        "ITINERARY_UPGRADE_REQUIRED",
        "STOP_NOT_FOUND",
        "OUTING_NOT_FOUND",
    ),
    "create_stop_checkin": ("ALREADY_CHECKED_IN", "STOP_NOT_FOUND", "SOMETHING_ELSE"),
    "accept_outing_invite": (
        "OUTING_INVITE_ALREADY_ACCEPTED",
        "OUTING_INVITE_NOT_FOUND",
        "OUTING_INVITE_NOT_REDEEMABLE",
        "SOMETHING_ELSE",
    ),
    "revoke_outing_invite": (
        "OUTING_INVITE_ALREADY_ACCEPTED",
        "OUTING_INVITE_NOT_FOUND",
        "SOMETHING_ELSE",
    ),
    "rotate_outing_invite_digest": ("OUTING_INVITE_NOT_FOUND", "SOMETHING_ELSE"),
    "create_outing": ("SOMETHING_ELSE",),
    "create_outing_invite": ("SOMETHING_ELSE",),
    "ensure_invited_membership": ("SOMETHING_ELSE",),
}
PEOPLE = ("TOI", "BAN", "LA", "XOA")
STOP_IDS = ("ST1", "ST2", "ST3", "tmp-mot", "tmp-")
PLACES = (None, "pho-ha-noi", "khong-co", "khong-toa-do")
DAYS = (D1, D2, "2026-04-01", None)


def fuzz_world(rng: random.Random) -> dict:
    world: dict = {}
    if rng.random() < 0.85:
        stops = [
            world_stop(
                sid,
                position=n,
                minute=rng.randrange(0, 1440),
                day=rng.choice(DAYS),
                dwell=rng.choice((None, 0, 30, 1440)),
                locked=rng.random() < 0.6,
                meeting=[10.5, 106.5, "Cổng"] if rng.random() < 0.2 else None,
            )
            for n, sid in enumerate(("ST1", "ST2")[: rng.randrange(0, 3)])
        ]
        world["outings"] = [
            world_outing(
                stops=stops,
                revision=rng.randrange(0, 3),
                version=rng.choice((1, 2)),
                days=[iday()] if rng.random() < 0.3 else [],
                starts=rng.choice((D1, D2)),
                ends=rng.choice((D2, "2026-03-05")),
            )
        ]
    else:
        world["outings"] = []
    if rng.random() < 0.3:
        world["members"] = {
            "HOI": [
                [pid, rng.choice(("active", "left", "invited"))]
                for pid in rng.sample(PEOPLE, rng.randrange(0, 4))
            ]
        }
    if rng.random() < 0.3:
        world["contexts"] = {"HOI": rng.choice(("group", "pair")), "RIENG": "pair"}
    if rng.random() < 0.45:
        source = rng.choice(("link", "group", "friend"))
        world["invites"] = [
            world_invite(
                outing=rng.choice(("OU1", "OU2")),
                source=source,
                person=None if source == "link" else rng.choice(PEOPLE),
                accepted=rng.choice((None, None, EARLIER)),
                accepted_by="BAN",
                expires=rng.choice((LATER, EARLIER, T)),
                revoked=rng.choice((None, None, EARLIER)),
            )
        ]
        world["digests"] = {rng.choice(list(TOKENS)): "IV1"}
    if rng.random() < 0.3:
        world["checkins"] = [
            [cid, rng.choice(("ST1", "ST2")), rng.choice(PEOPLE)]
            for cid in ("CI1", "CIN")[: rng.randrange(0, 3)]
        ]
    if rng.random() < 0.25:
        method = rng.choice(list(FUZZ_CONFLICTS))
        world["conflicts"] = {method: [rng.choice(FUZZ_CONFLICTS[method])]}
    if rng.random() < 0.15:
        world["membership"] = ["MB1", rng.choice(("invited", "active"))]
    return world


def fuzz_req(rng: random.Random, fn: str) -> dict:
    if fn == "create_outing":
        return {
            "context_id": rng.choice(("HOI", "RIENG")),
            "title": rng.choice(("Đà Lạt", "Nha Trang")),
            "starts_on": rng.choice((D1, D2)),
            "ends_on": rng.choice((D2, "2026-03-05")),
            "headcount": rng.randrange(1, 12),
            "budget_per_person_vnd": rng.randrange(0, 5_000_000),
        }
    if fn == "list_context_outings":
        return {"context_id": rng.choice(("HOI", "RIENG"))}
    if fn == "replace_outing_timeline":
        return {
            "outing_id": rng.choice(("OU1", "OU2")),
            "expected_revision": rng.choice((None, 0, 1, 3)),
            "stops": [
                rstop(fuzz_time(rng), place=rng.choice(PLACES))
                for _ in range(rng.randrange(0, 3))
            ],
        }
    if fn in {"authorize_outing_itinerary", "list_outing_checkins"}:
        return {"outing_id": rng.choice(("OU1", "OU2"))}
    if fn in {"preview_outing_itinerary", "replace_outing_itinerary"}:
        stops = [
            istop(
                rng.choice(STOP_IDS),
                fuzz_time(rng),
                day=rng.choice(DAYS),
                dwell=rng.choice((None, 0, 30, 1440)),
                locked=rng.random() < 0.6,
                place=rng.choice(PLACES),
                meeting=(
                    {"lat": 10.5, "lng": 106.5, "label": "Cổng"}
                    if rng.random() < 0.2
                    else None
                ),
            )
            for _ in range(rng.randrange(0, 3))
        ]
        anchors = [None, "", *[s["id"] for s in stops], "ST3"]
        days = [
            iday(
                day,
                mode=rng.choice(("car", "walk", "motorbike")),
                start=fuzz_time(rng),
                start_stop=rng.choice(anchors),
                end_stop=rng.choice(anchors),
                returning=rng.random() < 0.3,
            )
            for day in rng.sample((D1, D2, "2026-04-01"), rng.randrange(0, 3))
        ]
        return {
            "outing_id": rng.choice(("OU1", "OU2")),
            "request": {
                "expected_revision": rng.randrange(0, 3),
                "stops": stops,
                "days": days,
                "day": rng.choice((D1, D2, "2026-04-01")),
                "include_suggestion": rng.random() < 0.5,
            },
        }
    if fn == "check_in_to_stop":
        return {"stop_id": rng.choice(("ST1", "ST2", "ST3"))}
    if fn == "create_outing_invite":
        source = rng.choice(("link", "group", "friend"))
        return {
            "outing_id": rng.choice(("OU1", "OU2")),
            "source": source,
            "person_id": None if source == "link" else rng.choice(PEOPLE),
        }
    if fn == "accept_outing_invite":
        return {"token": rng.choice(list(TOKENS))}
    return {
        "outing_id": rng.choice(("OU1", "OU2")),
        "invite_id": rng.choice(("IV1", "IV2")),
    }


def outing_steps_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(f"w7-outing-steps/{seed}")
    methods = list(CALLERS)
    out = []
    for i in range(count):
        fn = rng.choice(methods)
        roles = rng.choice(((), ("member",), ("member",), ("group_admin",)))
        actor = (rng.choice(PEOPLE), roles)
        built = step_case(
            fn,
            f"fuzz/{i}",
            fuzz_req(rng, fn),
            fuzz_world(rng),
            now=T + timedelta(minutes=rng.randrange(-5000, 5000)),
            actor=actor,
        )
        if fits(built):
            out.append(built)
    return out


# ---------------------------------------------------------------------------
# Plumbing
# ---------------------------------------------------------------------------

MODULES = {
    "journey": (
        "internal/domain/journey",
        journey_module,
        journey_constants,
        journey_edges,
        journey_fuzz,
        120,
        20000,
    ),
    "valhalla": (
        "internal/domain/valhalla",
        routing_module,
        valhalla_constants,
        valhalla_edges,
        valhalla_fuzz,
        120,
        20000,
    ),
    "itinerary": (
        "internal/domain/itinerary",
        preview_module,
        itinerary_constants,
        itinerary_edges,
        itinerary_fuzz,
        50,
        6000,
    ),
    "outing_steps": (
        "internal/domain/outingsteps",
        api_service,
        outing_steps_constants,
        outing_steps_edges,
        outing_steps_fuzz,
        50,
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
        "generator": "scripts/render_domain_w7_goldens.py",
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
