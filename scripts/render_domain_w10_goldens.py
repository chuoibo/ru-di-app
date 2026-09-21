#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W10 people routes (ADR-0029).

The thirteen W10 routes (services/api/app/api/routes/people.py) reach these
pure Python pieces, each ported to one Go package:

    app.domain.account_lifecycle      services/core/internal/domain/accountlifecycle
    app.domain.direct (pair_key, ...) services/core/internal/domain/direct
    app.api.service (people methods)  services/core/internal/domain/peoplesteps

app.domain.blocking, app.domain.friendship, app.domain.permissions and
app.domain.interests were ported earlier; peoplesteps calls those packages.
direct's other functions were ported in W3 (scripts/render_domain_w3_goldens.py)
and keep their goldens there.

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each returned or raised. The service methods run as real `ApiService`
methods over a recording stub repository and a recording stub photo storage,
with the clock pinned to the case's `now`: the golden holds the answer (or the
ApiProblem, or the exception that would be a 500), every repository and
storage call the method made with its arguments in order, and what the service
logged. Each package's oracle_test.go replays every case. The image is the one
scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w10_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w10_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases;
`<module>-fuzz-0` is a small sample of that module's seeded fuzz. Both are
committed and replayed by plain `go test`. `--live MODULE SEED COUNT` draws
COUNT cases with SEED and prints them without the guard's checks; the Go tests
built with `-tags oracle` run it inside the image at test time (see
internal/oracletest/live.go). The committed sample is the first draw of the
same generator.

## Encoding

The encoding of scripts/render_domain_w8_goldens.py (`{"fn", "name", "args",
"result"}`, `"$i:<hex>"` for a large int, `"$sp:<text>"` for a str cut into
groups of six code points, a uuid.UUID as its name in ALIASES, a datetime as its
isoformat()), with these shapes:

* a dict whose key order matters (the person dict anonymised_person takes and
  returns, the changes dict update_person_profile receives) is a list of
  `[key, value]` pairs;
* a person record is the list of its values in PERSON_KEYS order, a context
  summary record the list in SUMMARY_KEYS order;
* the world a service case runs in lists only what differs from
  `world_defaults` in the people_steps constants.

