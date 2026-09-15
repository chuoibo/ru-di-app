#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W3 routes (ADR-0029).

The sixteen W3 routes (contexts, memberships, balances, the memory wall, its
check-ins, widget, hearts and comments) reach five pure Python pieces, each
ported to one Go package:

    app.domain.money             services/core/internal/domain/money
    app.domain.ledger            services/core/internal/domain/ledger
    app.domain.chat_theme        services/core/internal/domain/chattheme
    app.domain.direct            services/core/internal/domain/direct
    app.api.service (pure steps) services/core/internal/domain/contexts

plus the block of `ApiService.get_context_balances` between the repository
reads and the response, which the ledger package carries as ContextBalances.

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each one returned or raised. The service steps are not free functions in
Python; they run inside real `ApiService` methods over a recording stub
repository, so the golden holds what the method did (including which
repository reads it made) rather than a transcription of it. Every package's
oracle_test.go replays every case. The image is the one
scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w3_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w3_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases (the inputs of
its Python tests plus the boundaries found while porting); `<module>-fuzz-<k>`
is shard k of that module's seeded fuzz. Shards are contiguous slices of one
fixed case list, so one shard rendered alone gives the same bytes.

The inputs have the shapes the service builds: str ids, stored amounts inside
int64 (one allocation row is a BIGINT column), derived sums of any size (a
pair's receipts are a SQL SUM, and two confirmations of a BIGINT amount already
leave int64), dicts whose order matters written as lists of pairs. A derived
sum that is None probes the refusal Python gives a value that is not an int.
Non-ASCII characters whose identity matters are spelled with `chr()`.

## Encoding

The encoding of scripts/render_domain_w2_goldens.py, unchanged:

* a case is `{"fn", "name", "args": {parameter: value}, "result"}` and a result
  is `{"ok": value}` or `{"raised": {"type", "message", "code"}}`;
* `"$i:<hex>"` is an int of nine or more digits;
* `"$sp:<text>"` is a str cut into groups of six code points joined by `|`,
  used when a str starts with `$` or would trip scripts/repo_guard.py;
