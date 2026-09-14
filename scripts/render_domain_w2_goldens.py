#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W2 routes (ADR-0029).

Friends, identity, stories, posts and votes lean on eight pure Python modules,
each ported to one Go package:

    app.domain.friendship        services/core/internal/domain/friendship
    app.domain.blocking          services/core/internal/domain/blocking
    app.domain.visibility        services/core/internal/domain/visibility
    app.domain.story_visibility  services/core/internal/domain/storyvisibility
    app.domain.post_audience     services/core/internal/domain/postaudience
    app.domain.vote              services/core/internal/domain/vote
    app.api.cursors              services/core/internal/cursors
    app.api.person_identity      services/core/internal/identity

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each one returned or raised. Every package's oracle_test.go replays every
case. The image is the one scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w2_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w2_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases (the inputs of
its Python tests plus the boundaries found while porting); `<module>-fuzz-<k>`
is shard k of that module's seeded fuzz. Shards are contiguous slices of one
fixed case list, so one shard rendered alone gives the same bytes.

The inputs have the shapes the service builds (str ids, bool facts, aware
datetimes with a fixed offset, dicts with the keys the service writes), because
the Go ports are typed. Non-ASCII characters whose identity matters are spelled
with `chr()`: a literal U+0301 cannot be reviewed.

## Encoding

A case is `{"fn", "name", "args": {parameter: value}, "result"}` and a result
is `{"ok": value}` or `{"raised": {"type", "message", "code"}}`, `code` being
the exception's `.code` or null. Values are JSON, except:

* `"$i:<hex>"` is an int of nine or more digits and `"$f:<float.hex()>"` a
  float;
* `"$dt:<isoformat at microseconds>|<utc offset>"` is an aware datetime; the
  `|` keeps a negative offset from reading as one long digit run;
* `"$hex:<digits>"` is bytes, the hex digits in groups of eight joined by `:`;
* `"$sp:<text>"` is a str cut into groups of six code points joined by `|`
  (the last group may be shorter). A str takes this form when it starts with
  `$`, or when written plainly it would trip scripts/repo_guard.py: a digit
  run the phone and number rules refuse, or a base64-looking fragment that
  counts towards the per-file budget (cursors are base64);
