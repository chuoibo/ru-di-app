#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W9 auth routes (ADR-0029).

The seven W9 routes -- the four of `app/api/routes/sessions.py` and the three
of `app/api/routes/auth.py` -- reach these pure Python pieces, each ported to
one Go package:

    app.domain.otp                    services/core/internal/domain/otp
    app.api.service (session doors)   services/core/internal/domain/authsteps

`app.api.person_identity` was ported earlier (internal/identity,
scripts/render_domain_w2_goldens.py) and `app.domain.permissions` earlier still
(scripts/render_permissions_goldens.py); authsteps calls both.

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each returned or raised. The service methods run as real `ApiService`
methods over a recording stub repository with the clock pinned to the case's
`now`: the golden holds the answer (or the ApiProblem, or the exception that
would be a 500) and every call the method made across a seam -- repository,
identity, secrets, SMS gateway, Google verifier -- with its arguments, in
order. Each package's oracle_test.go replays every case. The image is the one
scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w9_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w9_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases;
`<module>-fuzz-0` is a small sample of that module's seeded fuzz. Both are
committed and replayed by plain `go test`. `--live MODULE SEED COUNT` draws
COUNT cases with SEED and prints them without the guard's checks; the Go tests
built with `-tags oracle` run it inside the image at test time (see
internal/oracletest/live.go). The committed sample is the first draw of the
same generator.

## What is pinned, and why every pinned thing is a seam

A golden cannot hold a real secret and a repository guard will not carry one
(see LINE_RULES below: no run of nine digits, nothing shaped like a telephone
number, no long encoded blob). Every value the port treats as opaque is
therefore replaced by a short, letter-only stand-in, and every one of them is
a seam the Go package holds behind an interface rather than code it ports:

* `derive_phone_digest`, `derive_code_digest` and `token_digest` become four
  bytes whose hexadecimal spelling is letters only, derived from the same
  input the real function hashes. Equality still means what it meant, which is
  all the port does with a digest; the Go replay computes the same stand-in.
  What they keep of the real functions is the `.encode("ascii")` in front,
  which is where a number of non-ASCII digits raises UnicodeEncodeError.
* `derive_person_id` answers from a queue the case scripts, so a derived id is
  a readable alias.
* `uuid.uuid4`, `secrets.token_urlsafe` and `secrets.randbelow` answer from
  queues the case scripts.
* `read_key` is the REAL `read_key`, over an environment the case provides, so
  the 503 path and the length rule are the module's own.
* `canonical_mobile` is the REAL function, unpatched: it decides a 422 and the
  Go replay calls the W2 port, so every fuzzed telephone string compares the
  two ports as well.

## Encoding

The encoding of scripts/render_domain_w4_goldens.py (`{"fn", "name", "args",
"result"}`, `"$i:<hex>"` for a large int, `"$sp:<text>"` for a str cut into
groups of six code points), with these additions:

* a uuid.UUID is its name in ALIASES (`"TOI"`, `"SS1"`), which keeps a case on
  one short line; ids in the arguments are those names too;