* a set is its sorted list; a tuple is a list.
"""

from __future__ import annotations

import inspect
import json
import platform
import random
import re
import string
import sys
import uuid
from datetime import UTC, datetime
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402
from app.api.repository import (  # noqa: E402
    AllocationRow,
    BatchInputs,
    ConfirmedExpense,
)
from app.api.schemas import ContextUpdateRequest  # noqa: E402
from app.domain import chat_theme, direct, ledger, money  # noqa: E402

SEED = 43
TARGET = "services/core/{path}/testdata/python_{mode}.json"
SMALL_INT = 10**8
GROUP = 6
INT64_MAX = 2**63 - 1
INT64_MIN = -(2**63)

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

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
C = "a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
D = "a4a4a4a4-b4b4-4c4c-8d4d-e4e4e4e4e4e4"
E = "a5a5a5a5-b5b5-4c5c-8d5d-e5e5e5e5e5e5"
F = "f0f0f0f0-b6b6-4c6c-8d6d-e6e6e6e6e6e6"
LOW = "0a0a0a0a-0b0b-4c0c-8d0d-0e0e0e0e0e0e"
HIGH = "fafafafa-fbfb-4cfc-8dfd-fefefefefefe"
CONTEXT = "c0c0c0c0-cdcd-4ece-8fcf-dadadadadada"
PEOPLE = (A, B, C, D, E, F, LOW, HIGH)

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
PARAGRAPH_SEPARATOR = chr(0x2029)
SOFT_HYPHEN = chr(0xAD)
COMBINING_ACUTE = chr(0x301)
BYTE_ORDER_MARK = chr(0xFEFF)
MONGOLIAN_VOWEL_SEPARATOR = chr(0x180E)
OGHAM_SPACE = chr(0x1680)
NARROW_NO_BREAK_SPACE = chr(0x202F)
PRIVATE_USE = chr(0xE000)
LAST_CODE_POINT = chr(sys.maxunicode)
FULLWIDTH_CAPITAL_A = chr(0xFF21)
FULLWIDTH_HYPHEN = chr(0xFF0D)
ARABIC_INDIC_ZERO = 0x660
FILE_SEPARATOR = "\x1c"
UNIT_SEPARATOR = "\x1f"

WHITESPACE = tuple(chr(c) for c in range(sys.maxunicode + 1) if chr(c).isspace())
NOT_QUITE_SPACE = (
    ZERO_WIDTH_SPACE,
    BYTE_ORDER_MARK,
    MONGOLIAN_VOWEL_SEPARATOR,
    SOFT_HYPHEN,
    "\x00",
    "\x1b",
    "\x7f",
    chr(0x2060),
)
ODD = (
    "'",
    '"',
    "\\",
    "\t",
    "\n",
    "\r",
    "\x00",
    "\x1f",
    "\x7f",
    NEXT_LINE,
    NO_BREAK_SPACE,
    SOFT_HYPHEN,
    E_ACUTE,
    COMBINING_ACUTE,
    ZERO_WIDTH_SPACE,
    LINE_SEPARATOR,
    BYTE_ORDER_MARK,
    PRIVATE_USE,
    REPLACEMENT_CHARACTER,
    IDEOGRAPHIC_SPACE,
    CJK_MIDDLE,
    E_DOT_CIRCUMFLEX,
    GRINNING_FACE,
    LAST_CODE_POINT,
)
PLAIN = string.ascii_lowercase + string.digits + " -_."


def random_text(rng: random.Random, most: int = 6) -> str:
    return "".join(
        rng.choice(ODD) if rng.random() < 0.35 else rng.choice(PLAIN)
        for _ in range(rng.randrange(most + 1))
    )


def random_uuid(rng: random.Random) -> str:
    return str(uuid.UUID(int=rng.getrandbits(128)))


def pairs(mapping: dict) -> list[list]:
    return [[key, value] for key, value in mapping.items()]


NOW = datetime(2026, 9, 15, 9, 30, 12, 345678, tzinfo=UTC)


class Captured(Exception):
    """Raised by the stub at the repository call that ends the pure part."""

    def __init__(self, value):
        super().__init__("captured")
        self.value = value


class Stub:
    """A repository that answers from `facts` and records what was asked."""

    def __init__(self, **facts):
        self.facts = facts
        self.calls: list[str] = []

    def is_member(self, context_id, person_id):
        self.calls.append("is_member")
        return self.facts.get("is_member", True)

    def load_batch_inputs(self, context_id, expense_version_ids):
        self.calls.append("load_batch_inputs")
        if expense_version_ids is not None:
            raise AssertionError("balances read the whole ledger")
        return self.facts["inputs"]

    def load_confirmed_receipts(self, context_id):
        self.calls.append("load_confirmed_receipts")
        return self.facts["receipts"]

    def get_context(self, context_id):
        self.calls.append("get_context")
        if self.facts.get("stop_at_get_context"):
            raise Captured("reached")
        return self.facts.get("context")

    def update_context(self, context_id, *, changes):
        self.calls.append("update_context")
        raise Captured(changes)

    def list_members(self, context_id):
        self.calls.append("list_members")
        return self.facts["members"]

    def get_membership(self, membership_id):
        self.calls.append("get_membership")
        return self.facts["membership"]


def actor(actor_id: str = A, roles=("member",)) -> Actor:
    return Actor(id=uuid.UUID(actor_id), roles=frozenset(roles), context_ids=frozenset())


def problem_of(exc: ApiProblem) -> dict:
    return {"status": exc.status_code, "code": exc.code, "detail": exc.detail}


def new_service(stub: Stub) -> api_service.ApiService:
    return api_service.ApiService(stub, photo_storage=object())


# ---------------------------------------------------------------------------
# money
# ---------------------------------------------------------------------------

MONEY_VALUES = (
    INT64_MIN,
    INT64_MIN + 1,
    -(2**31),
    -SMALL_INT,
    -2,
    -1,
    0,
    1,
    2,
    SMALL_INT - 1,
    SMALL_INT,
    2**31,
    INT64_MAX - 1,
    INT64_MAX,
)


def money_constants() -> dict:
    return {
        "names": public_names(money),
        "not_integer": money.NOT_INTEGER,
        "negative": money.NEGATIVE,
        "non_positive": money.NON_POSITIVE,
        "below_minimum": money.BELOW_MINIMUM,
    }


def money_edges() -> list[dict]:
    out = []
    for k, value in enumerate(MONEY_VALUES):
        for allow_negative in (False, True):
            for positive in (False, True):
                name = f"vnd_violation/{k}/{int(allow_negative)}{int(positive)}"
                args = {"value": value, "allow_negative": allow_negative, "positive": positive}
                out.append(case("vnd_violation", name, args))
    return out


def random_amount(rng: random.Random) -> int:
    roll = rng.random()
    if roll < 0.3:
        return rng.randrange(-3, 4)
    if roll < 0.6:
        return rng.randrange(-(10**7), 10**7) * 1000
    if roll < 0.8:
        return rng.randrange(INT64_MIN, INT64_MAX + 1)
    return rng.choice(MONEY_VALUES) + rng.randrange(-2, 3) * (rng.random() < 0.5)


def int64(value: int) -> int:
    return max(INT64_MIN, min(INT64_MAX, value))


def money_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 1)
    out = []
    for i in range(600):
        args = {
            "value": int64(random_amount(rng)),
            "allow_negative": rng.random() < 0.5,
            "positive": rng.random() < 0.5,
        }
        out.append(case("vnd_violation", f"fuzz/{i}", args))
    return out


# ---------------------------------------------------------------------------
# ledger
# ---------------------------------------------------------------------------

LEDGER_IDS = (
    "ha",
    "nam",
    "binh",
    "a",
    "b",
    "c",
    "",
    "Z",
    "z",
    E_ACUTE,
    "e" + COMBINING_ACUTE,
    CJK_MIDDLE,
    GRINNING_FACE,
    A,
    B,
)


def alloc_case(name, allocations: dict, advancer, version="v1") -> dict:
    return case(
        "obligations_from_allocations",
        name,
        {"allocations": pairs(allocations), "advancer_id": advancer, "expense_version_id": version},
        call={"allocations": allocations, "advancer_id": advancer, "expense_version_id": version},
    )


def ob(sender, recipient, amount, version="v1") -> dict:
    return {
        "sender_id": sender,
        "recipient_id": recipient,
        "amount_vnd": amount,
        "source_expense_version_id": version,
    }


def merge_case(name, obligations: list) -> dict:
    return case("merge_obligations", name, {"obligations": obligations})


def balances_case(name, obligations: list, receipts: list | None) -> dict:
    call_receipts = None if receipts is None else {(s, r): a for s, r, a in receipts}
    return case(
        "group_balances",
        name,
        {"obligations": obligations, "receipts": receipts},
        call={"obligations": obligations, "receipts": call_receipts},
        shape=pairs,
    )


def plan_case(name, balances: dict, exact_limit: int = 15) -> dict:
    return case(
        "settlement_plan",
        name,
        {"balances": pairs(balances), "exact_limit": exact_limit},
        call={"balances": balances, "exact_limit": exact_limit},
    )


def ledger_constants() -> dict:
    limit = inspect.signature(ledger.settlement_plan).parameters["exact_limit"].default
    return {"names": public_names(ledger), "default_exact_limit": limit}


def ledger_edges() -> list[dict]:
    out: list[dict] = []
    for k, value in enumerate(MONEY_VALUES):
        for positive in (False, True):
            args = {"value": value, "positive": positive}
            out.append(case("require_vnd", f"require_vnd/{k}/{int(positive)}", args))

    # obligations_from_allocations: the Python tests, then the boundaries.
    out += [
        alloc_case("test_advancer_owes_nothing_to_themselves", {"ha": 82000, "nam": 100000}, "nam"),
        alloc_case("test_zero_share_creates_no_obligation", {"ha": 0, "nam": 100}, "nam"),
        alloc_case(
            "test_obligations_sum_to_total_minus_the_advancer_share",
            {"ha": 82000, "nam": 100000, "binh": 55000},
            "nam",
        ),
        alloc_case("test_advancer_outside_the_participant_set", {"ha": 50, "nam": 50}, "outsider"),
        alloc_case("test_no_advancer_is_rejected", {"ha": 50}, None),
        alloc_case("no_advancer_beats_a_negative_amount", {"ha": -50}, None),
        alloc_case("test_a_negative_advancer_allocation_is_caught", {"a": 100, "nam": -100}, "nam"),
        alloc_case("empty_allocations", {}, "nam"),
        alloc_case("empty_advancer_is_an_id", {"": 10, "ha": 20}, ""),
        alloc_case("int64_extremes", {"ha": INT64_MAX, "nam": INT64_MAX, "binh": 1}, "binh"),
        alloc_case("negative_after_valid", {"ha": 10, "binh": -1, "nam": 5}, "nam"),
        alloc_case("int64_min_share", {"ha": INT64_MIN}, "nam"),
        alloc_case("unicode_ids_keep_dict_order", {GRINNING_FACE: 3, "z": 2, E_ACUTE: 1}, "a", "v" + CJK_MIDDLE),
    ]

    # merge_obligations
    out += [
        merge_case(
            "test_same_pair_is_summed",
            [ob("ha", "nam", 82000, "v1"), ob("ha", "nam", 18000, "v2")],
        ),
        merge_case(
            "test_opposite_directions_are_never_netted",
            [ob("ha", "nam", 50000, "v1"), ob("nam", "ha", 30000, "v2")],
        ),
        merge_case(
            "test_different_recipients_are_never_merged",
            [ob("ha", "nam", 50000, "v1"), ob("ha", "binh", 30000, "v1")],
        ),
        merge_case("test_self_obligation_is_rejected", [ob("ha", "ha", 1)]),
        merge_case(
            "test_merge_refuses_a_negative_amount",
            [ob("a", "b", 100, "v1"), ob("a", "b", -80, "v2")],
        ),
        merge_case("empty", []),
        merge_case("zero_amount", [ob("a", "b", 0)]),
        merge_case("self_before_negative", [ob("a", "a", -1), ob("a", "b", -1)]),
        merge_case("negative_before_self", [ob("a", "b", -1), ob("a", "a", 1)]),
        merge_case("zero_before_self", [ob("a", "b", 0), ob("a", "a", 1)]),
        merge_case(
            "sources_deduplicate_and_sort",
            [ob("a", "b", 1, "v2"), ob("a", "b", 2, "v1"), ob("a", "b", 3, "v2"), ob("a", "b", 4, "V1")],
        ),
        merge_case(
            "order_is_by_utf8_bytes",
            [
                ob(GRINNING_FACE, "a", 1),
                ob(CJK_MIDDLE, "a", 1),
                ob(E_ACUTE, "a", 1),
                ob("z", "a", 1),
                ob("Z", "a", 1),
                ob("", "a", 1),
                ob("z", "", 1),
                ob("z", E_ACUTE, 1),
            ],
        ),
        merge_case("total_exactly_int64_max", [ob("a", "b", 2**62), ob("a", "b", 2**62 - 1)]),
        merge_case("total_one_past_int64_max", [ob("a", "b", 2**62), ob("a", "b", 2**62)]),
        merge_case(
            "total_three_int64_max",
            [ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX)],
        ),
        merge_case(
            "past_int64_then_self_refused",
            [ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX), ob("c", "c", 1)],
        ),
    ]

    # group_balances
    sixty_forty = [{"sender_id": "a", "recipient_id": "b", "amount_vnd": 60}, {"sender_id": "a", "recipient_id": "b", "amount_vnd": 40}]
    out += [
        balances_case("test_two_obligations_in_one_pair_share_one_receipt", sixty_forty, [["a", "b", 50]]),
        balances_case("test_a_full_receipt_clears_the_pair", sixty_forty, [["a", "b", 100]]),
        balances_case(
            "test_group_balance_is_netted_and_sums_to_zero",
            [{"sender_id": "ha", "recipient_id": "nam", "amount_vnd": 50000}, {"sender_id": "nam", "recipient_id": "ha", "amount_vnd": 30000}],
            None,
        ),
        balances_case(
            "test_confirmed_receipts_reduce_the_balance",
            [{"sender_id": "ha", "recipient_id": "nam", "amount_vnd": 50000}],
            [["ha", "nam", 50000]],
        ),
        balances_case("empty_ledger", [], None),
        balances_case("empty_ledger_with_receipts", [], [["a", "b", 5]]),
        balances_case("empty_receipts_dict", sixty_forty, []),
        balances_case("receipt_over_the_debt_is_not_a_credit", sixty_forty, [["a", "b", 1000]]),
        balances_case("receipt_one_short", sixty_forty, [["a", "b", 99]]),
        balances_case("receipt_the_other_way_is_ignored", sixty_forty, [["b", "a", 100]]),
        balances_case("negative_receipt_is_refused", sixty_forty, [["a", "b", -1]]),
        balances_case("negative_receipt_for_an_unowed_pair_is_never_read", sixty_forty, [["b", "a", -1]]),
        balances_case(
            "receipt_checks_run_in_first_seen_pair_order",
            [ob("c", "d", 5), ob("a", "b", 5)],
            [["a", "b", -1], ["c", "d", -2]],
        ),
        balances_case("self_obligation", [ob("a", "b", 5), ob("b", "b", 5)], None),
        balances_case("self_before_bad_receipt", [ob("a", "b", 5), ob("b", "b", 5)], [["a", "b", -1]]),
        balances_case("zero_amount", [ob("a", "b", 0)], None),
        balances_case("cycle_nets_to_nothing", [ob("a", "b", 7), ob("b", "c", 7), ob("c", "a", 7)], None),
        balances_case(
            "people_sort_by_code_point",
            [ob(GRINNING_FACE, "Z", 3), ob(E_ACUTE, "z", 2), ob(CJK_MIDDLE, "", 1)],
            None,
        ),
        balances_case(
            "position_exactly_int64_min",
            [ob("a", "b", INT64_MAX), ob("a", "c", 1)],
            None,
        ),
        balances_case(
            "position_one_below_int64_min",
            [ob("a", "b", INT64_MAX), ob("a", "c", 2)],
            None,
        ),
        balances_case(
            "position_exactly_int64_max",
            [ob("b", "a", 2**62), ob("c", "a", 2**62 - 1)],
            None,
        ),
        balances_case(
            "position_one_past_int64_max",
            [ob("b", "a", 2**62), ob("c", "a", 2**62)],
            None,
        ),
        balances_case(
            "pair_total_past_int64_receipt_brings_it_back",
            [ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX)],
            [["a", "b", INT64_MAX]],
        ),
        balances_case(
            "past_int64_positions_that_cancel",
            [ob("a", "b", INT64_MAX), ob("b", "c", INT64_MAX), ob("c", "a", INT64_MAX), ob("a", "b", INT64_MAX)],
            [["a", "b", INT64_MAX]],
        ),
    ]

    # settlement_plan: the Python tests and corpus-shaped examples, then edges.
    out += [
        plan_case("test_settlement_suggestions_clear_the_same_positions", {"a": -70, "b": -30, "c": 60, "d": 40}),
        plan_case("test_settlement_suggestions_reject_unbalanced_input", {"a": -10, "b": 5}),
        plan_case("test_suggestions_are_shaped_as_drafts", {"a": -70, "b": 70}),
        plan_case(
            "test_within_the_exact_limit_the_plan_is_proven",
            {"a": -500000, "b": -400000, "c": 200000, "d": 300000, "e": 400000},
        ),
        plan_case(
            "test_raising_the_limit_4",
            {"a": -500000, "b": -400000, "c": 200000, "d": 300000, "e": 400000},
            4,
        ),
        plan_case(
            "test_raising_the_limit_5",
            {"a": -500000, "b": -400000, "c": 200000, "d": 300000, "e": 400000},
            5,
        ),
        plan_case(
            "test_beyond_the_exact_limit_the_plan_is_flagged_unproven",
            {**{f"p{i:02d}": 1000 for i in range(20)}, **{f"q{i:02d}": -1000 for i in range(20)}},
        ),
        plan_case("empty", {}),
        plan_case("everyone_square", {"a": 0, "b": 0}),
        plan_case("zeros_do_not_count_as_people", {"a": 0, "b": -5, "c": 5}, 2),
        plan_case("zero_limit_with_nobody", {}, 0),
        plan_case("zero_limit_greedy", {"a": -5, "b": 5}, 0),
        plan_case("negative_limit_greedy", {"a": -5, "b": 5}, -1),
        plan_case("limit_equal_to_people_is_exact", {"a": -3, "b": -2, "c": 2, "d": 3}, 4),
        plan_case("limit_one_below_people_is_greedy", {"a": -3, "b": -2, "c": 2, "d": 3}, 3),
        plan_case("greedy_is_not_minimal", {"a": -3, "b": -2, "c": 2, "d": 3}, 0),
        plan_case("one_dong_off_is_refused", {"a": -3, "b": 2}),
        plan_case("single_nonzero_is_refused", {"a": 5}),
        plan_case("ties_break_by_person", {"d": -5, "b": -5, "c": 5, "a": 5}),
        plan_case("ties_break_by_code_point", {E_ACUTE: -5, "z": -5, GRINNING_FACE: 5, CJK_MIDDLE: 5}),
        plan_case("empty_person_id", {"": -1, "a": 1}),
        plan_case("two_equal_partitions_pick_the_first_found", {"a": -1, "b": -1, "c": 1, "d": 1}),
        plan_case(
            "several_maximum_partitions",
            {"a": -2, "b": -1, "c": -1, "d": 1, "e": 1, "f": 2},
        ),
        plan_case(
            "greedy_loses_twice_six_people",
            {"a": -600000, "b": -300000, "c": -100000, "d": 200000, "e": 300000, "f": 500000},
        ),
        plan_case(
            "int64_min_debtor",
            {"a": INT64_MIN, "b": INT64_MAX, "c": 1},
        ),
        plan_case(
            "int64_min_debtor_greedy",
            {"a": INT64_MIN, "b": INT64_MAX, "c": 1},
            0,
        ),
        plan_case(
            "subset_sum_past_int64",
            {"a": 2**62, "b": 2**62, "c": INT64_MIN},
        ),
        plan_case(
            "subset_sums_that_wrap_to_zero_are_not_zero",
            {"a": INT64_MAX, "b": INT64_MAX, "c": 2, "d": INT64_MIN, "e": INT64_MIN},
        ),
        plan_case(
            "sum_that_wraps_to_zero_is_still_refused",
            {"a": INT64_MAX, "b": INT64_MAX, "c": 2},
        ),
        plan_case(
            "fifteen_people_exact",
            {f"p{i:02d}": v for i, v in enumerate((-7, -5, -3, -2, -1, 1, 2, 3, 4, 8, -9, 6, -4, 5, 2))},
        ),
        plan_case(
            "sixteen_people_greedy",
            {f"p{i:02d}": v for i, v in enumerate((-7, -5, -3, -2, -1, 1, 2, 3, 4, 8, -9, 6, -4, 5, 3, -1))},
        ),
    ]

    # Derived sums past int64, and derived sums that are not ints.
    out += [
        balances_case("receipt_sum_past_int64_clears_the_pair", sixty_forty, [["a", "b", 2 * INT64_MAX]]),
        balances_case(
            "receipt_exactly_two_to_the_64",
            [ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX), ob("a", "b", 2)],
            [["a", "b", 2**64]],
        ),
        balances_case(
            "receipt_past_int64_one_short_of_the_debt",
            [ob("a", "b", INT64_MAX), ob("a", "b", INT64_MAX)],
            [["a", "b", 2 * INT64_MAX - 1]],
        ),
        balances_case("negative_receipt_past_int64", sixty_forty, [["a", "b", -(2**64)]]),
        balances_case("receipt_that_is_not_an_int", sixty_forty, [["a", "b", None]]),
        balances_case("unread_receipt_that_is_not_an_int", sixty_forty, [["b", "a", None]]),
        balances_case(
            "amount_that_is_not_an_int",
            [{"sender_id": "a", "recipient_id": "b", "amount_vnd": None}],
            None,
        ),
        balances_case(
            "self_before_amount_that_is_not_an_int",
            [{"sender_id": "a", "recipient_id": "a", "amount_vnd": None}],
            None,
        ),
        balances_case(
            "merged_amount_past_int64",
            [{"sender_id": "a", "recipient_id": "b", "amount_vnd": 2**70}, ob("c", "b", 1)],
            [["a", "b", 2**69]],
        ),
        plan_case("balances_past_int64", {"a": -(2**64), "b": 2**64}),
        plan_case("balances_past_int64_three", {"a": 2**70, "b": -(2**69), "c": -(2**69)}),
        plan_case("balances_past_int64_greedy", {"a": 2**70, "b": -(2**69), "c": -(2**69)}, 0),
        plan_case("balance_that_is_not_an_int", {"a": None, "b": 0}),
        plan_case("not_an_int_before_not_netting", {"a": None, "b": 5}),
        plan_case(
            "twelve_people_past_int64",
            {f"p{i:02d}": v * 2**64 for i, v in enumerate((-7, -5, -3, -2, -1, 1, 2, 3, 4, 8, -9, 9))},
        ),
    ]
    return out


def random_ledger_id(rng: random.Random, pool=LEDGER_IDS[:8]) -> str:
    if rng.random() < 0.05:
        return random_text(rng, 3)
    if rng.random() < 0.1:
        return rng.choice(LEDGER_IDS)
    return rng.choice(pool)


def ledger_amount(rng: random.Random, allow_bad: float = 0.03) -> int:
    roll = rng.random()
    if roll < allow_bad:
        return rng.choice((0, -1, -1000, INT64_MIN))
    if roll < 0.06:
        return rng.choice((INT64_MAX, 2**62, INT64_MAX - rng.randrange(1000)))
    if roll < 0.2:
        return rng.randrange(1, 10)
    return rng.randrange(1, 500) * 1000


def zero_sum_balances(rng: random.Random, count: int, magnitude: int) -> dict:
    names = rng.sample(LEDGER_IDS[:12] + tuple(f"p{i:02d}" for i in range(12)), count)
    amounts = [rng.randint(-magnitude, magnitude) for _ in names[:-1]]
    if names:
        amounts.append(-sum(amounts))
    return dict(zip(names, amounts, strict=True))


def ledger_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 2)
    out: list[dict] = []
    for i in range(700):
        people = rng.sample(LEDGER_IDS, rng.randrange(0, 6))
        allocations = {person: ledger_amount(rng) for person in people}
        if rng.random() < 0.3:
            allocations = {person: rng.choice((0, rng.randrange(1, 9) * 1000)) for person in people}
        advancer = None if rng.random() < 0.03 else (rng.choice(people) if people and rng.random() < 0.7 else random_ledger_id(rng))
        version = rng.choice(("v1", "v2", "", E_ACUTE))
        out.append(alloc_case(f"fuzz/{i}", allocations, advancer, version))
    for i in range(700):
        obligations = []
        for _ in range(rng.randrange(0, 8)):
            sender = random_ledger_id(rng)
            recipient = sender if rng.random() < 0.03 else random_ledger_id(rng)
            obligations.append(ob(sender, recipient, ledger_amount(rng), rng.choice(("v1", "v2", "v3", "V1", ""))))
        out.append(merge_case(f"fuzz/{i}", obligations))
    for i in range(700):
        obligations = []
        for _ in range(rng.randrange(0, 9)):
            sender = random_ledger_id(rng)
            recipient = sender if rng.random() < 0.02 else random_ledger_id(rng)
            amount = ledger_amount(rng, 0.02)
            if rng.random() < 0.03:
                amount = rng.choice((2**64, 2**70, 3 * INT64_MAX))
            obligations.append({"sender_id": sender, "recipient_id": recipient, "amount_vnd": amount})
        receipts = None
        if rng.random() < 0.8:
            receipts = []
            for _ in range(rng.randrange(0, 5)):
                if obligations and rng.random() < 0.8:
                    chosen = rng.choice(obligations)
                    pair = (chosen["sender_id"], chosen["recipient_id"])
                    amount = rng.choice((chosen["amount_vnd"], rng.randrange(0, 600) * 1000, 0, 1))
                else:
                    pair = (random_ledger_id(rng), random_ledger_id(rng))
                    amount = rng.randrange(0, 600) * 1000
                if rng.random() < 0.02:
                    amount = -rng.randrange(1, 5)
                elif rng.random() < 0.05:
                    amount = rng.choice((2**63, 2**64, 2 * INT64_MAX, 2**71, -(2**64)))
                receipts.append([pair[0], pair[1], amount])
            receipts = [list(item) for item in {(s, r): [s, r, a] for s, r, a in receipts}.values()]
        out.append(balances_case(f"fuzz/{i}", obligations, receipts))
    for i in range(900):
        roll = rng.random()
        if roll < 0.55:
            balances = zero_sum_balances(rng, rng.randrange(0, 9), rng.choice((3, 10, 500000)))
        elif roll < 0.75:
            balances = zero_sum_balances(rng, rng.randrange(9, 13), rng.choice((5, 500000, 10**12)))
        elif roll < 0.8:
            balances = zero_sum_balances(rng, rng.randrange(13, 16), 10**9)
        elif roll < 0.9:
            count = rng.randrange(1, 8)
            names = rng.sample(LEDGER_IDS, count)
            balances = {name: rng.randint(-20, 20) for name in names}
        else:
            count = rng.randrange(2, 9)
            balances = zero_sum_balances(rng, count, rng.choice((2**61, 2**64, 2**90)))
        if rng.random() < 0.1 and balances:
            balances[rng.choice(list(balances))] = 0
        if rng.random() < 0.02 and balances:
            balances[rng.choice(list(balances))] = None
        exact_limit = rng.choice((15, 15, 15, 0, 1, 2, 3, len(balances), len(balances) - 1))
        out.append(plan_case(f"fuzz/{i}", balances, exact_limit))
    return out


# ---------------------------------------------------------------------------
# context balances: ApiService.get_context_balances over a stub repository
# ---------------------------------------------------------------------------


def context_balances(expenses: list, receipts: list) -> dict:
    rows = 0
    built = []
    for expense in expenses:
        allocations = []
        for participant, amount in expense["allocations"]:
            rows += 1
            allocations.append(
                AllocationRow(id=uuid.UUID(int=rows), participant_id=uuid.UUID(participant), amount_vnd=amount)
            )
        built.append(
            ConfirmedExpense(
                version_id=uuid.UUID(expense["version_id"]),
                context_id=uuid.UUID(CONTEXT),
                paid_by_id=uuid.UUID(expense["paid_by_id"]),
                payer_acknowledgement="acknowledged",
                allocations=tuple(allocations),
            )
        )
    stub = Stub(
        inputs=BatchInputs(expenses=tuple(built), unavailable_version_ids=()),
        receipts={(uuid.UUID(s), uuid.UUID(r)): amount for s, r, amount in receipts},
    )
    try:
        response = new_service(stub).get_context_balances(uuid.UUID(CONTEXT), actor())
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "response": None}
    return {"calls": stub.calls, "problem": None, "response": response.model_dump(mode="json")}


def expense(version: str, paid_by: str, allocations: list) -> dict:
    return {"version_id": version, "paid_by_id": paid_by, "allocations": allocations}


V1 = "d1d1d1d1-e1e1-4f1f-8a1a-b1b1b1b1b1b1"
V2 = "d2d2d2d2-e2e2-4f2f-8a2a-b2b2b2b2b2b2"
V3 = "d3d3d3d3-e3e3-4f3f-8a3a-b3b3b3b3b3b3"


def balances_service_case(name: str, expenses: list, receipts: list) -> dict:
    return case("context_balances", name, {"expenses": expenses, "receipts": receipts})


def context_balances_constants() -> dict:
    return {"names": []}


def context_balances_edges() -> list[dict]:
    return [
        balances_service_case("empty_ledger", [], []),
        balances_service_case("empty_ledger_with_receipts", [], [[A, B, 1000]]),
        balances_service_case(
            "one_dinner",
            [expense(V1, A, [[A, 100000], [B, 82000], [C, 18000]])],
            [],
        ),
        balances_service_case(
            "payer_outside_the_participants",
            [expense(V1, D, [[A, 50000], [B, 50000]])],
            [],
        ),
        balances_service_case(
            "receipt_clears_one_debt",
            [expense(V1, A, [[A, 100000], [B, 82000], [C, 18000]])],
            [[B, A, 82000]],
        ),
        balances_service_case(
            "receipt_over_the_debt",
            [expense(V1, A, [[B, 1000]])],
            [[B, A, 5000]],
        ),
        balances_service_case(
            "two_expenses_net_in_display",
            [expense(V1, A, [[B, 50000]]), expense(V2, B, [[A, 30000]])],
            [],
        ),
        balances_service_case(
            "cycle_needs_no_transfer",
            [expense(V1, A, [[B, 7]]), expense(V2, B, [[C, 7]]), expense(V3, C, [[A, 7]])],
            [],
        ),
        balances_service_case(
            "greedy_would_need_four",
            [
                expense(V1, D, [[A, 300000]]),
                expense(V2, E, [[B, 400000]]),
                expense(V3, F, [[A, 200000], [C, 300000]]),
            ],
            [],
        ),
        balances_service_case(
            "negative_allocation_is_a_conflict",
            [expense(V1, A, [[B, 100], [C, -1]])],
            [],
        ),
        balances_service_case(
            "negative_payer_allocation_is_a_conflict",
            [expense(V1, A, [[A, -100], [B, 100]])],
            [],
        ),
        balances_service_case(
            "negative_receipt_is_a_conflict",
            [expense(V1, A, [[B, 100]])],
            [[B, A, -5]],
        ),
        balances_service_case(
            "zero_rows_owe_nothing",
            [expense(V1, A, [[B, 0], [C, 0]])],
            [],
        ),
        balances_service_case(
            "duplicate_participant_row_keeps_the_last_amount",
            [expense(V1, A, [[B, 100], [C, 5], [B, 7]])],
            [],
        ),
        balances_service_case(
            "balances_sort_by_uuid_bytes",
            [expense(V1, HIGH, [[LOW, 3], [F, 2], [A, 1]])],
            [],
        ),
        balances_service_case(
            "balance_exactly_int64_min",
            [expense(V1, A, [[B, INT64_MAX]]), expense(V2, C, [[B, 1]])],
            [],
        ),
        balances_service_case(
            "balance_one_below_int64_min",
            [expense(V1, A, [[B, INT64_MAX]]), expense(V2, C, [[B, 2]])],
            [],
        ),
        balances_service_case(
            "pair_total_past_int64_repaid",
            [expense(V1, A, [[B, INT64_MAX]]), expense(V2, A, [[B, INT64_MAX]])],
            [[B, A, INT64_MAX]],
        ),
        balances_service_case(
            "past_int64_then_negative_receipt",
            [expense(V1, A, [[B, INT64_MAX]]), expense(V2, A, [[B, INT64_MAX]])],
            [[B, A, -1]],
        ),
        balances_service_case(
            "two_bigint_receipts_clear_the_pair",
            [expense(V1, A, [[B, 100000], [C, 5000]])],
            [[B, A, 2 * INT64_MAX]],
        ),
        balances_service_case(
            "two_bigint_receipts_on_an_unowed_pair",
            [expense(V1, A, [[B, 100000]])],
            [[A, B, 2 * INT64_MAX]],
        ),
        balances_service_case(
            "receipt_sum_past_int64_one_short",
            [expense(V1, A, [[B, INT64_MAX]]), expense(V2, A, [[B, INT64_MAX]]), expense(V3, A, [[B, 2]])],
            [[B, A, 2**64 - 1]],
        ),
    ]


def context_balances_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 3)
    out = []
    for i in range(600):
        people = list(PEOPLE) + [random_uuid(rng) for _ in range(2)]
        expenses = []
        for _ in range(rng.randrange(0, 6)):
            payer = rng.choice(people)
            rows = []
            for participant in rng.sample(people, rng.randrange(0, 5)):
                amount = rng.randrange(0, 400) * 1000
                roll = rng.random()
                if roll < 0.02:
                    amount = -rng.randrange(1, 1000)
                elif roll < 0.04:
                    amount = rng.choice((INT64_MAX, 2**62))
                elif roll < 0.15:
                    amount = 0
                rows.append([participant, amount])
            if rows and rng.random() < 0.03:
                rows.append([rows[0][0], rng.randrange(1, 9) * 1000])
            expenses.append(expense(random_uuid(rng), payer, rows))
        receipts = {}
        for _ in range(rng.randrange(0, 4)):
            sender, recipient = rng.sample(people, 2)
            amount = rng.randrange(0, 500) * 1000
            if rng.random() < 0.02:
                amount = -1
            elif rng.random() < 0.05:
                amount = rng.choice((2**63, 2**64 - 1, 2 * INT64_MAX))
            receipts[(sender, recipient)] = amount
        out.append(
            balances_service_case(f"fuzz/{i}", expenses, [[s, r, a] for (s, r), a in receipts.items()])
        )
    return out


# ---------------------------------------------------------------------------
# chat_theme
# ---------------------------------------------------------------------------


def chat_theme_constants() -> dict:
    return {
        "names": public_names(chat_theme),
        "themes": list(chat_theme.THEMES),
        "default_theme": chat_theme.DEFAULT_THEME,
    }


def theme_near_misses() -> list[str]:
    out = []
    for slug in chat_theme.THEMES:
        out += [
            slug.upper(),
            slug.capitalize(),
            " " + slug,
            slug + " ",
            slug + "\n",
            NO_BREAK_SPACE + slug,
            slug + ZERO_WIDTH_SPACE,
            slug.replace("-", "_"),
            slug.replace("-", ""),
            slug.replace("-", FULLWIDTH_HYPHEN),
            slug[:-1],
            slug + slug[-1],
            slug + "\x00",
        ]
    return out


def chat_theme_edges() -> list[dict]:
    values = list(chat_theme.THEMES) + theme_near_misses()
    values += ["", "hong", "#c93900", "mac dinh", "mặc-định", "default", "mac-dinh,hoang-hon"]
    return [case("is_theme", f"is_theme/{k}", {"value": value}) for k, value in enumerate(values)]


def chat_theme_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 4)
    out = []
    for i in range(400):
        if rng.random() < 0.5:
            slug = list(rng.choice(chat_theme.THEMES))
            for _ in range(rng.randrange(1, 3)):
                index = rng.randrange(len(slug) + 1)
                if rng.random() < 0.5 and index < len(slug):
                    del slug[index]
                else:
                    slug.insert(index, rng.choice(ODD + tuple(PLAIN)))
            value = "".join(slug)
        else:
            value = random_text(rng, 10)
        out.append(case("is_theme", f"fuzz/{i}", {"value": value}))
    return out


# ---------------------------------------------------------------------------
# direct
# ---------------------------------------------------------------------------

KINDS = ("group", "pair", "Pair", "PAIR", " pair", "pair ", "", "dm", "pairs", "groups")
NAMES = (None, "", " ", "Bình", "Hội đi Đà Lạt", "\t", ZERO_WIDTH_SPACE, "0", "None", A)


def direct_constants() -> dict:
    return {
        "names": public_names(direct),
        "kind_group": direct.KIND_GROUP,
        "kind_pair": direct.KIND_PAIR,
        "anonymous_counterpart": direct.ANONYMOUS_COUNTERPART,
    }


def direct_edges() -> list[dict]:
    out = []
    for k, kind in enumerate(KINDS):
        out.append(case("is_pair", f"is_pair/{k}", {"kind": kind}))
    rosters = (
        [],
        [A],
        [B],
        [A, B],
        [B, A],
        [A, B, C],
        [A, A],
        [A, A, B],
        [B, B],
        [A, B, B],
        ["", A],
        [A.upper(), A],
    )
    for k, roster in enumerate(rosters):
        for me in (A, B, ""):
            out.append(case("counterpart_of", f"counterpart_of/{k}/{me[:2]}", {"member_ids": roster, "me": me}))
    for k, kind in enumerate(KINDS):
        for j, stored in enumerate(("", "Hội đi Đà Lạt", " ")):
            for n, counterpart in enumerate(NAMES):
                args = {"kind": kind, "stored_name": stored, "counterpart_name": counterpart}
                out.append(case("display_name_for", f"display_name_for/{k}/{j}/{n}", args))
    return out


def direct_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 5)
    out = []
    for i in range(300):
        roster = [rng.choice((A, B, C, "", random_text(rng, 3))) for _ in range(rng.randrange(0, 5))]
        me = rng.choice((A, B, C, "", random_text(rng, 3)))
        out.append(case("counterpart_of", f"fuzz/{i}", {"member_ids": roster, "me": me}))
    for i in range(300):
        args = {
            "kind": rng.choice(KINDS + (random_text(rng, 4),)),
            "stored_name": random_text(rng),
            "counterpart_name": None if rng.random() < 0.2 else random_text(rng, 3),
        }
        out.append(case("display_name_for", f"fuzz/{i}", args))
    return out


# ---------------------------------------------------------------------------
# contexts: the pure steps of the context and memory routes in service.py
# ---------------------------------------------------------------------------


def update_changes(display_name, theme, kind) -> dict:
    record = None
    if kind is not None:
        record = SimpleNamespace(
            id=uuid.UUID(CONTEXT), kind=kind, display_name="x", created_by_id=uuid.UUID(A), created_at=NOW, theme="mac-dinh"
        )
    stub = Stub(context=record)
    request = ContextUpdateRequest.model_construct(display_name=display_name, theme=theme)
    try:
        new_service(stub).update_context(uuid.UUID(CONTEXT), request, actor())
    except Captured as captured:
        return {"calls": stub.calls, "changes": pairs(captured.value), "problem": None}
    except ApiProblem as exc:
        return {"calls": stub.calls, "changes": None, "problem": problem_of(exc)}
    raise AssertionError("update_context returned without writing")


def require_photo_url_context(context_id: str, image_url) -> dict:
    try:
        api_service._require_photo_url_context(uuid.UUID(context_id), image_url)
    except ApiProblem as exc:
        return {"problem": problem_of(exc)}
    return {"problem": None}


def context_view(kind: str, stored_name: str, actor_id: str, members: list) -> dict:
    record = SimpleNamespace(
        id=uuid.UUID(CONTEXT), kind=kind, display_name=stored_name, created_by_id=uuid.UUID(A), created_at=NOW, theme="mac-dinh"
    )
    stub = Stub(members=[SimpleNamespace(person_id=uuid.UUID(p), display_name=n) for p, n in members])
    response = new_service(stub)._context_response(record, actor(actor_id))
    counterpart = response.counterpart
    return {
        "calls": stub.calls,
        "display_name": response.display_name,
        "counterpart": None
        if counterpart is None
        else {"id": str(counterpart.id), "display_name": counterpart.display_name},
    }


def accept_permission(origin: str, roles: list, actor_id: str, person_id: str, is_member: bool) -> dict:
    membership = SimpleNamespace(
        id=uuid.UUID(int=7), context_id=uuid.UUID(CONTEXT), person_id=uuid.UUID(person_id), origin=origin
    )
    stub = Stub(membership=membership, is_member=is_member, stop_at_get_context=True)
    try:
        new_service(stub).accept_context_membership(uuid.UUID(int=7), actor(actor_id, roles))
    except Captured:
        return {"calls": stub.calls, "problem": None}
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc)}
    raise AssertionError("accept_context_membership passed the kind check")


FUNCTIONS = {
    "vnd_violation": money.vnd_violation,
    "require_vnd": ledger.require_vnd,
    "obligations_from_allocations": ledger.obligations_from_allocations,
    "merge_obligations": ledger.merge_obligations,
    "group_balances": ledger.group_balances,
    "settlement_plan": ledger.settlement_plan,
    "context_balances": context_balances,
    "is_theme": chat_theme.is_theme,
    "is_pair": direct.is_pair,
    "counterpart_of": direct.counterpart_of,
    "display_name_for": direct.display_name_for,
    "update_changes": update_changes,
    "require_photo_url_context": require_photo_url_context,
    "context_view": context_view,
    "accept_permission": accept_permission,
}

URL_CONTEXT = "b7b7b7b7-c8c8-4d9d-8eae-fbfbfbfbfbfb"
PHOTO = "e2e2e2e2-f3f3-4a4a-8b5b-c6c6c6c6c6c6"


def contexts_constants() -> dict:
    return {
        "somebody": api_service._SOMEBODY,
        "isspace": every_code_point(str.isspace),
    }


def photo_urls(context: str) -> list:
    hexed = context.replace("-", "")
    arabic = "".join(chr(ARABIC_INDIC_ZERO + int(c)) if c.isdigit() else c for c in context)
    return [
        None,
        "",
        "/",
        f"/contexts/{context}/photos/{PHOTO}",
        f"/contexts/{context.upper()}/photos/{PHOTO.upper()}",
        f"/contexts/{{{context}}}/photos/{PHOTO}",
        f"/contexts/urn:uuid:{context}/photos/{PHOTO}",
        f"/contexts/{hexed}/photos/{PHOTO}",
        f"/contexts/{arabic}/photos/{PHOTO}",
        f"/contexts/ {context} /photos/{PHOTO}",
        f"/contexts/{context}/photos/{PHOTO}/",
        f"/contexts/{context}/photos/",
        f"//contexts/{context}/photos/{PHOTO}",
        f"contexts/{context}/photos/{PHOTO}",
        f"x/contexts/{context}/photos/{PHOTO}",
        f"/people/{context}/photos/{PHOTO}",
        f"/Contexts/{context}/photos/{PHOTO}",
        f"/contexts/{context}/Photos/{PHOTO}",
        f"/contexts/{context}/photos/not-a-uuid",
        f"/contexts/{context[:-1]}/photos/{PHOTO}",
        f"/contexts/{context}0/photos/{PHOTO}",
        f"/contexts/{context}/photos/{PHOTO}?x=1",
        f"/contexts/{context}{ZERO_WIDTH_SPACE}/photos/{PHOTO}",
        f"/contexts/{A}/photos/{PHOTO}",
        f"/contexts/{context.replace('-', '_')}/photos/{PHOTO}",
    ]


def strip_samples() -> list[str]:
    samples = ["", "Hội đi Đà Lạt", "  Hội  ", "a b", " ", "\t\n", "x"]
    for space in WHITESPACE:
        samples.append(space + "Nhóm" + space)
        samples.append(space)
    for other in NOT_QUITE_SPACE:
        samples.append(other + "Nhóm" + other)
    samples += [FILE_SEPARATOR + UNIT_SEPARATOR + "a", "a" + NEXT_LINE + NO_BREAK_SPACE, OGHAM_SPACE + NARROW_NO_BREAK_SPACE + PARAGRAPH_SEPARATOR]
    return samples


def contexts_edges() -> list[dict]:
    out = []
    for k, name in enumerate(strip_samples()):
        for kind in ("group",):
            args = {"display_name": name, "theme": None, "kind": kind}
            out.append(case("update_changes", f"update_changes/strip/{k}", args))
    grid_names = (None, "", "  Nhóm mới  ", " ")
    grid_themes = (None, "mac-dinh", "ruc-ro", "hong", "", "MAC-DINH")
    for j, name in enumerate(grid_names):
        for n, theme in enumerate(grid_themes):
            for kind in ("group", "pair", None):
                args = {"display_name": name, "theme": theme, "kind": kind}
                out.append(case("update_changes", f"update_changes/grid/{j}/{n}/{kind}", args))
    for context in (URL_CONTEXT, A):
        for k, url in enumerate(photo_urls(URL_CONTEXT)):
            args = {"context_id": context, "image_url": url}
            out.append(case("require_photo_url_context", f"photo_url/{context[:2]}/{k}", args))
    rosters = (
        [],
        [[A, "An"]],
        [[A, "An"], [B, "Bình"]],
        [[B, "Bình"], [A, "An"]],
        [[A, "An"], [B, ""]],
        [[A, "An"], [B, " "]],
        [[A, "An"], [B, "Bình"], [C, "Chi"]],
        [[B, "Bình"]],
        [[B, "Bình"], [C, "Chi"]],
        [[A, "An"], [A, "An again"], [B, "Bình"]],
        [[A, "An"], [B, "Bình"], [B, "Bình hai"]],
    )
    for kind in ("group", "pair"):
        for j, stored in enumerate(("", "Hội đi Đà Lạt")):
            for k, roster in enumerate(rosters):
                args = {"kind": kind, "stored_name": stored, "actor_id": A, "members": roster}
                out.append(case("context_view", f"context_view/{kind}/{j}/{k}", args))
    for origin in ("named", "link", "", "LINK", "invite"):
        for roles in (["member"], ["group_admin"], [], ["guest"], ["member", "guest"]):
            for person in (A, B):
                for is_member in (False, True):
                    args = {"origin": origin, "roles": roles, "actor_id": A, "person_id": person, "is_member": is_member}
                    name = f"accept/{origin}/{'+'.join(roles)}/{person[:2]}/{int(is_member)}"
                    out.append(case("accept_permission", name, args))
    return out


def contexts_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 6)
    out = []
    for i in range(500):
        name = None if rng.random() < 0.1 else "".join(
            rng.choice(WHITESPACE) if rng.random() < 0.4 else rng.choice(ODD + tuple(PLAIN))
            for _ in range(rng.randrange(0, 8))
        )
        theme = rng.choice((None, None, "mac-dinh", "bien-dem", random_text(rng, 4)))
        kind = rng.choice(("group", "group", "pair", None))
        out.append(case("update_changes", f"fuzz/{i}", {"display_name": name, "theme": theme, "kind": kind}))
    for i in range(400):
        urls = photo_urls(rng.choice((URL_CONTEXT, A)))
        url = rng.choice(urls[3:])
        chars = list(url)
        for _ in range(rng.randrange(0, 3)):
            index = rng.randrange(len(chars) + 1)
            if rng.random() < 0.5 and index < len(chars):
                del chars[index]
            else:
                chars.insert(index, rng.choice(("/", "-", "0", "a", "A", "{", "}", ":", " ") + ODD))
        args = {"context_id": rng.choice((URL_CONTEXT, A)), "image_url": "".join(chars)}
        out.append(case("require_photo_url_context", f"fuzz/{i}", args))
    return out


#: module -> (Go package path, target, constants, edges, fuzz, fuzz shards)
MODULES = {
    "money": ("internal/domain/money", money, money_constants, money_edges, money_fuzz, 1),
    "ledger": ("internal/domain/ledger", ledger, ledger_constants, ledger_edges, ledger_fuzz, 4),
    "context_balances": (
        "internal/domain/ledger",
        api_service,
        context_balances_constants,
        context_balances_edges,
        context_balances_fuzz,
        4,
    ),
    "chat_theme": (
        "internal/domain/chattheme",
        chat_theme,
        chat_theme_constants,
        chat_theme_edges,
        chat_theme_fuzz,
        1,
    ),
    "direct": ("internal/domain/direct", direct, direct_constants, direct_edges, direct_fuzz, 1),
    "contexts": (
        "internal/domain/contexts",
        api_service,
        contexts_constants,
        contexts_edges,
        contexts_fuzz,
        2,
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
        "generator": "scripts/render_domain_w3_goldens.py",
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