A service case's result is `{"ok": {"calls", "problem", "raised", "response",
"logs"}}`: `calls` is `[method, argument, ...]` per repository call with
arguments in the Protocol's order (a photo storage call is named
`photo_storage.delete`), `problem` the ApiProblem, `raised` the class, the
`.code` (None when the class has none) and str() of an exception Python
answers with a 500, `response` the response model's
model_dump() (a `(body, created)` tuple is `{"body", "created"}`, a dataclass
its fields), `logs` every `[level, message]` the service logged.
"""

from __future__ import annotations

import dataclasses
import json
import logging
import platform
import random
import re
import sys
import uuid
from datetime import UTC, datetime, timedelta, timezone

sys.path.insert(0, "/srv")

from pydantic import ValidationError  # noqa: E402

from app.api import schemas  # noqa: E402
from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem, RepositoryConflict  # noqa: E402
from app.api.repository import (  # noqa: E402
    ContextRecord,
    ErasureReport,
    FriendEdgeRecord,
    LastMessageRecord,
    PersonContextSummaryRecord,
    PersonRecord,
    PlaceRecord,
    ProfileCounts,
    SavedPlaceRecord,
)
from app.domain import account_lifecycle, direct  # noqa: E402
from app.domain.friendship import FriendshipError  # noqa: E402
from app.domain.interests import INTEREST_IDS, InterestError  # noqa: E402
from app.domain.permissions import PermissionError_  # noqa: E402

SEED = 10
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


#: Every uuid a case names. People first: ME asks; BAN sorts before ME and
#: BAN2 after it, so a pair key is built both ways round; XOA is an ended
#: account; MAT names nobody. Then contexts (PXN is what create_pair_context
#: hands back), memberships, a message, edges and saved-place rows (SPN is
#: what save_place hands back).
ALIASES = {
    "ME": _uuid_for("b", "1"),
    "BAN": _uuid_for("a", "2"),
    "BAN2": _uuid_for("c", "3"),
    "LA": _uuid_for("d", "4"),
    "XOA": _uuid_for("e", "5"),
    "MAT": _uuid_for("f", "6"),
    "CX1": _uuid_for("a", "7"),
    "CX2": _uuid_for("b", "8"),
    "PX1": _uuid_for("c", "9"),
    "PXN": _uuid_for("d", "1"),
    "M1": _uuid_for("e", "2"),
    "M2": _uuid_for("f", "3"),
    "M3": _uuid_for("a", "4"),
    "MS1": _uuid_for("b", "5"),
    "FE1": _uuid_for("c", "6"),
    "FB1": _uuid_for("d", "7"),
    "FB2": _uuid_for("e", "8"),
    "SP1": _uuid_for("f", "9"),
    "SP2": _uuid_for("a", "1"),
    "SPN": _uuid_for("b", "2"),
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
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, int):
        return value if -SMALL_INT < value < SMALL_INT else f"$i:{value:#_x}"
    if isinstance(value, uuid.UUID):
        return ALIAS_OF.get(value) or enc_str(str(value))
    if isinstance(value, datetime):
        return enc_str(value.isoformat())
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
    except (account_lifecycle.AccountLifecycleError, FriendshipError) as exc:
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


CONSTANT_TYPES = (str, int, bool, tuple, frozenset, dict)


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

MICRO = timedelta(microseconds=1)
HOUR = timedelta(hours=1)
DAY = timedelta(days=1)

T = datetime(2030, 9, 18, 5, 0, tzinfo=UTC)
JOINED = datetime(2029, 1, 2, 3, 4, 5, 678901, tzinfo=UTC)
EARLIER = datetime(2030, 9, 1, 8, 30, tzinfo=UTC)

VIETNAM = timezone(timedelta(hours=7))
HONOLULU = timezone(timedelta(hours=-10))
KIRITIMATI = timezone(timedelta(hours=14))
KATHMANDU = timezone(timedelta(hours=5, minutes=45))
SAIGON_LMT = timezone(timedelta(hours=7, minutes=6, seconds=30))
ONE_SECOND_WEST = timezone(timedelta(seconds=-1))
OFFSETS = (UTC, VIETNAM, HONOLULU, KIRITIMATI, KATHMANDU, SAIGON_LMT, ONE_SECOND_WEST)

E_DOT_CIRCUMFLEX = chr(0x1EC7)
E_DOT_BELOW = chr(0x1EB9)
COMBINING_DOT_BELOW = chr(0x323)
COMBINING_CIRCUMFLEX = chr(0x302)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
NO_BREAK_SPACE = chr(0xA0)
IDEOGRAPHIC_SPACE = chr(0x3000)
ZERO_WIDTH_SPACE = chr(0x200B)
NEXT_LINE = chr(0x85)
FILE_SEPARATOR = "\x1c"
COMBINING_ACUTE = chr(0x301)
RIGHT_TO_LEFT_OVERRIDE = chr(0x202E)
LAST_BMP = chr(0xFFFF)

#: «Việt» composed, and the same letters decomposed: equal to a reader, not
#: to `==`.
VIET_NFC = "Vi" + E_DOT_CIRCUMFLEX + "t"
VIET_NFD = "Vie" + COMBINING_DOT_BELOW + COMBINING_CIRCUMFLEX + "t"

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
    "'; DROP TABLE people; --",
    "a" * 200,
    " " + "b" * 200 + " ",
    "c" * 201,
    "d" * 500,
    "e" * 501,
    "Ng" + E_DOT_CIRCUMFLEX + "p " * 60,
    "$sp:not|an|encoding",
    "$i:0x1",
    VIET_NFC,
    VIET_NFD,
)


def random_text(rng: random.Random, most: int = 6) -> str:
    alphabet = (
        "a",
        "B",
        " ",
        "-",
        ":",
        "\t",
        "$",
        NO_BREAK_SPACE,
        IDEOGRAPHIC_SPACE,
        ZERO_WIDTH_SPACE,
        FILE_SEPARATOR,
        E_DOT_CIRCUMFLEX,
        COMBINING_ACUTE,
        CJK_MIDDLE,
        GRINNING_FACE,
        LAST_BMP,
        "\x00",
    )
    return "".join(rng.choice(alphabet) for _ in range(rng.randrange(most + 1)))


# ---------------------------------------------------------------------------
# account_lifecycle
# ---------------------------------------------------------------------------


def al_anonymised(person: list, now: datetime) -> dict:
    """anonymised_person over a dict built from ordered pairs; says whether
    the caller's dict came back untouched."""
    row = dict((key, value) for key, value in person)
    snapshot = [[key, value] for key, value in row.items()]
    after = account_lifecycle.anonymised_person(row, now)
    return {
        "row": [[key, value] for key, value in after.items()],
        "input_unchanged": [[key, value] for key, value in row.items()] == snapshot,
    }


def account_lifecycle_constants() -> dict:
    return {
        "names": public_names(account_lifecycle),
        "anonymous_display_name": account_lifecycle.ANONYMOUS_DISPLAY_NAME,
        "confirmation_required": account_lifecycle.CONFIRMATION_REQUIRED,
        "erasure": [
            [action, list(tables)]
            for action, tables in account_lifecycle.ERASURE.items()
        ],
        "money_tables": list(account_lifecycle.MONEY_TABLES),
        "others_keep_tables": list(account_lifecycle.OTHERS_KEEP_TABLES),
    }


ODD_ACTIONS = (
    "",
    "DELETE",
    "delete ",
    " keep",
    "purge",
    "anonymize",
    "Anonymise",
    "untouched\x00",
    "k" + chr(0xE9) + "ep",
    "revoke" + ZERO_WIDTH_SPACE,
    RIGHT_TO_LEFT_OVERRIDE + "keep",
    "keep" * 50,
    "$sp:keep",
    "leave\n",
    "__class__",
    "items",
)

CONFIRMATIONS = (
    True,
    False,
    None,
    0,
    1,
    -1,
    2**70,
    "true",
    "True",
    "1",
    "",
    [],
    {},
    [True],
    {"confirm": True},
)

#: The dict erase_person passes: six fields, no id, no created_at.
REPO_PERSON = [
    ["display_name", "Minh"],
    ["bio", "thích cà phê"],
    ["city", "Đà Lạt"],
    ["budget_band", "vua-phai"],
    ["discoverable_by_phone", True],
    ["wall_comment_policy", "readers"],
]

#: The dict tests/domain/test_account_lifecycle.py passes.
TEST_PERSON = [
    ["id", "person-1"],
    ["display_name", "Minh"],
    ["bio", "thích cà phê"],
    ["city", "Đà Lạt"],
    ["budget_band", "vua-phai"],
    ["discoverable_by_phone", True],
    ["wall_comment_policy", "readers"],
    ["created_at", datetime(2026, 9, 6, 12, tzinfo=UTC)],
    ["deleted_at", None],
]

PERSONS = (
    REPO_PERSON,
    TEST_PERSON,
    [],
    [
        ["deleted_at", T - DAY],
        ["notify_prefs", {"tin_nhan": True, "moi": [1, 2]}],
        ["wall_comment_policy", "friends"],
        ["id", str(U("ME"))],
        ["counts", [1, 2]],
    ],
    [
        ["display_name", "<script>alert(1)</script>"],
        ["bio", RIGHT_TO_LEFT_OVERRIDE + "abc"],
        ["city", CJK_MIDDLE + GRINNING_FACE],
        ["discoverable_by_phone", None],
        ["$note", "a key starting with a dollar"],
    ],
    [
        ["display_name", account_lifecycle.ANONYMOUS_DISPLAY_NAME],
        ["bio", None],
        ["city", None],
        ["budget_band", None],
        ["discoverable_by_phone", False],
        ["wall_comment_policy", "nobody"],
        ["deleted_at", EARLIER],
    ],
    [["zzz", 2**70], ["", ""], ["display_name", ""], ["DISPLAY_NAME", "x"]],
)

NOWS = (
    T,
    T.astimezone(VIETNAM),
    datetime(2030, 9, 18, 5, 0, 0, 1, tzinfo=KATHMANDU),
    T.astimezone(SAIGON_LMT),
    T.astimezone(ONE_SECOND_WEST),
    datetime(1, 1, 1, tzinfo=UTC),
    datetime(9999, 12, 31, 23, 59, 59, 999999, tzinfo=KIRITIMATI),
    T.replace(tzinfo=None),
    datetime(2030, 9, 18, 5, 0, 0, 123456),
)


def account_lifecycle_edges() -> list[dict]:
    out = []
    for k, action in enumerate((*account_lifecycle.ERASURE, *ODD_ACTIONS)):
        out.append(case("tables_for", f"tables_for/{k}", {"action": action}))
    for k, confirm in enumerate(CONFIRMATIONS):
        out.append(
            case("check_confirmation", f"check_confirmation/{k}", {"confirm": confirm})
        )
    for p, person in enumerate(PERSONS):
        for n, now in enumerate(NOWS):
            name = f"anonymised_person/{p}/{n}"
            out.append(case("anonymised_person", name, {"person": person, "now": now}))
    return out


PERSON_KEYS_POOL = (
    "id",
    "display_name",
    "bio",
    "city",
    "budget_band",
    "discoverable_by_phone",
    "wall_comment_policy",
    "created_at",
    "deleted_at",
    "notify_prefs",
    "avatar",
    "",
    "Bio",
)


def random_value(rng: random.Random) -> object:
    return rng.choice(
        (
            None,
            True,
            False,
            0,
            rng.randint(-(2**70), 2**70),
            random_text(rng),
            rng.choice(TEXTS),
            T + timedelta(seconds=rng.randint(-(10**8), 10**8)),
            [rng.randint(0, 9), random_text(rng, 2)],
            {"k": random_text(rng, 2)},
        )
    )


def random_now(rng: random.Random) -> datetime:
    moment = T + timedelta(
        days=rng.randint(-4000, 4000),
        seconds=rng.randint(0, 86399),
        microseconds=rng.choice((0, rng.randint(0, 999999))),
    )
    if rng.random() < 0.06:
        return moment.replace(tzinfo=None)
    return moment.astimezone(rng.choice(OFFSETS))


def mutate_action(rng: random.Random) -> str:
    action = rng.choice(tuple(account_lifecycle.ERASURE))
    roll = rng.random()
    if roll < 0.5:
        return action
    if roll < 0.65:
        return action.upper() if rng.random() < 0.5 else action.capitalize()
    if roll < 0.8:
        return rng.choice((" ", "\t", ZERO_WIDTH_SPACE, "s", "\x00")) + action
    if roll < 0.9:
        return action[: rng.randrange(len(action))]
    return random_text(rng)


def account_lifecycle_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 10)
    out = []
    for i in range(count):
        roll = rng.random()
        if roll < 0.3:
            built = case("tables_for", f"fuzz/{i}", {"action": mutate_action(rng)})
        elif roll < 0.45:
            confirm = rng.choice((*CONFIRMATIONS, random_value(rng)))
            built = case("check_confirmation", f"fuzz/{i}", {"confirm": confirm})
        else:
            keys = rng.sample(
                PERSON_KEYS_POOL, rng.randrange(len(PERSON_KEYS_POOL) + 1)
            )
            person = [[key, random_value(rng)] for key in keys]
            args = {"person": person, "now": random_now(rng)}
            built = case("anonymised_person", f"fuzz/{i}", args)
        out.append(built)
    return out


# ---------------------------------------------------------------------------
# direct (the functions the direct-message route reaches)
# ---------------------------------------------------------------------------


def direct_people_constants() -> dict:
    return {
        "names": public_names(direct),
        "kinds": list(direct.KINDS),
        "roster_only_doors": list(direct.ROSTER_ONLY_DOORS),
    }


PAIR_IDS = (
    str(U("ME")),
    str(U("BAN")),
    str(U("BAN2")),
    "",
    "a",
    "A",
    "ab",
    "abc",
    "x:y",
    "\x00",
    chr(0xE9),
    "e" + COMBINING_ACUTE,
    E_DOT_CIRCUMFLEX,
    LAST_BMP,
    GRINNING_FACE,
    "$sp:x",
)

KIND_VALUES = (
    "group",
    "pair",
    "Pair",
    "pair ",
    " pair",
    "",
    "dm",
    "pair\x00",
    None,
    0,
    1,
    True,
    ["pair"],
    {"pair": "pair"},
)


def can_open_args(values: tuple) -> list[dict]:
    """Every subset of the keyword arguments, every truth value of each."""
    out = []
    for is_friend in values:
        out.append({"is_friend": is_friend})
        for other_exists in values:
            out.append({"is_friend": is_friend, "other_exists": other_exists})
            for other_deleted in values:
                out.append({"is_friend": is_friend, "other_deleted": other_deleted})
                out.append(
                    {
                        "is_friend": is_friend,
                        "other_exists": other_exists,
                        "other_deleted": other_deleted,
                    }
                )
    return out


def direct_people_edges() -> list[dict]:
    out = []
    for i, a in enumerate(PAIR_IDS):
        for j, b in enumerate(PAIR_IDS):
            out.append(case("pair_key", f"pair_key/{i}/{j}", {"a": a, "b": b}))
    seen = set()
    for k, args in enumerate(can_open_args((True, False))):
        key = tuple(sorted(args.items()))
        if key in seen:
            continue
        seen.add(key)
        out.append(case("can_open", f"can_open/{k}", args))
    for k, value in enumerate(KIND_VALUES):
        out.append(case("is_kind", f"is_kind/{k}", {"value": value}))
    return out


def direct_people_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 11)
    out = []
    for i in range(count):
        roll = rng.random()
        if roll < 0.6:

            def pick() -> str:
                if rng.random() < 0.4:
                    return rng.choice(PAIR_IDS)
                if rng.random() < 0.3:
                    return str(uuid.UUID(int=rng.getrandbits(128)))
                return random_text(rng, 4)

            a = pick()
            b = a if rng.random() < 0.1 else pick()
            built = case("pair_key", f"fuzz/{i}", {"a": a, "b": b})
        elif roll < 0.8:
            args = {"is_friend": rng.random() < 0.5}
            for key in ("other_exists", "other_deleted"):
                if rng.random() < 0.7:
                    args[key] = rng.random() < 0.5
            built = case("can_open", f"fuzz/{i}", args)
        else:
            value = rng.choice((*KIND_VALUES, random_text(rng, 5)))
            built = case("is_kind", f"fuzz/{i}", {"value": value})
        out.append(built)
    return out


# ---------------------------------------------------------------------------
# people_steps: the thirteen people methods of ApiService
# ---------------------------------------------------------------------------


class Unscripted(BaseException):
    """A repository call the world did not script: the corpus is wrong."""


PERSON_KEYS = (
    "id",
    "display_name",
    "created_at",
    "bio",
    "city",
    "budget_band",
    "wall_comment_policy",
    "discoverable_by_phone",
    "deleted_at",
)
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
POLICIES = ("readers", "friends", "nobody")
THEMES = ("mac-dinh", "hoang-hon", "bien-dem", "rung-thong", "ruc-ro")
MESSAGE_KINDS = ("text", "image", "ai_card", "sticker", "deleted")
ANONYMOUS = account_lifecycle.ANONYMOUS_DISPLAY_NAME


def person(
    pid: str,
    name: str,
    *,
    bio=None,
    city=None,
    band=None,
    policy="readers",
    phone=True,
    deleted=None,
    created=JOINED,
) -> list:
    return [pid, name, created, bio, city, band, policy, phone, deleted]


def erased(pid: str, deleted=T - DAY) -> list:
    return person(pid, ANONYMOUS, policy="nobody", phone=False, deleted=deleted)


def default_people() -> dict:
    return {
        "ME": person("ME", "Tôi", bio="Thích cà phê", city="Đà Lạt", band="vua-phai"),
        "BAN": person("BAN", "Bạn"),
        "BAN2": person("BAN2", "Bạn hai", city="Huế", policy="friends"),
        "LA": person("LA", "Người lạ", phone=False),
        "XOA": erased("XOA"),
    }


PLACES = (
    ["pho-ha-noi", "Phở Hà Nội", "an-uong"],
    ["cafe-doc", "Cà phê Dốc", "cafe"],
)

WORLD_DEFAULTS = {
    "people": default_people(),
    "friends": [["ME", "BAN"], ["ME", "BAN2"]],
    "groupmates": [["ME", "LA"]],
    "edges": [["ME", "BAN", "accepted", None], ["BAN2", "ME", "accepted", "ME"]],
    "summaries": [],
    "pair_contexts": [],
    "counts": [3, 2, 1, 0, 4],
    "providers": ["phone"],
    "interests": ["cafe", "an-uong"],
    "saved": [],
    "places": [list(row) for row in PLACES],
    "save_created": True,
    "unsave": True,
    "blocked": [],
    "storage_keys": [],
    "photo": [],
    "conflicts": {},
}


def person_record(row: list | None) -> PersonRecord | None:
    if row is None:
        return None
    pid, name, created, bio, city, band, policy, phone, deleted = row
    return PersonRecord(
        id=U(pid),
        display_name=name,
        created_at=created,
        bio=bio,
        city=city,
        budget_band=band,
        wall_comment_policy=policy,
        discoverable_by_phone=phone,
        deleted_at=deleted,
    )


def summary_record(row: list) -> PersonContextSummaryRecord:
    cid, name, count, role, state, membership, joined, last, unread = row[:9]
    theme, kind, counterpart, counterpart_name = row[9:]
    return PersonContextSummaryRecord(
        id=U(cid),
        display_name=name,
        member_count=count,
        my_role=role,
        my_state=state,
        membership_id=U(membership),
        joined_at=joined,
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
        unread_count=unread,
        theme=theme,
        kind=kind,
        counterpart_id=UN(counterpart),
        counterpart_display_name=counterpart_name,
    )


def place_record(place_id: str, name: str, category: str) -> PlaceRecord:
    return PlaceRecord(
        id=place_id,
        destination_id="da-lat",
        name=name,
        category=category,
        kinds=[],
        address=None,
        lat=0.0,
        lng=0.0,
        rating=None,
        rating_count=None,
        price_min_vnd=None,
        price_max_vnd=None,
        open_hours=None,
        open_now=None,
        travel_minutes=None,
        distance_km=None,
        photo_count=0,
        traits=[],
        group_fit=None,
        flag=None,
        description=None,
        reviews=None,
        source="stub",
        source_ref=None,
        license=None,
    )


def pair_in(pairs: list, a: uuid.UUID, b: uuid.UUID) -> bool:
    return any((U(x) == a and U(y) == b) or (U(x) == b and U(y) == a) for x, y in pairs)


def any_edge(blocker_id, addressee_id, now) -> FriendEdgeRecord:
    return FriendEdgeRecord(
        id=U("FE1"),
        requester_id=blocker_id,
        addressee_id=addressee_id,
        other_person_id=addressee_id,
        other_display_name="",
        state="blocked",
        decided_by_id=blocker_id,
        created_at=now,
        decided_at=now,
    )


PHOTO_ERRORS = {
    "OSError": OSError,
    "FileNotFoundError": FileNotFoundError,
    "PermissionError": PermissionError,
}


class Stub:
    """A repository that answers from the case's world and records every call."""

    def __init__(self, world: dict, now: datetime):
        self.world = {**WORLD_DEFAULTS, **world}
        self.now = now
        self.calls: list = []
        self.pair_contexts = list(self.world["pair_contexts"])
        self.photo = list(self.world["photo"])
        self.conflicts = {
            key: list(codes) for key, codes in self.world["conflicts"].items()
        }

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

    def person(self, person_id) -> PersonRecord | None:
        return person_record(self.world["people"].get(ALIAS_OF[person_id]))

    def get_person(self, person_id):
        self.rec("get_person", person_id)
        return self.person(person_id)

    def create_person(self, person_id, display_name):
        self.rec("create_person", person_id, display_name)
        self.maybe_conflict("create_person")
        return PersonRecord(
            id=person_id, display_name=display_name, created_at=self.now
        )

    def rename_person(self, person_id, display_name):
        self.rec("rename_person", person_id, display_name)
        if "renamed" in self.world:
            return person_record(self.world["renamed"])
        current = self.person(person_id)
        if current is None:
            return None
        return dataclasses.replace(current, display_name=display_name)

    def get_pair_context(self, pair_key):
        self.rec("get_pair_context", pair_key)
        if not self.pair_contexts:
            raise Unscripted("pair_contexts")
        alias = self.pair_contexts.pop(0)
        if alias is None:
            return None
        return ContextRecord(
            id=U(alias),
            display_name="",
            created_by_id=U("BAN"),
            created_at=EARLIER,
            kind="pair",
            pair_key=pair_key,
        )

    def create_pair_context(self, *, pair_key, member_ids, created_by_id, now):
        self.rec("create_pair_context", pair_key, list(member_ids), created_by_id, now)
        self.maybe_conflict("create_pair_context")
        return ContextRecord(
            id=U("PXN"),
            display_name="",
            created_by_id=created_by_id,
            created_at=now,
            kind="pair",
            pair_key=pair_key,
        )

    def list_person_context_summaries(self, person_id):
        self.rec("list_person_context_summaries", person_id)
        return [summary_record(row) for row in self.world["summaries"]]

    def update_person_profile(self, person_id, *, changes):
        self.rec(
            "update_person_profile",
            person_id,
            [[key, value] for key, value in changes.items()],
        )
        self.maybe_conflict("update_person_profile")
        if "updated" in self.world:
            return person_record(self.world["updated"])
        current = self.person(person_id)
        if current is None:
            return None
        return dataclasses.replace(current, **changes)

    def profile_counts(self, person_id):
        self.rec("profile_counts", person_id)
        return ProfileCounts(*self.world["counts"])

    def list_login_providers(self, person_id):
        self.rec("list_login_providers", person_id)
        return list(self.world["providers"])

    def list_person_interests(self, person_id):
        self.rec("list_person_interests", person_id)
        return list(self.world["interests"])

    def are_friends(self, a, b):
        self.rec("are_friends", a, b)
        return pair_in(self.world["friends"], a, b)

    def share_active_context(self, a, b):
        self.rec("share_active_context", a, b)
        return pair_in(self.world["groupmates"], a, b)

    def get_place(self, place_id):
        self.rec("get_place", place_id)
        for row in self.world["places"]:
            if row[0] == place_id:
                return place_record(*row)
        return None

    def list_saved_places(self, person_id):
        self.rec("list_saved_places", person_id)
        return [
            SavedPlaceRecord(
                id=U(row[0]), person_id=person_id, place_id=row[1], created_at=row[2]
            )
            for row in self.world["saved"]
        ]

    def save_place(self, person_id, place_id, now):
        self.rec("save_place", person_id, place_id, now)
        self.maybe_conflict("save_place")
        created = self.world["save_created"]
        record = SavedPlaceRecord(
            id=U("SPN"),
            person_id=person_id,
            place_id=place_id,
            created_at=now if created else EARLIER,
        )
        return record, created

    def unsave_place(self, person_id, place_id):
        self.rec("unsave_place", person_id, place_id)
        return self.world["unsave"]

    def open_block_edge(self, *, blocker_id, addressee_id, now):
        self.rec("open_block_edge", blocker_id, addressee_id, now)
        self.maybe_conflict("open_block_edge")
        return any_edge(blocker_id, addressee_id, now)

    def lift_block_edge(self, *, blocker_id, addressee_id, now):
        self.rec("lift_block_edge", blocker_id, addressee_id, now)
        self.maybe_conflict("lift_block_edge")
        return any_edge(blocker_id, addressee_id, now)

    def list_blocked(self, person_id):
        self.rec("list_blocked", person_id)
        return [
            FriendEdgeRecord(
                id=U(row[0]),
                requester_id=person_id,
                addressee_id=U(row[1]),
                other_person_id=U(row[1]),
                other_display_name=row[2],
                state="blocked",
                decided_by_id=person_id,
                created_at=row[3],
                decided_at=row[4],
            )
            for row in self.world["blocked"]
        ]

    def erase_person(self, person_id, *, now):
        self.rec("erase_person", person_id, now)
        self.maybe_conflict("erase_person")
        return ErasureReport(counts={}, storage_keys=tuple(self.world["storage_keys"]))

    def get_friend_edge(self, a, b):
        self.rec("get_friend_edge", a, b)
        for row in self.world["edges"]:
            x, y = U(row[0]), U(row[1])
            if not ((x == a and y == b) or (x == b and y == a)):
                continue
            requester, addressee = (
                (row[4], row[5]) if len(row) == 6 else (row[0], row[1])
            )
            return FriendEdgeRecord(
                id=U("FE1"),
                requester_id=U(requester),
                addressee_id=U(addressee),
                other_person_id=U(addressee) if U(requester) == a else U(requester),
                other_display_name="",
                state=row[2],
                decided_by_id=UN(row[3]),
                created_at=EARLIER,
                decided_at=None,
            )
        return None


class Photos:
    """A photo storage that records every unlink and answers from the world."""

    def __init__(self, stub: Stub):
        self.stub = stub

    def delete(self, storage_key: str) -> bool:
        self.stub.rec("photo_storage.delete", storage_key)
        outcome = self.stub.photo.pop(0) if self.stub.photo else True
        if isinstance(outcome, str):
            raise PHOTO_ERRORS[outcome](storage_key)
        return outcome


class Capture(logging.Handler):
    def __init__(self):
        super().__init__()
        self.records: list = []

    def emit(self, record: logging.LogRecord) -> None:
        self.records.append([record.levelname, record.getMessage()])


CAPTURE = Capture()
_SERVICE_LOGGER = logging.getLogger(api_service.__name__)
_SERVICE_LOGGER.addHandler(CAPTURE)
_SERVICE_LOGGER.setLevel(logging.DEBUG)
_SERVICE_LOGGER.propagate = False


def dump(response: object) -> object:
    if response is None:
        return None
    if isinstance(response, tuple):
        body, created = response
        return {"body": dump(body), "created": created}
    if dataclasses.is_dataclass(response):
        return {f.name: getattr(response, f.name) for f in dataclasses.fields(response)}
    return response.model_dump()


CALLERS = {
    "list_my_contexts": lambda s, a, r: s.list_my_contexts(a),
    "get_my_profile": lambda s, a, r: s.get_my_profile(a),
    "update_my_profile": lambda s, a, r: s.update_my_profile(
        schemas.ProfileUpdateRequest.model_construct(**dict(r["patch"])), a
    ),
    "list_saved_places": lambda s, a, r: s.list_saved_places(a),
    "save_place": lambda s, a, r: s.save_place(r["place_id"], a),
    "unsave_place": lambda s, a, r: s.unsave_place(r["place_id"], a),
    "list_blocked_people": lambda s, a, r: s.list_blocked_people(a),
    "delete_own_account": lambda s, a, r: s.delete_own_account(
        schemas.AccountDeleteRequest.model_construct(confirm=r["confirm"]), a
    ),
    "block_person": lambda s, a, r: s.block_person(U(r["person_id"]), a),
    "unblock_person": lambda s, a, r: s.unblock_person(U(r["person_id"]), a),
    "open_direct_message": lambda s, a, r: s.open_direct_message(U(r["person_id"]), a),
    "get_person_profile": lambda s, a, r: s.get_person_profile(U(r["person_id"]), a),
    "register_person": lambda s, a, r: s.register_person(
        U(r["person_id"]), r["display_name"], a
    ),
}


def run_step(fn: str, now: datetime, actor: list, req: dict, world: dict) -> dict:
    api_service._now = lambda: now
    stub = Stub(world, now)
    who = Actor(id=U(actor[0]), roles=frozenset(actor[1]), context_ids=frozenset())
    CAPTURE.records = []
    out = {
        "calls": stub.calls,
        "problem": None,
        "raised": None,
        "response": None,
        "logs": CAPTURE.records,
    }
    try:
        response = CALLERS[fn](
            api_service.ApiService(stub, photo_storage=Photos(stub)), who, req
        )
    except ApiProblem as exc:
        out["problem"] = {
            "status": exc.status_code,
            "code": exc.code,
            "detail": exc.detail,
        }
    except ValidationError:
        raise
    except (
        RepositoryConflict,
        AssertionError,
        PermissionError_,
        FriendshipError,
        InterestError,
        ValueError,
    ) as exc:
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
    actor=("ME", ("member",)),
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


# --- worlds ------------------------------------------------------------------


def summary(
    cid: str,
    name: str,
    *,
    count=2,
    role="member",
    state="active",
    membership="M1",
    joined=JOINED,
    last=None,
    unread=0,
    theme="mac-dinh",
    kind="group",
    cp=None,
    cp_name=None,
) -> list:
    return [
        cid,
        name,
        count,
        role,
        state,
        membership,
        joined,
        last,
        unread,
        theme,
        kind,
        cp,
        cp_name,
    ]


def last_message(
    author="BAN", name="Bạn", kind="text", preview="Chào nhé", created=T - HOUR
) -> list:
    return ["MS1", kind, preview, author, name, created]


def pair_row(cid: str, cp: str | None, cp_name: str | None = "Bạn", **kw) -> list:
    return summary(cid, cp_name or "", kind="pair", cp=cp, cp_name=cp_name, **kw)


def people_with(**rows) -> dict:
    """The default people with some rows replaced; a None row removes one."""
    people = default_people()
    for alias, row in rows.items():
        if row is None:
            people.pop(alias, None)
        else:
            people[alias] = row
    return people


DOOR_ACTORS = (
    ("no_role", ("ME", ())),
    ("unknown_role", ("ME", ("member", "BOGUS"))),
    ("admin_only", ("ME", ("group_admin",))),
)

DEFAULT_REQ = {
    "list_my_contexts": {},
    "get_my_profile": {},
    "update_my_profile": {"patch": [["bio", "Mới"]]},
    "list_saved_places": {},
    "save_place": {"place_id": "pho-ha-noi"},
    "unsave_place": {"place_id": "pho-ha-noi"},
    "list_blocked_people": {},
    "delete_own_account": {"confirm": True},
    "block_person": {"person_id": "LA"},
    "unblock_person": {"person_id": "BAN"},
    "open_direct_message": {"person_id": "BAN"},
    "get_person_profile": {"person_id": "BAN"},
    "register_person": {"person_id": "MAT", "display_name": "Người mới"},
}

DOOR_WORLDS = {
    "unblock_person": {"edges": [["ME", "BAN", "blocked", "ME"]]},
    "open_direct_message": {
        "pair_contexts": ["PX1"],
        "summaries": [pair_row("PX1", "BAN")],
    },
    "list_saved_places": {"saved": [["SP1", "pho-ha-noi", EARLIER]]},
    "delete_own_account": {"storage_keys": ["anh/me/mot.jpg"]},
    "list_blocked_people": {"blocked": [["FB1", "LA", "Người lạ", EARLIER, None]]},
}


def people_steps_constants() -> dict:
    return {
        "aliases": [[name, str(value)] for name, value in ALIASES.items()],
        "methods": list(CALLERS),
        "world_defaults": WORLD_DEFAULTS,
        "earlier": EARLIER,
        "interest_ids": list(INTEREST_IDS),
    }


def people_steps_edges() -> list[dict]:
    out = []
    S = step_case

    # --- the permission door every method has ------------------------------
    for fn in CALLERS:
        for label, actor in DOOR_ACTORS:
            out.append(
                S(
                    fn,
                    f"door/{label}",
                    DEFAULT_REQ[fn],
                    DOOR_WORLDS.get(fn),
                    actor=actor,
                )
            )
        out.append(S(fn, "door/happy", DEFAULT_REQ[fn], DOOR_WORLDS.get(fn)))

    # --- list_my_contexts ----------------------------------------------------
    fn = "list_my_contexts"
    group = summary("CX1", "Hội đi Đà Lạt", count=5, last=last_message(), unread=3)
    invited = summary(
        "CX2", "Nhóm mới", state="invited", membership="M2", joined=None, role="admin"
    )
    worlds = {
        "empty": {},
        "groups_only": {"summaries": [group, invited]},
        "pair_friend_alive": {"summaries": [pair_row("PX1", "BAN")]},
        "pair_blocked_by_me": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["ME", "BAN", "blocked", "ME"]],
        },
        "pair_blocked_by_them": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["BAN", "ME", "blocked", "BAN"]],
        },
        "pair_blocked_no_decider": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["ME", "BAN", "blocked", None]],
        },
        "pair_declined": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["ME", "BAN", "declined", "BAN"]],
        },
        "pair_pending": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["ME", "BAN", "pending", None]],
        },
        "pair_unknown_state": {
            "summaries": [pair_row("PX1", "BAN")],
            "edges": [["ME", "BAN", "weird", "ME"]],
        },
        "pair_no_edge": {"summaries": [pair_row("PX1", "BAN")], "edges": []},
        "pair_counterpart_erased": {"summaries": [pair_row("PX1", "XOA", ANONYMOUS)]},
        "pair_counterpart_missing": {"summaries": [pair_row("PX1", "MAT", "Mất")]},
        "pair_counterpart_erased_and_blocked": {
            "summaries": [pair_row("PX1", "XOA", ANONYMOUS)],
            "edges": [["XOA", "ME", "blocked", "ME"]],
        },
        "pair_without_counterpart": {"summaries": [pair_row("PX1", None, None)]},
        "group_with_counterpart": {
            "summaries": [summary("CX1", "Nhóm", cp="BAN", cp_name="Bạn")],
            "edges": [["ME", "BAN", "blocked", "ME"]],
        },
        "pair_counterpart_name_none": {"summaries": [pair_row("PX1", "BAN", None)]},
        "pair_counterpart_name_empty": {"summaries": [pair_row("PX1", "BAN", "")]},
        "mixed": {
            "summaries": [
                pair_row("PX1", "BAN", last=last_message(kind="sticker"), unread=1),
                group,
                pair_row("PXN", "BAN2", "Bạn hai", membership="M3"),
                pair_row("CX2", "XOA", ANONYMOUS),
            ],
            "edges": [["BAN", "ME", "blocked", "ME"]],
        },
        "last_message_without_author": {
            "summaries": [
                summary(
                    "CX1",
                    "Nhóm",
                    last=last_message(
                        author=None, name=None, kind="deleted", preview=""
                    ),
                )
            ]
        },
    }
    for name, world in worlds.items():
        out.append(S(fn, name, {}, world))
    for k, theme in enumerate(THEMES):
        out.append(
            S(
                fn,
                f"theme/{k}",
                {},
                {"summaries": [summary("CX1", "Nhóm", theme=theme)]},
            )
        )
    for k, kind in enumerate(MESSAGE_KINDS):
        world = {"summaries": [summary("CX1", "N", last=last_message(kind=kind))]}
        out.append(S(fn, f"message_kind/{k}", {}, world))
    for k, text in enumerate(TEXTS):
        if len(text) > 300:
            continue
        world = {
            "summaries": [
                pair_row("PX1", "BAN", text),
                summary("CX1", text, last=last_message(name=text, preview=text)),
            ]
        }
        out.append(S(fn, f"names/{k}", {}, world))

    # --- get_my_profile ------------------------------------------------------
    fn = "get_my_profile"
    worlds = {
        "found": {},
        "missing": {"people": people_with(ME=None)},
        "erased_self": {"people": people_with(ME=erased("ME"))},
        "interests_unordered_duplicates": {
            "interests": ["outdoor", "cafe", "cafe", "an-uong"]
        },
        "interests_every_id_reversed": {"interests": list(reversed(INTEREST_IDS))},
        "interests_unknown": {"interests": ["cafe", "bogus"]},
        "interests_empty": {"interests": []},
        "no_providers": {"providers": []},
        "providers_many": {"providers": ["google", "phone", "google"]},
        "counts_large": {"counts": [10**7, 0, 99, 12, 3]},
        "bare_person": {
            "people": people_with(ME=person("ME", "", policy="nobody", phone=False))
        },
        "hostile_person": {
            "people": people_with(
                ME=person(
                    "ME",
                    RIGHT_TO_LEFT_OVERRIDE + "abc",
                    bio="<script>alert(1)</script>",
                    city=CJK_MIDDLE + GRINNING_FACE,
                    band="khong-co",
                )
            )
        },
    }
    for name, world in worlds.items():
        out.append(S(fn, name, {}, world))

    # --- update_my_profile ---------------------------------------------------
    fn = "update_my_profile"
    patches = {
        "display_name": [["display_name", "Minh"]],
        "display_name_padded": [["display_name", "  Minh  "]],
        "bio_only": [["bio", "Thích đi bộ"]],
        "city_only": [["city", "Hà Nội"]],
        "bio_empty_clears": [["bio", ""]],
        "bio_blank_clears": [["bio", " \t\n"]],
        "city_ideographic_space_clears": [["city", IDEOGRAPHIC_SPACE]],
        "policy_friends": [["wall_comment_policy", "friends"]],
        "policy_nobody": [["wall_comment_policy", "nobody"]],
        "phone_off": [["discoverable_by_phone", False]],
        "phone_on": [["discoverable_by_phone", True]],
        "everything": [
            ["display_name", " Minh "],
            ["bio", ""],
            ["city", " Huế "],
            ["wall_comment_policy", "readers"],
            ["discoverable_by_phone", False],
        ],
        "everything_reversed_order": [
            ["discoverable_by_phone", True],
            ["wall_comment_policy", "nobody"],
            ["city", ""],
            ["bio", " x "],
            ["display_name", "y"],
        ],
        "nothing_to_change": [],
        "all_none": [
            ["display_name", None],
            ["bio", None],
            ["city", None],
            ["wall_comment_policy", None],
            ["discoverable_by_phone", None],
        ],
        "same_as_stored": [
            ["display_name", "Tôi"],
            ["bio", "Thích cà phê"],
            ["city", "Đà Lạt"],
        ],
        "blank_display_name": [["display_name", "   "]],
        "empty_display_name": [["display_name", ""]],
        "name_200": [["display_name", "a" * 200]],
        "name_201": [["display_name", "c" * 201]],
        "name_201_padded_to_200": [["display_name", " " + "b" * 200 + " "]],
        "bio_500": [["bio", "d" * 500]],
        "bio_501": [["bio", "e" * 501]],
        "city_120": [["city", "f" * 120]],
        "city_121": [["city", "g" * 121]],
    }
    for name, patch in patches.items():
        out.append(S(fn, name, {"patch": patch}))
    for k, text in enumerate(TEXTS):
        if len(text) > 300:
            continue
        patch = [["display_name", text], ["bio", text], ["city", text]]
        out.append(S(fn, f"texts/{k}", {"patch": patch}))
    base = {"patch": [["bio", "x"]]}
    out += [
        S(fn, "update_answers_none", base, {"updated": None}),
        S(fn, "update_answers_other_row", base, {"updated": person("BAN", "Bạn")}),
        S(fn, "person_missing", base, {"people": people_with(ME=None)}),
        S(
            fn,
            "update_conflict",
            base,
            {"conflicts": {"update_person_profile": ["GONE"]}},
        ),
        S(fn, "interests_unknown_after_write", base, {"interests": ["nope"]}),
    ]

    # --- saved places --------------------------------------------------------
    fn = "list_saved_places"
    worlds = {
        "empty": {},
        "two": {"saved": [["SP1", "pho-ha-noi", EARLIER], ["SP2", "cafe-doc", JOINED]]},
        "one_gone_from_catalogue": {
            "saved": [["SP1", "bun-cha", EARLIER], ["SP2", "cafe-doc", JOINED]]
        },
        "all_gone": {"saved": [["SP1", "bun-cha", EARLIER]], "places": []},
        "same_place_twice": {
            "saved": [["SP1", "cafe-doc", EARLIER], ["SP2", "cafe-doc", T]]
        },
        "hostile_catalogue": {
            "saved": [["SP1", "$sp:x", EARLIER], ["SP2", "", T]],
            "places": [
                ["$sp:x", "<script>alert(1)</script>", RIGHT_TO_LEFT_OVERRIDE],
                ["", "", ""],
            ],
        },
    }
    for name, world in worlds.items():
        out.append(S(fn, name, {}, world))

    fn = "save_place"
    out += [
        S(fn, "created", {"place_id": "pho-ha-noi"}),
        S(fn, "already_saved", {"place_id": "cafe-doc"}, {"save_created": False}),
        S(fn, "unknown_place", {"place_id": "bun-cha"}),
        S(fn, "case_differs", {"place_id": "Pho-Ha-Noi"}),
        S(fn, "trailing_space", {"place_id": "pho-ha-noi "}),
        S(fn, "empty_id", {"place_id": ""}),
        S(fn, "empty_id_known", {"place_id": ""}, {"places": [["", "Trống", "x"]]}),
        S(
            fn,
            "hostile_id_known",
            {"place_id": "$sp:x"},
            {"places": [["$sp:x", CJK_MIDDLE + GRINNING_FACE, "an-uong"]]},
        ),
        S(
            fn,
            "conflict",
            {"place_id": "pho-ha-noi"},
            {"conflicts": {"save_place": ["RACE"]}},
        ),
    ]
    fn = "unsave_place"
    out += [
        S(fn, "saved", {"place_id": "pho-ha-noi"}),
        S(fn, "not_saved", {"place_id": "cafe-doc"}, {"unsave": False}),
        S(fn, "unknown_place", {"place_id": "bun-cha"}),
        S(fn, "hostile_unknown", {"place_id": "'; DROP TABLE saved_places; --"}),
    ]

    # --- list_blocked_people -------------------------------------------------
    fn = "list_blocked_people"
    worlds = {
        "empty": {},
        "undecided_falls_back_to_created": {
            "blocked": [["FB1", "LA", "Người lạ", EARLIER, None]]
        },
        "decided": {"blocked": [["FB1", "BAN", "Bạn", EARLIER, T - DAY]]},
        "two_and_erased": {
            "blocked": [
                ["FB1", "XOA", ANONYMOUS, JOINED, T],
                ["FB2", "MAT", "", EARLIER, None],
            ]
        },
        "hostile_names": {
            "blocked": [["FB1", "LA", text, EARLIER, None] for text in TEXTS[:8]]
        },
    }
    for name, world in worlds.items():
        out.append(S(fn, name, {}, world))

    # --- delete_own_account --------------------------------------------------
    fn = "delete_own_account"
    keys = [
        "anh/me/mot.jpg",
        "anh/me/hai.webp",
        "anh/me/ba.png",
        "anh/me/bon.jpg",
        "anh/me/nam",
    ]
    out += [
        S(fn, "confirmed_no_photos", {"confirm": True}),
        S(fn, "not_confirmed", {"confirm": False}),
        S(fn, "not_confirmed_no_role", {"confirm": False}, actor=("ME", ())),
        S(
            fn,
            "not_confirmed_unknown_role",
            {"confirm": False},
            actor=("ME", ("BOGUS",)),
        ),
        S(fn, "photos_all_removed", {"confirm": True}, {"storage_keys": keys[:2]}),
        S(
            fn,
            "photos_mixed",
            {"confirm": True},
            {
                "storage_keys": keys,
                "photo": [
                    True,
                    False,
                    "OSError",
                    "FileNotFoundError",
                    "PermissionError",
                ],
            },
        ),
        S(
            fn,
            "photos_all_fail",
            {"confirm": True},
            {
                "storage_keys": keys[:3],
                "photo": ["OSError", "OSError", "PermissionError"],
            },
        ),
        S(
            fn,
            "photos_none_existed",
            {"confirm": True},
            {"storage_keys": keys[:2], "photo": [False, False]},
        ),
        S(
            fn,
            "same_key_twice",
            {"confirm": True},
            {"storage_keys": [keys[0], keys[0]], "photo": [True, False]},
        ),
        S(
            fn,
            "erase_conflict",
            {"confirm": True},
            {"conflicts": {"erase_person": ["PERSON_NOT_FOUND"]}, "storage_keys": keys},
        ),
        S(
            fn,
            "erase_conflict_any_code",
            {"confirm": True},
            {"conflicts": {"erase_person": ["WHATEVER"]}},
        ),
        S(
            fn,
            "erased_account_again",
            {"confirm": True},
            {"people": people_with(ME=erased("ME"))},
        ),
    ]

    # --- block_person --------------------------------------------------------
    fn = "block_person"
    cases = {
        "self": ("ME", {}),
        "stranger_no_edge": ("LA", {}),
        "friend_accepted": ("BAN", {}),
        "friend_accepted_decided_by_them": ("BAN2", {}),
        "pending_from_me": ("LA", {"edges": [["ME", "LA", "pending", None]]}),
        "pending_to_me": ("LA", {"edges": [["LA", "ME", "pending", None]]}),
        "declined": ("LA", {"edges": [["LA", "ME", "declined", "ME"]]}),
        "already_blocked_by_me": ("BAN", {"edges": [["ME", "BAN", "blocked", "ME"]]}),
        "already_blocked_by_them": (
            "BAN",
            {"edges": [["BAN", "ME", "blocked", "BAN"]]},
        ),
        "blocked_no_decider": ("BAN", {"edges": [["ME", "BAN", "blocked", None]]}),
        "missing_person": ("MAT", {}),
        "erased_person": ("XOA", {}),
        "erased_person_blocked": ("XOA", {"edges": [["ME", "XOA", "blocked", "ME"]]}),
        "unknown_state": ("BAN", {"edges": [["ME", "BAN", "weird", "ME"]]}),
        "empty_state": ("BAN", {"edges": [["ME", "BAN", "", None]]}),
        "others_pending": (
            "LA",
            {"edges": [["ME", "LA", "pending", None, "BAN", "BAN2"]]},
        ),
        "others_accepted": (
            "LA",
            {"edges": [["ME", "LA", "accepted", None, "BAN2", "BAN"]]},
        ),
        "others_blocked": (
            "LA",
            {"edges": [["ME", "LA", "blocked", "BAN", "BAN", "BAN2"]]},
        ),
        "others_declined": (
            "LA",
            {"edges": [["ME", "LA", "declined", "BAN", "BAN", "BAN2"]]},
        ),
        "others_unknown_state": (
            "LA",
            {"edges": [["ME", "LA", "weird", None, "BAN", "BAN2"]]},
        ),
        "race_edge_exists": ("LA", {"conflicts": {"open_block_edge": ["EDGE_EXISTS"]}}),
        "race_other_code": (
            "LA",
            {"conflicts": {"open_block_edge": ["FRIEND_EDGE_EXISTS"]}},
        ),
    }
    for name, (target, world) in cases.items():
        out.append(S(fn, name, {"person_id": target}, world))
    out.append(S(fn, "self_no_role", {"person_id": "ME"}, actor=("ME", ())))

    # --- unblock_person ------------------------------------------------------
    fn = "unblock_person"
    cases = {
        "my_block": ("BAN", {"edges": [["ME", "BAN", "blocked", "ME"]]}),
        "my_block_as_addressee": ("BAN", {"edges": [["BAN", "ME", "blocked", "ME"]]}),
        "their_block": ("BAN", {"edges": [["ME", "BAN", "blocked", "BAN"]]}),
        "block_no_decider": ("BAN", {"edges": [["ME", "BAN", "blocked", None]]}),
        "no_edge": ("LA", {}),
        "accepted": ("BAN", {}),
        "declined_decided_by_me": ("LA", {"edges": [["ME", "LA", "declined", "ME"]]}),
        "self": ("ME", {}),
        "missing_person": ("MAT", {"edges": [["ME", "MAT", "blocked", "ME"]]}),
        "erased_person": ("XOA", {"edges": [["ME", "XOA", "blocked", "ME"]]}),
        "lift_not_blocked": (
            "BAN",
            {
                "edges": [["ME", "BAN", "blocked", "ME"]],
                "conflicts": {"lift_block_edge": ["NOT_BLOCKED"]},
            },
        ),
        "lift_only_blocker": (
            "BAN",
            {
                "edges": [["ME", "BAN", "blocked", "ME"]],
                "conflicts": {"lift_block_edge": ["ONLY_BLOCKER_MAY_UNBLOCK"]},
            },
        ),
        "lift_other_code": (
            "BAN",
            {
                "edges": [["ME", "BAN", "blocked", "ME"]],
                "conflicts": {"lift_block_edge": ["Weird_Code"]},
            },
        ),
    }
    for name, (target, world) in cases.items():
        out.append(S(fn, name, {"person_id": target}, world))

    # --- open_direct_message -------------------------------------------------
    fn = "open_direct_message"
    existing = {"pair_contexts": ["PX1"], "summaries": [pair_row("PX1", "BAN")]}
    cases = {
        "self": ("ME", {}),
        "not_friends": ("LA", {}),
        "friend_missing": ("MAT", {"friends": [["ME", "MAT"]]}),
        "friend_erased": ("XOA", {"friends": [["XOA", "ME"]]}),
        "friend_erased_and_blocked": (
            "XOA",
            {"friends": [["ME", "XOA"]], "edges": [["ME", "XOA", "blocked", "ME"]]},
        ),
        "friend_blocked_by_me": (
            "BAN",
            {**existing, "edges": [["ME", "BAN", "blocked", "ME"]]},
        ),
        "friend_blocked_by_them": (
            "BAN",
            {**existing, "edges": [["ME", "BAN", "blocked", "BAN"]]},
        ),
        "friend_edge_declined": (
            "BAN",
            {**existing, "edges": [["ME", "BAN", "declined", "BAN"]]},
        ),
        "friend_edge_unknown_state": (
            "BAN",
            {**existing, "edges": [["ME", "BAN", "weird", "BAN"]]},
        ),
        "existing_pair": ("BAN", existing),
        "existing_pair_among_many": (
            "BAN",
            {
                "pair_contexts": ["PX1"],
                "summaries": [
                    summary("CX1", "Nhóm", last=last_message()),
                    pair_row("PX1", "BAN", "Bạn", unread=4, last=last_message()),
                    pair_row("PX1", "BAN", "Bản sao"),
                ],
            },
        ),
        "new_pair_other_sorts_first": (
            "BAN",
            {"pair_contexts": [None], "summaries": [pair_row("PXN", "BAN")]},
        ),
        "new_pair_actor_sorts_first": (
            "BAN2",
            {"pair_contexts": [None], "summaries": [pair_row("PXN", "BAN2", None)]},
        ),
        "race_pair_exists_found": (
            "BAN",
            {
                "pair_contexts": [None, "PX1"],
                "summaries": [pair_row("PX1", "BAN")],
                "conflicts": {"create_pair_context": ["PAIR_EXISTS"]},
            },
        ),
        "race_pair_exists_lost": (
            "BAN",
            {
                "pair_contexts": [None, None],
                "conflicts": {"create_pair_context": ["PAIR_EXISTS"]},
            },
        ),
        "create_other_conflict": (
            "BAN",
            {
                "pair_contexts": [None],
                "conflicts": {"create_pair_context": ["pair_exists"]},
            },
        ),
        "summary_missing": (
            "BAN",
            {"pair_contexts": ["PX1"], "summaries": [summary("CX1", "Nhóm")]},
        ),
        "summary_of_new_pair_missing": (
            "BAN",
            {"pair_contexts": [None], "summaries": [pair_row("PX1", "BAN")]},
        ),
        "summary_blocked_still_available": (
            "BAN2",
            {
                "pair_contexts": ["PX1"],
                "summaries": [pair_row("PX1", "XOA", ANONYMOUS)],
            },
        ),
        "friends_but_groupmates_only": ("LA", {"groupmates": [["ME", "LA"]]}),
    }
    for name, (target, world) in cases.items():
        out.append(S(fn, name, {"person_id": target}, world))
    out += [
        S(fn, "self_no_role", {"person_id": "ME"}, actor=("ME", ())),
        S(
            fn,
            "not_friends_unknown_role",
            {"person_id": "LA"},
            actor=("ME", ("BOGUS",)),
        ),
        S(
            fn,
            "admin_friend",
            {"person_id": "BAN"},
            existing,
            actor=("ME", ("group_admin", "member")),
        ),
    ]

    # --- get_person_profile --------------------------------------------------
    fn = "get_person_profile"
    cases = {
        "self": ("ME", {}),
        "friend": ("BAN", {}),
        "groupmate": ("LA", {}),
        "friend_and_groupmate": ("BAN", {"groupmates": [["ME", "BAN"]]}),
        "stranger": ("BAN2", {"friends": []}),
        "missing_friend": ("MAT", {"friends": [["ME", "MAT"]]}),
        "missing_stranger": ("MAT", {}),
        "erased_groupmate": ("XOA", {"groupmates": [["ME", "XOA"]]}),
        "erased_stranger": ("XOA", {}),
        "self_erased": ("ME", {"people": people_with(ME=erased("ME"))}),
        "self_missing": ("ME", {"people": people_with(ME=None)}),
        "blocked_friend": ("BAN", {"edges": [["ME", "BAN", "blocked", "BAN"]]}),
        "hostile_friend": (
            "BAN",
            {
                "people": people_with(
                    BAN=person(
                        "BAN", "<script>alert(1)</script>", bio=VIET_NFD, city=" "
                    )
                )
            },
        ),
    }
    for name, (target, world) in cases.items():
        out.append(S(fn, name, {"person_id": target}, world))
    out += [
        S(fn, "self_no_role", {"person_id": "ME"}, actor=("ME", ())),
        S(
            fn,
            "stranger_unknown_role",
            {"person_id": "BAN2"},
            {"friends": []},
            actor=("ME", ("BOGUS",)),
        ),
    ]

    # --- register_person -----------------------------------------------------
    fn = "register_person"
    cases = {
        "new_id": ("MAT", "Người mới", {}),
        "new_id_empty_name": ("MAT", "", {}),
        "new_self": ("ME", "Tôi", {"people": people_with(ME=None)}),
        "create_conflict": (
            "MAT",
            "Mới",
            {"conflicts": {"create_person": ["PERSON_EXISTS"]}},
        ),
        "create_conflict_mixed_case": (
            "MAT",
            "Mới",
            {"conflicts": {"create_person": ["Identity_Taken"]}},
        ),
        "same_name_other": ("BAN", "Bạn", {}),
        "same_name_self": ("ME", "Tôi", {}),
        "rename_self": ("ME", "Tôi mới", {}),
        "rename_self_trailing_space": ("ME", "Tôi ", {}),
        "rename_other": ("BAN", "Khác", {}),
        "rename_vanished": ("ME", "Tôi mới", {"renamed": None}),
        "rename_answers_other_row": (
            "ME",
            "Tôi mới",
            {"renamed": person("BAN", "Bạn")},
        ),
        "erased_same_name": ("XOA", ANONYMOUS, {}),
        "erased_other_name": ("XOA", "Tên cũ", {}),
        "erased_self": ("ME", "Tôi", {"people": people_with(ME=erased("ME"))}),
        "nfc_stored_nfd_sent": (
            "ME",
            VIET_NFD,
            {"people": people_with(ME=person("ME", VIET_NFC))},
        ),
        "nfd_stored_nfd_sent": (
            "ME",
            VIET_NFD,
            {"people": people_with(ME=person("ME", VIET_NFD))},
        ),
    }
    for name, (target, display_name, world) in cases.items():
        out.append(
            S(fn, name, {"person_id": target, "display_name": display_name}, world)
        )
    for k, text in enumerate(TEXTS):
        out.append(S(fn, f"texts/new/{k}", {"person_id": "MAT", "display_name": text}))
        out.append(
            S(fn, f"texts/rename/{k}", {"person_id": "ME", "display_name": text})
        )
    for label, actor in DOOR_ACTORS:
        out += [
            S(
                fn,
                f"same_name/{label}",
                {"person_id": "BAN", "display_name": "Bạn"},
                actor=actor,
            ),
            S(
                fn,
                f"rename_self/{label}",
                {"person_id": "ME", "display_name": "X"},
                actor=actor,
            ),
            S(
                fn,
                f"erased/{label}",
                {"person_id": "XOA", "display_name": "X"},
                actor=actor,
            ),
        ]
    return out


# --- fuzz --------------------------------------------------------------------

PEOPLE_IDS = ("ME", "BAN", "BAN2", "LA", "XOA", "MAT")
PAIRS = (
    ("ME", "BAN"),
    ("ME", "BAN2"),
    ("ME", "LA"),
    ("ME", "XOA"),
    ("ME", "MAT"),
    ("BAN", "LA"),
    ("BAN2", "XOA"),
)
PLACE_IDS = (
    "pho-ha-noi",
    "cafe-doc",
    "bun-cha",
    "",
    "$sp:x",
    "Pho-Ha-Noi",
    "pho-ha-noi ",
)
CATALOGUE = (
    ["pho-ha-noi", "Phở Hà Nội", "an-uong"],
    ["cafe-doc", "Cà phê Dốc", "cafe"],
    ["bun-cha", "Bún chả", "an-uong"],
    ["$sp:x", "<script>", "x"],
    ["", "Trống", ""],
)
NAMES = ("Tôi", "Bạn", "Minh", VIET_NFC, VIET_NFD, ANONYMOUS, "")
CONFLICT_CODES = {
    "create_person": ("PERSON_EXISTS", "Identity_Taken"),
    "create_pair_context": ("PAIR_EXISTS", "PAIR_EXISTS", "OTHER"),
    "open_block_edge": ("EDGE_EXISTS", "EDGE_EXISTS", "OTHER"),
    "lift_block_edge": ("NOT_BLOCKED", "ONLY_BLOCKER_MAY_UNBLOCK", "Weird"),
    "erase_person": ("PERSON_NOT_FOUND", "X"),
    "update_person_profile": ("GONE",),
    "save_place": ("RACE",),
}
FN_CONFLICTS = {
    "register_person": ("create_person",),
    "open_direct_message": ("create_pair_context",),
    "block_person": ("open_block_edge",),
    "unblock_person": ("lift_block_edge",),
    "delete_own_account": ("erase_person",),
    "update_my_profile": ("update_person_profile",),
    "save_place": ("save_place",),
}


def fuzz_time(rng: random.Random) -> datetime:
    return rng.choice(
        (
            T,
            T + MICRO,
            T
            + timedelta(
                days=rng.randint(-900, 900),
                seconds=rng.randint(0, 86399),
                microseconds=rng.randint(0, 999999),
            ),
        )
    )


def fuzz_text(rng: random.Random) -> str:
    return rng.choice((*TEXTS, *NAMES, random_text(rng)))


def fuzz_person(rng: random.Random, alias: str) -> list:
    return person(
        alias,
        fuzz_text(rng),
        bio=rng.choice((None, None, fuzz_text(rng))),
        city=rng.choice((None, None, fuzz_text(rng))),
        band=rng.choice((None, "vua-phai", "tiet-kiem")),
        policy=rng.choice(POLICIES),
        phone=rng.random() < 0.7,
        created=rng.choice((JOINED, EARLIER)),
        deleted=None if rng.random() < 0.85 else T - DAY * rng.randint(0, 40),
    )


def fuzz_summary(rng: random.Random) -> list:
    kind = rng.choice(("group", "pair", "pair"))
    counterpart = None
    if (kind == "pair" and rng.random() < 0.9) or rng.random() < 0.1:
        counterpart = rng.choice(PEOPLE_IDS)
    counterpart_name = (
        rng.choice((None, "", "Bạn", fuzz_text(rng))) if counterpart else None
    )
    last = None
    if rng.random() < 0.5:
        author = rng.choice((None, *PEOPLE_IDS))
        last = last_message(
            author=author,
            name=None if author is None else fuzz_text(rng),
            kind=rng.choice(MESSAGE_KINDS),
            preview=fuzz_text(rng),
            created=fuzz_time(rng),
        )
    return summary(
        rng.choice(("CX1", "CX2", "PX1", "PXN")),
        fuzz_text(rng),
        count=rng.randint(0, 9),
        role=rng.choice(("member", "admin")),
        state=rng.choice(("active", "invited")),
        membership=rng.choice(("M1", "M2", "M3")),
        joined=rng.choice((None, JOINED)),
        last=last,
        unread=rng.randint(0, 40),
        theme=rng.choice(THEMES),
        kind=kind,
        cp=counterpart,
        cp_name=counterpart_name,
    )


def fuzz_patch(rng: random.Random) -> list:
    patch = []
    for field in ("display_name", "bio", "city"):
        if rng.random() < 0.4:
            patch.append([field, fuzz_text(rng)])
    if rng.random() < 0.3:
        patch.append(["wall_comment_policy", rng.choice(POLICIES)])
    if rng.random() < 0.3:
        patch.append(["discoverable_by_phone", rng.random() < 0.5])
    rng.shuffle(patch)
    return patch


def fuzz_step(rng: random.Random, i: int) -> dict:
    fn = rng.choice(tuple(CALLERS))
    now = fuzz_time(rng)
    actor = rng.choice(("ME",) * 8 + ("BAN", "XOA", "MAT"))
    roles = rng.choice(
        (("member",),) * 20
        + ((), ("group_admin",), ("member", "BOGUS"), ("group_admin", "member"))
    )
    target = rng.choice((*PEOPLE_IDS, actor))
    w: dict = {}

    if rng.random() < 0.5:
        people = default_people()
        for alias in PEOPLE_IDS:
            roll = rng.random()
            if roll < 0.1:
                people.pop(alias, None)
            elif roll < 0.2:
                people[alias] = erased(alias, deleted=T - DAY * rng.randint(0, 30))
            elif roll < 0.35:
                people[alias] = fuzz_person(rng, alias)
        w["people"] = people

    def pairs_subset(chance: float) -> list:
        chosen = []
        for x, y in PAIRS:
            if rng.random() < chance:
                chosen.append([x, y] if rng.random() < 0.5 else [y, x])
        return chosen

    if rng.random() < 0.6:
        w["friends"] = pairs_subset(0.4)
    if rng.random() < 0.5:
        w["groupmates"] = pairs_subset(0.35)
    if rng.random() < 0.7:
        edges = []
        for x, y in PAIRS:
            if rng.random() < 0.45:
                state = rng.choice(
                    ("accepted", "pending", "declined", "blocked") * 6 + ("weird",)
                )
                row = [x, y, state, rng.choice((None, x, y, "LA"))]
                if rng.random() < 0.5:
                    row[0], row[1] = row[1], row[0]
                if rng.random() < 0.05:
                    row += [rng.choice(PEOPLE_IDS), rng.choice(PEOPLE_IDS)]
                edges.append(row)
        w["edges"] = edges
    if (
        fn in ("block_person", "unblock_person", "open_direct_message")
        and rng.random() < 0.6
    ):
        # The edge between the two people the method is about, so the block
        # state machine and the unblock door see every state, not just the
        # few the pairs above happen to draw.
        state = rng.choice(
            ("blocked", "blocked", "accepted", "pending", "declined", "weird")
        )
        row = [actor, target, state, rng.choice((actor, actor, target, None))]
        if rng.random() < 0.5:
            row[0], row[1] = row[1], row[0]
        w["edges"] = [row, *w.get("edges", WORLD_DEFAULTS["edges"])]
        if fn == "open_direct_message" and rng.random() < 0.7:
            w["friends"] = [
                [actor, target],
                *w.get("friends", WORLD_DEFAULTS["friends"]),
            ]
    if fn in ("list_my_contexts", "open_direct_message"):
        w["summaries"] = [fuzz_summary(rng) for _ in range(rng.choice((0, 1, 1, 2, 3)))]
        if fn == "open_direct_message" and rng.random() < 0.6 and w["summaries"]:
            w["summaries"][rng.randrange(len(w["summaries"]))][0] = rng.choice(
                ("PX1", "PXN")
            )
        w["pair_contexts"] = [
            rng.choice((None, None, "PX1")),
            rng.choice((None, "PX1", "PXN")),
        ]
    if fn in ("get_my_profile", "update_my_profile"):
        w["counts"] = [rng.randint(0, 10**6) for _ in range(5)]
        w["providers"] = rng.sample(("phone", "google", "zalo"), rng.randint(0, 3))
        interests = rng.sample(INTEREST_IDS, rng.randint(0, len(INTEREST_IDS)))
        if rng.random() < 0.2:
            interests += rng.sample(interests, len(interests) // 2)
        if rng.random() < 0.07:
            interests.insert(
                rng.randint(0, len(interests)), rng.choice(("bogus", "Cafe", ""))
            )
        w["interests"] = interests
        if rng.random() < 0.08:
            w["updated"] = rng.choice((None, fuzz_person(rng, rng.choice(PEOPLE_IDS))))
    if fn in ("list_saved_places", "save_place", "unsave_place"):
        w["places"] = [row for row in CATALOGUE if rng.random() < 0.6]
        w["saved"] = [
            [
                rng.choice(("SP1", "SP2")),
                rng.choice(PLACE_IDS),
                rng.choice((EARLIER, JOINED, fuzz_time(rng))),
            ]
            for _ in range(rng.choice((0, 1, 2, 3)))
        ]
        w["save_created"] = rng.random() < 0.6
        w["unsave"] = rng.random() < 0.6
    if fn == "list_blocked_people":
        w["blocked"] = [
            [
                rng.choice(("FB1", "FB2")),
                rng.choice(PEOPLE_IDS),
                fuzz_text(rng),
                rng.choice((EARLIER, JOINED)),
                rng.choice((None, fuzz_time(rng))),
            ]
            for _ in range(rng.choice((0, 1, 2, 3)))
        ]
    if fn == "delete_own_account":
        w["storage_keys"] = [
            f"anh/{n}/{rng.choice(('a', 'b'))}.jpg"
            for n in range(rng.choice((0, 1, 2, 4)))
        ]
        w["photo"] = [
            rng.choice(
                (True, True, False, "OSError", "FileNotFoundError", "PermissionError")
            )
            for _ in w["storage_keys"]
        ]
    if fn == "register_person" and rng.random() < 0.1:
        w["renamed"] = rng.choice((None, fuzz_person(rng, rng.choice(PEOPLE_IDS))))
    conflicts = {}
    for method in FN_CONFLICTS.get(fn, ()):
        if rng.random() < 0.15:
            conflicts[method] = [rng.choice(CONFLICT_CODES[method])]
    if conflicts:
        w["conflicts"] = conflicts

    r: dict = {}
    if fn == "update_my_profile":
        r["patch"] = fuzz_patch(rng)
    elif fn in ("save_place", "unsave_place"):
        r["place_id"] = rng.choice((*PLACE_IDS, random_text(rng, 3)))
    elif fn == "delete_own_account":
        r["confirm"] = rng.random() < 0.85
    elif fn in (
        "block_person",
        "unblock_person",
        "open_direct_message",
        "get_person_profile",
    ):
        r["person_id"] = target
    elif fn == "register_person":
        r["person_id"] = target
        row = w.get("people", WORLD_DEFAULTS["people"]).get(target)
        r["display_name"] = (
            row[1] if row is not None and rng.random() < 0.4 else fuzz_text(rng)
        )
    return step_case(fn, f"fuzz/{i}", r, w, now=now, actor=(actor, roles))


def people_steps_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 12)
    out = []
    for i in range(count):
        while True:
            built = fuzz_step(rng, i)
            if fits(built):
                break
        out.append(built)
    return out


# ---------------------------------------------------------------------------
# Driver
# ---------------------------------------------------------------------------

FUNCTIONS = {
    "tables_for": account_lifecycle.tables_for,
    "check_confirmation": account_lifecycle.check_confirmation,
    "anonymised_person": al_anonymised,
    "pair_key": direct.pair_key,
    "can_open": direct.can_open,
    "is_kind": direct.is_kind,
    **{
        name: (lambda name: lambda **kw: run_step(name, **kw))(name) for name in CALLERS
    },
}

#: module -> (Go package path, target, constants, edges, fuzz,
#:            cases in the committed sample, cases a live oracle run draws)
MODULES = {
    "account_lifecycle": (
        "internal/domain/accountlifecycle",
        account_lifecycle,
        account_lifecycle_constants,
        account_lifecycle_edges,
        account_lifecycle_fuzz,
        120,
        20000,
    ),
    "direct_people": (
        "internal/domain/direct",
        direct,
        direct_people_constants,
        direct_people_edges,
        direct_people_fuzz,
        120,
        20000,
    ),
    "people_steps": (
        "internal/domain/peoplesteps",
        api_service,
        people_steps_constants,
        people_steps_edges,
        people_steps_fuzz,
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
        "generator": "scripts/render_domain_w10_goldens.py",
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