* a set is its sorted list; a tuple is a list.
"""

from __future__ import annotations

import base64
import json
import os
import platform
import random
import re
import string
import sys
import unicodedata
import uuid
from datetime import UTC, datetime, timedelta, timezone

sys.path.insert(0, "/srv")

from app.api import cursors, person_identity  # noqa: E402
from app.domain import (  # noqa: E402
    blocking,
    friendship,
    post_audience,
    story_visibility,
    visibility,
    vote,
)

SEED = 41
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
    if isinstance(value, float):
        return f"$f:{value.hex()}"
    if isinstance(value, datetime):
        return enc_datetime(value)
    if isinstance(value, uuid.UUID):
        return enc_str(str(value))
    if isinstance(value, bytes):
        digits = value.hex()
        return "$hex:" + ":".join(digits[i : i + 8] for i in range(0, len(digits), 8))
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


def enc_datetime(value: datetime) -> str:
    if value.utcoffset() is None:
        raise TypeError("naive datetimes never reach the service")
    full = value.isoformat(timespec="microseconds")
    naive = value.replace(tzinfo=None).isoformat(timespec="microseconds")
    return "$dt:" + naive + "|" + full[len(naive) :]


def case(
    fn: str,
    name: str,
    kwargs: dict,
    call: dict | None = None,
    shape=None,
) -> dict:
    args = {key: enc(value) for key, value in kwargs.items()}
    try:
        value = FUNCTIONS[fn](**(kwargs if call is None else call))
    except Exception as exc:  # every refusal is data for the oracle
        result = {
            "raised": {
                "type": type(exc).__name__,
                "message": enc_str(str(exc)),
                "code": enc(getattr(exc, "code", None)),
            }
        }
    else:
        result = {"ok": enc(value if shape is None else shape(value))}
    return {"fn": fn, "name": name, "args": args, "result": result}


def code_point_ranges(points) -> list[list[int]]:
    ranges: list[list[int]] = []
    for point in points:
        if ranges and ranges[-1][1] == point - 1:
            ranges[-1][1] = point
        else:
            ranges.append([point, point])
    return ranges


def every_code_point(predicate) -> list[list[int]]:
    return code_point_ranges(
        point for point in range(sys.maxunicode + 1) if predicate(chr(point))
    )


def decimal_ranges() -> list[list[int]]:
    """[first, last, value of first] for runs whose values count up mod 10."""
    ranges: list[list[int]] = []
    for point in range(sys.maxunicode + 1):
        value = unicodedata.decimal(chr(point), -1)
        if value < 0:
            continue
        if (
            ranges
            and ranges[-1][1] == point - 1
            and (ranges[-1][2] + point - ranges[-1][0]) % 10 == value
        ):
            ranges[-1][1] = point
        else:
            ranges.append([point, point, value])
    return ranges


CONSTANT_TYPES = (str, int, bool, tuple, frozenset, dict, bytes, timedelta)


def public_names(module) -> list[str]:
    names = []
    for name, value in vars(module).items():
        if name.startswith("_"):
            continue
        if callable(value):
            if getattr(value, "__module__", None) == module.__name__:
                names.append(name)
        elif type(value) in CONSTANT_TYPES:
            names.append(name)
    return sorted(names)


# ---------------------------------------------------------------------------
# Shared pieces
# ---------------------------------------------------------------------------

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
C = "a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
MISSING = object()

E_ACUTE = chr(0xE9)
E_DOT_CIRCUMFLEX = chr(0x1EC7)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
REPLACEMENT_CHARACTER = chr(0xFFFD)
ZERO_WIDTH_SPACE = chr(0x200B)
NO_BREAK_SPACE = chr(0xA0)
NEXT_LINE = chr(0x85)
IDEOGRAPHIC_SPACE = chr(0x3000)
LINE_SEPARATOR = chr(0x2028)
SOFT_HYPHEN = chr(0xAD)
COMBINING_ACUTE = chr(0x301)
BYTE_ORDER_MARK = chr(0xFEFF)
PRIVATE_USE = chr(0xE000)
UNASSIGNED_GREEK = chr(0x378)
UNASSIGNED_TAG = chr(0xE0080)
LANGUAGE_TAG = chr(0xE0001)
LAST_CODE_POINT = chr(sys.maxunicode)
APPLICATION_COMMAND = chr(0x9F)
SUPERSCRIPT_TWO = chr(0xB2)
ROMAN_NUMERAL_ONE = chr(0x2160)
FULLWIDTH_CAPITAL_A = chr(0xFF21)


def digit_run(zero: int) -> str:
    return "".join(chr(zero + value) for value in range(10))


ARABIC_INDIC = digit_run(0x660)
DEVANAGARI = digit_run(0x966)
FULLWIDTH = digit_run(0xFF10)
MATH_BOLD = digit_run(0x1D7CE)
ADLAM = digit_run(0x1E950)
DIGIT_SETS = (string.digits, ARABIC_INDIC, DEVANAGARI, FULLWIDTH, MATH_BOLD, ADLAM)

WHITESPACE = tuple(chr(c) for c in range(sys.maxunicode + 1) if chr(c).isspace())
ODD = (
    "'",
    '"',
    "\\",
    "\t",
    "\n",
    "\r",
    "\x00",
    "\x07",
    "\x1f",
    "\x7f",
    NEXT_LINE,
    APPLICATION_COMMAND,
    NO_BREAK_SPACE,
    SOFT_HYPHEN,
    E_ACUTE,
    COMBINING_ACUTE,
    ZERO_WIDTH_SPACE,
    LINE_SEPARATOR,
    UNASSIGNED_GREEK,
    BYTE_ORDER_MARK,
    PRIVATE_USE,
    REPLACEMENT_CHARACTER,
    IDEOGRAPHIC_SPACE,
    CJK_MIDDLE,
    E_DOT_CIRCUMFLEX,
    GRINNING_FACE,
    UNASSIGNED_TAG,
    LANGUAGE_TAG,
    LAST_CODE_POINT,
)
PLAIN = string.ascii_letters + string.digits + " -_."


def random_text(rng: random.Random, most: int = 6) -> str:
    return "".join(
        rng.choice(ODD) if rng.random() < 0.35 else rng.choice(PLAIN)
        for _ in range(rng.randrange(most + 1))
    )


NOW = datetime(2026, 9, 15, 9, 30, 12, 345678, tzinfo=UTC)
EPOCH = datetime(1970, 1, 1, tzinfo=UTC)
MICRO = timedelta(microseconds=1)
OFFSETS = (
    timedelta(0),
    timedelta(0),
    timedelta(hours=7),
    timedelta(hours=-10),
    timedelta(hours=5, minutes=30),
    timedelta(hours=-3, minutes=-30),
    timedelta(hours=23, minutes=59),
    timedelta(hours=-23, minutes=-59),
    timedelta(hours=5, minutes=30, seconds=15),
    timedelta(seconds=-1),
)
MICRO_OFFSETS = (
    timedelta(microseconds=1),
    timedelta(hours=1, microseconds=500000),
    timedelta(seconds=-1, microseconds=-250000),
    timedelta(hours=-23, minutes=-59, seconds=-59, microseconds=-999999),
)


def zone(offset: timedelta) -> timezone:
    return UTC if offset == timedelta(0) else timezone(offset)


def random_moment(rng: random.Random, micro_offsets: bool = False) -> datetime:
    roll = rng.random()
    if roll < 0.5:
        moment = NOW + timedelta(microseconds=rng.randrange(-(10**14), 10**14))
    elif roll < 0.85:
        moment = datetime(
            rng.randrange(2, 9999),
            rng.randrange(1, 13),
            rng.randrange(1, 29),
            rng.randrange(24),
            rng.randrange(60),
            rng.randrange(60),
            rng.choice((0, rng.randrange(10**6))),
            tzinfo=UTC,
        )
    else:
        moment = rng.choice(
            (
                datetime(2, 1, 1, tzinfo=UTC),
                datetime(9998, 12, 31, 23, 59, 59, 999999, tzinfo=UTC),
                NOW.replace(microsecond=0),
                datetime(1970, 1, 1, tzinfo=UTC),
            )
        )
    offsets = OFFSETS + (MICRO_OFFSETS if micro_offsets else ())
    return moment.astimezone(zone(rng.choice(offsets)))


# ---------------------------------------------------------------------------
# friendship
# ---------------------------------------------------------------------------

FRIEND_STATES = tuple(state.value for state in friendship.FriendState)
DECISIONS = tuple(answer.value for answer in friendship.Decision)
NEAR_STATES = (
    "Pending",
    "ACCEPTED",
    "blocked ",
    " declined",
    "",
    "unknown",
    "it's",
    'say "hi"',
    "both ' and \"",
    "back\\slash",
    "tab\tline\n",
    "nul\x00",
    "del\x7f",
    NO_BREAK_SPACE + "pending",
    "pending" + ZERO_WIDTH_SPACE,
    "blocked" + COMBINING_ACUTE,
    GRINNING_FACE,
    PRIVATE_USE,
    UNASSIGNED_GREEK,
    LINE_SEPARATOR,
    SOFT_HYPHEN,
    IDEOGRAPHIC_SPACE,
    APPLICATION_COMMAND,
    UNASSIGNED_TAG,
    LAST_CODE_POINT,
    "accept",
    "block",
    "decline",
)
NEAR_DECISIONS = (
    "Accept",
    "BLOCK",
    "accepted",
    "blocked",
    "declined",
    "pending",
    "",
    " accept",
    "accept\n",
    "it's",
    GRINNING_FACE,
    "decline" + ZERO_WIDTH_SPACE,
)
PEOPLE = (
    A,
    B,
    C,
    "",
    A.upper(),
    "ngu" + E_DOT_CIRCUMFLEX,
    E_ACUTE,
    "z",
    "Z",
    REPLACEMENT_CHARACTER,
    GRINNING_FACE,
    "a",
    "a\x00",
    "ab",
)


def friend_edge(requester, addressee, state, decided_by=MISSING, pair=MISSING):
    row = {"requester_id": requester, "addressee_id": addressee, "state": state}
    if decided_by is not MISSING:
        row["decided_by_id"] = decided_by
    if pair is not MISSING:
        row["pair"] = pair
    return row


def friendship_constants() -> dict:
    return {
        "states": list(FRIEND_STATES),
        "decisions": list(DECISIONS),
        "blocked_is_silent": friendship.BLOCKED_IS_SILENT,
        "live": [state.value for state in friendship._LIVE],
        "printable": every_code_point(str.isprintable),
        "names": public_names(friendship),
    }


def friendship_edges() -> list[dict]:
    out: list[dict] = []

    def add(fn: str, name: str, **kwargs: object) -> None:
        out.append(case(fn, name, kwargs))

    for i, first in enumerate(PEOPLE):
        for j, second in enumerate(PEOPLE):
            add("pair_key", f"pair_key/{i}/{j}", a=first, b=second)
    for i, state in enumerate(FRIEND_STATES + NEAR_STATES + DECISIONS):
        add("is_live_edge", f"is_live_edge/{i}", state=state)

    existing = [
        None,
        *({"state": state} for state in FRIEND_STATES + NEAR_STATES[:6]),
        *(friend_edge(A, B, state) for state in FRIEND_STATES),
        friend_edge(B, A, "declined", decided_by=B, pair=[A, B]),
        friend_edge(C, C, "blocked", decided_by=None),
    ]
    for label, requester, addressee in (
        ("ab", A, B),
        ("ba", B, A),
        ("self", A, A),
        ("no_requester", "", B),
        ("no_addressee", A, ""),
        ("upper", A, A.upper()),
    ):
        for k, row in enumerate(existing):
            add(
                "open_request",
                f"open_request/{label}/{k}",
                requester_id=requester,
                addressee_id=addressee,
                existing=row,
            )

    actors = (
        ("requester", A),
        ("addressee", B),
        ("stranger", C),
        ("upper", B.upper()),
        ("empty", ""),
    )
    for s, state in enumerate(FRIEND_STATES + ("Pending", "", "it's")):
        for d, decision in enumerate(DECISIONS + ("Accept", "", "blocked", 'x"y')):
            for who, actor in actors:
                add(
                    "decide",
                    f"decide/{s}/{d}/{who}",
                    edge=friend_edge(A, B, state),
                    actor_id=actor,
                    decision=decision,
                )
    add(
        "decide",
        "decide/keeps_pair_and_decider",
        edge=friend_edge(A, B, "pending", decided_by=None, pair=[A, B]),
        actor_id=B,
        decision="accept",
    )
    add(
        "decide",
        "decide/block_keeps_the_old_decider",
        edge=friend_edge(A, B, "accepted", decided_by=B, pair=[A, B]),
        actor_id=A,
        decision="block",
    )
    add(
        "decide",
        "decide/self_edge_row",
        edge=friend_edge(A, A, "pending"),
        actor_id=A,
        decision="accept",
    )
    add(
        "decide",
        "decide/empty_parties",
        edge=friend_edge("", "", "pending"),
        actor_id="",
        decision="decline",
    )

    for s, state in enumerate(FRIEND_STATES + ("Declined", "x")):
        for r, (requester, addressee) in enumerate(((A, B), (B, A), (C, B))):
            for d, decider in enumerate((MISSING, None, requester, addressee)):
                row = friend_edge(requester, addressee, state, decided_by=decider)
                for who, blocker, other in (("ab", A, B), ("ba", B, A)):
                    add(
                        "open_block",
                        f"open_block/{s}/{r}/{d}/{who}",
                        blocker_id=blocker,
                        addressee_id=other,
                        existing=row,
                    )
    for label, blocker, other in (
        ("stranger", A, B),
        ("self", A, A),
        ("no_blocker", "", B),
        ("no_addressee", A, ""),
    ):
        add(
            "open_block",
            f"open_block/none/{label}",
            blocker_id=blocker,
            addressee_id=other,
            existing=None,
        )
    add(
        "open_block",
        "open_block/self_before_the_row",
        blocker_id=A,
        addressee_id=A,
        existing=friend_edge(A, "x", "zzz"),
    )

    for s, state in enumerate(FRIEND_STATES + ("Blocked", "")):
        for d, decider in enumerate((MISSING, None, A, B, A.upper(), "")):
            for who, actor in (("a", A), ("b", B), ("empty", "")):
                add(
                    "unblock",
                    f"unblock/{s}/{d}/{who}",
                    edge=friend_edge(A, B, state, decided_by=decider),
                    actor_id=actor,
                )
    add("unblock", "unblock/none", edge=None, actor_id=A)
    add("unblock", "unblock/none_empty_actor", edge=None, actor_id="")

    add("are_friends", "are_friends/none", edge=None)
    for s, state in enumerate(FRIEND_STATES + NEAR_STATES):
        add("are_friends", f"are_friends/{s}", edge=friend_edge(A, B, state))
    return out


def friendship_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 1)
    pool = (A, B, C, "", A.upper(), "ngu" + E_DOT_CIRCUMFLEX, E_ACUTE, "z")
    count = 900

    def person() -> str:
        return rng.choice(pool) if rng.random() < 0.92 else random_text(rng)

    def word(valid: tuple, near: tuple) -> str:
        roll = rng.random()
        if roll < 0.8:
            return rng.choice(valid)
        if roll < 0.93:
            return rng.choice(near)
        return random_text(rng)

    def some_edge(partial: bool = False) -> dict:
        state = word(FRIEND_STATES, NEAR_STATES)
        if partial and rng.random() < 0.3:
            return {"state": state}
        if rng.random() < 0.4:
            requester, addressee = rng.sample((A, B), 2)
        else:
            requester, addressee = person(), person()
        row = friend_edge(requester, addressee, state)
        if rng.random() < 0.5:
            row["decided_by_id"] = rng.choice((None, requester, addressee, person()))
        if rng.random() < 0.3:
            row["pair"] = [person(), person()]
        return row

    def maybe_edge(partial: bool = False) -> dict | None:
        return None if rng.random() < 0.2 else some_edge(partial)

    def party(row: dict | None) -> str:
        if row is None or "requester_id" not in row or rng.random() < 0.2:
            return person()
        choices = [row["requester_id"], row["addressee_id"]]
        if isinstance(row.get("decided_by_id"), str):
            choices.append(row["decided_by_id"])
        return rng.choice(choices)

    out: list[dict] = []
    for i in range(count):
        out.append(case("pair_key", f"fuzz/{i}", {"a": person(), "b": person()}))
    for i in range(count):
        state = word(FRIEND_STATES, NEAR_STATES)
        out.append(case("is_live_edge", f"fuzz/{i}", {"state": state}))
    for i in range(count):
        args = {
            "requester_id": person(),
            "addressee_id": person(),
            "existing": maybe_edge(partial=True),
        }
        out.append(case("open_request", f"fuzz/{i}", args))
    for i in range(count):
        row = some_edge()
        args = {
            "edge": row,
            "actor_id": party(row),
            "decision": word(DECISIONS, NEAR_DECISIONS),
        }
        out.append(case("decide", f"fuzz/{i}", args))
    for i in range(count):
        row = maybe_edge()
        blocker = party(row)
        other = rng.choice((A, B, person()))
        args = {"blocker_id": blocker, "addressee_id": other, "existing": row}
        out.append(case("open_block", f"fuzz/{i}", args))
    for i in range(count):
        row = maybe_edge()
        if row is not None and rng.random() < 0.5:
            row["state"] = "blocked"
            row["decided_by_id"] = rng.choice((row["requester_id"], None, person()))
        out.append(case("unblock", f"fuzz/{i}", {"edge": row, "actor_id": party(row)}))
    for i in range(count):
        out.append(case("are_friends", f"fuzz/{i}", {"edge": maybe_edge()}))
    return out


# ---------------------------------------------------------------------------
# blocking
# ---------------------------------------------------------------------------


def blocking_constants() -> dict:
    return {
        "direct_message_unavailable": blocking.DIRECT_MESSAGE_UNAVAILABLE,
        "names": public_names(blocking),
    }


def blocking_edges() -> list[dict]:
    out: list[dict] = []
    rows: list[dict | None] = [None]
    for state in FRIEND_STATES + ("Blocked", "blocked ", ""):
        rows.append({"state": state})
        for decider in (None, A, ""):
            rows.append({"state": state, "decided_by_id": decider})
    for k, row in enumerate(rows):
        out.append(case("is_blocked", f"is_blocked/{k}", {"edge": row}))
        out.append(case("blocker_of", f"blocker_of/{k}", {"edge": row}))
        out.append(case("hidden_between", f"hidden_between/{k}", {"edge": row}))
        for flag in (False, True):
            args = {"edge": row, "other_deleted": flag}
            out.append(case("dm_allowed", f"dm_allowed/{k}/{str(flag).lower()}", args))
    return out


def blocking_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 2)
    count = 500

    def row() -> dict | None:
        if rng.random() < 0.15:
            return None
        roll = rng.random()
        state = rng.choice(FRIEND_STATES) if roll < 0.85 else random_text(rng)
        edge: dict = {"state": state}
        if rng.random() < 0.7:
            edge["decided_by_id"] = rng.choice((None, A, B, "", random_text(rng)))
        return edge

    out: list[dict] = []
    for fn in ("is_blocked", "blocker_of", "hidden_between"):
        for i in range(count):
            out.append(case(fn, f"fuzz/{i}", {"edge": row()}))
    for i in range(count):
        args = {"edge": row(), "other_deleted": rng.random() < 0.3}
        out.append(case("dm_allowed", f"fuzz/{i}", args))
    return out


# ---------------------------------------------------------------------------
# visibility
# ---------------------------------------------------------------------------

LEVELS = visibility.LEVELS
NEAR_LEVELS = (
    "Group_visible",
    "group_visible ",
    "",
    "public",
    "private",
    "group_summary",
    "it's",
)
COMPONENTS = ("bank_account_number", "attachment", "output_summary", "unknown")
REDACTABLE = tuple(sorted(visibility.REDACTABLE))
REDACTIONS = (
    {},
    {"applied": None},
    {"applied": []},
    {"applied": ["mask_phone"]},
    {"applied": ["mask_phone", "mask_phone"]},
    {"applied": list(REDACTABLE)},
    {"applied": ["mask_phone", "bogus"]},
    {"applied": ("drop_attachment",)},
    {"applied": "mask_phone"},
    {"applied": ""},
    {"applied": "m"},
    {"applied": 0},
    {"applied": 7},
    {"applied": 1.5},
    {"applied": 0.0},
    {"applied": True},
    {"applied": False},
    {"applied": {"mask_phone": 1}},
    {"applied": {"bogus": 1}},
    {"applied": {}},
    {"applied": [["mask_phone"]]},
    {"applied": ["bogus", ["x"]]},
    {"applied": [{"a": 1}]},
    {"applied": [1, "mask_phone"]},
    {"applied": [None]},
    {"applied": [True]},
    {"note": "x"},
    {"applied": ["mask_phone"], "note": "kept as given"},
    {"applied": ["Mask_phone"]},
)


def field_row(owner=A, component="attachment", level="private_to_invoker") -> dict:
    row = {"id": "field-1", "component": component, "visibility": level}
    if owner is not MISSING:
        row["owner_id"] = owner
    return row


def visibility_constants() -> dict:
    return {
        "levels": list(LEVELS),
        "default_visibility": [
            [component, level]
            for component, level in visibility.DEFAULT_VISIBILITY.items()
        ],
        "never_group_visible": sorted(visibility.NEVER_GROUP_VISIBLE),
        "redactable": list(REDACTABLE),
        "settlement_view_fields": list(visibility.SETTLEMENT_VIEW_FIELDS),
        "names": public_names(visibility),
    }


def visibility_edges() -> list[dict]:
    out: list[dict] = []

    def add(fn: str, name: str, **kwargs: object) -> None:
        out.append(case(fn, name, kwargs))

    for i, level in enumerate(LEVELS + NEAR_LEVELS):
        add("rank", f"rank/{i}", level=level)

    lists: list[list[str]] = [[]]
    for size in (1, 2, 3):
        stack: list[list[str]] = [[]]
        for _ in range(size):
            stack = [prefix + [level] for prefix in stack for level in LEVELS]
        lists += stack
    lists += [["public"], ["public", "group_visible"], ["group_visible", ""]]
    lists += [["private_to_invoker", "it's"], ["", ""]]
    for i, levels in enumerate(lists):
        add("permitted_output_visibility", f"permitted/{i}", input_levels=levels)

    inputs = (
        [],
        ["private_to_invoker"],
        ["group_summary_private_details"],
        ["group_visible"],
        ["group_visible", "private_to_invoker"],
        ["public"],
        ["group_visible", "public"],
    )
    for c, component in enumerate(COMPONENTS):
        for r, requested in enumerate(LEVELS + ("", "public")):
            for n, levels in enumerate(inputs):
                for redacted in (False, True):
                    for consented in (False, True):
                        add(
                            "check_no_context_laundering",
                            f"laundering/{c}/{r}/{n}/{int(redacted)}{int(consented)}",
                            component=component,
                            requested_level=requested,
                            input_levels=levels,
                            redacted=redacted,
                            owner_consented=consented,
                        )

    for r, redaction in enumerate(REDACTIONS):
        for o, owner in enumerate((A, B, None, MISSING)):
            add(
                "declassify",
                f"declassify/redaction/{r}/{o}",
                field=field_row(owner=owner),
                to_level="group_visible",
                actor_id=A,
                redaction=redaction,
            )
    for c, component in enumerate(COMPONENTS):
        for t, to_level in enumerate(LEVELS + ("public",)):
            for s, level in enumerate(LEVELS + ("public",)):
                add(
                    "declassify",
                    f"declassify/levels/{c}/{t}/{s}",
                    field=field_row(component=component, level=level),
                    to_level=to_level,
                    actor_id=A,
                    redaction={"applied": ["mask_full_name"]},
                )

    created = NOW
    joins = (
        None,
        created - MICRO,
        created,
        created + MICRO,
        created.astimezone(zone(timedelta(hours=7))),
    )
    lefts = (None, created - MICRO, created, created + MICRO)
    for v, level in enumerate(LEVELS + ("public", "")):
        for j, joined in enumerate(joins):
            for g, left in enumerate(lefts):
                for listed in (False, True):
                    add(
                        "can_view_history",
                        f"history/{v}/{j}/{g}/{int(listed)}",
                        object_visibility=level,
                        viewer_joined_at=joined,
                        viewer_left_at=left,
                        object_created_at=created,
                        audience_snapshot={A, B} if listed else {B},
                        viewer_id=A,
                    )
    add(
        "can_view_history",
        "history/empty_snapshot",
        object_visibility="group_visible",
        viewer_joined_at=created,
        viewer_left_at=None,
        object_created_at=created,
        audience_snapshot=set(),
        viewer_id=A,
    )
    add(
        "can_view_history",
        "history/offsets_compare_as_instants",
        object_visibility="group_visible",
        viewer_joined_at=(created + MICRO).astimezone(zone(timedelta(hours=-10))),
        viewer_left_at=created.astimezone(zone(timedelta(hours=7))),
        object_created_at=created.astimezone(zone(timedelta(hours=5, minutes=30))),
        audience_snapshot={A},
        viewer_id=A,
    )

    fields = visibility.SETTLEMENT_VIEW_FIELDS
    full = {field: f"value {index}" for index, field in enumerate(fields)}
    add("settlement_view", "settlement/complete", obligation=full)
    add(
        "settlement_view",
        "settlement/extra_fields_are_dropped",
        obligation={
            "other_allocations": [{"person": B, "amount_vnd": 5}],
            **full,
            "bank_account_number": "redacted",
        },
    )
    for index, field in enumerate(fields):
        partial = {key: value for key, value in full.items() if key != field}
        add("settlement_view", f"settlement/missing/{index}", obligation=partial)
    add("settlement_view", "settlement/empty", obligation={})
    add(
        "settlement_view",
        "settlement/values_of_every_shape",
        obligation=dict(
            zip(
                fields,
                (None, 12, [1, "two", None], {"nested": {"x": True}}, -2.5),
                strict=True,
            )
        ),
    )
    add(
        "settlement_view",
        "settlement/reordered",
        obligation=dict(reversed(list(full.items()))),
    )
    return out


def visibility_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 3)
    count = 700

    def level() -> str:
        roll = rng.random()
        if roll < 0.8:
            return rng.choice(LEVELS)
        if roll < 0.93:
            return rng.choice(NEAR_LEVELS)
        return random_text(rng)

    def levels() -> list[str]:
        return [level() for _ in range(rng.choice((0, 1, 1, 2, 2, 3, 4)))]

    def value(depth: int = 0) -> object:
        roll = rng.random()
        if roll < 0.25:
            return random_text(rng)
        if roll < 0.35:
            return rng.choice(REDACTABLE)
        if roll < 0.45:
            return rng.randrange(-5, 5)
        if roll < 0.5:
            return rng.choice((0.0, 1.5, -2.25))
        if roll < 0.6:
            return rng.choice((None, True, False))
        if depth < 2 and roll < 0.8:
            return [value(depth + 1) for _ in range(rng.randrange(4))]
        if depth < 2:
            return {rng.choice(REDACTABLE + ("x", "")): value(depth + 1)}
        return "leaf"

    def redaction() -> dict:
        roll = rng.random()
        if roll < 0.1:
            return {}
        if roll < 0.55:
            chosen = [rng.choice(REDACTABLE) for _ in range(rng.randrange(1, 4))]
            if rng.random() < 0.3:
                chosen.append(rng.choice(("bogus", "", "Mask_phone")))
            return {"applied": chosen}
        body: dict = {"applied": value()}
        if rng.random() < 0.2:
            body["note"] = value()
        return body

    out: list[dict] = []
    for i in range(count):
        out.append(case("rank", f"fuzz/{i}", {"level": level()}))
    for i in range(count):
        args = {"input_levels": levels()}
        out.append(case("permitted_output_visibility", f"fuzz/{i}", args))
    for i in range(count):
        args = {
            "component": rng.choice(COMPONENTS + tuple(visibility.DEFAULT_VISIBILITY)),
            "requested_level": level(),
            "input_levels": levels(),
            "redacted": rng.random() < 0.5,
            "owner_consented": rng.random() < 0.5,
        }
        out.append(case("check_no_context_laundering", f"fuzz/{i}", args))
    for i in range(count):
        owner = rng.choice((A, A, B, None, MISSING, ""))
        component = rng.choice(COMPONENTS + tuple(visibility.DEFAULT_VISIBILITY))
        actor = rng.choice((A, B, ""))
        if isinstance(owner, str) and rng.random() < 0.75:
            actor = owner
        args = {
            "field": field_row(owner=owner, component=component, level=level()),
            "to_level": level(),
            "actor_id": actor,
            "redaction": redaction(),
        }
        out.append(case("declassify", f"fuzz/{i}", args))
    for i in range(count):
        created = random_moment(rng)
        joined = (
            None
            if rng.random() < 0.15
            else (created + timedelta(microseconds=rng.randrange(-3, 4))).astimezone(
                zone(rng.choice(OFFSETS))
            )
        )
        left = (
            None
            if rng.random() < 0.5
            else (created + timedelta(microseconds=rng.randrange(-3, 4))).astimezone(
                zone(rng.choice(OFFSETS))
            )
        )
        viewer = rng.choice((A, B, ""))
        args = {
            "object_visibility": level(),
            "viewer_joined_at": joined,
            "viewer_left_at": left,
            "object_created_at": created,
            "audience_snapshot": set(rng.sample((A, B, C, ""), rng.randrange(5))),
            "viewer_id": viewer,
        }
        out.append(case("can_view_history", f"fuzz/{i}", args))
    fields = visibility.SETTLEMENT_VIEW_FIELDS
    for i in range(count):
        keys = [field for field in fields if rng.random() < 0.9]
        keys += [
            rng.choice(("amount_vnd", "other", "")) for _ in range(rng.randrange(3))
        ]
        rng.shuffle(keys)
        obligation = {key: value() for key in keys}
        out.append(case("settlement_view", f"fuzz/{i}", {"obligation": obligation}))
    return out


# ---------------------------------------------------------------------------
# story_visibility
# ---------------------------------------------------------------------------


def story(author=A, audience="friends", expires_at=NOW) -> dict:
    return {"author_id": author, "audience": audience, "expires_at": expires_at}


def rail_order(groups: list[dict]) -> list[int]:
    return [group["index"] for group in groups]


def order_case(name: str, groups: list[dict]) -> dict:
    indexed = [{**group, "index": index} for index, group in enumerate(groups)]
    return case(
        "order_authors",
        name,
        {"groups": groups},
        call={"groups": indexed},
        shape=rail_order,
    )


def rail(mine: bool, all_seen: bool, latest_at: datetime) -> dict:
    return {"mine": mine, "all_seen": all_seen, "latest_at": latest_at}


def story_constants() -> dict:
    return {
        "ttl_microseconds": enc(story_visibility.STORY_TTL // MICRO),
        "audiences": list(story_visibility.STORY_AUDIENCES),
        "default_audience": story_visibility.DEFAULT_STORY_AUDIENCE,
        "max_caption_length": story_visibility.MAX_CAPTION_LENGTH,
        "names": public_names(story_visibility),
    }


def story_edges() -> list[dict]:
    out: list[dict] = []

    def add(fn: str, name: str, **kwargs: object) -> None:
        out.append(case(fn, name, kwargs))

    vietnam = zone(timedelta(hours=7))
    honolulu = zone(timedelta(hours=-10))
    for i, created in enumerate(
        (
            NOW,
            NOW.astimezone(vietnam),
            NOW.astimezone(honolulu),
            NOW.astimezone(zone(timedelta(hours=5, minutes=30, seconds=15))),
            datetime(9999, 12, 30, 23, 59, 59, 999999, tzinfo=UTC),
            datetime(9999, 12, 31, tzinfo=UTC),
            datetime(9999, 12, 31, tzinfo=honolulu),
            datetime(9999, 12, 30, 23, tzinfo=vietnam),
            datetime(1, 1, 1, tzinfo=UTC),
            datetime(2024, 2, 28, 12, tzinfo=UTC),
            datetime(2023, 2, 28, 23, 59, 59, tzinfo=UTC),
        )
    ):
        add("expires_at_for", f"expires_at_for/{i}", created_at=created)

    deltas = (
        -MICRO,
        timedelta(0),
        MICRO,
        timedelta(seconds=-1),
        timedelta(seconds=1),
        timedelta(hours=24),
    )
    for d, delta in enumerate(deltas):
        for n, now in enumerate((NOW, NOW.astimezone(vietnam))):
            expires = (NOW + delta).astimezone(honolulu if d % 2 else UTC)
            add("is_live", f"is_live/{d}/{n}", story=story(expires_at=expires), now=now)

    captions = (
        None,
        "",
        "x" * 200,
        "x" * 201,
        GRINNING_FACE * 200,
        GRINNING_FACE * 201,
        ("e" + COMBINING_ACUTE) * 100,
        ("e" + COMBINING_ACUTE) * 100 + "e",
        CJK_MIDDLE * 201,
        E_DOT_CIRCUMFLEX * 200,
        "\x00" * 201,
        " " * 201,
    )
    for i, caption in enumerate(captions):
        add("check_caption", f"check_caption/{i}", caption=caption)

    for a, audience in enumerate(("friends", "Friends", "", "public", "friends ")):
        for reader_label, reader in (("author", A), ("other", B)):
            for is_friend in (False, True):
                for is_blocked in (False, True):
                    for e, delta in enumerate((MICRO, timedelta(0), -MICRO)):
                        add(
                            "can_view",
                            f"can_view/{a}/{reader_label}/"
                            f"{int(is_friend)}{int(is_blocked)}/{e}",
                            story=story(audience=audience, expires_at=NOW + delta),
                            reader_id=reader,
                            is_friend=is_friend,
                            is_blocked=is_blocked,
                            now=NOW,
                        )

    hour = timedelta(hours=1)
    out.append(order_case("order/empty", []))
    out.append(order_case("order/one", [rail(False, True, NOW)]))
    out.append(
        order_case(
            "order/mine_then_unseen_then_seen_newest_first",
            [
                rail(False, True, NOW - hour),
                rail(False, False, NOW - 3 * hour),
                rail(True, True, NOW - 5 * hour),
                rail(False, False, NOW - hour),
                rail(False, True, NOW),
            ],
        )
    )
    out.append(
        order_case(
            "order/ties_keep_the_callers_order",
            [rail(False, False, NOW), rail(False, False, NOW), rail(False, False, NOW)],
        )
    )
    out.append(
        order_case(
            "order/same_instant_different_offsets_tie",
            [
                rail(False, True, NOW.astimezone(vietnam)),
                rail(False, True, NOW),
                rail(False, True, NOW.astimezone(honolulu)),
            ],
        )
    )
    far = datetime(9999, 12, 31, 23, 59, 59, 999990, tzinfo=UTC)
    out.append(
        order_case(
            "order/far_future_microseconds_collide_as_floats",
            [rail(False, False, far + MICRO * k) for k in (0, 3, 1, 9, 5, 2, 8)],
        )
    )
    out.append(
        order_case(
            "order/year_one",
            [
                rail(False, False, datetime(1, 1, 1, tzinfo=UTC) + MICRO * k)
                for k in (0, 7, 3)
            ],
        )
    )
    out.append(
        order_case(
            "order/two_mine",
            [rail(True, False, NOW), rail(True, True, NOW + hour)],
        )
    )
    return out


def story_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 4)
    count = 800

    def audience() -> str:
        roll = rng.random()
        if roll < 0.85:
            return "friends"
        return rng.choice(("Friends", "", "public", random_text(rng)))

    def caption() -> str | None:
        if rng.random() < 0.1:
            return None
        unit = rng.choice(("x", E_DOT_CIRCUMFLEX, "e" + COMBINING_ACUTE, GRINNING_FACE))
        size = rng.choice((0, 1, rng.randrange(150, 250), 199, 200, 201, 202))
        return (unit * size)[:size] if len(unit) == 1 else unit * (size // 2)

    def near(moment: datetime, rezone: bool = True) -> datetime:
        shift = timedelta(microseconds=rng.choice((-1, 0, 1, rng.randrange(-9, 10))))
        if not rezone:
            return moment + shift
        return (moment + shift).astimezone(zone(rng.choice(OFFSETS)))

    def far(hours: int = 0) -> datetime:
        """Wall time at the end of year 9999 in some offset, never re-zoned: the
        same instant in a later offset is a year Python cannot hold."""
        at = datetime(
            9999, 12, 31, 23, 59, 59, 999000, tzinfo=zone(rng.choice(OFFSETS))
        )
        return at - timedelta(hours=hours)

    out: list[dict] = []
    for i in range(count):
        created = random_moment(rng)
        if rng.random() < 0.1:
            created = far(rng.randrange(0, 30))
        out.append(case("expires_at_for", f"fuzz/{i}", {"created_at": created}))
    for i in range(count):
        now = random_moment(rng)
        args = {"story": story(expires_at=near(now)), "now": now}
        out.append(case("is_live", f"fuzz/{i}", args))
    for i in range(count):
        out.append(case("check_caption", f"fuzz/{i}", {"caption": caption()}))
    for i in range(count):
        now = random_moment(rng)
        args = {
            "story": story(
                author=rng.choice((A, B, "")),
                audience=audience(),
                expires_at=near(now),
            ),
            "reader_id": rng.choice((A, A, B, "")),
            "is_friend": rng.random() < 0.5,
            "is_blocked": rng.random() < 0.3,
            "now": now,
        }
        out.append(case("can_view", f"fuzz/{i}", args))
    for i in range(count):
        base, rezone = random_moment(rng), True
        if rng.random() < 0.2:
            base, rezone = far(), False
        groups = [
            rail(
                rng.random() < 0.15,
                rng.random() < 0.5,
                near(
                    base + timedelta(microseconds=rng.choice((0, 0, 1, 2, 17))),
                    rezone,
                ),
            )
            for _ in range(rng.randrange(9))
        ]
        out.append(order_case(f"fuzz/{i}", groups))
    return out


# ---------------------------------------------------------------------------
# post_audience
# ---------------------------------------------------------------------------

AUDIENCES = post_audience.AUDIENCES
POLICIES = post_audience.COMMENT_POLICIES


def post(author=A, audience="friends", context=None) -> dict:
    return {"author_id": author, "audience": audience, "context_id": context}


def post_constants() -> dict:
    return {
        "audiences": list(AUDIENCES),
        "default_audience": post_audience.DEFAULT_AUDIENCE,
        "comment_policies": list(POLICIES),
        "default_comment_policy": post_audience.DEFAULT_COMMENT_POLICY,
        "names": public_names(post_audience),
    }


def post_edges() -> list[dict]:
    out: list[dict] = []

    def add(fn: str, name: str, **kwargs: object) -> None:
        out.append(case(fn, name, kwargs))

    near = ("Public", "", "public ", "everyone", "only-me")
    for a, audience in enumerate(AUDIENCES + near):
        add("needs_context", f"needs_context/{a}", audience=audience)
        for c, context in enumerate((None, "ctx-1", "")):
            add(
                "check_writable",
                f"check_writable/{a}/{c}",
                audience=audience,
                context_id=context,
            )
    for p, policy in enumerate(POLICIES + ("Readers", "", "everyone", "friends ")):
        add("is_comment_policy", f"is_comment_policy/{p}", value=policy)

    for a, audience in enumerate(AUDIENCES + ("Public", "")):
        for c, context in enumerate((None, "ctx-1")):
            row = post(audience=audience, context=context)
            for r, reader in enumerate((A, B, "")):
                for is_friend in (False, True):
                    for member in (False, True):
                        tag = f"{a}/{c}/{r}/{int(is_friend)}{int(member)}"
                        facts = {
                            "reader_id": reader,
                            "is_friend": is_friend,
                            "is_group_member": member,
                        }
                        add("can_read", f"can_read/{tag}", post=row, **facts)
                        for blocked in (False, True):
                            add(
                                "visible_to",
                                f"visible_to/{tag}/{int(blocked)}",
                                post=row,
                                is_blocked=blocked,
                                **facts,
                            )
                        for p, policy in enumerate(POLICIES + ("", "Readers")):
                            add(
                                "can_comment",
                                f"can_comment/{tag}/{p}",
                                post=row,
                                policy=policy,
                                **facts,
                            )
    for w, writer in enumerate((A, B, C, "")):
        for o, owner in enumerate((A, B)):
            for t, actor in enumerate((A, B, C, "")):
                add(
                    "can_delete_comment",
                    f"can_delete_comment/{w}/{o}/{t}",
                    comment={"author_id": writer},
                    post=post(author=owner),
                    actor_id=actor,
                )
    return out


def post_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 5)
    count = 600
    people = (A, B, C, "", A.upper())

    def audience() -> str:
        roll = rng.random()
        if roll < 0.85:
            return rng.choice(AUDIENCES)
        return rng.choice(("Public", "", "group ", random_text(rng)))

    def some_post() -> dict:
        return post(
            author=rng.choice(people),
            audience=audience(),
            context=rng.choice((None, None, "ctx-1", "", random_text(rng))),
        )

    def facts() -> dict:
        return {
            "reader_id": rng.choice(people),
            "is_friend": rng.random() < 0.5,
            "is_group_member": rng.random() < 0.5,
        }

    def policy() -> str:
        return rng.choice(POLICIES) if rng.random() < 0.85 else random_text(rng)

    out: list[dict] = []
    for i in range(count):
        out.append(case("needs_context", f"fuzz/{i}", {"audience": audience()}))
    for i in range(count):
        context = rng.choice((None, "ctx-1", "", random_text(rng)))
        args = {"audience": audience(), "context_id": context}
        out.append(case("check_writable", f"fuzz/{i}", args))
    for i in range(count):
        out.append(case("is_comment_policy", f"fuzz/{i}", {"value": policy()}))
    for i in range(count):
        out.append(case("can_read", f"fuzz/{i}", {"post": some_post(), **facts()}))
    for i in range(count):
        args = {"post": some_post(), "is_blocked": rng.random() < 0.4, **facts()}
        out.append(case("visible_to", f"fuzz/{i}", args))
    for i in range(count):
        args = {"post": some_post(), "policy": policy(), **facts()}
        out.append(case("can_comment", f"fuzz/{i}", args))
    for i in range(count):
        args = {
            "comment": {"author_id": rng.choice(people)},
            "post": some_post(),
            "actor_id": rng.choice(people),
        }
        out.append(case("can_delete_comment", f"fuzz/{i}", args))
    return out


# ---------------------------------------------------------------------------
# vote
# ---------------------------------------------------------------------------


def tally_shape(result: dict) -> dict:
    return {
        **result,
        "counts": [[key, value] for key, value in result["counts"].items()],
    }


def tally_case(name: str, options: list, ballots: list) -> dict:
    return case(
        "tally",
        name,
        {"options": options, "ballots": ballots},
        shape=tally_shape,
    )


def options_of(*ids: str) -> list[dict]:
    return [{"id": option, "position": index} for index, option in enumerate(ids)]


def ballots_of(*pairs: tuple[str, str]) -> list[dict]:
    return [{"voter_id": voter, "option_id": option} for voter, option in pairs]


def vote_constants() -> dict:
    return {"names": public_names(vote)}


def vote_edges() -> list[dict]:
    three = options_of("pizza", "bep-me-in", "som-tum")
    out = [
        tally_case(
            "test_the_spec_example_reports_a_clear_winner",
            three,
            ballots_of(
                ("an", "pizza"),
                ("binh", "pizza"),
                ("chi", "pizza"),
                ("dung", "pizza"),
                ("em", "bep-me-in"),
                ("phuc", "bep-me-in"),
                ("giang", "som-tum"),
            ),
        ),
        tally_case(
            "test_an_option_nobody_chose_still_appears_with_zero",
            options_of("pizza", "bun-cha", "som-tum"),
            ballots_of(("an", "pizza")),
        ),
        tally_case(
            "test_a_level_result_is_reported_as_a_tie_and_decides_nothing",
            options_of("pizza", "bun-cha"),
            ballots_of(
                ("an", "pizza"),
                ("binh", "bun-cha"),
                ("chi", "pizza"),
                ("dung", "bun-cha"),
            ),
        ),
        tally_case(
            "test_a_three_way_tie_lists_every_leader_in_order",
            options_of("pizza", "bun-cha", "som-tum"),
            ballots_of(("an", "som-tum"), ("binh", "bun-cha"), ("chi", "pizza")),
        ),
        tally_case("no_ballots", options_of("pizza", "bun-cha"), []),
        tally_case("single_option_no_ballots", options_of("pizza"), []),
        tally_case(
            "duplicate_ballot",
            options_of("pizza", "bun-cha"),
            ballots_of(("an", "pizza"), ("an", "bun-cha")),
        ),
        tally_case(
            "unknown_option",
            options_of("pizza"),
            ballots_of(("an", "somewhere-else")),
        ),
        tally_case("no_options", [], []),
        tally_case("no_options_with_ballots", [], ballots_of(("an", "x"))),
        tally_case(
            "duplicate_position",
            [{"id": "a", "position": 1}, {"id": "b", "position": 1}],
            [],
        ),
        tally_case(
            "duplicate_position_is_checked_before_ballots",
            [{"id": "a", "position": 1}, {"id": "b", "position": 1}],
            ballots_of(("an", "zzz")),
        ),
        tally_case(
            "positions_sort_the_options",
            [
                {"id": "c", "position": 30},
                {"id": "a", "position": -4},
                {"id": "b", "position": 7},
            ],
            ballots_of(("an", "b"), ("binh", "c")),
        ),
        tally_case(
            "one_id_at_two_positions_counts_once_and_leads_twice",
            [{"id": "a", "position": 1}, {"id": "a", "position": 2}],
            ballots_of(("an", "a")),
        ),
        tally_case(
            "one_id_at_two_positions_with_no_ballots",
            [{"id": "a", "position": 1}, {"id": "a", "position": 2}],
            [],
        ),
        tally_case(
            "duplicate_voter_wins_over_a_later_unknown_option",
            options_of("a"),
            ballots_of(("an", "a"), ("an", "zzz")),
        ),
        tally_case(
            "unknown_option_wins_over_a_later_duplicate_voter",
            options_of("a"),
            ballots_of(("an", "zzz"), ("an", "a")),
        ),
        tally_case(
            "empty_ids",
            options_of("", "x"),
            ballots_of(("", ""), ("x", "x"), ("y", "")),
        ),
        tally_case(
            "uuid_ids",
            options_of(A, B),
            ballots_of((C, B), (A, B), (B, A)),
        ),
    ]
    return out


def vote_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 6)
    ids = ("o1", "o2", "o3", "o4", "o5", "o6", "", A)
    voters = tuple(f"v{index}" for index in range(14))
    out: list[dict] = []
    for i in range(2400):
        size = rng.choice((0, 1, 2, 2, 3, 3, 4, 5, 6))
        chosen = rng.sample(ids, size)
        positions = rng.sample(range(-3, 12), size)
        if size >= 2 and rng.random() < 0.05:
            positions[rng.randrange(size)] = positions[0]
        if size >= 2 and rng.random() < 0.05:
            chosen[rng.randrange(size)] = chosen[0]
        options = [
            {"id": option, "position": position}
            for option, position in zip(chosen, positions, strict=True)
        ]
        people = rng.sample(voters, rng.randrange(len(voters)))
        ballots = []
        for voter in people:
            if chosen and rng.random() < 0.97:
                option = rng.choice(chosen)
            else:
                option = rng.choice(ids + ("unknown",))
            ballots.append({"voter_id": voter, "option_id": option})
        if ballots and rng.random() < 0.03:
            ballots.append(dict(rng.choice(ballots)))
        out.append(tally_case(f"fuzz/{i}", options, ballots))
    return out


# ---------------------------------------------------------------------------
# cursors
# ---------------------------------------------------------------------------


def decoded_shape(value: tuple) -> dict:
    created_at, message_id = value
    offset = created_at.utcoffset() // MICRO
    return {
        "created_at": [
            created_at.year,
            created_at.month,
            created_at.day,
            created_at.hour,
            created_at.minute,
            created_at.second,
            created_at.microsecond,
            offset // 10**6,
            offset % 10**6,
        ],
        "message_id": str(message_id),
        "reencoded": cursors.encode_cursor(created_at, message_id),
        "utc_microseconds": (created_at - EPOCH) // MICRO,
    }


def decode_case(name: str, raw: str) -> dict:
    return case("decode_cursor", name, {"raw": raw}, shape=decoded_shape)


def encode_case(name: str, created_at: datetime, message_id: uuid.UUID) -> dict:
    return case(
        "encode_cursor",
        name,
        {"created_at": created_at, "message_id": message_id},
    )


def b64(text: str | bytes) -> str:
    raw = text.encode("utf-8") if isinstance(text, str) else text
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def cursors_constants() -> dict:
    return {
        "isspace": every_code_point(str.isspace),
        "decimal": decimal_ranges(),
        "names": public_names(cursors),
    }


GOOD_ID = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
HEX_ID = GOOD_ID.replace("-", "")
TIMESTAMPS = (
    "2026-08-29T12:30:45+00:00",
    "2026-08-29T12:30:45.123456+00:00",
    "2026-08-29T12:30:45",
    "2026-08-29",
    "20260829",
    "2026-W35",
    "2026W35",
    "2026-W35-6",
    "2026W356",
    "2026W35T12",
    "2026W356T12",
    "2026W3561230",
    "2026-W35-612:30",
    "2026-W35-6T12:30",
    "2026-W35-",
    "2026-W3",
    "2020-W53",
    "2026-W53",
    "2026-W00",
    "2026-W35-0",
    "2026-W35-8",
    "9999-W52-5",
    "9999-W52-6",
    "0001-W01-1",
    "0000-W01-1",
    "0000-01-01",
    "0001-01-01T00:00:00+01:00",
    "9999-12-31T23:59:59.999999" + "-10:00",
    "2026-02-29",
    "2024-02-29",
    "2026-13-01",
    "2026-00-10",
    "2026-01-32",
    "2026-08-29" + " 12:30",
    "2026-08-29t12:30",
    "2026-08-29x12:30",
    "2026-08-29" + E_ACUTE + "12:30",
    "2026-08-29" + CJK_MIDDLE + "12:30",
    "2026-08-29" + GRINNING_FACE + "12:30",
    "2026-08-29" + "12:30",
    "2026-08-29T12",
    "2026-08-29T1230",
    "2026-08-29T123045",
    "2026-08-29T12:3045",
    "2026-08-29T1230:45",
    "2026-08-29T12:30:45,5",
    "2026-08-29T12:30:45.",
    "2026-08-29T12:30:45." + "1234567",
    "2026-08-29T12:30:45.1234567x",
    "2026-08-29T12:30:45.12x",
    "2026-08-29T24:00",
    "2026-08-29T23:60",
    "2026-08-29T23:59:60",
    "2026-08-29T12:30Z",
    "2026-08-29T12:30z",
    "2026-08-29T12:30Z ",
    "2026-08-29T12:30Z\x00junk",
    "2026-08-29T12:30+07",
    "2026-08-29T12:30+0700",
    "2026-08-29T12:30+07:00:30",
    "2026-08-29T12:30+07:00:30.5",
    "2026-08-29T12:30+00:00:00.5",
    "2026-08-29T12:30-00:00:01.5",
    "2026-08-29T12:30+24:00",
    "2026-08-29T12:30-24:00",
    "2026-08-29T12:30+23:59:59.999999",
    "2026-08-29T12:30-23:59:59.999999",
    "2026-08-29T12:30+00:99",
    "2026-08-29T12:30+7",
    "2026-08-29T12:30X+01:00",
    "2026-08-29T1230X+01:00",
    "2026-08-29T",
    "2026-08-29T+01:00",
    "2026-08-29TZ",
    "2026-08",
    "2026-8-29",
    "26-08-29",
    "2026/08/29",
    "short",
    "",
    "2026-08-29T12:30:45.123456\x00",
    "2026-08-29T12:30\x00",
    "2026-08-29\x0012:30",
    "2026-08-29T12:30:45+00:00 ",
    " 2026-08-29T12:30:45+00:00",
    "2026-08-29T12:30:45+00:00\n",
    "2026-001",
    "2026001",
    "2026-08-29T12:30:45.123456+05:30:15.000001",
    "2026-08-29T12-30",
    "2026-08-29T12+30",
    "2026-08-29T12:30:45.5-",
)
UUID_TEXTS = (
    GOOD_ID,
    GOOD_ID.upper(),
    HEX_ID,
    "{" + GOOD_ID + "}",
    "{{" + HEX_ID + "}}",
    "urn:uuid:" + GOOD_ID,
    "uuid:" + GOOD_ID,
    "uurn:uid:" + GOOD_ID,
    "not-a-uuid",
    "",
    " " + HEX_ID[1:],
    HEX_ID[1:] + " ",
    "\t" + HEX_ID[1:],
    NO_BREAK_SPACE + HEX_ID[1:],
    NEXT_LINE + HEX_ID[1:],
    IDEOGRAPHIC_SPACE + HEX_ID[1:],
    "\x1c" + HEX_ID[1:],
    "0x" + HEX_ID[2:],
    "0X_" + HEX_ID[3:],
    "0x__" + HEX_ID[4:],
    "+" + HEX_ID[1:],
    "-" + HEX_ID[1:],
    "-0" + "0" * 30,
    "+0x" + HEX_ID[3:],
    " -0x" + "0" * 28,
    HEX_ID[:10] + "_" + HEX_ID[11:],
    HEX_ID[:10] + "__" + HEX_ID[12:],
    "_" + HEX_ID[1:],
    HEX_ID[:31] + "_",
    HEX_ID[:31],
    HEX_ID + "0",
    HEX_ID[:31] + "g",
    ARABIC_INDIC[1] + HEX_ID[1:],
    FULLWIDTH[9] * 32,
    MATH_BOLD[7] * 31 + "a",
    SUPERSCRIPT_TWO + HEX_ID[1:],
    ROMAN_NUMERAL_ONE + HEX_ID[1:],
    FULLWIDTH_CAPITAL_A + HEX_ID[1:],
    HEX_ID[:16] + "\x00" + HEX_ID[17:],
    HEX_ID[:31] + "\x7f",
    "{" + HEX_ID + "}}}",
    "urn:" + HEX_ID,
    GOOD_ID + "-",
    "-" * 7 + HEX_ID,
    "ffffffff-ffff-ffff-ffff-ffffffffffff",
    str(uuid.UUID(int=0)),
)


def cursors_edges() -> list[dict]:
    out: list[dict] = []
    ids = (
        uuid.UUID(GOOD_ID),
        uuid.UUID(int=0),
        uuid.UUID(int=2**128 - 1),
    )
    moments = (
        NOW,
        NOW.replace(microsecond=0),
        NOW.astimezone(zone(timedelta(hours=7))),
        NOW.astimezone(zone(timedelta(hours=-10))),
        NOW.astimezone(zone(timedelta(hours=5, minutes=30, seconds=15))),
        NOW.astimezone(zone(timedelta(seconds=-1))),
        datetime(1, 1, 1, tzinfo=UTC),
        datetime(9999, 12, 31, 23, 59, 59, 999999, tzinfo=UTC),
        datetime(999, 1, 2, 3, 4, 5, 6, tzinfo=UTC),
    ) + tuple(NOW.astimezone(zone(offset)) for offset in MICRO_OFFSETS)
    for m, moment in enumerate(moments):
        for i, message_id in enumerate(ids):
            out.append(encode_case(f"encode/{m}/{i}", moment, message_id))

    for index, raw in enumerate(
        ("", "not-a-cursor", "!!!!", "YWJj", "eyJhIjoxfQ", "=", "==", "A", "AB")
    ):
        out.append(decode_case(f"decode/garbage/{index}", raw))
    out.append(
        decode_case(
            "decode/unparseable_id", b64("2026-08-29T12:30:45+00:00|not-a-uuid")
        )
    )
    for m, moment in enumerate(moments):
        raw = cursors.encode_cursor(moment, ids[m % len(ids)])
        out.append(decode_case(f"decode/round_trip/{m}", raw))
    for t, stamp in enumerate(TIMESTAMPS):
        out.append(decode_case(f"decode/timestamp/{t}", b64(stamp + "|" + GOOD_ID)))
    for u, text in enumerate(UUID_TEXTS):
        payload = "2026-08-29T12:30:45+00:00|" + text
        out.append(decode_case(f"decode/uuid/{u}", b64(payload)))

    good = "2026-08-29T12:30:45.5+07:00|" + GOOD_ID
    standard = base64.b64encode(good.encode()).decode("ascii")
    for label, raw in (
        ("padding_kept", base64.urlsafe_b64encode(good.encode()).decode("ascii")),
        ("standard_alphabet", standard.rstrip("=")),
        ("standard_alphabet_padded", standard),
        ("extra_padding", b64(good) + "===="),
        ("leading_padding", "=" + b64(good)),
        ("space_inside", b64(good)[:8] + " " + b64(good)[8:]),
        ("newline_at_end", b64(good) + "\n"),
        ("non_ascii", b64(good)[:5] + E_ACUTE + b64(good)[6:]),
        ("one_char_over_a_quad", b64(good) + "A"),
        ("two_separators", b64(good + "|" + GOOD_ID)),
        ("no_separator", b64(good.replace("|", ""))),
        ("separator_first", b64("|" + GOOD_ID)),
        ("invalid_utf8", b64(b"2026-08-29T12:30:45+00:00|\xff" + HEX_ID.encode())),
        ("surrogate_bytes", b64(b"2026-08-29T12:30\xed\xa0\x80|" + GOOD_ID.encode())),
        ("padding_mid_quad", b64("ab") + "=" + b64("cd")),
        ("double_quad_padding", b64("a") + "==" + b64("b") + "=="),
        ("nonzero_trailing_bits", b64(good)[:-1] + "B"),
    ):
        out.append(decode_case(f"decode/base64/{label}", raw))
    whole = b64("2026-08-29T12:30:45.125+00:00|" + GOOD_ID)
    if len(whole) % 4:
        raise AssertionError("this payload must end on a complete quad")
    for label, raw in (
        ("whole_quads", whole),
        ("whole_quads_then_one_pad", whole + "="),
        ("whole_quads_then_two_pads", whole + "=="),
        ("whole_quads_then_three_pads", whole + "==="),
        ("whole_quads_then_four_pads", whole + "===="),
        ("whole_quads_then_pad_and_data", whole + "=AAA"),
        ("pad_between_whole_quads", whole[:8] + "=" + whole[8:]),
    ):
        out.append(decode_case(f"decode/base64/{label}", raw))
    return out


def iso_date(rng: random.Random) -> str:
    year = rng.choice(
        (rng.randrange(1, 10000), rng.randrange(1, 10000), 0, 1, 2020, 9999)
    )
    y = f"{year:04d}"
    if rng.random() < 0.03:
        y = y[:3] if rng.random() < 0.5 else y + rng.choice(string.digits)
    month = rng.choice((rng.randrange(1, 13), rng.randrange(1, 13), 0, 2, 12, 13))
    day = rng.choice((rng.randrange(1, 29), rng.randrange(1, 29), 0, 29, 30, 31, 32))
    week = rng.choice((rng.randrange(1, 53), rng.randrange(1, 53), 0, 52, 53, 54))
    weekday = rng.choice((rng.randrange(1, 8), rng.randrange(1, 8), 0, 8, 9))
    shape = rng.randrange(10)
    if shape < 4:
        return f"{y}-{month:02d}-{day:02d}"
    if shape == 4:
        return f"{y}{month:02d}{day:02d}"
    if shape == 5:
        return f"{y}-W{week:02d}"
    if shape == 6:
        return f"{y}W{week:02d}"
    if shape == 7:
        return f"{y}-W{week:02d}-{weekday}"
    if shape == 8:
        return f"{y}W{week:02d}{weekday}"
    return f"{y}-{rng.randrange(400):03d}"


def iso_time(rng: random.Random) -> str:
    hour = rng.choice((rng.randrange(24), rng.randrange(24), 0, 23, 24, 99))
    minute = rng.choice((rng.randrange(60), rng.randrange(60), 59, 60))
    second = rng.choice((rng.randrange(60), rng.randrange(60), 59, 60))
    size = rng.choice((1, 2, 3, 3, 3))
    parts = [f"{hour:02d}", f"{minute:02d}", f"{second:02d}"][:size]
    text = (":" if rng.random() < 0.75 else "").join(parts)
    if size == 3 and rng.random() < 0.06:
        text = rng.choice(
            (
                f"{hour:02d}:{minute:02d}{second:02d}",
                f"{hour:02d}{minute:02d}:{second:02d}",
            )
        )
    if rng.random() < 0.4:
        count = rng.choice((0, 1, 2, 3, 6, 6, 7, 9))
        digits = "".join(rng.choice(string.digits) for _ in range(count))
        text += rng.choice((".", ".", ",")) + digits
    return text


def iso_zone(rng: random.Random) -> str:
    roll = rng.random()
    if roll < 0.3:
        return ""
    if roll < 0.4:
        return rng.choice(("Z", "Z", "z", "Z0", "UTC"))
    sign = rng.choice("+-")
    hours = rng.choice((rng.randrange(24), rng.randrange(24), 0, 23, 24, 99))
    minutes = rng.choice((rng.randrange(60), 0, 30, 59, 60, 99))
    seconds = rng.choice((rng.randrange(60), 0, 59))
    shape = rng.randrange(8)
    if shape == 0:
        return f"{sign}{hours:02d}"
    if shape in (1, 2):
        return f"{sign}{hours:02d}:{minutes:02d}"
    if shape == 3:
        return f"{sign}{hours:02d}{minutes:02d}"
    if shape == 4:
        return f"{sign}{hours:02d}:{minutes:02d}:{seconds:02d}"
    if shape == 5:
        count = rng.choice((1, 3, 6, 7))
        digits = "".join(rng.choice(string.digits) for _ in range(count))
        return f"{sign}{hours:02d}:{minutes:02d}:{seconds:02d}.{digits}"
    if shape == 6:
        return f"{sign}{rng.randrange(10)}"
    return f"{sign}{hours:02d}{minutes:02d}{seconds:02d}"


SEPARATORS = (
    "T",
    "T",
    "T",
    " ",
    "t",
    "_",
    "x",
    "0",
    "-",
    "W",
    E_ACUTE,
    CJK_MIDDLE,
    GRINNING_FACE,
    "\x00",
)
TIME_MUTATIONS = tuple(string.digits) + (
    ":",
    ".",
    ",",
    "-",
    "+",
    "Z",
    "W",
    "T",
    " ",
    "\x00",
    "|",
    E_ACUTE,
    GRINNING_FACE,
)
UUID_MUTATIONS = tuple(string.hexdigits) + (
    "g",
    "-",
    "_",
    " ",
    "{",
    "}",
    "x",
    "+",
    ":",
    NO_BREAK_SPACE,
    E_ACUTE,
    ARABIC_INDIC[3],
    SUPERSCRIPT_TWO,
    "\x00",
    "\x1f",
)
BASE64_MUTATIONS = tuple(string.ascii_letters + string.digits) + (
    "-",
    "_",
    "+",
    "/",
    "=",
    "==",
    " ",
    "\n",
    "\t",
    ".",
    "!",
    E_ACUTE,
    "\x00",
)


def mutate(rng: random.Random, text: str, pool: tuple) -> str:
    if not text:
        return rng.choice(pool)
    index = rng.randrange(len(text) + 1)
    kind = rng.randrange(3)
    if kind == 0:
        return text[:index] + rng.choice(pool) + text[index:]
    if kind == 1:
        return text[:index] + text[index + 1 :]
    return text[:index] + rng.choice(pool) + text[index + 1 :]


def timestamp_text(rng: random.Random) -> str:
    if rng.random() < 0.2:
        moment = random_moment(rng, micro_offsets=True)
        spec = rng.choice(
            (
                "auto",
                "auto",
                "microseconds",
                "seconds",
                "minutes",
                "hours",
                "milliseconds",
            )
        )
        text = moment.isoformat(timespec=spec)
    else:
        text = iso_date(rng)
        if rng.random() < 0.85:
            text += rng.choice(SEPARATORS) + iso_time(rng) + iso_zone(rng)
    for _ in range(rng.choice((0, 0, 0, 1, 2))):
        text = mutate(rng, text, TIME_MUTATIONS)
    return text


def uuid_text(rng: random.Random) -> str:
    value = uuid.UUID(int=rng.getrandbits(128))
    canonical, plain = str(value), value.hex
    shape = rng.randrange(16)
    if shape < 5:
        text = canonical
    elif shape == 5:
        text = canonical.upper()
    elif shape == 6:
        text = plain
    elif shape == 7:
        text = "{" + canonical + "}"
    elif shape == 8:
        text = "urn:uuid:" + canonical
    elif shape == 9:
        text = rng.choice(("{{", "{", "}", "")) + plain + rng.choice(("}}", "", "{"))
    elif shape == 10:
        prefix = rng.choice(
            ("0x", "0X", "+", "-", "-0", "+0x", " ", " 0x_", "\t", "0x_")
        )
        text = prefix + plain[len(prefix) :]
    elif shape == 11:
        at = rng.randrange(1, 31)
        text = plain[:at] + rng.choice(("_", "__")) + plain[at + 1 :]
    elif shape == 12:
        digits = rng.choice(DIGIT_SETS)
        text = "".join(
            digits[int(char)] if char.isdigit() and rng.random() < 0.5 else char
            for char in rng.choice((canonical, plain))
        )
    elif shape == 13:
        pad = rng.choice(WHITESPACE + ("\x1c", "\x00", ZERO_WIDTH_SPACE))
        text = pad + plain[1:] if rng.random() < 0.5 else plain[:-1] + pad
    elif shape == 14:
        text = rng.choice(
            (
                plain[:10] + "urn:" + plain[10:],
                "uuid:" + canonical,
                "uurn:uid:" + canonical,
                "urn:uuid:urn:uuid:" + canonical,
                "urn:uuid:{" + canonical + "}",
            )
        )
    else:
        text = rng.choice(
            (
                plain[:31],
                plain + "0",
                canonical + "-",
                "-" + canonical,
                canonical.replace("-", "", 2),
                canonical.replace("-", "--"),
            )
        )
    for _ in range(rng.choice((0, 0, 0, 0, 1))):
        text = mutate(rng, text, UUID_MUTATIONS)
    return text


def raw_garbage(rng: random.Random) -> str:
    roll = rng.random()
    if roll < 0.3:
        return rng.choice(("", "=", "==", "====", "A", "AB", "ABC", "ABCD", "YWJj"))
    alphabet = string.ascii_letters + string.digits + "-_+/="
    size = rng.randrange(16)
    if roll < 0.8:
        return "".join(rng.choice(alphabet) for _ in range(size))
    return random_text(rng, 12)


def cursor_text(rng: random.Random) -> str:
    roll = rng.random()
    if roll < 0.06:
        return raw_garbage(rng)
    if roll < 0.14:
        moment = random_moment(rng, micro_offsets=True)
        return cursors.encode_cursor(moment, uuid.UUID(int=rng.getrandbits(128)))
    stamp = timestamp_text(rng)
    ident = uuid_text(rng) if rng.random() < 0.9 else random_text(rng)
    separator = "|" if rng.random() < 0.93 else rng.choice(("", "||", ":", " | "))
    payload = (stamp + separator + ident).encode("utf-8")
    if rng.random() < 0.04:
        at = rng.randrange(len(payload) + 1)
        broken = rng.choice(
            (b"\xff", b"\xc0\xaf", b"\xed\xa0\x80", b"\xe2\x82", b"\x80", b"\xf4\x90")
        )
        payload = payload[:at] + broken + payload[at:]
    style = rng.random()
    if style < 0.75:
        return b64(payload)
    if style < 0.83:
        return base64.urlsafe_b64encode(payload).decode("ascii")
    if style < 0.88:
        return base64.b64encode(payload).decode("ascii").rstrip("=")
    text = b64(payload)
    for _ in range(rng.choice((1, 1, 2))):
        text = mutate(rng, text, BASE64_MUTATIONS)
    return text


def cursors_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 7)
    out: list[dict] = []
    for i in range(900):
        moment = random_moment(rng, micro_offsets=True)
        message_id = uuid.UUID(int=rng.getrandbits(128))
        out.append(encode_case(f"fuzz/{i}", moment, message_id))
    for i in range(12000):
        out.append(decode_case(f"fuzz/{i}", cursor_text(rng)))
    return out


# ---------------------------------------------------------------------------
# person_identity
# ---------------------------------------------------------------------------

KEY = b"k" * 40
PHONE_SEPARATORS = (
    " ",
    ".",
    "-",
    "(",
    ")",
    NO_BREAK_SPACE,
    IDEOGRAPHIC_SPACE,
    "\t",
    "\n",
    "/",
    "_",
    "\x1c",
    ZERO_WIDTH_SPACE,
    "+",
    E_ACUTE,
    NEXT_LINE,
)


def mobile(first: str, rest: str, digits: str = string.digits) -> str:
    """A nine-digit subscriber number: `first` then `rest` read through `digits`."""
    return first + "".join(digits[int(char)] for char in rest)


def identity_constants() -> dict:
    return {
        "key_env_var": person_identity.KEY_ENV_VAR,
        "min_key_length": person_identity.MIN_KEY_LENGTH,
        "domain": enc(person_identity.DOMAIN),
        "otp_phone_domain": enc(person_identity.OTP_PHONE_DOMAIN),
        "otp_code_domain": enc(person_identity.OTP_CODE_DOMAIN),
        "isspace": every_code_point(str.isspace),
        "decimal": decimal_ranges(),
        "re_digit": every_code_point(
            lambda char: re.fullmatch(r"\d", char) is not None
        ),
        "re_space": every_code_point(
            lambda char: re.fullmatch(r"\s", char) is not None
        ),
        "names": public_names(person_identity),
    }


def read_key_case(name: str, env: bytes | None) -> dict:
    environ = {} if env is None else {person_identity.KEY_ENV_VAR: os.fsdecode(env)}
    return case("read_key", name, {"env": env}, call={"environ": environ})


def identity_edges() -> list[dict]:
    out: list[dict] = []

    def add(fn: str, name: str, **kwargs: object) -> None:
        out.append(case(fn, name, kwargs))

    subscriber = mobile("9", "12345678")
    spellings = (
        "0" + subscriber,
        "+84" + subscriber,
        "84" + subscriber,
        subscriber,
        "0" + subscriber[:2] + " " + subscriber[2:5] + " " + subscriber[5:],
        "0" + subscriber[:2] + "." + subscriber[2:5] + "." + subscriber[5:],
        "(+84) " + subscriber[:2] + "-" + subscriber[2:5] + "-" + subscriber[5:],
        "+84 (0)" + subscriber,
        "0084" + subscriber,
        "840" + subscriber,
        "+840" + subscriber,
        "8484" + subscriber,
        "00" + subscriber,
        "0" + mobile("2", "12345678"),
        "0" + mobile("1", "12345678"),
        "0" + mobile("4", "12345678"),
        "0" + mobile("6", "12345678"),
        "0" + subscriber[:-1],
        "0" + subscriber + "0",
        "0" + subscriber[:4] + "x" + subscriber[5:],
        "",
        "   ",
        "+",
        "+84",
        "84",
        "0",
        "()-. ",
        "0" + subscriber[:4] + NO_BREAK_SPACE + subscriber[4:],
        "0" + subscriber[:4] + IDEOGRAPHIC_SPACE + subscriber[4:],
        "0" + subscriber[:4] + ZERO_WIDTH_SPACE + subscriber[4:],
        "0" + subscriber[:4] + "\x1c" + subscriber[4:],
        "0" + subscriber[:4] + "/" + subscriber[4:],
        "0" + subscriber + "\n",
        "\t0" + subscriber,
        "0" + mobile("9", "12345678", ARABIC_INDIC),
        "0" + mobile("9", "12345678", FULLWIDTH),
        "0" + mobile("9", "12345678", MATH_BOLD),
        "0" + mobile("9", "1234567", DEVANAGARI) + "8",
        "0" + ARABIC_INDIC[9] + mobile("1", "2345678"),
        "0" + FULLWIDTH[9] + mobile("1", "2345678", FULLWIDTH),
        "0" + subscriber[:8] + SUPERSCRIPT_TWO,
        "0" + subscriber[:8] + ROMAN_NUMERAL_ONE,
        FULLWIDTH[0] + subscriber,
        "+" + FULLWIDTH[8] + "4" + subscriber,
        "0" + subscriber[:4] + COMBINING_ACUTE + subscriber[4:],
        "0" + subscriber + GRINNING_FACE,
    )
    for index, raw in enumerate(spellings):
        add("canonical_mobile", f"canonical_mobile/{index}", raw=raw)

    canonical = person_identity.canonical_mobile("0" + subscriber)
    keys = (
        KEY,
        b"x" * 40,
        b"k" * 32,
        b"k" * 64,
        b"k" * 65,
        bytes(range(1, 101)),
        "kh" + E_DOT_CIRCUMFLEX * 15 + "a",
        b"",
    )
    for k, key in enumerate(keys):
        key_bytes = key if isinstance(key, bytes) else key.encode("utf-8")
        add(
            "derive_person_id",
            f"derive_person_id/key/{k}",
            canonical=canonical,
            key=key_bytes,
        )
        add(
            "derive_phone_digest",
            f"derive_phone_digest/key/{k}",
            canonical=canonical,
            key=key_bytes,
        )
    for index, raw in enumerate(spellings):
        text = person_identity.canonical_mobile(raw)
        if text is None:
            continue
        add(
            "derive_person_id",
            f"derive_person_id/canonical/{index}",
            canonical=text,
            key=KEY,
        )
        add(
            "derive_phone_digest",
            f"derive_phone_digest/canonical/{index}",
            canonical=text,
            key=KEY,
        )
    for index, text in enumerate(
        (
            "",
            "84",
            "hello",
            "8" + E_ACUTE,
            E_ACUTE + GRINNING_FACE + "a" + ARABIC_INDIC[:3],
            "\x00\x7f",
            GRINNING_FACE,
            "ab" + NO_BREAK_SPACE,
        )
    ):
        add(
            "derive_person_id",
            f"derive_person_id/text/{index}",
            canonical=text,
            key=KEY,
        )
        add(
            "derive_phone_digest",
            f"derive_phone_digest/text/{index}",
            canonical=text,
            key=KEY,
        )

    challenges = (uuid.UUID(GOOD_ID), uuid.UUID(int=0), uuid.UUID(int=2**128 - 1))
    codes = (
        "123456",
        "000000",
        "",
        " 12345",
        "1" * 64,
        "12345" + E_ACUTE,
        FULLWIDTH[1:7],
        GRINNING_FACE + "1" + GRINNING_FACE + GRINNING_FACE,
        "\x00",
    )
    for c, challenge in enumerate(challenges):
        for d, code in enumerate(codes):
            add(
                "derive_code_digest",
                f"derive_code_digest/{c}/{d}",
                challenge_id=challenge,
                code=code,
                key=KEY,
            )

    envs = (
        None,
        b"",
        b"   ",
        b"x" * 31,
        b"x" * 32,
        b"  " + b"x" * 32 + b"\n",
        b" " + b"x" * 31 + b" ",
        NEXT_LINE.encode() + b"x" * 32,
        IDEOGRAPHIC_SPACE.encode() + b"x" * 31,
        b"\x1c" + b"x" * 32 + b"\x1f",
        E_DOT_CIRCUMFLEX.encode() * 16,
        E_DOT_CIRCUMFLEX.encode() * 32,
        b"x" * 31 + b"\xff",
        b"x" * 32 + b"\xff",
        b"\xff" + b"x" * 32,
        b"x" * 16 + b"\xff\xfe" + b"x" * 16,
        b"x" * 16 + b"\xed\xa0\x80" + b"x" * 16,
        b"x" * 16 + b"\xe2\x82" + b"x" * 16,
        b"x" * 32 + b"\xf0\x9f\x98",
        b"x" * 32 + b"\xc0\xaf",
        b"\xff" * 40,
        b"x" * 20 + b"\xc3" + b"x" * 20 + b"\x80",
        "SECRET-DO-NOT-LEAK".encode() + b"y" * 20,
    )
    for index, env in enumerate(envs):
        out.append(read_key_case(f"read_key/{index}", env))
    return out


def phone_text(rng: random.Random) -> str:
    digits = string.digits if rng.random() < 0.85 else rng.choice(DIGIT_SETS[1:])
    first = rng.choice("35789") if rng.random() < 0.8 else rng.choice(string.digits)
    if rng.random() < 0.05:
        first = rng.choice(DIGIT_SETS)[int(first)]
    size = rng.choice((8, 8, 8, 8, 8, 7, 9, 10))
    body = first + "".join(rng.choice(digits) for _ in range(size))
    prefix = rng.choice(
        ("0", "0", "0", "+84", "84", "", "0084", "+840", "+", "8", "00")
    )
    text = prefix + body
    if rng.random() < 0.45:
        for _ in range(rng.randrange(1, 5)):
            at = rng.randrange(len(text) + 1)
            text = text[:at] + rng.choice(PHONE_SEPARATORS) + text[at:]
    if rng.random() < 0.08:
        at = rng.randrange(len(text) + 1)
        text = text[:at] + rng.choice(("x", SUPERSCRIPT_TWO, "|", "#")) + text[at:]
    return text


INVALID_UTF8 = (
    b"\xff",
    b"\xfe",
    b"\x80",
    b"\xc3",
    b"\xe2\x82",
    b"\xed\xa0\x80",
    b"\xf0\x9f\x98",
    b"\xc0\xaf",
    b"\xf4\x90\x80\x80",
    b"\xe0\x80\x80",
)


def env_bytes(rng: random.Random) -> bytes | None:
    if rng.random() < 0.05:
        return None
    size = rng.choice((0, 1, 16, 30, 31, 32, 32, 33, 40, 40, 64, 65))
    broken = 0.05 if rng.random() < 0.25 else 0.0
    pieces: list[bytes] = []
    for _ in range(size):
        roll = rng.random()
        if roll < broken:
            pieces.append(rng.choice(INVALID_UTF8))
        elif roll < 0.85:
            pieces.append(rng.choice(string.ascii_letters + string.digits).encode())
        else:
            pieces.append(rng.choice((E_ACUTE, CJK_MIDDLE, GRINNING_FACE)).encode())
    if rng.random() < 0.3:
        pad = rng.choice(WHITESPACE).encode()
        if rng.random() < 0.5:
            pieces.insert(0, pad)
        else:
            pieces.append(pad)
    return b"".join(pieces)


def identity_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 8)
    out: list[dict] = []

    def key() -> bytes:
        size = rng.choice((0, 1, 32, 40, 64, 65, 128))
        return bytes(rng.randrange(256) for _ in range(size))

    def canonical() -> str:
        if rng.random() < 0.85:
            text = person_identity.canonical_mobile(phone_text(rng))
            if text is not None:
                return text
        return random_text(rng, 12)

    for i in range(3000):
        out.append(case("canonical_mobile", f"fuzz/{i}", {"raw": phone_text(rng)}))
    for i in range(500):
        args = {"canonical": canonical(), "key": key()}
        out.append(case("derive_person_id", f"fuzz/{i}", args))
    for i in range(500):
        args = {"canonical": canonical(), "key": key()}
        out.append(case("derive_phone_digest", f"fuzz/{i}", args))
    for i in range(500):
        code = "".join(
            rng.choice(string.digits) if rng.random() < 0.9 else rng.choice(ODD)
            for _ in range(rng.choice((0, 5, 6, 6, 6, 7)))
        )
        args = {
            "challenge_id": uuid.UUID(int=rng.getrandbits(128)),
            "code": code,
            "key": key(),
        }
        out.append(case("derive_code_digest", f"fuzz/{i}", args))
    for i in range(800):
        out.append(read_key_case(f"fuzz/{i}", env_bytes(rng)))
    return out


# ---------------------------------------------------------------------------
# Modes
# ---------------------------------------------------------------------------

FUNCTIONS = {
    "pair_key": friendship.pair_key,
    "is_live_edge": friendship.is_live_edge,
    "open_request": friendship.open_request,
    "decide": friendship.decide,
    "open_block": friendship.open_block,
    "unblock": friendship.unblock,
    "are_friends": friendship.are_friends,
    "is_blocked": blocking.is_blocked,
    "blocker_of": blocking.blocker_of,
    "hidden_between": blocking.hidden_between,
    "dm_allowed": blocking.dm_allowed,
    "rank": visibility.rank,
    "permitted_output_visibility": visibility.permitted_output_visibility,
    "check_no_context_laundering": visibility.check_no_context_laundering,
    "declassify": visibility.declassify,
    "can_view_history": visibility.can_view_history,
    "settlement_view": visibility.settlement_view,
    "expires_at_for": story_visibility.expires_at_for,
    "is_live": story_visibility.is_live,
    "check_caption": story_visibility.check_caption,
    "can_view": story_visibility.can_view,
    "order_authors": story_visibility.order_authors,
    "needs_context": post_audience.needs_context,
    "check_writable": post_audience.check_writable,
    "is_comment_policy": post_audience.is_comment_policy,
    "can_comment": post_audience.can_comment,
    "visible_to": post_audience.visible_to,
    "can_delete_comment": post_audience.can_delete_comment,
    "can_read": post_audience.can_read,
    "tally": vote.tally,
    "encode_cursor": cursors.encode_cursor,
    "decode_cursor": cursors.decode_cursor,
    "canonical_mobile": person_identity.canonical_mobile,
    "derive_person_id": person_identity.derive_person_id,
    "derive_phone_digest": person_identity.derive_phone_digest,
    "derive_code_digest": person_identity.derive_code_digest,
    "read_key": person_identity.read_key,
}

#: module -> (Go package path, module, constants, edges, fuzz, fuzz shards)
MODULES = {
    "friendship": (
        "internal/domain/friendship",
        friendship,
        friendship_constants,
        friendship_edges,
        friendship_fuzz,
        4,
    ),
    "blocking": (
        "internal/domain/blocking",
        blocking,
        blocking_constants,
        blocking_edges,
        blocking_fuzz,
        1,
    ),
    "visibility": (
        "internal/domain/visibility",
        visibility,
        visibility_constants,
        visibility_edges,
        visibility_fuzz,
        3,
    ),
    "story_visibility": (
        "internal/domain/storyvisibility",
        story_visibility,
        story_constants,
        story_edges,
        story_fuzz,
        3,
    ),
    "post_audience": (
        "internal/domain/postaudience",
        post_audience,
        post_constants,
        post_edges,
        post_fuzz,
        2,
    ),
    "vote": ("internal/domain/vote", vote, vote_constants, vote_edges, vote_fuzz, 3),
    "cursors": (
        "internal/cursors",
        cursors,
        cursors_constants,
        cursors_edges,
        cursors_fuzz,
        8,
    ),
    "person_identity": (
        "internal/identity",
        person_identity,
        identity_constants,
        identity_edges,
        identity_fuzz,
        4,
    ),
}


def modes() -> dict[str, tuple[str, int | None]]:
    table: dict[str, tuple[str, int | None]] = {}
    for module, spec in MODULES.items():
        table[module] = (module, None)
        for shard in range(spec[5]):
            table[f"{module}-fuzz-{shard}"] = (module, shard)
    return table


def render(mode: str) -> dict:
    module, part = modes()[mode]
    _path, target, constants, edges, fuzz, shards = MODULES[module]
    document: dict[str, object] = {
        "generator": "scripts/render_domain_w2_goldens.py",
        "module": target.__name__,
        "mode": mode,
        "python": platform.python_version(),
        "seed": SEED,
    }
    if part is None:
        document["constants"] = constants()
        document["cases"] = edges()
    else:
        cases = fuzz()
        start = part * len(cases) // shards
        stop = (part + 1) * len(cases) // shards
        document["fuzz"] = {"shard": part, "shards": shards, "total": len(cases)}
        document["cases"] = cases[start:stop]
    return document


def main(argv: list[str]) -> int:
    table = modes()
    if argv == ["--list"]:
        for mode, (module, _part) in table.items():
            path = TARGET.format(path=MODULES[module][0], mode=mode.replace("-", "_"))
            print(f"{mode}\t{path}")
        return 0
    if len(argv) != 1 or argv[0] not in table:
        print(f"usage: - MODE | --list; modes: {' '.join(table)}", file=sys.stderr)
        return 2
    rendered = json.dumps(render(argv[0]), ensure_ascii=True, indent=1) + "\n"
    problem = guard_problem(rendered)
    if problem is not None:
        print(problem, file=sys.stderr)
        return 1
    sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