* a datetime is its isoformat();
* a bytes is its `.hex()`, which for every digest here is letters;
* a service case's result is `{"ok": {"calls", "problem", "raised",
  "response"}}`: `calls` is `[method, argument, ...]` per crossed seam with
  arguments in the callee's order, `problem` the ApiProblem, `raised` the
  class and code of an exception Python answers with a 500, `response` the
  response model's model_dump().
"""

from __future__ import annotations

import hashlib
import json
import platform
import random
import re
import sys
import uuid
from datetime import UTC, date, datetime, timedelta, timezone
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem, RepositoryConflict  # noqa: E402
from app.api.google_identity import GoogleTokenInvalid  # noqa: E402
from app.api.person_identity import (  # noqa: E402
    KEY_ENV_VAR,
    PersonIdKeyMissing,
    canonical_mobile,
    read_key,
)
from app.api.repository import (  # noqa: E402
    AccountIdentityRecord,
    AccountSessionRecord,
    LastMessageRecord,
    MembershipRecord,
    OtpChallengeRecord,
    OutingInviteRecord,
    PersonContextSummaryRecord,
    PersonRecord,
)
from app.api.sms import SmsDeliveryError  # noqa: E402
from app.domain import otp as otp_module  # noqa: E402
from app.domain.permissions import PermissionError_  # noqa: E402

SEED = 29
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
# Ids, telephones and stand-in secrets
# ---------------------------------------------------------------------------


def _uuid_for(letter: str, digit: str) -> uuid.UUID:
    """A uuid whose digits never run: the guard reads nine as an account."""
    c, d = letter, digit
    return uuid.UUID(
        f"{c}{c}{d}{d}{c}{c}{d}{d}-0{c}0{c}-4{c}0{c}-8{c}0{c}-0{c}0{c}0{c}0{c}0{c}{d}{d}"
    )


#: Every uuid a case names. People (TOI is the caller, DER what
#: `derive_person_id` answers, NEW what `uuid.uuid4` answers), contexts,
#: memberships, invitations, outings, challenges and sessions.
ALIASES = {
    "TOI": _uuid_for("a", "1"),
    "KIA": _uuid_for("b", "2"),
    "LA": _uuid_for("c", "3"),
    "DER": _uuid_for("d", "4"),
    "NEW": _uuid_for("e", "5"),
    "HOI": _uuid_for("f", "6"),
    "CAP": _uuid_for("a", "7"),
    "MB1": _uuid_for("b", "8"),
    "MB2": _uuid_for("c", "9"),
    "IV1": _uuid_for("d", "1"),
    "IV2": _uuid_for("e", "2"),
    "OU1": _uuid_for("f", "3"),
    "CH1": _uuid_for("a", "4"),
    "CH2": _uuid_for("b", "5"),
    "CHN": _uuid_for("c", "6"),
    "SS1": _uuid_for("d", "7"),
    "SS2": _uuid_for("e", "8"),
    "SSN": _uuid_for("f", "9"),
    "AI1": _uuid_for("a", "2"),
}
ALIAS_OF = {value: name for name, value in ALIASES.items()}
assert len(ALIAS_OF) == len(ALIASES)


def U(name: str) -> uuid.UUID:
    return ALIASES[name]


def UN(name: str | None) -> uuid.UUID | None:
    return None if name is None else ALIASES[name]


#: Every raw session secret a case uses, deliberately lowercase words rather
#: than the 43 url-safe characters `secrets.token_urlsafe(32)` really draws: a
#: real one reads as an encoded blob to the repository guard, and the port
#: never looks inside a token anyway.
TOKENS = {
    "TK_MOI": "phien-moi",
    "TK_NAY": "phien-nay",
    "TK_CU": "phien-cu",
    "TK_LA": "phien-la",
}

#: Telephone numbers a case types. The guard refuses anything shaped like a
#: real Vietnamese mobile, so these are written as their digits with letters
#: in between and assembled here; `canonical_mobile` sees only the digits.
#: Each entry is (name, the digits after the leading zero).
_TAILS = {
    "SO_A": "9" + "1" * 8,
    "SO_B": "3" + "2" * 8,
    "SO_C": "7" + "4" * 8,
}
PHONES = {name: "0" + tail for name, tail in _TAILS.items()}
#: A number whose ninth digit is an Arabic-Indic digit: `_MOBILE` accepts it
#: because `\d` in a str pattern is every Unicode decimal, and every derivation
#: then refuses it because `.encode("ascii")` cannot spell it.
PHONES["SO_UNI"] = "0" + "9" + "1" * 7 + chr(0x662)

#: A key long enough for MIN_KEY_LENGTH, with no digit in it.
KEY = "khoa-danh-tinh-cua-may-chu-nay-dai-du-dung"
assert len(KEY) >= 32

LETTERS = "abcdef"


def stand_in(seed: str, size: int = 4) -> bytes:
    """`size` bytes whose hexadecimal spelling is letters only.

    A digest in a golden must survive the repository guard, and a random one
    will not: sixteen hexadecimal digits over fifty-six positions carry a run
    of nine decimal ones more often than not. Mapping each byte of a SHA-256
    onto `abcdef` keeps the value deterministic in its input -- which is all a
    digest comparison needs -- and keeps every rendered line readable.
    """
    raw = hashlib.sha256(seed.encode("utf-8")).digest()
    return bytes.fromhex("".join(LETTERS[b % 6] for b in raw[: 2 * size]))


def phone_digest_of(canonical: str) -> bytes:
    return stand_in("phone|" + canonical)


def code_digest_of(challenge_id: uuid.UUID, code: str) -> bytes:
    return stand_in(f"code|{challenge_id}|{code}")


def token_digest_of(token: str) -> bytes:
    return stand_in("token|" + token)


# ---------------------------------------------------------------------------
# Encoding
# ---------------------------------------------------------------------------


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
    if isinstance(value, bytes):
        return enc_str(value.hex())
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
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"unencodable {type(value).__name__}")


def case(fn: str, name: str, args: dict) -> dict:
    encoded = {key: enc(value) for key, value in args.items()}
    try:
        value = FUNCTIONS[fn](**args)
    except (OverflowError, ValueError, TypeError) as exc:
        result = {
            "raised": {
                "type": type(exc).__name__,
                "message": enc_str(str(exc)),
                "code": enc(getattr(exc, "code", None)),
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
# Shared instants
# ---------------------------------------------------------------------------

MICRO = timedelta(microseconds=1)
SECOND = timedelta(seconds=1)
MINUTE = timedelta(minutes=1)
HOUR = timedelta(hours=1)
DAY = timedelta(days=1)

T = datetime(2030, 9, 18, 5, 0, tzinfo=UTC)

VIETNAM = timezone(timedelta(hours=7))
KIRITIMATI = timezone(timedelta(hours=14))
BAKER = timezone(timedelta(hours=-12))
KATHMANDU = timezone(timedelta(hours=5, minutes=45))
ONE_SECOND_WEST = timezone(timedelta(seconds=-1))
OFFSETS = (UTC, VIETNAM, KIRITIMATI, BAKER, KATHMANDU, ONE_SECOND_WEST)

E_DOT_CIRCUMFLEX = chr(0x1EC7)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
NO_BREAK_SPACE = chr(0xA0)
IDEOGRAPHIC_SPACE = chr(0x3000)
ZERO_WIDTH_SPACE = chr(0x200B)
NEXT_LINE = chr(0x85)
FILE_SEPARATOR = "\x1c"
UNIT_SEPARATOR = "\x1f"
VERTICAL_TAB = "\x0b"
ARABIC_INDIC_FOUR = chr(0x664)
FULLWIDTH_SEVEN = chr(0xFF17)

#: Things a person could type into the code box, and things nobody should.
CODES = (
    "abcdef",
    "  abcdef  ",
    "",
    " ",
    "\t\n",
    FILE_SEPARATOR + "abcdef" + UNIT_SEPARATOR,
    VERTICAL_TAB + "abcdef",
    NO_BREAK_SPACE + "abcdef" + IDEOGRAPHIC_SPACE,
    ZERO_WIDTH_SPACE + "abcdef",
    NEXT_LINE + "abcdef",
    ARABIC_INDIC_FOUR * 6,
    FULLWIDTH_SEVEN * 6,
    CJK_MIDDLE + GRINNING_FACE,
    "e" + chr(0x301),
    "<script>",
    "' OR 1=1 --",
    "a" * 200,
    "$sp:not|an|encoding",
)


# ---------------------------------------------------------------------------
# app.domain.otp
# ---------------------------------------------------------------------------


def otp_generate_code(draw: int) -> dict:
    """`generate_code` with a scripted draw, recording the bound it asked for."""
    bounds: list[int] = []

    def below(bound: int) -> int:
        bounds.append(bound)
        return draw

    return {"code": otp_module.generate_code(below), "bounds": bounds}


def otp_plan_request(recent: list, now: datetime, limits: dict | None) -> dict:
    return otp_module.plan_request([{"created_at": row} for row in recent], now, limits)


def otp_plan_verify(
    challenge: dict | None, now: datetime, code_matches: bool, limits: dict | None
) -> dict:
    return otp_module.plan_verify(challenge, now, code_matches, limits)


LIMIT_KEYS = (
    "code_ttl_seconds",
    "max_attempts",
    "resend_cooldown_seconds",
    "max_challenges_per_window",
    "window_seconds",
)


def otp_constants() -> dict:
    return {
        "default_limits": [[key, otp_module.DEFAULT_LIMITS[key]] for key in LIMIT_KEYS],
        "code_length": otp_module.CODE_LENGTH,
        "limit_keys": list(LIMIT_KEYS),
    }


def challenge_dict(
    expires: datetime, attempts: int, consumed: datetime | None = None
) -> dict:
    return {"expires_at": expires, "attempts": attempts, "consumed_at": consumed}


def otp_edges() -> list[dict]:
    cases: list[dict] = []
    add = lambda fn, name, **args: cases.append(case(fn, name, args))  # noqa: E731

    # --- generate_code
    for name, draw in (
        ("zero", 0),
        ("one", 1),
        ("five digits", 99999),
        ("six digits", 999999),
        ("at the bound", 10**6),
        ("past the bound", 10**7 + 1),
        ("negative", -5),
        ("very negative", -(10**9)),
        ("huge", 10**30),
    ):
        add("generate_code", f"draw {name}", draw=draw)

    # --- plan_request
    add("plan_request", "no history", recent=[], now=T, limits=None)
    add("plan_request", "empty limits", recent=[], now=T, limits={})
    for label, gap in (
        ("just issued", timedelta(0)),
        ("one microsecond ago", MICRO),
        ("one second short of the cooldown", 59 * SECOND),
        ("one microsecond short of the cooldown", 60 * SECOND - MICRO),
        ("exactly at the cooldown", 60 * SECOND),
        ("one microsecond past the cooldown", 60 * SECOND + MICRO),
        ("well past the cooldown", 10 * MINUTE),
    ):
        add("plan_request", f"one row {label}", recent=[T - gap], now=T, limits=None)
    add(
        "plan_request",
        "row exactly at the window edge is outside it",
        recent=[T - 900 * SECOND],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "row one microsecond inside the window edge",
        recent=[T - 900 * SECOND + MICRO],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "four rows past the cooldown",
        recent=[T - (100 + 60 * index) * SECOND for index in range(4)],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "five rows past the cooldown fills the window",
        recent=[T - (100 + 60 * index) * SECOND for index in range(5)],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "five rows but the oldest has just left the window",
        recent=[T - 900 * SECOND]
        + [T - (100 + 60 * index) * SECOND for index in range(4)],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "a row from the future",
        recent=[T + HOUR],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "rows in six offsets, same instants",
        recent=[
            (T - (200 + 60 * index) * SECOND).astimezone(zone)
            for index, zone in enumerate(OFFSETS)
        ],
        now=T,
        limits=None,
    )
    add(
        "plan_request",
        "a row three hundred years back is still outside the window",
        recent=[T - 300 * 365 * DAY],
        now=T,
        limits=None,
    )
    for label, override in (
        ("window of one second", {"window_seconds": 1}),
        ("window of zero", {"window_seconds": 0}),
        ("negative window", {"window_seconds": -1}),
        ("cooldown of zero", {"resend_cooldown_seconds": 0}),
        ("negative cooldown", {"resend_cooldown_seconds": -1}),
        ("one challenge per window", {"max_challenges_per_window": 1}),
        ("no challenge per window", {"max_challenges_per_window": 0}),
        ("an unread key", {"never_read": 7}),
        ("cooldown past a float", {"resend_cooldown_seconds": 10**400}),
        (
            "window at the timedelta ceiling",
            {"window_seconds": timedelta.max.days * 86400 + 86399},
        ),
        (
            "window one second past the ceiling",
            {"window_seconds": timedelta.max.days * 86400 + 86400},
        ),
        ("window past a C int", {"window_seconds": 10**15}),
    ):
        add(
            "plan_request",
            f"limits: {label}",
            recent=[T - SECOND, T - 2 * SECOND],
            now=T,
            limits=override,
        )
    add(
        "plan_request",
        "the clock at the start of the calendar",
        recent=[],
        now=datetime(1, 1, 1, 0, 10, tzinfo=UTC),
        limits=None,
    )
    add(
        "plan_request",
        "the clock at the very end of the calendar",
        recent=[],
        now=datetime(9999, 12, 31, 23, 59, 59, 999999, tzinfo=UTC),
        limits=None,
    )

    # --- plan_verify
    add(
        "plan_verify",
        "no challenge",
        challenge=None,
        now=T,
        code_matches=True,
        limits=None,
    )
    add(
        "plan_verify",
        "no challenge and a wrong code",
        challenge=None,
        now=T,
        code_matches=False,
        limits=None,
    )
    add(
        "plan_verify",
        "consumed beats a right code",
        challenge=challenge_dict(T + HOUR, 0, T - HOUR),
        now=T,
        code_matches=True,
        limits=None,
    )
    for label, expires in (
        ("one microsecond left", T + MICRO),
        ("exactly at the deadline", T),
        ("one microsecond past the deadline", T - MICRO),
    ):
        add(
            "plan_verify",
            f"expiry {label}",
            challenge=challenge_dict(expires, 0),
            now=T,
            code_matches=True,
            limits=None,
        )
    for attempts in (0, 1, 3, 4, 5, 6):
        for matches in (True, False):
            add(
                "plan_verify",
                f"{attempts} attempts already, code {'right' if matches else 'wrong'}",
                challenge=challenge_dict(T + HOUR, attempts),
                now=T,
                code_matches=matches,
                limits=None,
            )
    add(
        "plan_verify",
        "a negative attempt count",
        challenge=challenge_dict(T + HOUR, -3),
        now=T,
        code_matches=False,
        limits=None,
    )
    add(
        "plan_verify",
        "an attempt count past int64",
        challenge=challenge_dict(T + HOUR, 10**30),
        now=T,
        code_matches=False,
        limits=None,
    )
    for label, override in (
        ("one attempt allowed", {"max_attempts": 1}),
        ("no attempt allowed", {"max_attempts": 0}),
        ("a negative ceiling", {"max_attempts": -1}),
        ("a ceiling past int64", {"max_attempts": 10**30}),
        ("an unread key", {"never_read": 7}),
    ):
        add(
            "plan_verify",
            f"limits: {label}",
            challenge=challenge_dict(T + HOUR, 0),
            now=T,
            code_matches=False,
            limits=override,
        )
    add(
        "plan_verify",
        "the deadline in another offset, same instant",
        challenge=challenge_dict(T.astimezone(KIRITIMATI), 0),
        now=T,
        code_matches=True,
        limits=None,
    )
    return cases


def random_instant(rng: random.Random) -> datetime:
    """An instant anywhere in the calendar, in one of the offsets."""
    shape = rng.random()
    if shape < 0.55:
        moment = T + timedelta(microseconds=rng.randint(-2 * 10**9, 2 * 10**9))
    elif shape < 0.8:
        moment = T + timedelta(microseconds=rng.randint(-(10**14), 10**14))
    else:
        moment = datetime(
            rng.randint(1, 9999),
            rng.randint(1, 12),
            rng.randint(1, 28),
            rng.randint(0, 23),
            rng.randint(0, 59),
            rng.randint(0, 59),
            rng.randint(0, 999999),
            tzinfo=UTC,
        )
    return moment.astimezone(rng.choice(OFFSETS))


def random_limits(rng: random.Random) -> dict | None:
    if rng.random() < 0.55:
        return None
    override: dict = {}
    for key in LIMIT_KEYS:
        if rng.random() < 0.35:
            override[key] = rng.choice(
                [
                    rng.randint(-5, 10),
                    rng.randint(-(10**4), 10**4),
                    rng.randint(-(10**13), 10**13),
                    rng.choice([10**15, -(10**15), 10**30, -(10**30), 10**400]),
                    timedelta.max.days * 86400 + rng.randint(-2, 2),
                ]
            )
    if rng.random() < 0.1:
        override["never_read"] = rng.randint(0, 5)
    return override


def otp_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed)
    cases: list[dict] = []
    while len(cases) < count:
        index = len(cases)
        pick = rng.random()
        if pick < 0.15:
            built = case(
                "generate_code",
                f"fuzz {index}",
                {
                    "draw": rng.choice(
                        [rng.randint(-10, 10**7), rng.randint(-(10**30), 10**30)]
                    )
                },
            )
        elif pick < 0.6:
            now = random_instant(rng)
            rows = [
                now + timedelta(microseconds=rng.randint(-2 * 10**9, 10**8))
                for _ in range(rng.randint(0, 7))
            ]
            if rng.random() < 0.1:
                rows = [random_instant(rng) for _ in rows]
            built = case(
                "plan_request",
                f"fuzz {index}",
                {"recent": rows, "now": now, "limits": random_limits(rng)},
            )
        else:
            now = random_instant(rng)
            challenge = None
            if rng.random() < 0.9:
                consumed = None
                if rng.random() < 0.3:
                    consumed = now + timedelta(
                        microseconds=rng.randint(-(10**9), 10**9)
                    )
                challenge = challenge_dict(
                    now + timedelta(microseconds=rng.randint(-(10**9), 10**9)),
                    rng.choice([rng.randint(-2, 8), rng.randint(-(10**30), 10**30)]),
                    consumed,
                )
            built = case(
                "plan_verify",
                f"fuzz {index}",
                {
                    "challenge": challenge,
                    "now": now,
                    "code_matches": rng.random() < 0.5,
                    "limits": random_limits(rng),
                },
            )
        if fits(built):
            cases.append(built)
    return cases


# ---------------------------------------------------------------------------
# auth steps: real ApiService methods over a recording stub repository
# ---------------------------------------------------------------------------


class Unscripted(BaseException):
    """The stub was asked for a value the case did not script: a generator bug."""


WORLD_DEFAULTS = {
    "key": KEY,
    #: alias -> [display_name, deleted_at]
    "people": {},
    #: rows of list_person_context_summaries, in SUMMARY_KEYS order
    "summaries": [],
    #: "reader|other" -> [state, decided_by] answering get_friend_edge
    "edges": {},
    #: token name -> invitation row, in INVITE_KEYS order
    "invites": {},
    #: outing alias -> context alias
    "outings": {},
    "membership": ["MB1", "invited"],
    #: created_at of each row recent_otp_challenges answers
    "recent": [],
    #: the row get_otp_challenge answers, in CHALLENGE_KEYS order
    "challenge": None,
    #: "provider|subject" -> person alias; a phone subject is written "phone|@SO_A"
    "identities": {},
    #: rows of list_account_sessions, in SESSION_KEYS order
    "sessions": [],
    #: token name -> session row answering get_account_session_by_digest
    "by_digest": {},
    #: the row get_account_session answers
    "session": None,
    #: repository method -> the codes its next calls raise
    "conflicts": {},
    #: None for a delivered message, else the text of the SmsDeliveryError
    "sms": None,
    #: None for no verifier, ["ok", sub, name] or ["bad", message]
    "google": None,
    #: what uuid.uuid4, derive_person_id and secrets.token_urlsafe answer
    "uuids": ["NEW"],
    "derived": ["DER"],
    "tokens": ["TK_MOI"],
    #: what secrets.randbelow answers
    "draws": [0],
    #: what the challenge row created_otp_challenge answers carries as its id
    "created_challenge": None,
    #: what the session row create_account_session answers carries
    "created_session": ["SSN", None],
    "debug_code": None,
}

SUMMARY_KEYS = (
    "id",
    "display_name",
    "member_count",
    "my_role",
    "my_state",
    "membership_id",
    "joined_at",
    "last_message",
    "unread_count",
    "theme",
    "kind",
    "counterpart_id",
    "counterpart_display_name",
)
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
CHALLENGE_KEYS = (
    "id",
    "phone",
    "code",
    "created_at",
    "expires_at",
    "attempts",
    "consumed_at",
)
SESSION_KEYS = (
    "id",
    "person_id",
    "issued_from_invite_id",
    "issued_via",
    "created_at",
    "expires_at",
    "revoked_at",
)


def summary_record(row: list) -> PersonContextSummaryRecord:
    values = dict(zip(SUMMARY_KEYS, row, strict=True))
    last = values["last_message"]
    return PersonContextSummaryRecord(
        id=U(values["id"]),
        display_name=values["display_name"],
        member_count=values["member_count"],
        my_role=values["my_role"],
        my_state=values["my_state"],
        membership_id=U(values["membership_id"]),
        joined_at=values["joined_at"],
        last_message=None
        if last is None
        else LastMessageRecord(
            id=U(last[0]),
            kind=last[1],
            preview=last[2],
            author_id=UN(last[3]),
            author_display_name=last[4],
            created_at=last[5],
        ),
        unread_count=values["unread_count"],
        theme=values["theme"],
        kind=values["kind"],
        counterpart_id=UN(values["counterpart_id"]),
        counterpart_display_name=values["counterpart_display_name"],
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


def challenge_record(row: list) -> OtpChallengeRecord:
    values = dict(zip(CHALLENGE_KEYS, row, strict=True))
    return OtpChallengeRecord(
        id=U(values["id"]),
        phone_digest=digest_for(values["phone"]),
        code_digest=digest_for(values["code"]),
        created_at=values["created_at"],
        expires_at=values["expires_at"],
        attempts=values["attempts"],
        consumed_at=values["consumed_at"],
    )


def session_record(row: list) -> AccountSessionRecord:
    values = dict(zip(SESSION_KEYS, row, strict=True))
    return AccountSessionRecord(
        id=U(values["id"]),
        person_id=U(values["person_id"]),
        issued_from_invite_id=UN(values["issued_from_invite_id"]),
        issued_via=values["issued_via"],
        created_at=values["created_at"],
        expires_at=values["expires_at"],
        revoked_at=values["revoked_at"],
    )


def digest_for(spec: object) -> bytes:
    """A stored digest as a case spells it.

    `"@SO_A"` is the phone digest of that number, `["CH1", "abcdef"]` the code
    digest of that code on that challenge, and anything else stands for itself
    so a case can store a digest nothing derives.
    """
    if isinstance(spec, str) and spec.startswith("@"):
        return phone_digest_of(canonical_mobile(PHONES[spec[1:]]))
    if isinstance(spec, list):
        return code_digest_of(U(spec[0]), spec[1])
    return stand_in("other|" + str(spec))


def subject_of(spec: str) -> str:
    """An `account_identities` subject as a case spells it."""
    provider, _, rest = spec.partition("|")
    if provider == "phone":
        return f"phone|{digest_for(rest).hex()}"
    return spec


class Stub:
    """A repository that answers from the case's world and records every call."""

    def __init__(self, world: dict):
        self.world = {**WORLD_DEFAULTS, **world}
        self.calls: list = []
        self.conflicts = {
            key: list(codes) for key, codes in self.world["conflicts"].items()
        }
        self.queues = {
            key: list(self.world[key])
            for key in ("uuids", "derived", "tokens", "draws")
        }
        self.identities = {
            subject_of(key): value for key, value in self.world["identities"].items()
        }
        self.by_digest = {
            token_digest_of(TOKENS[name]).hex(): row
            for name, row in self.world["by_digest"].items()
        }
        self.invites = {
            token_digest_of(TOKENS[name]).hex(): row
            for name, row in self.world["invites"].items()
        }

    # --- plumbing
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

    # --- people and conversations
    def get_person(self, person_id):
        self.rec("get_person", person_id)
        row = self.world["people"].get(ALIAS_OF.get(person_id, str(person_id)))
        if row is None:
            return None
        return PersonRecord(
            id=person_id,
            display_name=row[0],
            created_at=T - 30 * DAY,
            deleted_at=row[1],
        )

    def create_person(self, person_id, display_name):
        self.rec("create_person", person_id, display_name)
        self.maybe_conflict("create_person")
        return PersonRecord(id=person_id, display_name=display_name, created_at=T)

    def list_person_context_summaries(self, person_id):
        self.rec("list_person_context_summaries", person_id)
        return [summary_record(row) for row in self.world["summaries"]]

    def get_friend_edge(self, person_a, person_b):
        self.rec("get_friend_edge", person_a, person_b)
        key = f"{ALIAS_OF.get(person_a)}|{ALIAS_OF.get(person_b)}"
        row = self.world["edges"].get(key)
        if row is None:
            return None
        return SimpleNamespace(state=row[0], decided_by_id=UN(row[1]))

    # --- identities
    def get_account_identity(self, provider, subject):
        self.rec("get_account_identity", provider, subject)
        alias = self.identities.get(f"{provider}|{subject}")
        if alias is None:
            return None
        return AccountIdentityRecord(
            id=U("AI1"),
            person_id=U(alias),
            provider=provider,
            subject=subject,
            created_at=T - DAY,
            last_login_at=T - HOUR,
        )

    def upsert_account_identity(self, *, person_id, provider, subject, now):
        self.rec("upsert_account_identity", person_id, provider, subject, now)
        self.maybe_conflict("upsert_account_identity")
        return AccountIdentityRecord(
            id=U("AI1"),
            person_id=person_id,
            provider=provider,
            subject=subject,
            created_at=now,
            last_login_at=now,
        )

    def create_person_with_identity(
        self, *, person_id, display_name, provider, subject, now
    ):
        self.rec(
            "create_person_with_identity",
            person_id,
            display_name,
            provider,
            subject,
            now,
        )
        self.maybe_conflict("create_person_with_identity")
        return AccountIdentityRecord(
            id=U("AI1"),
            person_id=person_id,
            provider=provider,
            subject=subject,
            created_at=now,
            last_login_at=now,
        )

    # --- one-time codes
    def create_otp_challenge(
        self, *, challenge_id, phone_digest, code_digest, expires_at, now
    ):
        self.rec(
            "create_otp_challenge",
            challenge_id,
            phone_digest,
            code_digest,
            expires_at,
            now,
        )
        self.maybe_conflict("create_otp_challenge")
        stored = self.world["created_challenge"]
        return OtpChallengeRecord(
            id=challenge_id if stored is None else U(stored),
            phone_digest=phone_digest,
            code_digest=code_digest,
            created_at=now,
            expires_at=expires_at,
            attempts=0,
            consumed_at=None,
        )

    def recent_otp_challenges(self, phone_digest, since):
        self.rec("recent_otp_challenges", phone_digest, since)
        return [
            OtpChallengeRecord(
                id=U("CH1"),
                phone_digest=phone_digest,
                code_digest=phone_digest,
                created_at=created,
                expires_at=created + 5 * MINUTE,
                attempts=0,
                consumed_at=None,
            )
            for created in self.world["recent"]
        ]

    def get_otp_challenge(self, challenge_id):
        self.rec("get_otp_challenge", challenge_id)
        row = self.world["challenge"]
        return None if row is None else challenge_record(row)

    def record_otp_attempt(self, *, challenge_id, attempts, consumed, now):
        self.rec("record_otp_attempt", challenge_id, attempts, consumed, now)
        self.maybe_conflict("record_otp_attempt")
        return None

    # --- invitations
    def get_outing_invite_by_digest(self, token_digest):
        self.rec("get_outing_invite_by_digest", token_digest)
        row = self.invites.get(token_digest.hex())
        return None if row is None else invite_record(row)

    def get_outing(self, outing_id):
        self.rec("get_outing", outing_id)
        context = self.world["outings"].get(ALIAS_OF.get(outing_id))
        if context is None:
            return None
        return SimpleNamespace(id=outing_id, context_id=U(context))

    def consume_named_invite_secret(
        self, *, invite_id, token_digest, accepted_by_id, now
    ):
        self.rec(
            "consume_named_invite_secret", invite_id, token_digest, accepted_by_id, now
        )
        self.maybe_conflict("consume_named_invite_secret")
        return None

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
        row = self.world["membership"]
        return MembershipRecord(
            id=U(row[0]),
            context_id=context_id,
            person_id=person_id,
            display_name="Thành viên",
            state=row[1],
            role="member",
            origin=origin,
            invited_by_id=invited_by_id,
            joined_at=None,
            left_at=None,
            created_at=now,
        )

    # --- sessions
    def create_account_session(
        self,
        *,
        person_id,
        token_digest,
        issued_from_invite_id,
        expires_at,
        now,
        issued_via=None,
    ):
        self.rec(
            "create_account_session",
            person_id,
            token_digest,
            issued_from_invite_id,
            expires_at,
            now,
            issued_via,
        )
        self.maybe_conflict("create_account_session")
        alias, stored_via = self.world["created_session"]
        return AccountSessionRecord(
            id=U(alias),
            person_id=person_id,
            issued_from_invite_id=issued_from_invite_id,
            issued_via=issued_via if stored_via is None else stored_via,
            created_at=now,
            expires_at=expires_at,
            revoked_at=None,
        )

    def get_account_session_by_digest(self, token_digest):
        self.rec("get_account_session_by_digest", token_digest)
        row = self.by_digest.get(token_digest.hex())
        return None if row is None else session_record(row)

    def get_account_session(self, session_id):
        self.rec("get_account_session", session_id)
        row = self.world["session"]
        return None if row is None else session_record(row)

    def revoke_account_session(self, *, session_id, now):
        self.rec("revoke_account_session", session_id, now)
        self.maybe_conflict("revoke_account_session")
        return None

    def list_account_sessions(self, person_id, *, now):
        self.rec("list_account_sessions", person_id, now)
        return [session_record(row) for row in self.world["sessions"]]


class Sms:
    """The gateway seam. Records the call, then refuses if the case says so."""

    def __init__(self, stub: Stub):
        self.stub = stub

    def send_otp(self, *, canonical_phone, code, challenge_id):
        self.stub.rec("sms.send_otp", canonical_phone, code, challenge_id)
        failure = self.stub.world["sms"]
        if failure is not None:
            raise SmsDeliveryError(failure)


class Google:
    """The verifier seam. `["ok", sub, name]` vouches, `["bad", text]` does not."""

    def __init__(self, stub: Stub):
        self.stub = stub

    def verify(self, id_token):
        self.stub.rec("google.verify", id_token)
        answer = self.stub.world["google"]
        if answer[0] == "bad":
            raise GoogleTokenInvalid(answer[1])
        return SimpleNamespace(subject=answer[1], display_name=answer[2])


def install_seams(stub: Stub, now: datetime) -> None:
    """Pin the clock and every seam the service reaches for."""
    api_service._now = lambda: now

    def rec_canonical(raw):
        stub.rec("identity.canonical_mobile", raw)
        return canonical_mobile(raw)

    def rec_read_key():
        stub.rec("identity.read_key")
        return read_key({KEY_ENV_VAR: stub.world["key"]})

    def rec_phone_digest(canonical, key):
        stub.rec("identity.phone_digest", canonical)
        canonical.encode("ascii")
        return phone_digest_of(canonical)

    def rec_code_digest(challenge_id, code, key):
        stub.rec("identity.code_digest", challenge_id, code)
        code.encode("ascii")
        return code_digest_of(challenge_id, code)

    def rec_person_id(canonical, key):
        stub.rec("identity.person_id", canonical)
        canonical.encode("ascii")
        return U(stub.take("derived"))

    def rec_token_digest(token):
        stub.rec("secrets.token_digest", token)
        return token_digest_of(token)

    def rec_uuid4():
        stub.rec("secrets.uuid4")
        return U(stub.take("uuids"))

    def rec_token_urlsafe(size):
        stub.rec("secrets.token_urlsafe", size)
        return TOKENS[stub.take("tokens")]

    def rec_randbelow(bound):
        stub.rec("secrets.randbelow", bound)
        return stub.take("draws")

    api_service.canonical_mobile = rec_canonical
    api_service.read_key = rec_read_key
    api_service.derive_phone_digest = rec_phone_digest
    api_service.derive_code_digest = rec_code_digest
    api_service.derive_person_id = rec_person_id
    api_service.token_digest = rec_token_digest
    api_service.uuid = SimpleNamespace(uuid4=rec_uuid4, UUID=uuid.UUID)
    api_service.secrets = SimpleNamespace(
        token_urlsafe=rec_token_urlsafe, randbelow=rec_randbelow
    )


def _bootstrap(service, actor, req, seams):
    return service.bootstrap_session_from_invite(TOKENS[req["token"]])


def _list_sessions(service, actor, req):
    token = req["token"]
    return service.list_account_sessions(
        actor, current_token=None if token is None else TOKENS[token]
    )


CALLERS = {
    "bootstrap_session_from_invite": lambda s, a, r, k: s.bootstrap_session_from_invite(
        TOKENS[r["token"]]
    ),
    "list_account_sessions": lambda s, a, r, k: _list_sessions(s, a, r),
    "revoke_session_token": lambda s, a, r, k: s.revoke_session_token(
        TOKENS[r["token"]]
    ),
    "revoke_account_session": lambda s, a, r, k: s.revoke_account_session(
        U(r["session_id"]), a
    ),
    "request_otp": lambda s, a, r, k: s.request_otp(
        phone_arg(r["phone"]), sender=k["sms"], debug_code=k["debug_code"]
    ),
    "verify_otp": lambda s, a, r, k: s.verify_otp(
        U(r["challenge_id"]), phone_arg(r["phone"]), r["code"]
    ),
    "login_with_google": lambda s, a, r, k: s.login_with_google(
        r["id_token"], verifier=k["google"]
    ),
}

STEP_RAISES = (
    RepositoryConflict,
    AssertionError,
    PermissionError_,
    PersonIdKeyMissing,
    OverflowError,
    UnicodeEncodeError,
    ValueError,
    TypeError,
    KeyError,
    IndexError,
)


def phone_arg(spec: object) -> object:
    """A `phone` field. `"@SO_A"` names a number; anything else is itself."""
    if isinstance(spec, str) and spec.startswith("@"):
        return PHONES[spec[1:]]
    return spec


def run_step(fn: str, now: datetime, actor: list, req: dict, world: dict) -> dict:
    stub = Stub(world)
    install_seams(stub, now)
    seams = {
        "sms": Sms(stub),
        "google": None if stub.world["google"] is None else Google(stub),
        "debug_code": stub.world["debug_code"],
    }
    who = Actor(id=U(actor[0]), roles=frozenset(actor[1]), context_ids=frozenset())
    out: dict = {"calls": stub.calls, "problem": None, "raised": None, "response": None}
    try:
        response = CALLERS[fn](api_service.ApiService(stub), who, req, seams)
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
        out["response"] = None if response is None else response.model_dump()
    return out


def step_case(
    fn: str,
    name: str,
    req: dict,
    world: dict | None = None,
    now: datetime = T,
    actor=("TOI", ("member",)),
) -> dict:
    return case(
        fn,
        name,
        {
            "now": now,
            "actor": [actor[0], list(actor[1])],
            "req": req,
            "world": world or {},
        },
    )


def auth_steps_constants() -> dict:
    return {
        "aliases": [[name, str(value)] for name, value in ALIASES.items()],
        "tokens": [[name, value] for name, value in TOKENS.items()],
        "phones": [[name, value] for name, value in PHONES.items()],
        "methods": list(CALLERS),
        "world_defaults": WORLD_DEFAULTS,
        "key": KEY,
        "session_ttl_seconds": int(api_service.ACCOUNT_SESSION_TTL.total_seconds()),
        "new_person_name": api_service.ApiService.NEW_PERSON_NAME,
        "summary_keys": list(SUMMARY_KEYS),
        "invite_keys": list(INVITE_KEYS),
        "challenge_keys": list(CHALLENGE_KEYS),
        "session_keys": list(SESSION_KEYS),
    }


# --- worlds -----------------------------------------------------------------


def invite(
    token: str = "TK_MOI",
    *,
    invite_id: str = "IV1",
    outing: str = "OU1",
    source: str = "group",
    person: str | None = "KIA",
    by: str = "TOI",
    expires: datetime | None = None,
    revoked: datetime | None = None,
    accepted: datetime | None = None,
) -> dict:
    return {
        token: [
            invite_id,
            outing,
            source,
            person,
            by,
            accepted,
            "TOI" if accepted else None,
            T - DAY,
            T + 6 * DAY if expires is None else expires,
            revoked,
        ]
    }


def sess(
    alias: str = "SS1",
    *,
    person: str = "TOI",
    via: str = "otp",
    created: datetime | None = None,
    expires: datetime | None = None,
    revoked: datetime | None = None,
    invite_id: str | None = None,
) -> list:
    return [
        alias,
        person,
        invite_id,
        via,
        T - DAY if created is None else created,
        T + 29 * DAY if expires is None else expires,
        revoked,
    ]


def challenge(
    *,
    alias: str = "CH1",
    phone: object = "@SO_A",
    code: object = None,
    expires: datetime | None = None,
    attempts: int = 0,
    consumed: datetime | None = None,
    created: datetime | None = None,
) -> list:
    return [
        alias,
        phone,
        ["CH1", "abcdef"] if code is None else code,
        T - MINUTE if created is None else created,
        T + 4 * MINUTE if expires is None else expires,
        attempts,
        consumed,
    ]


def group_summary(ctx: str = "HOI", membership: str = "MB1") -> list:
    return [
        ctx,
        "Nhóm bạn",
        3,
        "member",
        "active",
        membership,
        T - 20 * DAY,
        ["MB2", "text", "Đi chưa", "KIA", "Bạn Kia", T - HOUR],
        2,
        "bien-dem",
        "group",
        None,
        None,
    ]


def pair_summary(ctx: str = "CAP", other: str = "KIA") -> list:
    return [
        ctx,
        "Bạn Kia",
        2,
        "member",
        "active",
        "MB2",
        T - 10 * DAY,
        None,
        0,
        "bien-dem",
        "pair",
        other,
        "Bạn Kia",
    ]


def auth_steps_edges() -> list[dict]:
    cases: list[dict] = []
    add = cases.append

    live_people = {"KIA": ["Bạn Kia", None], "TOI": ["Tôi", None]}

    # --- POST /sessions
    add(
        step_case(
            "bootstrap_session_from_invite",
            "a named invitation mints a session",
            {"token": "TK_MOI"},
            {"invites": invite(), "outings": {"OU1": "HOI"}, "people": live_people},
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "and answers with the groups the person is in",
            {"token": "TK_MOI"},
            {
                "invites": invite(),
                "outings": {"OU1": "HOI"},
                "people": live_people,
                "summaries": [group_summary(), pair_summary()],
                "edges": {"KIA|KIA": ["accepted", None]},
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "a pair whose counterpart ended their account is unavailable",
            {"token": "TK_MOI"},
            {
                "invites": invite(),
                "outings": {"OU1": "HOI"},
                "people": live_people,
                "summaries": [pair_summary()],
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "an unknown secret",
            {"token": "TK_LA"},
            {"invites": invite(), "outings": {"OU1": "HOI"}},
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "a link invitation names nobody",
            {"token": "TK_MOI"},
            {
                "invites": invite(source="link", person=None),
                "outings": {"OU1": "HOI"},
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "a revoked invitation",
            {"token": "TK_MOI"},
            {"invites": invite(revoked=T - HOUR), "outings": {"OU1": "HOI"}},
        )
    )
    for label, expires in (
        ("one microsecond left", T + MICRO),
        ("exactly at the deadline", T),
        ("one microsecond past the deadline", T - MICRO),
    ):
        add(
            step_case(
                "bootstrap_session_from_invite",
                f"an invitation with {label}",
                {"token": "TK_MOI"},
                {
                    "invites": invite(expires=expires),
                    "outings": {"OU1": "HOI"},
                    "people": live_people,
                },
            )
        )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "the outing behind the invitation is gone",
            {"token": "TK_MOI"},
            {"invites": invite(), "outings": {}},
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "the secret was spent between the read and the write",
            {"token": "TK_MOI"},
            {
                "invites": invite(),
                "outings": {"OU1": "HOI"},
                "conflicts": {
                    "consume_named_invite_secret": ["INVITE_ALREADY_ACCEPTED"]
                },
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "the membership write refuses",
            {"token": "TK_MOI"},
            {
                "invites": invite(),
                "outings": {"OU1": "HOI"},
                "conflicts": {"ensure_invited_membership": ["CONTEXT_NOT_FOUND"]},
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "the person row cannot be read back",
            {"token": "TK_MOI"},
            {"invites": invite(), "outings": {"OU1": "HOI"}, "people": {}},
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "the stored row names another door than the one that minted it",
            {"token": "TK_MOI"},
            {
                "invites": invite(),
                "outings": {"OU1": "HOI"},
                "people": live_people,
                "created_session": ["SSN", "genesis"],
            },
        )
    )
    add(
        step_case(
            "bootstrap_session_from_invite",
            "a clock at the end of the calendar cannot hold the expiry",
            {"token": "TK_MOI"},
            {
                "invites": invite(expires=datetime(9999, 12, 31, tzinfo=UTC)),
                "outings": {"OU1": "HOI"},
                "people": live_people,
            },
            now=datetime(9999, 12, 20, tzinfo=UTC),
        )
    )

    # --- GET /sessions
    add(
        step_case(
            "list_account_sessions",
            "no session at all",
            {"token": None},
            {"sessions": []},
        )
    )
    add(
        step_case(
            "list_account_sessions",
            "three rows and no bearer, so none is current",
            {"token": None},
            {
                "sessions": [
                    sess("SS1"),
                    sess("SS2", via="google"),
                    sess("SSN", via="invite", invite_id="IV1"),
                ]
            },
        )
    )
    add(
        step_case(
            "list_account_sessions",
            "the bearer names one of them",
            {"token": "TK_NAY"},
            {
                "sessions": [sess("SS1"), sess("SS2", via="google")],
                "by_digest": {"TK_NAY": sess("SS2", via="google")},
            },
        )
    )
    add(
        step_case(
            "list_account_sessions",
            "the bearer names a row that is not in the list",
            {"token": "TK_NAY"},
            {"sessions": [sess("SS1")], "by_digest": {"TK_NAY": sess("SSN")}},
        )
    )
    add(
        step_case(
            "list_account_sessions",
            "the bearer names nothing",
            {"token": "TK_LA"},
            {"sessions": [sess("SS1")], "by_digest": {}},
        )
    )
    add(
        step_case(
            "list_account_sessions",
            "a caller with no role",
            {"token": None},
            {"sessions": [sess("SS1")]},
            actor=("TOI", ()),
        )
    )

    # --- DELETE /sessions/current
    add(
        step_case(
            "revoke_session_token",
            "a live token",
            {"token": "TK_NAY"},
            {"by_digest": {"TK_NAY": sess("SS1")}},
        )
    )
    add(
        step_case(
            "revoke_session_token",
            "a token nobody ever minted",
            {"token": "TK_LA"},
            {"by_digest": {}},
        )
    )
    add(
        step_case(
            "revoke_session_token",
            "somebody else's live token is still revoked",
            {"token": "TK_NAY"},
            {"by_digest": {"TK_NAY": sess("SS1", person="KIA")}},
        )
    )

    # --- DELETE /sessions/{id}
    add(
        step_case(
            "revoke_account_session",
            "one of my own",
            {"session_id": "SS1"},
            {"session": sess("SS1")},
        )
    )
    add(
        step_case(
            "revoke_account_session",
            "somebody else's is a 404",
            {"session_id": "SS1"},
            {"session": sess("SS1", person="KIA")},
        )
    )
    add(
        step_case(
            "revoke_account_session",
            "an id that names nothing",
            {"session_id": "SSN"},
            {"session": None},
        )
    )
    add(
        step_case(
            "revoke_account_session",
            "a caller with no role",
            {"session_id": "SS1"},
            {"session": sess("SS1")},
            actor=("TOI", ()),
        )
    )
    add(
        step_case(
            "revoke_account_session",
            "an already revoked row is revoked again",
            {"session_id": "SS1"},
            {"session": sess("SS1", revoked=T - HOUR)},
        )
    )

    # --- POST /auth/otp/request
    add(step_case("request_otp", "the first code for a number", {"phone": "@SO_A"}))
    add(
        step_case(
            "request_otp",
            "the drawn code is six digits wide",
            {"phone": "@SO_A"},
            {"draws": [7]},
        )
    )
    add(
        step_case(
            "request_otp",
            "a debug code replaces the draw entirely",
            {"phone": "@SO_A"},
            {"debug_code": "abcabc"},
        )
    )
    for label, value in (
        ("missing", None),
        ("a number", 7),
        ("a bool", True),
        ("a list", ["@SO_A"]),
        ("an object", {"phone": "@SO_A"}),
    ):
        add(step_case("request_otp", f"phone {label}", {"phone": value}))
    add(step_case("request_otp", "not a mobile", {"phone": "mot hai ba"}))
    add(step_case("request_otp", "a blank number", {"phone": ""}))
    add(
        step_case(
            "request_otp",
            "a number whose last digit is not ASCII",
            {"phone": "@SO_UNI"},
        )
    )
    add(step_case("request_otp", "no identity key", {"phone": "@SO_A"}, {"key": ""}))
    add(
        step_case(
            "request_otp",
            "an identity key too short",
            {"phone": "@SO_A"},
            {"key": "khoa-ngan"},
        )
    )
    add(
        step_case(
            "request_otp",
            "a code was just sent",
            {"phone": "@SO_A"},
            {"recent": [T - SECOND]},
        )
    )
    add(
        step_case(
            "request_otp",
            "exactly at the cooldown",
            {"phone": "@SO_A"},
            {"recent": [T - 60 * SECOND]},
        )
    )
    add(
        step_case(
            "request_otp",
            "five in the window",
            {"phone": "@SO_A"},
            {"recent": [T - (100 + 60 * index) * SECOND for index in range(5)]},
        )
    )
    add(
        step_case(
            "request_otp",
            "the gateway refuses",
            {"phone": "@SO_A"},
            {"sms": "gateway answered 502"},
        )
    )
    add(
        step_case(
            "request_otp",
            "the stored challenge carries another id than the one drawn",
            {"phone": "@SO_A"},
            {"created_challenge": "CHN"},
        )
    )
    add(
        step_case(
            "request_otp",
            "and the refusal is recorded against the stored id",
            {"phone": "@SO_A"},
            {"created_challenge": "CHN", "sms": "no route to host"},
        )
    )
    add(
        step_case(
            "request_otp",
            "the challenge write refuses",
            {"phone": "@SO_A"},
            {"conflicts": {"create_otp_challenge": ["OTP_CHALLENGE_EXISTS"]}},
        )
    )
    add(
        step_case(
            "request_otp",
            "a clock at the end of the calendar cannot hold the expiry",
            {"phone": "@SO_A"},
            now=datetime(9999, 12, 31, 23, 50, tzinfo=UTC),
        )
    )
    add(
        step_case(
            "request_otp",
            "a clock at the start of the calendar cannot look back a window",
            {"phone": "@SO_A"},
            now=datetime(1, 1, 1, 0, 10, tzinfo=UTC),
        )
    )

    # --- POST /auth/otp/verify
    bound = {"identities": {"phone|@SO_A": "KIA"}}
    add(
        step_case(
            "verify_otp",
            "the right code on a live challenge, for a number already bound",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(), **bound, "people": live_people},
        )
    )
    add(
        step_case(
            "verify_otp",
            "a number nobody has used yet mints a person",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(), "people": {}},
        )
    )
    add(
        step_case(
            "verify_otp",
            "a derived id whose person already exists is not new",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(), "people": {"DER": ["Người cũ", None]}},
        )
    )
    add(
        step_case(
            "verify_otp",
            "a derived id whose account ended comes back as somebody else",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(), "people": {"DER": ["Người cũ", T - DAY]}},
        )
    )
    add(
        step_case(
            "verify_otp",
            "two verifies raced on a brand-new number",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {
                "challenge": challenge(),
                "people": {},
                "conflicts": {"create_person": ["PERSON_EXISTS"]},
            },
        )
    )
    add(
        step_case(
            "verify_otp",
            "a wrong code",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "zzzzzz"},
            {"challenge": challenge()},
        )
    )
    for attempts in (3, 4, 5):
        add(
            step_case(
                "verify_otp",
                f"a wrong code with {attempts} attempts already spent",
                {"challenge_id": "CH1", "phone": "@SO_A", "code": "zzzzzz"},
                {"challenge": challenge(attempts=attempts)},
            )
        )
    add(
        step_case(
            "verify_otp",
            "a right code on a challenge already at the ceiling",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(attempts=5), **bound},
        )
    )
    for label, expires in (
        ("one microsecond left", T + MICRO),
        ("exactly at the deadline", T),
        ("one microsecond past", T - MICRO),
    ):
        add(
            step_case(
                "verify_otp",
                f"a right code with {label}",
                {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
                {
                    "challenge": challenge(expires=expires),
                    **bound,
                    "people": live_people,
                },
            )
        )
    add(
        step_case(
            "verify_otp",
            "a right code on a consumed challenge",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(consumed=T - MINUTE), **bound},
        )
    )
    add(
        step_case(
            "verify_otp",
            "no challenge at all",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": None},
        )
    )
    add(
        step_case(
            "verify_otp",
            "the challenge belongs to another number",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(phone="@SO_B")},
        )
    )
    add(
        step_case(
            "verify_otp",
            "a stored phone digest of another length",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(phone="short")},
        )
    )
    add(
        step_case(
            "verify_otp",
            "the stored row carries another id than the one asked for",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(alias="CH2"), **bound, "people": live_people},
        )
    )
    for label, value in (
        ("missing", None),
        ("a number", 7),
        ("a bool", False),
        ("a list", ["abcdef"]),
    ):
        add(
            step_case(
                "verify_otp",
                f"code {label}",
                {"challenge_id": "CH1", "phone": "@SO_A", "code": value},
                {"challenge": challenge()},
            )
        )
    for index, typed in enumerate(CODES):
        add(
            step_case(
                "verify_otp",
                f"code text {index}",
                {"challenge_id": "CH1", "phone": "@SO_A", "code": typed},
                {"challenge": challenge(), **bound, "people": live_people},
            )
        )
    add(
        step_case(
            "verify_otp",
            "the attempt write refuses",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"challenge": challenge(), "conflicts": {"record_otp_attempt": ["GONE"]}},
        )
    )
    add(
        step_case(
            "verify_otp",
            "no identity key",
            {"challenge_id": "CH1", "phone": "@SO_A", "code": "abcdef"},
            {"key": "", "challenge": challenge()},
        )
    )

    # --- POST /auth/google
    vouched = {"google": ["ok", "sub-cua-nguoi-nay", "Người Google"]}
    add(
        step_case(
            "login_with_google",
            "a first token mints a person",
            {"id_token": "the-id-token"},
            vouched,
        )
    )
    add(
        step_case(
            "login_with_google",
            "a sub already bound signs that person in",
            {"id_token": "the-id-token"},
            {
                **vouched,
                "identities": {"google|sub-cua-nguoi-nay": "KIA"},
                "people": live_people,
            },
        )
    )
    add(
        step_case(
            "login_with_google",
            "a token with no name falls back to the placeholder",
            {"id_token": "the-id-token"},
            {"google": ["ok", "sub-cua-nguoi-nay", None]},
        )
    )
    add(
        step_case(
            "login_with_google",
            "a token with a blank name falls back too",
            {"id_token": "the-id-token"},
            {"google": ["ok", "sub-cua-nguoi-nay", ""]},
        )
    )
    add(
        step_case(
            "login_with_google",
            "no verifier on this host",
            {"id_token": "the-id-token"},
            {"google": None},
        )
    )
    for label, value in (
        ("missing", None),
        ("a number", 7),
        ("blank", ""),
        ("only spaces", "   "),
        ("a list", ["the-id-token"]),
    ):
        add(
            step_case(
                "login_with_google", f"id_token {label}", {"id_token": value}, vouched
            )
        )
    add(
        step_case(
            "login_with_google",
            "surrounded by whitespace",
            {"id_token": "  the-id-token\t"},
            vouched,
        )
    )
    add(
        step_case(
            "login_with_google",
            "Google does not vouch for it",
            {"id_token": "the-id-token"},
            {"google": ["bad", "Token expired"]},
        )
    )
    add(
        step_case(
            "login_with_google",
            "two first logins raced and the winner is read back",
            {"id_token": "the-id-token"},
            {
                **vouched,
                "identities": {"google|sub-cua-nguoi-nay": "KIA"},
                "conflicts": {"create_person_with_identity": ["IDENTITY_TAKEN"]},
                "people": live_people,
            },
        )
    )
    add(
        step_case(
            "login_with_google",
            "a conflict with no winner behind it is re-raised",
            {"id_token": "the-id-token"},
            {
                **vouched,
                "conflicts": {"create_person_with_identity": ["PERSON_EXISTS"]},
            },
        )
    )
    return cases


# --- the fuzz ---------------------------------------------------------------

FUZZ_PHONES = [f"@{name}" for name in PHONES] + [
    None,
    7,
    True,
    "",
    "   ",
    "mot hai ba",
    "+84" + _TAILS["SO_A"],
    "84" + _TAILS["SO_B"],
    _TAILS["SO_C"],
    "0" + _TAILS["SO_A"][:8],
    "0" + _TAILS["SO_A"] + "0",
    "0 " + " ".join(_TAILS["SO_A"]),
    "0(" + _TAILS["SO_B"] + ")",
    "0-" + "-".join(_TAILS["SO_C"]),
    "0" + "2" * 9,
    ["0" + _TAILS["SO_A"]],
    {"so": "0" + _TAILS["SO_A"]},
]

FUZZ_KEYS = [KEY, "", "   ", "khoa-ngan", "k" * 32, " " + KEY + " "]


def fuzz_now(rng: random.Random) -> datetime:
    if rng.random() < 0.85:
        return T + timedelta(microseconds=rng.randint(-(10**10), 10**10))
    return random_instant(rng)


def fuzz_world(rng: random.Random, fn: str) -> dict:
    world: dict = {}
    if rng.random() < 0.35:
        world["key"] = rng.choice(FUZZ_KEYS)
    if rng.random() < 0.6:
        world["people"] = {
            alias: [
                rng.choice(["Tôi", "Bạn Kia", "Người cũ", ""]),
                None if rng.random() < 0.7 else T - DAY,
            ]
            for alias in rng.sample(["TOI", "KIA", "DER", "NEW"], rng.randint(1, 3))
        }
    if rng.random() < 0.3:
        rows = []
        if rng.random() < 0.7:
            rows.append(group_summary())
        if rng.random() < 0.7:
            rows.append(pair_summary())
        world["summaries"] = rows
        if rng.random() < 0.5:
            world["edges"] = {
                f"{who}|KIA": [
                    rng.choice(["accepted", "blocked", "declined", "pending"]),
                    rng.choice([None, "TOI", "KIA"]),
                ]
                for who in ("TOI", "KIA", "DER", "NEW")
            }
    if fn == "bootstrap_session_from_invite":
        if rng.random() < 0.85:
            world["invites"] = invite(
                token=rng.choice(list(TOKENS)),
                source=rng.choice(["group", "friend", "link"]),
                person=rng.choice(["KIA", "TOI", None]),
                expires=T + timedelta(microseconds=rng.randint(-(10**9), 10**12)),
                revoked=None if rng.random() < 0.8 else T - HOUR,
                accepted=None if rng.random() < 0.8 else T - HOUR,
            )
        if rng.random() < 0.85:
            world["outings"] = {"OU1": rng.choice(["HOI", "CAP"])}
        if rng.random() < 0.25:
            world["conflicts"] = {
                rng.choice(
                    [
                        "consume_named_invite_secret",
                        "ensure_invited_membership",
                        "create_account_session",
                    ]
                ): [
                    rng.choice(
                        [
                            "INVITE_ALREADY_ACCEPTED",
                            "INVITE_NOT_FOUND",
                            "CONTEXT_NOT_FOUND",
                        ]
                    )
                ]
            }
        if rng.random() < 0.2:
            world["membership"] = [
                rng.choice(["MB1", "MB2"]),
                rng.choice(["invited", "active", "left"]),
            ]
        if rng.random() < 0.2:
            world["created_session"] = ["SSN", rng.choice([None, "genesis", "otp"])]
    elif fn in (
        "list_account_sessions",
        "revoke_session_token",
        "revoke_account_session",
    ):
        world["sessions"] = [
            sess(
                rng.choice(["SS1", "SS2", "SSN"]),
                person=rng.choice(["TOI", "KIA"]),
                via=rng.choice(["invite", "otp", "google", "genesis"]),
                revoked=None if rng.random() < 0.8 else T - HOUR,
            )
            for _ in range(rng.randint(0, 3))
        ]
        if rng.random() < 0.6:
            world["by_digest"] = {
                rng.choice(list(TOKENS)): sess(
                    rng.choice(["SS1", "SS2", "SSN"]), person=rng.choice(["TOI", "KIA"])
                )
            }
        world["session"] = (
            None
            if rng.random() < 0.25
            else sess(
                rng.choice(["SS1", "SS2", "SSN"]),
                person=rng.choice(["TOI", "KIA"]),
                revoked=None if rng.random() < 0.8 else T - HOUR,
            )
        )
        if rng.random() < 0.15:
            world["conflicts"] = {"revoke_account_session": ["SESSION_GONE"]}
    elif fn == "request_otp":
        if rng.random() < 0.6:
            world["recent"] = [
                T + timedelta(microseconds=rng.randint(-(10**9), 10**8))
                for _ in range(rng.randint(0, 7))
            ]
        if rng.random() < 0.2:
            world["sms"] = rng.choice(
                ["gateway answered 502", "URLError", "no route to host"]
            )
        if rng.random() < 0.2:
            world["debug_code"] = rng.choice(["abcabc", "aaaaaa", "abcdef"])
        if rng.random() < 0.3:
            world["draws"] = [rng.choice([0, 7, 99999, 999999, 10**6])]
        if rng.random() < 0.15:
            world["created_challenge"] = rng.choice(["CH1", "CH2", "CHN"])
        if rng.random() < 0.15:
            world["conflicts"] = {
                rng.choice(["create_otp_challenge", "record_otp_attempt"]): [
                    "OTP_CHALLENGE_EXISTS"
                ]
            }
    elif fn == "verify_otp":
        if rng.random() < 0.85:
            world["challenge"] = challenge(
                alias=rng.choice(["CH1", "CH2"]),
                phone=rng.choice(["@SO_A", "@SO_B", "@SO_C", "short", "other"]),
                code=rng.choice(
                    [["CH1", "abcdef"], ["CH2", "abcdef"], ["CH1", "zzzzzz"], "other"]
                ),
                expires=T + timedelta(microseconds=rng.randint(-(10**9), 10**9)),
                attempts=rng.choice([0, 1, 2, 3, 4, 5, 6, -1]),
                consumed=None if rng.random() < 0.75 else T - MINUTE,
            )
        if rng.random() < 0.5:
            world["identities"] = {"phone|@SO_A": rng.choice(["KIA", "TOI", "DER"])}
        if rng.random() < 0.2:
            world["conflicts"] = {
                rng.choice(
                    ["record_otp_attempt", "create_person", "upsert_account_identity"]
                ): ["EXISTS"]
            }
    else:
        answer = rng.random()
        if answer < 0.12:
            world["google"] = None
        elif answer < 0.3:
            world["google"] = [
                "bad",
                rng.choice(["Token expired", "Wrong audience", ""]),
            ]
        else:
            world["google"] = [
                "ok",
                rng.choice(["sub-cua-nguoi-nay", "sub-khac", ""]),
                rng.choice(["Người Google", "", None, "   "]),
            ]
        if rng.random() < 0.4:
            world["identities"] = {
                f"google|{rng.choice(['sub-cua-nguoi-nay', 'sub-khac'])}": rng.choice(
                    ["KIA", "TOI"]
                )
            }
        if rng.random() < 0.25:
            world["conflicts"] = {"create_person_with_identity": ["IDENTITY_TAKEN"]}
    return world


def fuzz_request(rng: random.Random, fn: str) -> dict:
    if fn == "bootstrap_session_from_invite":
        return {"token": rng.choice(list(TOKENS))}
    if fn == "list_account_sessions":
        return {"token": rng.choice([None, *TOKENS])}
    if fn == "revoke_session_token":
        return {"token": rng.choice(list(TOKENS))}
    if fn == "revoke_account_session":
        return {"session_id": rng.choice(["SS1", "SS2", "SSN"])}
    if fn == "request_otp":
        return {"phone": rng.choice(FUZZ_PHONES)}
    if fn == "verify_otp":
        return {
            "challenge_id": rng.choice(["CH1", "CH2", "CHN"]),
            "phone": rng.choice(FUZZ_PHONES),
            "code": rng.choice([*CODES, "abcdef", "zzzzzz", None, 7, True, ["abcdef"]]),
        }
    return {
        "id_token": rng.choice(
            [None, 7, "", "   ", "the-id-token", "  the-id-token  ", ["the-id-token"]]
        )
    }


def auth_steps_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed)
    names = list(CALLERS)
    cases: list[dict] = []
    while len(cases) < count:
        index = len(cases)
        fn = names[index % len(names)] if index < len(names) else rng.choice(names)
        built = step_case(
            fn,
            f"fuzz {index}",
            fuzz_request(rng, fn),
            fuzz_world(rng, fn),
            now=fuzz_now(rng),
            actor=(
                rng.choice(["TOI", "KIA"]),
                tuple(
                    rng.sample(
                        ["member", "advancer", "former_member"], rng.randint(0, 2)
                    )
                ),
            ),
        )
        if fits(built):
            cases.append(built)
    return cases


# ---------------------------------------------------------------------------
# Modes
# ---------------------------------------------------------------------------

FUNCTIONS = {
    "generate_code": otp_generate_code,
    "plan_request": otp_plan_request,
    "plan_verify": otp_plan_verify,
    **{
        name: (lambda name: lambda **kw: run_step(name, **kw))(name) for name in CALLERS
    },
}

#: module -> (Go package path, module object, constants, edges, fuzz,
#:            cases in the committed sample, cases a live oracle run draws)
MODULES = {
    "otp": (
        "internal/domain/otp",
        otp_module,
        otp_constants,
        otp_edges,
        otp_fuzz,
        150,
        20000,
    ),
    "auth_steps": (
        "internal/domain/authsteps",
        api_service,
        auth_steps_constants,
        auth_steps_edges,
        auth_steps_fuzz,
        40,
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
        "generator": "scripts/render_domain_w9_goldens.py",
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
