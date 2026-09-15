#!/usr/bin/env python3
"""Oracle for the Go port of the pure logic behind the W4 routes (ADR-0029).

The fourteen W4 routes (expenses, bills, the group budget, batches, receipt
confirmation and the personal finance read) reach these pure Python pieces,
each ported to one Go package:

    app.domain.contract, allocator  services/core/internal/domain/allocator
    app.domain.bill                 services/core/internal/domain/billdraft
    app.domain.budget               services/core/internal/domain/budget
    app.domain.expense              services/core/internal/domain/expense
    app.domain.collection           services/core/internal/domain/collection
    app.domain.capability           services/core/internal/domain/capability
    app.domain.ledger (status)      services/core/internal/domain/ledger
    app.api.service (pure steps)    services/core/internal/domain/moneysteps

Go must answer exactly as Python answers, so instead of restating the rules
this script calls the real functions inside the parity API image and records
what each one returned or raised. The service steps are not free functions in
Python; they run inside real `ApiService` methods over a recording stub
repository, so the golden holds what the method did (including which
repository calls it made) rather than a transcription of it. Every package's
oracle_test.go replays every case. The image is the one
scripts/go_postgres_tier.sh builds from this tree:

    IMAGE="mobile-parity-api:$(git rev-parse --short HEAD)-$(printf '%s' "$PWD" |
      cksum | cut -d' ' -f1)"
    (cd services/api && docker build -q -t "$IMAGE" .)

Each invocation renders one file, chosen by MODE; `--list` prints every MODE
and its target path, tab separated, so the whole set regenerates with:

    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w4_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w4_goldens.py > "$path"
    done

`<module>` holds the module's constants and the named edge cases (the inputs of
its Python tests plus the boundaries found while porting); `<module>-fuzz-0` is
a small sample of that module's seeded fuzz (SEED, the sample count in
MODULES). Both are committed and replayed by plain `go test`.

The full differential fuzz is not committed. `--live MODULE SEED COUNT` draws
COUNT cases with SEED and prints them, without the repository guard's checks
since nothing is written to the tree; the Go tests built with `-tags oracle`
run it inside the parity image at test time (see internal/oracletest/live.go)
and replay every case. A live run draws 21,000 allocator cases by default:
valid expenses built relationally, then corrupted at random so that every
refusal code comes back, with amounts at 0, at MAX_AMOUNT_VND and one past it,
and past int64. The committed and live case lists come from the same
generators, so the sample is the first draw of the same distribution.
`_apportion`, the one rounding point, is also called directly with exact
shares that sum to the total but may be negative, which is the only way a
floor on a negative numerator is ever exercised: `allocate` never builds one.

The inputs have the shapes the service builds: str ids, stored amounts inside
int64, request amounts of any size (pydantic's strict int has no bound),
derived sums of any size, dicts whose order matters written as lists of pairs.
Non-ASCII characters whose identity matters are spelled with `chr()`.

## Encoding

The encoding of scripts/render_domain_w2_goldens.py, unchanged:

* a case is `{"fn", "name", "args": {parameter: value}, "result"}` and a result
  is `{"ok": value}` or `{"raised": {"type", "message", "code"}}`;
* `"$i:<hex>"` is an int of nine or more digits;
* `"$sp:<text>"` is a str cut into groups of six code points joined by `|`,
  used when a str starts with `$` or would trip scripts/repo_guard.py;
* a set is its sorted list; a tuple is a list.

A datetime argument is its isoformat() string with an offset. Every file
writes one header key per line and then one case per line, which keeps the
committed files small and every line under the guard's line limit.
"""

from __future__ import annotations

import json
import platform
import random
import re
import sys
import uuid
from datetime import UTC, datetime, timedelta, timezone
from fractions import Fraction
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import schemas  # noqa: E402
from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402
from app.api.repository import (  # noqa: E402
    AllocationRow,
    BatchForPublish,
    BatchInputs,
    BillDiscountRecord,
    BillItemRecord,
    BillRecord,
    BillShareRecord,
    BillSurchargeRecord,
    ConfirmedExpense,
    ExpenseIdentity,
    PublishObligation,
    ReceiptRecord,
    ReceiptTarget,
)
from app.domain import (  # noqa: E402
    allocator,
    bill,
    budget,
    capability,
    collection,
    contract,
    expense,
    ledger,
)

SEED = 44
TARGET = "services/core/{path}/testdata/python_{mode}.json"
SMALL_INT = 10**8
GROUP = 6
INT64_MAX = 2**63 - 1
INT64_MIN = -(2**63)
MAX = contract.MAX_AMOUNT_VND

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
    if isinstance(value, datetime):
        return enc_str(value.isoformat())
    if isinstance(value, Fraction):
        return enc_str(f"{value.numerator}/{value.denominator}")
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


def case(fn: str, name: str, kwargs: dict, call: dict | None = None, shape=None) -> dict:
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


def split_count(count: int, defaults: tuple) -> list[int]:
    """Cases per part of a fuzz of `count` cases, in proportion to the defaults."""
    total = sum(defaults)
    return [max(1, default * count // total) for default in defaults]


def pairs(mapping: dict) -> list[list]:
    return [[key, value] for key, value in mapping.items()]


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

E_ACUTE = chr(0xE9)
E_DOT_CIRCUMFLEX = chr(0x1EC7)
COMBINING_ACUTE = chr(0x301)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
OMEGA = chr(0x3C9)
IDEOGRAPHIC_SPACE = chr(0x3000)
NO_BREAK_SPACE = chr(0xA0)
ZERO_WIDTH_SPACE = chr(0x200B)
NEXT_LINE = chr(0x85)
FILE_SEPARATOR = "\x1c"
LONG_S = chr(0x17F)
KELVIN = chr(0x212A)
SHARP_S = chr(0xDF)
DOTTED_CAPITAL_I = chr(0x130)
FULLWIDTH_V = chr(0xFF36)

# Amounts at the boundaries the allocator draws, and past int64.
BIG_AMOUNTS = (MAX - 1, MAX, MAX + 1, INT64_MAX, INT64_MAX + 1, 2**64, 2**70)
BAD_AMOUNTS = (0, -1, INT64_MIN, INT64_MIN - 1, -(2**64)) + BIG_AMOUNTS

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
C = "a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
D = "a4a4a4a4-b4b4-4c4c-8d4d-e4e4e4e4e4e4"
E = "a5a5a5a5-b5b5-4c5c-8d5d-e5e5e5e5e5e5"
F = "f0f0f0f0-b6b6-4c6c-8d6d-e6e6e6e6e6e6"
LOW = "0a0a0a0a-0b0b-4c0c-8d0d-0e0e0e0e0e0e"
HIGH = "fafafafa-fbfb-4cfc-8dfd-fefefefefefe"
CONTEXT = "c0c0c0c0-cdcd-4ece-8fcf-dadadadadada"
OTHER_CONTEXT = "c1c1c1c1-cdcd-4ece-8fcf-dadadadadada"
PEOPLE = (A, B, C, D, E, F, LOW, HIGH)


def random_uuid(rng: random.Random) -> str:
    return str(uuid.UUID(int=rng.getrandbits(128)))


# ---------------------------------------------------------------------------
# allocator
# ---------------------------------------------------------------------------


def expense_dict(participants, total, items=(), surcharges=(), discounts=(), advancer=None) -> dict:
    return {
        "participants": list(participants),
        "total_vnd": total,
        "items": [dict(item) for item in items],
        "surcharges": [dict(surcharge) for surcharge in surcharges],
        "discounts": [dict(discount) for discount in discounts],
        "advancer_id": advancer,
    }


def item(item_id, amount, shared_by) -> dict:
    return {"item_id": item_id, "amount_vnd": amount, "shared_by": list(shared_by)}


def surcharge(surcharge_id, kind, amount, mode) -> dict:
    return {"surcharge_id": surcharge_id, "kind": kind, "amount_vnd": amount, "mode": mode}


def discount(discount_id, amount, scope, item_id=None) -> dict:
    return {"discount_id": discount_id, "amount_vnd": amount, "scope": scope, "item_id": item_id}


def allocation_shape(result: dict) -> dict:
    return {
        "allocations": pairs(result["allocations"]),
        "exact_shares": pairs(result["exact_shares"]),
        "rounding_gainers": result["rounding_gainers"],
        "warnings": result["warnings"],
    }


def allocate_case(name: str, expense_input: dict) -> dict:
    return case("allocate", name, {"expense": expense_input}, shape=allocation_shape)


def allocator_constants() -> dict:
    return {
        "names": public_names(allocator) + public_names(contract),
        "max_amount_vnd": contract.MAX_AMOUNT_VND,
        "max_id_bytes": contract.MAX_ID_BYTES,
        "error_precedence": list(contract.ERROR_PRECEDENCE),
        "warnings": list(contract.WARNINGS),
        "surcharge_modes": list(contract.SURCHARGE_MODES),
        "discount_scopes": list(contract.DISCOUNT_SCOPES),
    }


def allocator_edges() -> list[dict]:
    out: list[dict] = []
    three = ["a", "b", "c"]

    # Even split: every total boundary, three advancer positions.
    totals = (0, 1, 2, 3, 4, 100, 100000, MAX - 1, MAX, MAX + 1, INT64_MAX, INT64_MAX + 1, 2**64, -1, INT64_MIN, INT64_MIN - 1)
    for k, total in enumerate(totals):
        for advancer in ("a", "c", None, "z", ""):
            out.append(allocate_case(f"even/{k}/{advancer}", expense_dict(three, total, advancer=advancer)))
    for count in (1, 2, 7, 40):
        people = [f"p{i:03d}" for i in range(count)]
        for total in (count - 1, count + 1, MAX):
            out.append(allocate_case(f"many/{count}/{total % 1000}", expense_dict(people, total, advancer=people[-1])))

    # The ranking: remainder first, then the advancer, then UTF-8 bytes.
    ranking_ids = ["Z", "a", E_ACUTE, "e" + COMBINING_ACUTE, CJK_MIDDLE, GRINNING_FACE, "aa", "_"]
    for advancer in [None, "outsider"] + ranking_ids:
        out.append(allocate_case(f"rank/even/{advancer}", expense_dict(ranking_ids, 13, advancer=advancer)))
        out.append(
            allocate_case(
                f"rank/items/{advancer}",
                expense_dict(
                    ranking_ids,
                    31,
                    [item("i1", 10, ranking_ids[:3]), item("i2", 20, ranking_ids[2:]), item("i3", 1, ranking_ids[::2])],
                    advancer=advancer,
                ),
            )
        )

    # Zero amounts, and the id "total" the zero check reads as the total.
    out += [
        allocate_case("zero_item", expense_dict(three, 0, [item("i", 0, three)])),
        allocate_case("zero_item_named_total", expense_dict(three, 0, [item("total", 0, three)])),
        allocate_case("zero_item_named_total_with_another", expense_dict(three, 7, [item("total", 0, ["a"]), item("i", 7, three)])),
        allocate_case("zero_surcharge_named_total", expense_dict(three, 5, [item("i", 5, three)], [surcharge("total", "fee", 0, "even")])),
        allocate_case("zero_proportional_surcharge_named_total", expense_dict(three, 5, [item("i", 5, ["a"])], [surcharge("total", "vat", 0, "proportional")])),
        allocate_case("zero_item_discount_named_total", expense_dict(three, 5, [item("i", 5, three)], [], [discount("total", 0, "item", "i")])),
        allocate_case("zero_global_discount_named_total", expense_dict(three, 5, [item("i", 5, three)], [], [discount("total", 0, "global_proportional")])),
        allocate_case("zero_named_Total_is_refused", expense_dict(three, 0, [item("Total", 0, three)])),
        allocate_case("total_zero_item_one_mismatch", expense_dict(three, 0, [item("i", 1, three)])),
    ]

    # Cap and sign, one past each boundary and past int64, in every slot.
    for k, amount in enumerate(BAD_AMOUNTS):
        out.append(allocate_case(f"cap/item/{k}", expense_dict(three, 5, [item("i", amount, three)])))
        out.append(allocate_case(f"cap/surcharge/{k}", expense_dict(three, 5, [item("i", 5, three)], [surcharge("s", "fee", amount, "even")])))
        out.append(allocate_case(f"cap/discount/{k}", expense_dict(three, 5, [item("i", 5, three)], [], [discount("d", amount, "item", "i")])))
        out.append(allocate_case(f"cap/total_itemized/{k}", expense_dict(three, amount, [item("i", 5, three)])))
    out += [
        allocate_case("cap/max_item_exact", expense_dict(three, MAX, [item("i", MAX, three)])),
        allocate_case("cap/two_max_items_total_past_max", expense_dict(three, 2 * MAX, [item("i", MAX, three), item("j", MAX, ["a"])])),
        allocate_case("cap/two_max_items_total_max", expense_dict(three, MAX, [item("i", MAX, three), item("j", MAX, ["a"])])),
        allocate_case("cap/max_even_seven", expense_dict([f"q{i}" for i in range(7)], MAX, advancer="q3")),
        allocate_case("precedence/negative_beats_zero", expense_dict(three, 5, [item("i", 0, three), item("j", -1, three)])),
        allocate_case("precedence/zero_beats_too_large", expense_dict(three, 5, [item("i", MAX + 1, three), item("j", 0, three)])),
        allocate_case("precedence/too_large_beats_kind", expense_dict(three, 5, [item("i", 2**64, three)], [surcharge("s", "", 1, "even")])),
        allocate_case("precedence/negative_past_int64_beats_too_large", expense_dict(three, 2**70, [item("i", -(2**64), three)])),
        allocate_case("precedence/mode_beats_scope", expense_dict(three, 5, [item("i", 5, three)], [surcharge("s", "fee", 1, "Even")], [discount("d", 1, "per_item", "i")])),
        allocate_case("precedence/scope_beats_mismatch", expense_dict(three, 5, [item("i", 5, three)], [], [discount("d", 1, "bogus"), discount("e", 1, "global_proportional", "i")])),
        allocate_case("precedence/mismatch_beats_empty_shared_by", expense_dict(three, 5, [item("i", 5, [])], [], [discount("d", 1, "item")])),
        allocate_case("precedence/empty_beats_duplicate_shared_by", expense_dict(three, 5, [item("i", 5, ["a", "a"]), item("j", 5, [])])),
        allocate_case("precedence/duplicate_shared_by_beats_unknown", expense_dict(three, 5, [item("i", 5, ["z"]), item("j", 5, ["a", "a"])])),
        allocate_case("precedence/unknown_participant_beats_unknown_item", expense_dict(three, 5, [item("i", 5, ["z"])], [], [discount("d", 1, "item", "nope")])),
        allocate_case("precedence/unknown_item_beats_exceeds_item", expense_dict(three, 5, [item("i", 5, three)], [], [discount("d", 9, "item", "i"), discount("e", 1, "item", "nope")])),
        allocate_case("precedence/exceeds_item_beats_exceeds_base", expense_dict(three, 0, [item("i", 5, three)], [], [discount("d", 6, "item", "i"), discount("g", 100, "global_proportional")])),
        allocate_case("precedence/exceeds_base_beats_reconciliation", expense_dict(three, 999, [item("i", 5, three)], [], [discount("g", 6, "global_proportional")])),
        allocate_case("precedence/invalid_participant_beats_no_duplicate", expense_dict([" a", " a"], 5)),
        allocate_case("precedence/duplicate_participant_beats_entity", expense_dict(["a", "a"], 5, [item("", 5, ["a"])])),
        allocate_case("precedence/entity_beats_duplicate_entity", expense_dict(three, 5, [item("i", 5, three), item("i", 5, three)], [surcharge(" s", "fee", 1, "even")])),
        allocate_case("duplicate_surcharge_id", expense_dict(three, 7, [item("i", 5, three)], [surcharge("s", "fee", 1, "even"), surcharge("s", "vat", 1, "even")])),
        allocate_case("namespaces_may_share_an_id", expense_dict(three, 9, [item("x", 5, three)], [surcharge("x", "fee", 5, "even")], [discount("x", 1, "item", "x")])),
    ]

    # Ids: Python whitespace at either end, the 64-byte bound in UTF-8.
    for k, bad in enumerate((" a", "a ", "\ta", "a\n", IDEOGRAPHIC_SPACE + "a", "a" + NO_BREAK_SPACE, NEXT_LINE + "a", "a" + FILE_SEPARATOR, "", "   ")):
        out.append(allocate_case(f"id/participant/{k}", expense_dict([bad, "b"], 10)))
        out.append(allocate_case(f"id/item/{k}", expense_dict(three, 5, [item(bad, 5, three)])))
    for k, good in enumerate(("a" + ZERO_WIDTH_SPACE, ZERO_WIDTH_SPACE, "a b", "x" * 64, E_DOT_CIRCUMFLEX * 21 + "a", GRINNING_FACE * 16)):
        out.append(allocate_case(f"id/good/{k}", expense_dict([good, "b"], 10, [item(good, 10, [good])])))
    for k, long in enumerate(("x" * 65, E_DOT_CIRCUMFLEX * 21 + "ab", GRINNING_FACE * 16 + "a")):
        out.append(allocate_case(f"id/long_participant/{k}", expense_dict([long, "b"], 10)))
        out.append(allocate_case(f"id/long_discount/{k}", expense_dict(three, 4, [item("i", 5, three)], [], [discount(long, 1, "global_proportional")])))

    # Kinds, modes and scopes.
    for k, kind in enumerate(("", "k" * 32, "k" * 33, E_DOT_CIRCUMFLEX * 10 + "ab", E_DOT_CIRCUMFLEX * 11, " ", GRINNING_FACE * 8, GRINNING_FACE * 8 + "a")):
        out.append(allocate_case(f"kind/{k}", expense_dict(three, 6, [item("i", 5, three)], [surcharge("s", kind, 1, "even")])))
    for k, mode in enumerate(("even", "proportional", "Even", "even ", "", "split_evenly")):
        out.append(allocate_case(f"mode/{k}", expense_dict(three, 6, [item("i", 5, ["a"])], [surcharge("s", "fee", 1, mode)])))
    for k, scope in enumerate(("item", "global_proportional", "Item", "", "per_item")):
        out.append(allocate_case(f"scope/{k}/target", expense_dict(three, 4, [item("i", 5, three)], [], [discount("d", 1, scope, "i")])))
        out.append(allocate_case(f"scope/{k}/none", expense_dict(three, 4, [item("i", 5, three)], [], [discount("d", 1, scope)])))

    # Arithmetic corners.
    out += [
        allocate_case("fallback/item_fully_discounted", expense_dict(three, 4, [item("i", 5, ["a"])], [surcharge("s", "fee", 3, "proportional"), surcharge("t", "fee", 1, "proportional")], [discount("d", 5, "item", "i")])),
        allocate_case("fallback/global_discount_equals_base", expense_dict(three, 7, [item("i", 5, ["a", "b"])], [surcharge("s", "fee", 7, "proportional")], [discount("g", 5, "global_proportional")])),
        allocate_case("fallback/twice_warns_once", expense_dict(three, 8, [item("i", 5, ["a"])], [surcharge("s", "fee", 4, "proportional"), surcharge("t", "vat", 4, "proportional")], [discount("g", 5, "global_proportional")])),
        allocate_case("even_surcharge_with_zero_basis_no_warning", expense_dict(three, 4, [item("i", 5, ["a"])], [surcharge("s", "fee", 4, "even")], [discount("g", 5, "global_proportional")])),
        allocate_case("global_discount_exactly_base_zero_total", expense_dict(three, 0, [item("i", 5, three)], [], [discount("g", 5, "global_proportional")])),
        allocate_case("global_discount_one_past_base", expense_dict(three, -1, [item("i", 5, three)], [], [discount("g", 6, "global_proportional")])),
        allocate_case("item_discount_equals_item", expense_dict(three, 5, [item("i", 5, ["a"]), item("j", 5, ["b"])], [], [discount("d", 5, "item", "i")])),
        allocate_case("two_item_discounts_exceed_together", expense_dict(three, -1, [item("i", 5, ["a"])], [], [discount("d", 3, "item", "i"), discount("e", 3, "item", "i")])),
        allocate_case("zero_share_warning_with_outside_advancer", expense_dict(three, 9, [item("i", 9, ["a", "b"])], advancer="zz")),
        allocate_case("zero_share_no_warning_at_total_zero", expense_dict(three, 0, [item("i", 5, ["a"])], [], [discount("g", 5, "global_proportional")])),
        allocate_case("proportional_thirds", expense_dict(three, 101, [item("i", 1, ["a"]), item("j", 2, ["b"]), item("k", 4, ["c"])], [surcharge("s", "fee", 94, "proportional")])),
        allocate_case("big_numerators", expense_dict(["a", "b", "c", "d", "e", "f", "g"], MAX, [item("i", MAX - 12345, ["a", "b", "c"]), item("j", 999_983, ["d", "e", "f", "g"])], [surcharge("s", "fee", 12345, "proportional")], [discount("d", 999_983, "global_proportional")])),
        allocate_case("advancer_empty_string_in_participants", expense_dict(["", "b"], 3)),
        allocate_case("advancer_empty_string_outside", expense_dict(three, 4, advancer="")),
        allocate_case("no_participants_total_negative", expense_dict([], -5)),
        allocate_case("no_participants_with_items", expense_dict([], 5, [item("i", 5, ["a"])])),
    ]
    return out


PARTICIPANT_POOL = (
    "a", "aa", "b", "z", "Z", "_", "-", "an", "x1", "x10", "ha", "nam", "Nam", "total",
    chr(0xE1), "a" + COMBINING_ACUTE, "B" + chr(0x1EA3) + "o", "b" + chr(0x1EA3) + "o",
    "H" + chr(0xE0), OMEGA, GRINNING_FACE, CJK_MIDDLE, E_ACUTE,
) + tuple(f"p{i}" for i in range(40))
ENTITY_POOL = ("i0", "i1", "i2", "i3", "i4", "i5", "i6", "i7", "total", "Total", "x", E_ACUTE, GRINNING_FACE)
KINDS = ("fee", "vat", "VAT", "Vat", "shipping", "SHIPPING", LONG_S + "hipping", "service", "unlisted", KELVIN + "ind")


def fuzz_amount(rng: random.Random, low: int = 1) -> int:
    roll = rng.random()
    if roll < 0.3:
        return rng.randint(low, 9)
    if roll < 0.7:
        return rng.randint(low, 500_000)
    if roll < 0.92:
        return rng.randint(low, 10**9)
    return rng.choice((MAX, MAX - 1, rng.randint(10**11, MAX)))


def fuzz_advancer(rng: random.Random, participants: list) -> str | None:
    roll = rng.random()
    if roll < 0.5:
        return rng.choice(participants)
    if roll < 0.75:
        return None
    return rng.choice(("outsider", "", "zzz", "a"))


def fuzz_valid_expense(rng: random.Random) -> dict:
    count = rng.choice((1, 2, 2, 3, 3, 3, 4, 4, 5, 6, 8)) if rng.random() < 0.97 else rng.randint(9, 20)
    participants = rng.sample(PARTICIPANT_POOL, count)
    if rng.random() < 0.18:
        total = rng.choice((0, 1, count - 1, count, count + 1, fuzz_amount(rng), MAX))
        return expense_dict(participants, total, advancer=fuzz_advancer(rng, participants))

    ids = rng.sample(ENTITY_POOL, rng.choice((1, 1, 2, 2, 3, 4, 5)))
    items = [item(item_id, fuzz_amount(rng), rng.sample(participants, rng.randint(1, count))) for item_id in ids]
    discounts = []
    discount_ids = iter(rng.sample(ENTITY_POOL, len(ENTITY_POOL)))
    for _ in range(rng.choice((0, 0, 1, 2, 3))):
        target = rng.choice(items)
        headroom = target["amount_vnd"] - sum(d["amount_vnd"] for d in discounts if d["item_id"] == target["item_id"])
        if headroom <= 0:
            continue
        amount = headroom if rng.random() < 0.15 else rng.randint(1, headroom)
        discounts.append(discount(next(discount_ids), amount, "item", target["item_id"]))
    base = sum(i["amount_vnd"] for i in items) - sum(d["amount_vnd"] for d in discounts)
    for _ in range(rng.choice((0, 0, 1, 2))):
        headroom = base - sum(d["amount_vnd"] for d in discounts if d["scope"] == "global_proportional")
        if headroom <= 0:
            continue
        amount = headroom if rng.random() < 0.15 else rng.randint(1, headroom)
        discounts.append(discount(next(discount_ids), amount, "global_proportional"))
    surcharges = [
        surcharge(surcharge_id, rng.choice(KINDS), fuzz_amount(rng), rng.choice(("proportional", "even")))
        for surcharge_id in rng.sample(ENTITY_POOL, rng.choice((0, 0, 0, 1, 1, 2, 3)))
    ]
    total = sum(i["amount_vnd"] for i in items) + sum(s["amount_vnd"] for s in surcharges) - sum(d["amount_vnd"] for d in discounts)
    return expense_dict(participants, total, items, surcharges, discounts, fuzz_advancer(rng, participants))


def amount_slots(expense_input: dict) -> list[tuple[dict, str]]:
    slots = [(expense_input, "total_vnd")]
    for group in ("items", "surcharges", "discounts"):
        slots += [(element, "amount_vnd") for element in expense_input[group]]
    return slots


def corrupt(rng: random.Random, e: dict) -> None:
    """Break one rule at random, in place. Some choices change nothing."""
    roll = rng.randrange(20)
    participants = e["participants"]
    if not participants and roll in (3, 4, 13):
        roll = 0
    if roll == 0:
        e["total_vnd"] += rng.choice((1, -1))
    elif roll in (1, 2):
        target, key = rng.choice(amount_slots(e))
        target[key] = rng.choice(BAD_AMOUNTS)
    elif roll == 3:
        participants.append(rng.choice(participants))
    elif roll == 4:
        participants[rng.randrange(len(participants))] = rng.choice((" a", "a ", "", "x" * 65, IDEOGRAPHIC_SPACE, "b\n"))
    elif roll == 5 and e["items"]:
        element = rng.choice(e["items"] + e["surcharges"] + e["discounts"])
        key = next(k for k in ("item_id", "surcharge_id", "discount_id") if k in element and (k != "item_id" or "shared_by" in element))
        element[key] = rng.choice((" i0", "", "y" * 65, "i0", "i1", "total"))
    elif roll == 6 and e["surcharges"]:
        rng.choice(e["surcharges"])["kind"] = rng.choice(("", "k" * 33, E_DOT_CIRCUMFLEX * 11, "k" * 32))
    elif roll == 7 and e["surcharges"]:
        rng.choice(e["surcharges"])["mode"] = rng.choice(("Even", "", "evenly", "proportional"))
    elif roll == 8 and e["discounts"]:
        rng.choice(e["discounts"])["scope"] = rng.choice(("Item", "", "global", "item"))
    elif roll == 9 and e["discounts"]:
        target = rng.choice(e["discounts"])
        target["item_id"] = None if target["item_id"] is not None else rng.choice([i["item_id"] for i in e["items"]] or ["i0"])
    elif roll == 10 and e["items"]:
        target = rng.choice(e["items"])
        target["shared_by"] = rng.choice(([], target["shared_by"] + target["shared_by"][:1], target["shared_by"] + ["stranger"]))
    elif roll == 11 and e["discounts"]:
        rng.choice(e["discounts"])["item_id"] = rng.choice(("nope", "i9"))
    elif roll == 12 and e["discounts"]:
        target = rng.choice(e["discounts"])
        target["amount_vnd"] += rng.choice((1, 2, 1000, target["amount_vnd"]))
    elif roll == 13:
        e["advancer_id"] = rng.choice((None, "outsider", "", rng.choice(participants)))
    elif roll == 14 and e["items"]:
        rng.choice(e["items"])["amount_vnd"] = 0
    elif roll == 15:
        e["participants"] = []
    elif roll == 16 and e["items"]:
        e["discounts"].append(discount("gx", rng.randint(1, 3 * MAX), "global_proportional"))
    else:
        # A permutation: the answer must not depend on input order.
        rng.shuffle(participants)
        rng.shuffle(e["items"])
        rng.shuffle(e["surcharges"])
        rng.shuffle(e["discounts"])
        for element in e["items"]:
            rng.shuffle(element["shared_by"])


def allocator_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 1)
    out = []
    for i in range(count):
        e = fuzz_valid_expense(rng)
        if rng.random() < 0.5:
            for _ in range(rng.choice((1, 1, 1, 2, 3))):
                corrupt(rng, e)
        out.append(allocate_case(f"fuzz/{i}", e))
    return out


# _apportion called directly. The exact shares always sum to the total, the
# only way allocate calls it, but may be negative, which allocate never builds.


def apportion_case(name: str, total: int, exact: dict, advancer) -> dict:
    return case(
        "apportion",
        name,
        {"total_vnd": total, "exact": pairs(exact), "advancer_id": advancer},
        call={"total_vnd": total, "exact": exact, "advancer_id": advancer},
        shape=lambda value: {"allocations": pairs(value[0]), "gainers": value[1]},
    )


def exact_summing_to(total: int, people: list, fractions: list) -> dict:
    shares = dict(zip(people[:-1], fractions, strict=True))
    shares[people[-1]] = Fraction(total) - sum(fractions, Fraction(0))
    return shares


def apportion_constants() -> dict:
    return {"names": []}


def apportion_edges() -> list[dict]:
    out = []
    thirds = ["a", "b", "c"]
    out.append(apportion_case("thirds_advancer_b", 1, {p: Fraction(1, 3) for p in thirds}, "b"))
    out.append(apportion_case("thirds_no_advancer", 2, {p: Fraction(2, 3) for p in thirds}, None))
    out.append(apportion_case("negative_thirds", -1, {p: Fraction(-1, 3) for p in thirds}, None))
    out.append(apportion_case("negative_two_thirds_advancer_c", -2, {p: Fraction(-2, 3) for p in thirds}, "c"))
    out.append(apportion_case("negative_and_positive", 1, {"a": Fraction(-4, 3), "b": Fraction(7, 3)}, None))
    out.append(apportion_case("negative_integer_shares", -5, {"a": Fraction(-2), "b": Fraction(-3)}, "a"))
    out.append(apportion_case("half_ties", 1, {"a": Fraction(1, 2), "b": Fraction(1, 2)}, None))
    out.append(apportion_case("half_ties_advancer_b", 1, {"a": Fraction(1, 2), "b": Fraction(1, 2)}, "b"))
    out.append(apportion_case("negative_half_ties", -1, {"a": Fraction(-1, 2), "b": Fraction(-1, 2)}, "b"))
    out.append(apportion_case("three_halves", 3, {"a": Fraction(3, 2), "b": Fraction(3, 2)}, None))
    out.append(apportion_case("minus_three_halves", -3, {"a": Fraction(-3, 2), "b": Fraction(-3, 2)}, None))
    out.append(apportion_case("utf8_order", 2, {GRINNING_FACE: Fraction(1, 2), CJK_MIDDLE: Fraction(1, 2), "Z": Fraction(1, 2), "a": Fraction(1, 2)}, None))
    out.append(apportion_case("past_int64", 2**64 + 1, {"a": Fraction(2**64 + 1, 2), "b": Fraction(2**64 + 1, 2)}, None))
    out.append(apportion_case("below_int64", -(2**64) - 1, {"a": Fraction(-(2**64) - 1, 2), "b": Fraction(-(2**64) - 1, 2)}, "a"))
    out.append(apportion_case("empty", 0, {}, None))
    return out


def apportion_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 2)
    out = []
    for i in range(count):
        count = rng.randint(1, 9)
        people = rng.sample(PARTICIPANT_POOL, count)
        denominator = rng.choice((1, 2, 3, 4, 6, 7, 12, 30, 97))
        magnitude = rng.choice((3, 100, 10**6, MAX, 2**66))
        fractions = [Fraction(rng.randint(-magnitude, magnitude), rng.choice((denominator, rng.randint(1, 13)))) for _ in range(count - 1)]
        if rng.random() < 0.3 and fractions:
            fractions = [fractions[0]] * len(fractions)
        total = rng.randint(-magnitude, magnitude)
        advancer = rng.choice(people + [None, "outsider"])
        out.append(apportion_case(f"fuzz/{i}", total, exact_summing_to(total, people, fractions), advancer))
    return out


# ---------------------------------------------------------------------------
# bill (Go package billdraft)
# ---------------------------------------------------------------------------


def bill_item(key, amount, shares) -> dict:
    return {
        "item_key": key,
        "amount_vnd": amount,
        "shares": [{"participant_id": who, "source": source} for who, source in shares],
    }


def bill_dict(participants, printed, items, surcharges=(), discounts=(), advancer=None) -> dict:
    return {
        "participants": list(participants),
        "printed_total_vnd": printed,
        "items": list(items),
        "surcharges": [dict(s) for s in surcharges],
        "discounts": [dict(d) for d in discounts],
        "advancer_id": advancer,
    }


def bill_case(name: str, draft: dict) -> dict:
    return case("allocator_input_from_bill", name, {"bill": draft})


def billdraft_constants() -> dict:
    return {
        "names": public_names(bill),
        "share_suggested": bill.SHARE_SUGGESTED,
        "share_confirmed": bill.SHARE_CONFIRMED,
    }


def billdraft_edges() -> list[dict]:
    ok, ai = "confirmed", "ai_suggested"
    people = ["a", "b", "c"]
    out = [
        bill_case("no_items", bill_dict(people, 100, [])),
        bill_case("no_items_no_total", bill_dict(people, None, [])),
        bill_case("one_confirmed", bill_dict(people, 100, [bill_item("pho", 100, [("a", ok)])], advancer="a")),
        bill_case("one_suggested", bill_dict(people, 100, [bill_item("pho", 100, [("a", ai)])])),
        bill_case("mixed_item", bill_dict(people, 100, [bill_item("pho", 100, [("a", ok), ("b", ai)])])),
        bill_case("no_assignee", bill_dict(people, 100, [bill_item("pho", 100, [])])),
        bill_case("no_assignee_beats_bad_source", bill_dict(people, 100, [bill_item("a", 1, [("a", "AI")]), bill_item("b", 1, [])])),
        bill_case("bad_source_later_key_still_found", bill_dict(people, 100, [bill_item("z", 1, [("a", "")]), bill_item("a", 1, [("a", ok)])])),
        bill_case("source_with_space", bill_dict(people, 100, [bill_item("a", 1, [("a", "confirmed ")])])),
        bill_case("source_upper", bill_dict(people, 100, [bill_item("a", 1, [("a", "CONFIRMED")])])),
        bill_case(
            "suggested_keys_in_byte_order",
            bill_dict(people, None, [bill_item(k, 10, [("a", ai)]) for k in ("b", "a", "Z", E_ACUTE, GRINNING_FACE, CJK_MIDDLE, "aa")]),
        ),
        bill_case(
            "listed_total_with_surcharges_and_discounts",
            bill_dict(people, None, [bill_item("i", 100, [("a", ok)])], [surcharge("s", "vat", 8, "proportional")], [discount("d", 30, "item", "i"), discount("g", 5, "global_proportional")]),
        ),
        bill_case("printed_total_wins", bill_dict(people, 7, [bill_item("i", 100, [("a", ok)])], [surcharge("s", "vat", 8, "even")])),
        bill_case("printed_total_zero", bill_dict(people, 0, [bill_item("i", 100, [("a", ok)])])),
        bill_case("listed_total_negative", bill_dict(people, None, [bill_item("i", 1, [("a", ok)])], [], [discount("g", 5, "global_proportional")])),
        bill_case("listed_total_past_int64", bill_dict(people, None, [bill_item("i", INT64_MAX, [("a", ok)]), bill_item("j", INT64_MAX, [("b", ok)])])),
        bill_case("listed_total_below_int64", bill_dict(people, None, [bill_item("i", 1, [("a", ok)])], [], [discount("g", INT64_MAX, "global_proportional"), discount("h", INT64_MAX, "global_proportional")])),
        bill_case("negative_item_amount_passes_through", bill_dict(people, None, [bill_item("i", -5, [("a", ok)])])),
        bill_case("duplicate_keys_pass_through", bill_dict(people, None, [bill_item("i", 5, [("a", ai)]), bill_item("i", 6, [("b", ai)])])),
        bill_case("stranger_is_left_for_the_allocator", bill_dict(people, None, [bill_item("i", 5, [("zz", ok)])])),
        bill_case("no_participants", bill_dict([], None, [bill_item("i", 5, [("a", ok)])])),
        bill_case("repeated_share", bill_dict(people, None, [bill_item("i", 5, [("a", ok), ("a", ai)])])),
    ]
    return out


def billdraft_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 3)
    out = []
    sources = ("confirmed", "confirmed", "confirmed", "ai_suggested", "ai_suggested", "AI", "", "Confirmed")
    for i in range(count):
        participants = rng.sample(PARTICIPANT_POOL[:20], rng.randint(0, 5))
        items = []
        for key in rng.sample(ENTITY_POOL + ("pho", "bia", "b", "a"), rng.choice((0, 1, 1, 2, 3, 5))):
            shares = []
            for _ in range(rng.choice((0, 1, 1, 2, 3)) if rng.random() < 0.95 else 0):
                who = rng.choice(participants + ["stranger"]) if participants else "a"
                shares.append((who, rng.choice(sources) if rng.random() < 0.3 else rng.choice(sources[:5])))
            amount = rng.choice((fuzz_amount(rng), 0, -1, INT64_MAX)) if rng.random() < 0.1 else fuzz_amount(rng)
            items.append(bill_item(key, amount, shares))
        surcharges = [surcharge(f"s{k}", rng.choice(KINDS), rng.choice((fuzz_amount(rng), INT64_MAX)), rng.choice(("even", "proportional"))) for k in range(rng.choice((0, 0, 1, 2)))]
        discounts = [discount(f"d{k}", rng.choice((fuzz_amount(rng), INT64_MAX)), "global_proportional") for k in range(rng.choice((0, 0, 1)))]
        printed = None if rng.random() < 0.5 else rng.choice((0, fuzz_amount(rng), INT64_MAX))
        advancer = rng.choice(participants + [None, "outsider"])
        out.append(bill_case(f"fuzz/{i}", bill_dict(participants, printed, items, surcharges, discounts, advancer)))
    return out


# ---------------------------------------------------------------------------
# budget
# ---------------------------------------------------------------------------


def outing(title="Đà Lạt", headcount=4, budget_vnd=300_000, split_total=1_200_000, in_progress=False, outing_id="outing-1") -> dict:
    return {
        "outing_id": outing_id,
        "title": title,
        "headcount": headcount,
        "budget_per_person_vnd": budget_vnd,
        "split_total_vnd": split_total,
        "in_progress": in_progress,
    }


def budget_case(name: str, outings: list, active, candidate) -> dict:
    args = {"outings": outings, "active_member_count": active, "candidate_per_person_vnd": candidate}
    return case("build_group_budget", name, args)


def budget_constants() -> dict:
    return {
        "names": public_names(budget),
        "comparison_tolerance_percent": budget.COMPARISON_TOLERANCE_PERCENT,
    }


def budget_edges() -> list[dict]:
    out = [
        budget_case(
            "test_budget_computes_history_current_spend_and_candidate_together",
            [
                outing(),
                outing(outing_id="outing-2", title="Nướng cuối tuần", headcount=5, split_total=800_000),
                outing(outing_id="outing-live", title="Đang đi biển", headcount=3, split_total=1_000_001, in_progress=True),
            ],
            2,
            450_000,
        ),
        budget_case("no_candidate", [outing()], 2, None),
        budget_case("no_finished_history", [outing(in_progress=True)], 2, 450_000),
        budget_case("zero_finished_headcount", [outing(headcount=0, split_total=0)], 0, 450_000),
        budget_case("zero_live_headcount", [outing(headcount=0, budget_vnd=400_000, split_total=340_000, in_progress=True)], 0, None),
        budget_case("zero_baseline_same", [outing(headcount=2, split_total=0)], 2, 0),
        budget_case("zero_baseline_higher", [outing(headcount=2, split_total=0)], 2, 1),
        budget_case("empty", [], 0, None),
        budget_case("empty_with_candidate", [], 5, 100),
        budget_case("over_budget", [outing(headcount=3, budget_vnd=100, split_total=301, in_progress=True)], 3, None),
        budget_case("exactly_on_budget", [outing(headcount=3, budget_vnd=100, split_total=300, in_progress=True)], 3, None),
        budget_case("floor_not_round", [outing(headcount=3, split_total=2), outing(headcount=3, split_total=2)], 3, 0),
        budget_case("split_total_past_int64", [outing(headcount=3, split_total=2**64 + 5), outing(headcount=2, split_total=2**70, in_progress=True)], 3, INT64_MAX),
        budget_case("candidate_int64_max", [outing(headcount=1, split_total=INT64_MAX)], 1, INT64_MAX),
        budget_case("title_is_stripped", [outing(title=IDEOGRAPHIC_SPACE + " Hội An\t" + NEXT_LINE, in_progress=True)], 1, None),
        budget_case("title_zero_width_is_not_blank", [outing(title=ZERO_WIDTH_SPACE, in_progress=True)], 1, None),
        budget_case("finished_title_stripped_nowhere", [outing(title=" x ")], 1, None),
    ]
    for candidate, _verdict in ((161_999, "re-hon"), (162_000, "nhu-thuong"), (198_000, "nhu-thuong"), (198_001, "cao-hon")):
        out.append(budget_case(f"band/{candidate}", [outing(headcount=3, split_total=540_000)], 3, candidate))
    for field, value in (("headcount", -1), ("budget_per_person_vnd", -1), ("split_total_vnd", -1), ("split_total_vnd", None), ("split_total_vnd", -(2**64)), ("headcount", INT64_MIN)):
        bad = outing()
        bad[field] = value
        out.append(budget_case(f"invalid/{field}/{len(out)}", [bad], 2, None))
        out.append(budget_case(f"invalid_live/{field}/{len(out)}", [outing(), dict(bad, in_progress=True)], 2, None))
    for k, title in enumerate(("", " ", "\t\n", IDEOGRAPHIC_SPACE, NO_BREAK_SPACE + NEXT_LINE, FILE_SEPARATOR)):
        out.append(budget_case(f"invalid/title/{k}", [outing(title=title)], 2, None))
    out += [
        budget_case("invalid/candidate", [outing()], 2, -1),
        budget_case("invalid/candidate_min", [outing()], 2, INT64_MIN),
        budget_case("invalid/active", [outing()], -1, None),
        budget_case("invalid/active_beats_outing", [outing(headcount=-1)], -1, None),
        budget_case("invalid_after_valid_live", [outing(in_progress=True), outing(title="")], 2, 5),
    ]
    return out


def budget_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 4)
    titles = ("Đà Lạt", " Hội An ", "", " ", "x", IDEOGRAPHIC_SPACE, ZERO_WIDTH_SPACE, "a" + NEXT_LINE)
    out = []
    for i in range(count):
        outings = []
        for k in range(rng.choice((0, 1, 2, 3, 5, 8))):
            headcount = rng.choice((0, 1, 2, 3, 4, 7, 12, rng.randint(0, 50)))
            split_total = rng.choice((0, rng.randint(0, 10**7), rng.randint(0, 10**12), rng.randint(0, 2**70)))
            outings.append(
                outing(
                    title=rng.choice(titles[:2]) if rng.random() < 0.97 else rng.choice(titles),
                    headcount=headcount if rng.random() < 0.98 else -1,
                    budget_vnd=rng.choice((0, 100_000, 300_000, rng.randint(0, 10**7))) if rng.random() < 0.98 else -1,
                    split_total=split_total if rng.random() < 0.98 else rng.choice((None, -1)),
                    in_progress=rng.random() < 0.4,
                    outing_id=f"o{k}",
                )
            )
        active = rng.randint(0, 20) if rng.random() < 0.98 else -1
        candidate = None if rng.random() < 0.3 else rng.choice((0, rng.randint(0, 10**6), rng.randint(0, 10**12), INT64_MAX, -1 if rng.random() < 0.05 else 0))
        out.append(budget_case(f"fuzz/{i}", outings, active, candidate))
    return out


# ---------------------------------------------------------------------------
# expense
# ---------------------------------------------------------------------------


ROLLUP_KINDS = (
    "vat", "VAT", "Vat", "vAt", "shipping", "SHIPPING", "Shipping", LONG_S + "hipping", "SHIPP" + DOTTED_CAPITAL_I + "NG",
    "shipp" + DOTTED_CAPITAL_I.lower() + "ng", "vat ", " vat", FULLWIDTH_V + "AT", KELVIN, "fee", "service", SHARP_S, "", "v" + chr(0x391) + "t",
)


def rollup_case(name: str, e: dict) -> dict:
    return case("component_rollups", name, {"expense": e})


def expense_constants() -> dict:
    return {"names": public_names(expense)}


def expense_edges() -> list[dict]:
    out = [
        rollup_case("test_pure_even_split_projects_total_to_subtotal", {"items": [], "surcharges": [], "discounts": [], "total_vnd": 82000}),
        rollup_case(
            "test_itemized_rollups_keep_vat_shipping_and_catch_all_fee_separate",
            {
                "items": [{"amount_vnd": 100000}, {"amount_vnd": 50000}],
                "surcharges": [{"kind": "VAT", "amount_vnd": 12000}, {"kind": "shipping", "amount_vnd": 5000}, {"kind": "service", "amount_vnd": 3000}],
                "discounts": [{"amount_vnd": 10000}],
                "total_vnd": 160000,
            },
        ),
        rollup_case("only_a_discount_is_not_an_even_split", {"items": [], "surcharges": [], "discounts": [{"amount_vnd": 5}], "total_vnd": 0}),
        rollup_case("only_a_surcharge", {"items": [], "surcharges": [{"kind": "vat", "amount_vnd": 5}], "discounts": [], "total_vnd": 5}),
        rollup_case(
            "sums_past_int64",
            {
                "items": [{"amount_vnd": INT64_MAX}] * 3,
                "surcharges": [{"kind": "VAT", "amount_vnd": INT64_MAX}] * 2 + [{"kind": "x", "amount_vnd": INT64_MAX}] * 2,
                "discounts": [{"amount_vnd": INT64_MAX}] * 2,
                "total_vnd": INT64_MAX,
            },
        ),
    ]
    for k, kind in enumerate(ROLLUP_KINDS):
        out.append(rollup_case(f"kind/{k}", {"items": [{"amount_vnd": 1}], "surcharges": [{"kind": kind, "amount_vnd": 7}], "discounts": [], "total_vnd": 8}))
    return out


def expense_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 5)
    out = []
    for i in range(count):
        e = {
            "items": [{"amount_vnd": rng.choice((fuzz_amount(rng), INT64_MAX))} for _ in range(rng.choice((0, 0, 1, 2, 4)))],
            "surcharges": [{"kind": rng.choice(ROLLUP_KINDS + KINDS), "amount_vnd": rng.choice((fuzz_amount(rng), INT64_MAX))} for _ in range(rng.choice((0, 0, 1, 2, 4)))],
            "discounts": [{"amount_vnd": rng.choice((fuzz_amount(rng), INT64_MAX))} for _ in range(rng.choice((0, 0, 1, 2)))],
            "total_vnd": rng.choice((0, fuzz_amount(rng), MAX, INT64_MAX)),
        }
        out.append(rollup_case(f"fuzz/{i}", e))
    return out


# ---------------------------------------------------------------------------
# collection
# ---------------------------------------------------------------------------

STATUSES = ("confirmed", "over_confirmed", "waived", "disputed", "cancelled", "outstanding", "partially_confirmed", "", "Confirmed")
EVENTS = ("freeze", "cancel", "publish", "reopen", "expose_capability", "close", "", "Freeze", "complete")
EXPOSED = "2026-09-15T09:30:12.345678+00:00"


def obligations_of(rng: random.Random, most: int = 5) -> list[dict]:
    return [
        {"sender_id": rng.choice(("ha", "nam", "binh", "")), "status": rng.choice(STATUSES)}
        for _ in range(rng.randrange(most + 1))
    ]


def context_of(rng: random.Random):
    if rng.random() < 0.08:
        return None
    context = {}
    if rng.random() < 0.8:
        context["obligations"] = obligations_of(rng, 4)
    for key in ("advancer_acknowledged", "delivery_method_chosen", "all_affected_parties_consented"):
        if rng.random() < 0.8:
            context[key] = rng.random() < 0.6
    if rng.random() < 0.5:
        context["capability_exposed_at"] = EXPOSED if rng.random() < 0.6 else None
    return context


def iso(value: str) -> datetime:
    return datetime.fromisoformat(value)


def collection_constants() -> dict:
    return {"names": public_names(collection), "states": list(collection.STATES)}


def collection_edges() -> list[dict]:
    out = []
    contexts = (
        None,
        {},
        {"obligations": [{"sender_id": "ha", "status": "confirmed"}]},
        {"obligations": []},
        {"advancer_acknowledged": True},
        {"delivery_method_chosen": True},
        {"advancer_acknowledged": True, "delivery_method_chosen": True},
        {"capability_exposed_at": EXPOSED},
        {"capability_exposed_at": EXPOSED, "all_affected_parties_consented": True},
        {"capability_exposed_at": None},
        {"obligations": [{"sender_id": "ha", "status": "waived"}, {"sender_id": "nam", "status": "confirmed"}]},
        {"obligations": [{"sender_id": "ha", "status": "outstanding"}, {"sender_id": "nam", "status": "waived"}]},
    )
    for state in collection.STATES + ("", "Accruing", "delivered"):
        for event in EVENTS:
            # Only a legal event reads its context; the rest refuse first.
            legal = event in collection._ALLOWED.get(state, {})
            for k, context in enumerate(contexts if legal else contexts[:2]):
                out.append(case("transition", f"transition/{state}/{event}/{k}", {"state": state, "event": event, "context": context}))
    for k, context in enumerate(contexts[1:]):
        out.append(case("unmet_freeze_requirements", f"freeze/{k}", {"context": context}))
        out.append(case("unmet_publish_gates", f"gates/{k}", {"context": context}))
    lists = (
        [],
        [{"sender_id": "ha", "status": "confirmed"}],
        [{"sender_id": "ha", "status": "over_confirmed"}, {"sender_id": "nam", "status": "confirmed"}],
        [{"sender_id": "ha", "status": "waived"}, {"sender_id": "nam", "status": "confirmed"}],
        [{"sender_id": "ha", "status": "disputed"}, {"sender_id": "ha", "status": "outstanding"}],
        [{"sender_id": "ha", "status": "outstanding"}, {"sender_id": "ha", "status": "waived"}],
        [{"sender_id": "ha", "status": "cancelled"}],
        [{"sender_id": "ha", "status": "confirmed"}, {"sender_id": "ha", "status": "waived"}, {"sender_id": "nam", "status": "disputed"}],
    )
    for k, obligations in enumerate(lists):
        out.append(case("terminal_state_for", f"terminal/{k}", {"obligations": obligations}))
        out.append(case("progress", f"progress/{k}", {"obligations": obligations}))
    base = "2026-09-15T09:30:12.345678+00:00"
    stamps = (
        base,
        "2026-09-01T09:30:12.345678+00:00",
        "2026-09-01T09:30:12.345677+00:00",
        "2026-09-01T09:30:12.345679+00:00",
        "2026-09-08T09:30:12.345678+00:00",
        "2026-09-08T09:30:12.345677+00:00",
        "2026-09-01T16:30:12.345677+07:00",
        # The instant above, written at UTC-12; as a literal it reads like a
        # long digit run to the guard, so it is built here.
        datetime(2026, 9, 1, 9, 30, 12, 345678, tzinfo=UTC).astimezone(timezone(timedelta(hours=-12))).isoformat(),
        "0001-01-01T00:00:00+00:00",
        "9999-12-31T23:59:59.999999+00:00",
        "0001-01-01T00:00:00+14:00",
    )
    for j, due in enumerate(stamps):
        for k, last in enumerate(stamps):
            out.append(case("is_stale", f"stale/0/{j}/{k}", {"now": base, "due_at": due, "last_meaningful_activity_at": last}))
    corners = (stamps[0], stamps[8], stamps[9], stamps[10])
    for n, now in enumerate((stamps[8], stamps[9]), 1):
        for j, due in enumerate(corners):
            for k, last in enumerate(corners):
                out.append(case("is_stale", f"stale/{n}/{j}/{k}", {"now": now, "due_at": due, "last_meaningful_activity_at": last}))
    for state in (None, "cancelled", "collecting", "", "Cancelled"):
        for exposed in (None, EXPOSED):
            batch = {"capability_exposed_at": exposed}
            if state is not None:
                batch["state"] = state
            out.append(case("counts_toward_collection_rate", f"rate/{state}/{exposed is not None}", {"batch": batch}))
    out.append(case("counts_toward_collection_rate", "rate/empty", {"batch": {}}))
    return out


def collection_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 6)
    transitions, terminal, stale = split_count(count, (2000, 1200, 600))
    out = []
    for i in range(transitions):
        state = rng.choice(collection.STATES + ("", "delivered"))
        event = rng.choice(EVENTS)
        out.append(case("transition", f"fuzz/{i}", {"state": state, "event": event, "context": context_of(rng)}))
    for i in range(max(1, terminal // 2)):
        obligations = obligations_of(rng, 6)
        out.append(case("terminal_state_for", f"fuzz/{i}", {"obligations": obligations}))
        out.append(case("progress", f"fuzz/{i}", {"obligations": obligations}))
    start = datetime(2026, 9, 15, 9, 30, 12, 345678, tzinfo=UTC)
    for i in range(stale):
        def stamp():
            offset = timezone(timedelta(minutes=rng.choice((0, 420, -720, 840, 330))))
            shift = timedelta(days=rng.choice((0, 7, 14, 21)), microseconds=rng.choice((0, 1, -1, rng.randint(-10**11, 10**11))))
            return (start - shift).astimezone(offset).isoformat()

        out.append(case("is_stale", f"fuzz/{i}", {"now": start.isoformat(), "due_at": stamp(), "last_meaningful_activity_at": stamp()}))
    return out


# ---------------------------------------------------------------------------
# capability
# ---------------------------------------------------------------------------


def scope_case(name: str, envelope: dict, obligations: list) -> dict:
    return case("capability_scope", name, {"envelope": envelope, "obligations": obligations})


def scoped(obligation_id, version="v1", sender="ha") -> dict:
    return {"obligation_id": obligation_id, "batch_version_id": version, "sender_id": sender}


def capability_constants() -> dict:
    return {"names": public_names(capability)}


def capability_edges() -> list[dict]:
    env = {"batch_version_id": "v1", "sender_id": "ha"}
    return [
        scope_case("test_scope_is_one_sender_and_one_immutable_batch_version", env, [scoped("o2"), scoped("o1")]),
        scope_case("crosses_sender", env, [scoped("o1", sender="someone-else")]),
        scope_case("crosses_version", env, [scoped("o1", version="v2")]),
        scope_case("version_checked_before_sender", env, [scoped("o1", version="v2", sender="x")]),
        scope_case("first_crossing_wins", env, [scoped("o1", sender="x"), scoped("o2", version="v2")]),
        scope_case("empty", env, []),
        scope_case("duplicate", env, [scoped("o1"), scoped("o1")]),
        scope_case("crossing_after_duplicate", env, [scoped("o1"), scoped("o1"), scoped("o2", sender="x")]),
        scope_case("ids_sort_by_code_point", env, [scoped(GRINNING_FACE), scoped(CJK_MIDDLE), scoped(E_ACUTE), scoped("z"), scoped("Z"), scoped("")]),
        scope_case("uuid_ids", {"batch_version_id": C, "sender_id": A}, [scoped(HIGH, C, A), scoped(LOW, C, A), scoped(F, C, A)]),
        scope_case("uuid_case_differs", {"batch_version_id": C, "sender_id": A}, [scoped(LOW, C, A.upper())]),
    ]


def capability_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 7)
    out = []
    for i in range(count):
        envelope = {"batch_version_id": rng.choice(("v1", "v2")), "sender_id": rng.choice(("ha", "nam"))}
        obligations = [
            scoped(rng.choice(("o1", "o2", "o3", "O1", E_ACUTE, "o10")), rng.choice(("v1", "v1", "v1", "v2")) if rng.random() < 0.9 else envelope["batch_version_id"], rng.choice(("ha", "ha", "nam")))
            for _ in range(rng.randrange(5))
        ]
        out.append(scope_case(f"fuzz/{i}", envelope, obligations))
    return out


# ---------------------------------------------------------------------------
# ledger status: confirmed_total, obligation_status, settlement_suggestions
# ---------------------------------------------------------------------------


def receipts_of(amounts) -> list[dict]:
    return [{"amount_vnd": amount} for amount in amounts]


def ledger_status_constants() -> dict:
    return {"names": public_names(ledger)}


def ledger_status_edges() -> list[dict]:
    out = []
    receipt_lists = ((), (1,), (50, 50), (100,), (101,), (0,), (-1,), (5, 0), (5, -1), (0, -1), (INT64_MAX, INT64_MAX), (INT64_MAX, 1), (INT64_MIN,), (99, 1, -5))
    for k, receipts in enumerate(receipt_lists):
        out.append(case("confirmed_total", f"total/{k}", {"receipt_confirmations": receipts_of(receipts)}))
        for j, declared in enumerate((100, 1, 0, -1, INT64_MAX, INT64_MIN)):
            out.append(case("obligation_status", f"status/{j}/{k}", {"declared_amount_vnd": declared, "receipt_confirmations": receipts_of(receipts)}))
    balances = (
        {},
        {"a": -70, "b": -30, "c": 60, "d": 40},
        {"a": -10, "b": 5},
        {"a": 0, "b": 0},
        {"a": -(2**64), "b": 2**64},
        {"a": None, "b": 0},
        {**{f"p{i:02d}": 1000 for i in range(20)}, **{f"q{i:02d}": -1000 for i in range(20)}},
        {"d": -5, "b": -5, "c": 5, "a": 5},
    )
    for k, balance in enumerate(balances):
        out.append(case("settlement_suggestions", f"suggestions/{k}", {"balances": pairs(balance)}, call={"balances": balance}))
    return out


def ledger_status_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 8)
    statuses, suggestions = split_count(count, (2500, 400))
    out = []
    for i in range(statuses):
        declared = rng.choice((rng.randint(1, 10**6), rng.randint(1, 9) * 1000, INT64_MAX, 0, -1)) if rng.random() < 0.97 else INT64_MIN
        receipts = []
        for _ in range(rng.choice((0, 1, 1, 2, 3, 5))):
            roll = rng.random()
            if roll < 0.4 and declared > 0:
                amount = rng.choice((declared, declared - 1, declared + 1 if declared < INT64_MAX else declared, max(1, declared // 2)))
            elif roll < 0.95:
                amount = rng.randint(1, 10**6)
            else:
                amount = rng.choice((0, -1, INT64_MAX, INT64_MIN))
            receipts.append(amount)
        if rng.random() < 0.5:
            out.append(case("obligation_status", f"fuzz/{i}", {"declared_amount_vnd": declared, "receipt_confirmations": receipts_of(receipts)}))
        else:
            out.append(case("confirmed_total", f"fuzz/{i}", {"receipt_confirmations": receipts_of(receipts)}))
    for i in range(suggestions):
        names = rng.sample(("a", "b", "c", "d", "e", "f", "g", "h", E_ACUTE, GRINNING_FACE), rng.randint(0, 8))
        amounts = [rng.randint(-50, 50) for _ in names[:-1]]
        if names:
            amounts.append(-sum(amounts) + (rng.choice((0, 0, 0, 1)) if rng.random() < 0.3 else 0))
        balance = dict(zip(names, amounts, strict=True))
        out.append(case("settlement_suggestions", f"fuzz/{i}", {"balances": pairs(balance)}, call={"balances": balance}))
    return out


# ---------------------------------------------------------------------------
# money steps: real ApiService methods over a recording stub repository
# ---------------------------------------------------------------------------

from pydantic import ValidationError  # noqa: E402

NOW = datetime(2026, 9, 15, 9, 30, 12, 345678, tzinfo=UTC)
# The service reads the clock through _now; the oracle pins it.
api_service._now = lambda: NOW


class Captured(Exception):
    """Raised by the stub at the repository write that ends the pure part."""

    def __init__(self, value):
        super().__init__("captured")
        self.value = value


class Stub:
    """A repository that answers from `facts` and records what was called."""

    def __init__(self, **facts):
        self.facts = facts
        self.calls: list = []

    def is_member(self, context_id, person_id):
        self.calls.append("is_member")
        return True

    def list_members(self, context_id):
        self.calls.append("list_members")
        return [SimpleNamespace(person_id=uuid.UUID(p), state=s) for p, s in self.facts["roster"]]

    def create_expense(self, context_id):
        self.calls.append("create_expense")
        return ExpenseIdentity(id=uuid.UUID(E), context_id=context_id)

    def get_expense(self, expense_id):
        self.calls.append("get_expense")
        return ExpenseIdentity(id=expense_id, context_id=uuid.UUID(CONTEXT))

    def save_expense_confirmation(self, **kwargs):
        self.calls.append("save_expense_confirmation")
        raise Captured(kwargs)

    def create_bill(self, **kwargs):
        self.calls.append("create_bill")
        raise Captured(kwargs)

    def get_bill(self, bill_id):
        self.calls.append("get_bill")
        return self.facts["bill"]

    def load_batch_inputs(self, context_id, expense_version_ids):
        if expense_version_ids is None:
            self.calls.append(["load_batch_inputs", None])
            if self.facts["all"] is None:
                raise AssertionError("no unselected inputs were given")
            return self.facts["all"]
        self.calls.append(["load_batch_inputs", [str(v) for v in expense_version_ids]])
        return self.facts["selected"]

    def save_frozen_batch(self, **kwargs):
        self.calls.append("save_frozen_batch")
        raise Captured(kwargs)

    def load_batch_for_publish(self, batch_id):
        self.calls.append("load_batch_for_publish")
        return self.facts["batch"]

    def save_published_batch(self, **kwargs):
        self.calls.append("save_published_batch")
        self.saved = kwargs
        return [SimpleNamespace(sender_id=link.sender_id) for link in kwargs["links"]]

    def get_receipt_target(self, obligation_id):
        self.calls.append("get_receipt_target")
        return self.facts["target"]

    def save_receipt_confirmation(self, **kwargs):
        self.calls.append("save_receipt_confirmation")
        return self.facts["record"]

    def person_finance_summary(self, person_id, movement_limit):
        self.calls.append("person_finance_summary")
        raise Captured(None)

    def group_recap(self, context_id, today):
        self.calls.append("group_recap")
        return self.facts["recap"]


def actor(actor_id: str = A, roles=("member",)) -> Actor:
    return Actor(id=uuid.UUID(actor_id), roles=frozenset(roles), context_ids=frozenset())


def problem_of(exc: ApiProblem) -> dict:
    return {"status": exc.status_code, "code": exc.code, "detail": exc.detail}


def new_service(stub: Stub) -> api_service.ApiService:
    return api_service.ApiService(stub, photo_storage=object())


def wire_allocation_shape(dumped: dict) -> dict:
    return {
        "allocations": pairs(dumped["allocations"]),
        "exact_shares": pairs(dumped["exact_shares"]),
        "rounding_gainers": dumped["rounding_gainers"],
        "warnings": dumped["warnings"],
    }


def expense_input(e: dict, recorded_by: str = A):
    return schemas.ExpenseInput.model_construct(
        context_id=uuid.UUID(CONTEXT),
        description=None,
        recorded_by_id=uuid.UUID(recorded_by),
        paid_by_id=uuid.UUID(e["advancer_id"]),
        verification_scope="totals_only",
        occurred_at=NOW,
        participants=[uuid.UUID(p) for p in e["participants"]],
        total_amount_vnd=e["total_vnd"],
        items=[
            schemas.ExpenseItemInput.model_construct(
                item_id=i["item_id"], label=None, amount_vnd=i["amount_vnd"], shared_by=[uuid.UUID(p) for p in i["shared_by"]]
            )
            for i in e["items"]
        ],
        surcharges=[schemas.ExpenseSurchargeInput.model_construct(**s) for s in e["surcharges"]],
        discounts=[schemas.ExpenseDiscountInput.model_construct(**d) for d in e["discounts"]],
    )


def run_propose(expense: dict) -> dict:
    stub = Stub()
    try:
        response = new_service(stub).propose_expense(expense_input(expense))
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "allocation": None}
    return {"calls": stub.calls, "problem": None, "allocation": wire_allocation_shape(response.allocation.model_dump(mode="json"))}


def run_confirm(actor_id, actor_roles, roster, expense, recorded_by_id, expected_allocations, acknowledge_as_advancer) -> dict:
    stub = Stub(roster=roster)
    request = schemas.ExpenseConfirmationRequest.model_construct(
        proposal=expense_input(expense, recorded_by_id),
        expected_allocations={uuid.UUID(k): v for k, v in expected_allocations},
        acknowledge_as_advancer=acknowledge_as_advancer,
    )
    try:
        new_service(stub).confirm_expense(uuid.UUID(F), request, actor(actor_id, actor_roles))
    except Captured as captured:
        saved = captured.value
        return {
            "calls": stub.calls,
            "problem": None,
            "saved": {
                "payer_acknowledgement": saved["payer_acknowledgement"],
                "allocation": allocation_shape(saved["allocator_expense"]),
                "rollups": saved["rollups"],
            },
        }
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "saved": None}
    raise AssertionError("confirm_expense returned without saving")


def run_create_bill(items_total_vnd, lines, roster) -> dict:
    stub = Stub(roster=roster)
    request = schemas.BillCreateRequest.model_construct(
        context_id=uuid.UUID(CONTEXT),
        printed_total_vnd=None,
        items_total_vnd=items_total_vnd,
        confidence=90,
        needs_review=False,
        items=[
            schemas.BillItemCreateRequest.model_construct(
                item_key=f"k{k}",
                name="x",
                quantity=1,
                unit_price_vnd=None,
                line_total_vnd=line["line_total_vnd"],
                suggested_participant_ids=[uuid.UUID(p) for p in line["suggested_participant_ids"]],
            )
            for k, line in enumerate(lines)
        ],
        surcharges=[],
        discounts=[],
    )
    try:
        new_service(stub).create_bill(request, actor())
    except Captured:
        return {"calls": stub.calls, "problem": None}
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc)}
    raise AssertionError("create_bill returned without writing")


def bill_record(bill_spec: dict) -> BillRecord:
    return BillRecord(
        id=uuid.UUID(D),
        context_id=uuid.UUID(CONTEXT),
        printed_total_vnd=bill_spec["printed_total_vnd"],
        items_total_vnd=0,
        confidence=90,
        needs_review=False,
        created_by_id=uuid.UUID(A),
        created_at=NOW,
        items=[
            BillItemRecord(
                item_key=i["item_key"],
                name="x",
                quantity=1,
                unit_price_vnd=None,
                line_total_vnd=i["line_total_vnd"],
                position=k,
                shares=[BillShareRecord(participant_id=uuid.UUID(p), source=s, decided_by_id=None, decided_at=None) for p, s in i["shares"]],
            )
            for k, i in enumerate(bill_spec["items"])
        ],
        surcharges=[BillSurchargeRecord(**s) for s in bill_spec["surcharges"]],
        discounts=[BillDiscountRecord(**d) for d in bill_spec["discounts"]],
    )


def run_bill_assignment(items) -> dict:
    spec = {
        "printed_total_vnd": None,
        "items": [{"item_key": i["item_key"], "line_total_vnd": 1, "shares": [[A, s] for s in i["sources"]]} for i in items],
        "surcharges": [],
        "discounts": [],
    }
    response = api_service._wire_bill(bill_record(spec))
    return {"assignment_state": response.assignment_state, "suggested_item_keys": response.suggested_item_keys}


def run_split(bill_spec, for_ledger, paid_by_id, roster) -> dict:
    stub = Stub(bill=bill_record(bill_spec), roster=roster)
    request = schemas.BillSplitRequest.model_construct(
        for_ledger=for_ledger, paid_by_id=None if paid_by_id is None else uuid.UUID(paid_by_id)
    )
    outcome = {"calls": stub.calls, "problem": None, "split": None, "response_error": False}
    try:
        response = new_service(stub).split_bill(uuid.UUID(D), request, actor())
    except ApiProblem as exc:
        outcome["problem"] = problem_of(exc)
        return outcome
    except ValidationError:
        outcome["response_error"] = True
        return outcome
    dumped = response.model_dump(mode="json")
    outcome["split"] = {
        "allocation": wire_allocation_shape(dumped["allocation"]),
        "assignment_state": dumped["assignment_state"],
        "suggested_item_keys": dumped["suggested_item_keys"],
        "total_amount_vnd": dumped["total_amount_vnd"],
        "participant_ids": dumped["participant_ids"],
        "excluded_member_ids": dumped["excluded_member_ids"],
    }
    return outcome


def batch_inputs(spec: dict) -> BatchInputs:
    return BatchInputs(
        expenses=tuple(
            ConfirmedExpense(
                version_id=uuid.UUID(e["version_id"]),
                context_id=uuid.UUID(CONTEXT),
                paid_by_id=uuid.UUID(e["paid_by_id"]),
                payer_acknowledgement="acknowledged",
                allocations=tuple(AllocationRow(id=uuid.UUID(r), participant_id=uuid.UUID(p), amount_vnd=a) for r, p, a in e["allocations"]),
            )
            for e in spec["expenses"]
        ),
        unavailable_version_ids=tuple(uuid.UUID(v) for v in spec["unavailable_version_ids"]),
    )


def run_create_batch(due_at, expense_version_ids, all_inputs, selected_inputs) -> dict:
    stub = Stub(all=None if all_inputs is None else batch_inputs(all_inputs), selected=batch_inputs(selected_inputs))
    request = schemas.BatchCreateRequest.model_construct(
        context_id=uuid.UUID(CONTEXT),
        expense_version_ids=None if expense_version_ids is None else [uuid.UUID(v) for v in expense_version_ids],
        due_at=iso(due_at),
    )
    try:
        new_service(stub).create_batch(request, actor())
    except Captured as captured:
        drafts = [
            {
                "sender_id": str(d.sender_id),
                "recipient_id": str(d.recipient_id),
                "amount_vnd": d.amount_vnd,
                "source_expense_version_ids": [str(v) for v in d.source_expense_version_ids],
                "sources": [[str(r.id), str(r.participant_id), r.amount_vnd] for r in d.sources],
            }
            for d in captured.value["obligations"]
        ]
        return {"calls": stub.calls, "problem": None, "drafts": drafts}
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "drafts": None}
    raise AssertionError("create_batch returned without saving")


def run_publish(batch, delivery_method, expires_at) -> dict:
    record = BatchForPublish(
        id=uuid.UUID(D),
        version_id=uuid.UUID(batch["version_id"]),
        owner_id=uuid.UUID(A),
        status=batch["status"],
        context_id=uuid.UUID(CONTEXT),
        advancer_acknowledged=batch["advancer_acknowledged"],
        obligations=tuple(
            PublishObligation(id=uuid.UUID(o), batch_version_id=uuid.UUID(v), sender_id=uuid.UUID(s), recipient_id=uuid.UUID(r), amount_vnd=a)
            for o, v, s, r, a in batch["obligations"]
        ),
    )
    stub = Stub(batch=record)
    request = schemas.BatchPublishRequest.model_construct(delivery_method=delivery_method, guest_link_expires_at=iso(expires_at))
    try:
        response = new_service(stub).publish_batch(uuid.UUID(D), request, actor())
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "status": None, "links": None}
    links = [
        {
            "sender_id": str(link.sender_id),
            "path_is_a_token": link.path.startswith("/g/") and len(link.path) == 46,
            "obligations": [[str(o.obligation_id), o.amount_vnd] for o in link.obligations],
        }
        for link in response.guest_links
    ]
    saved = [str(link.sender_id) for link in stub.saved["links"]]
    if saved != [link["sender_id"] for link in links]:
        raise AssertionError("saved links and published links disagree")
    return {"calls": stub.calls, "problem": None, "status": stub.saved["status"], "links": links}


def run_receipt(declared_amount_vnd, receipt_amounts_vnd) -> dict:
    stub = Stub(
        target=ReceiptTarget(obligation_id=uuid.UUID(D), recipient_id=uuid.UUID(A), amount_vnd=declared_amount_vnd),
        record=ReceiptRecord(id=uuid.UUID(E), obligation_id=uuid.UUID(D), amount_vnd=1000, receipt_amounts_vnd=tuple(receipt_amounts_vnd)),
    )
    request = schemas.ReceiptConfirmationRequest.model_construct(amount_vnd=1000, idempotency_key=uuid.UUID(F), payment_report_id=None)
    try:
        response = new_service(stub).confirm_receipt(uuid.UUID(D), request, actor(A, ("recipient",)))
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc), "status": None}
    return {"calls": stub.calls, "problem": None, "status": response.obligation_status}


def run_finance(actor_id, person_id) -> dict:
    stub = Stub()
    try:
        new_service(stub).person_finance_summary(uuid.UUID(person_id), actor(actor_id))
    except Captured:
        return {"calls": stub.calls, "problem": None}
    except ApiProblem as exc:
        return {"calls": stub.calls, "problem": problem_of(exc)}
    raise AssertionError("person_finance_summary returned without reading")


def run_group_budget(outings, roster, candidate_per_person_vnd) -> dict:
    recap = [
        SimpleNamespace(
            outing=SimpleNamespace(id=uuid.UUID(o["outing_id"]), title=o["title"], headcount=o["headcount"], budget_per_person_vnd=o["budget_per_person_vnd"]),
            in_progress=o["in_progress"],
            split_total_vnd=o["split_total_vnd"],
        )
        for o in outings
    ]
    stub = Stub(recap=recap, roster=roster)
    response = new_service(stub).group_budget(uuid.UUID(CONTEXT), actor(), candidate_per_person_vnd=candidate_per_person_vnd)
    return {"calls": stub.calls, "response": response.model_dump(mode="json")}


def uuid_expense(rng: random.Random) -> dict:
    """A fuzz expense with uuid participants, the only ids a request carries.

    At most five people and three items, so that one case stays within the
    guard's line limit.
    """
    e = fuzz_valid_expense(rng)
    while len(e["participants"]) > 5 or len(e["items"]) > 3:
        e = fuzz_valid_expense(rng)
    mapping: dict[str, str] = {}

    def to_uuid(name: str) -> str:
        if name not in mapping:
            mapping[name] = PEOPLE[len(mapping)] if len(mapping) < len(PEOPLE) else random_uuid(rng)
        return mapping[name]

    e["participants"] = [to_uuid(p) for p in e["participants"]]
    for element in e["items"]:
        element["shared_by"] = [to_uuid(p) for p in element["shared_by"]]
    advancer = e["advancer_id"]
    e["advancer_id"] = mapping[advancer] if advancer in mapping else rng.choice((F, HIGH, random_uuid(rng)))
    if rng.random() < 0.45:
        roll = rng.randrange(9)
        if roll == 0:
            e["total_vnd"] += rng.choice((1, -1))
        elif roll == 1:
            target, key = rng.choice(amount_slots(e))
            target[key] = rng.choice(BAD_AMOUNTS)
        elif roll == 2:
            e["participants"].append(rng.choice(e["participants"]))
        elif roll == 3 and e["items"]:
            rng.choice(e["items"])["shared_by"] = rng.choice(([], [random_uuid(rng)], [e["participants"][0]] * 2))
        elif roll == 4 and e["surcharges"]:
            rng.choice(e["surcharges"])["mode"] = rng.choice(("Even", ""))
        elif roll == 5 and e["discounts"]:
            rng.choice(e["discounts"])["amount_vnd"] += rng.choice((1, 10**6))
        elif roll == 6:
            e["participants"] = []
        elif roll == 7 and e["items"]:
            rng.choice(e["items"])["item_id"] = rng.choice(("", " i", "total"))
        else:
            rng.shuffle(e["participants"])
            rng.shuffle(e["items"])
    return e


def roster_of(rng: random.Random, ids) -> list[list[str]]:
    roster = [[person, "active"] for person in dict.fromkeys(ids)]
    roll = rng.random()
    if roll < 0.15 and roster:
        roster.pop(rng.randrange(len(roster)))
    elif roll < 0.25 and roster:
        roster[rng.randrange(len(roster))][1] = rng.choice(("invited", "left", "ACTIVE", ""))
    elif roll < 0.3:
        roster.append([random_uuid(rng), rng.choice(("active", "invited"))])
    elif roll < 0.33 and roster:
        roster.append([roster[0][0], "invited"])
    rng.shuffle(roster)
    return roster


def propose_case(name: str, e: dict) -> dict:
    return case("propose_expense", name, {"expense": e})


def expected_for(rng: random.Random, e: dict) -> list:
    try:
        result = allocator.allocate(api_service._allocator_input(expense_input(e)))
        expected = pairs(result["allocations"])
    except Exception:  # an allocation that fails is compared with anything
        expected = [[p, rng.randint(0, 9)] for p in dict.fromkeys(e["participants"])]
    roll = rng.random()
    if roll < 0.12 and expected:
        expected[rng.randrange(len(expected))][1] += rng.choice((1, -1, INT64_MAX + 1))
    elif roll < 0.2 and expected:
        expected.pop()
    elif roll < 0.26:
        expected.append([random_uuid(rng), 0])
    elif roll < 0.5:
        rng.shuffle(expected)
    return expected


def confirm_case(name, e, roster, recorded_by, expected, acknowledge, actor_id=A, roles=("member",)) -> dict:
    args = {
        "actor_id": actor_id,
        "actor_roles": list(roles),
        "roster": roster,
        "expense": e,
        "recorded_by_id": recorded_by,
        "expected_allocations": expected,
        "acknowledge_as_advancer": acknowledge,
    }
    return case("confirm_expense", name, args)


def create_bill_case(name, items_total, lines, roster) -> dict:
    return case("create_bill", name, {"items_total_vnd": items_total, "lines": lines, "roster": roster})


def split_case(name, bill_spec, for_ledger, paid_by, roster) -> dict:
    return case("split_bill", name, {"bill": bill_spec, "for_ledger": for_ledger, "paid_by_id": paid_by, "roster": roster})


def batch_case(name, due_at, versions, all_inputs, selected) -> dict:
    return case("create_batch", name, {"due_at": due_at, "expense_version_ids": versions, "all": all_inputs, "selected": selected})


def publish_case(name, batch, delivery_method, expires_at) -> dict:
    return case("publish_batch", name, {"batch": batch, "delivery_method": delivery_method, "expires_at": expires_at})


def active(*ids) -> list[list[str]]:
    return [[person, "active"] for person in ids]


LATER = (NOW + timedelta(days=3)).isoformat()


def money_steps_constants() -> dict:
    return {"names": []}


def money_steps_edges() -> list[dict]:
    out = []
    dinner = expense_dict([A, B, C], 100000, advancer=B)
    out += [
        propose_case("even_three", dinner),
        propose_case("refused_negative", expense_dict([A, B], -1, advancer=A)),
        propose_case("refused_past_int64", expense_dict([A, B], 2**64, advancer=A)),
        propose_case("no_participants", expense_dict([], 0, advancer=A)),
        propose_case("uuid_byte_order", expense_dict([HIGH, LOW, F, A], 7, advancer=HIGH)),
    ]
    right = [[A, 33333], [B, 33334], [C, 33333]]
    out += [
        confirm_case("happy", dinner, active(A, B, C), A, right, False),
        confirm_case("happy_reordered", dinner, active(C, B, A), A, right[::-1], False),
        confirm_case("acknowledged", dinner, active(A, B, C), B, right, True, actor_id=B, roles=("member", "advancer")),
        confirm_case("acknowledge_without_role", dinner, active(A, B, C), B, right, True, actor_id=B),
        confirm_case("acknowledge_not_the_payer", dinner, active(A, B, C), A, right, True, actor_id=A, roles=("member", "advancer")),
        confirm_case("stranger_participant", dinner, active(A, B), A, right, False),
        confirm_case("stranger_payer_and_recorder", expense_dict([A, B], 10, advancer=HIGH), active(A, B), LOW, [[A, 5], [B, 5]], False),
        confirm_case("invited_is_not_a_member", dinner, [[A, "active"], [B, "active"], [C, "invited"]], A, right, False),
        confirm_case("strangers_before_acknowledgement", dinner, active(A), A, right, True, actor_id=A),
        confirm_case("proposal_changed_value", dinner, active(A, B, C), A, [[A, 33334], [B, 33333], [C, 33333]], False),
        confirm_case("proposal_changed_missing", dinner, active(A, B, C), A, right[:2], False),
        confirm_case("proposal_changed_past_int64", dinner, active(A, B, C), A, [[A, 2**64], [B, 33334], [C, 33333]], False),
        confirm_case("allocation_refused_before_compare", expense_dict([A, B], -5, advancer=A), active(A, B), A, [], False),
        confirm_case(
            "rollups",
            expense_dict([A, B], 160000, [item("i", 100000, [A]), item("j", 50000, [A, B])], [surcharge("s", "VAT", 12000, "proportional"), surcharge("t", LONG_S + "hipping", 5000, "even"), surcharge("u", "service", 3000, "even")], [discount("d", 10000, "global_proportional")], A),
            active(A, B),
            A,
            [[A, 131667], [B, 28333]],
            False,
        ),
    ]
    out += [
        create_bill_case("ok", 300, [{"line_total_vnd": 100, "suggested_participant_ids": [A]}, {"line_total_vnd": 200, "suggested_participant_ids": []}], active(A)),
        create_bill_case("mismatch", 299, [{"line_total_vnd": 100, "suggested_participant_ids": [A]}, {"line_total_vnd": 200, "suggested_participant_ids": []}], active(A)),
        create_bill_case("stranger_beats_mismatch", 1, [{"line_total_vnd": 100, "suggested_participant_ids": [HIGH, LOW, HIGH]}], active(A)),
        create_bill_case("no_lines", 0, [], []),
        create_bill_case("sum_past_int64", INT64_MAX, [{"line_total_vnd": INT64_MAX, "suggested_participant_ids": []}] * 2, []),
    ]
    for k, sources in enumerate(([], [["confirmed"]], [["ai_suggested"]], [["confirmed"], []], [["confirmed", "ai_suggested"]], [["confirmed", "confirmed"], ["confirmed"]])):
        items = [{"item_key": key, "sources": item_sources} for key, item_sources in zip(("b", "a", E_ACUTE), sources, strict=False)]
        out.append(case("bill_assignment", f"assignment/{k}", {"items": items}))
    out.append(case("bill_assignment", "assignment/keys_in_byte_order", {"items": [{"item_key": key, "sources": ["ai_suggested"]} for key in ("b", "Z", GRINNING_FACE, CJK_MIDDLE, "a")]}))

    pho = {"item_key": "pho", "line_total_vnd": 90000, "shares": [[A, "confirmed"], [B, "confirmed"]]}
    bill_spec = {"printed_total_vnd": 90000, "items": [pho], "surcharges": [], "discounts": []}
    out += [
        split_case("confirmed", bill_spec, True, A, active(A, B, C)),
        split_case("roster_order_and_excluded", bill_spec, False, None, [[HIGH, "invited"], [C, "active"], [B, "active"], [A, "active"], [LOW, "left"]]),
        split_case("suggested_for_ledger", dict(bill_spec, items=[dict(pho, shares=[[A, "ai_suggested"]])]), True, A, active(A, B)),
        split_case("suggested_preview", dict(bill_spec, items=[dict(pho, shares=[[A, "ai_suggested"]])]), False, A, active(A, B)),
        split_case("no_items", dict(bill_spec, items=[]), False, A, active(A)),
        split_case("unknown_source", dict(bill_spec, items=[dict(pho, shares=[[A, "AI"]])]), False, A, active(A)),
        split_case("share_of_a_stranger", dict(bill_spec, items=[dict(pho, shares=[[HIGH, "confirmed"]])]), False, A, active(A)),
        split_case("empty_roster", bill_spec, False, A, []),
        split_case("printed_total_disagrees", dict(bill_spec, printed_total_vnd=100000), False, A, active(A, B)),
        split_case("both_active_and_invited", bill_spec, False, A, [[A, "active"], [B, "active"], [B, "invited"]]),
        split_case(
            "surcharge_and_discount",
            {
                "printed_total_vnd": None,
                "items": [pho, {"item_key": "bia", "line_total_vnd": 30000, "shares": [[C, "confirmed"]]}],
                "surcharges": [{"surcharge_key": "vat", "kind": "vat", "amount_vnd": 9600, "mode": "proportional"}],
                "discounts": [{"discount_key": "km", "amount_vnd": 10000, "scope": "item", "target_item_key": "pho"}],
            },
            True,
            C,
            active(A, B, C),
        ),
    ]

    v1, v2, v3 = D, E, F
    row = lambda k: str(uuid.UUID(int=(k + 1) * 0x9E3779B97F4A7C15F39CC0605CEDC835 % 2**128))  # noqa: E731
    one = {"expenses": [{"version_id": v1, "paid_by_id": A, "allocations": [[row(0), A, 50], [row(1), B, 30], [row(2), C, 20]]}], "unavailable_version_ids": []}
    out += [
        batch_case("all_confirmed", LATER, None, one, one),
        batch_case("named", LATER, [v1], None, one),
        batch_case("named_unavailable", LATER, [v1, v2], None, dict(one, unavailable_version_ids=[v2])),
        batch_case("unavailable_ignored_when_not_named", LATER, None, one, dict(one, unavailable_version_ids=[v2])),
        batch_case("nothing_to_batch", LATER, None, {"expenses": [], "unavailable_version_ids": []}, {"expenses": [], "unavailable_version_ids": []}),
        batch_case("named_empty_list", LATER, [], None, {"expenses": [], "unavailable_version_ids": []}),
        batch_case("due_now", NOW.isoformat(), None, one, one),
        batch_case("due_in_the_past_other_offset", (NOW - timedelta(microseconds=1)).astimezone(timezone(timedelta(hours=7))).isoformat(), None, one, one),
        batch_case("due_a_microsecond_later", (NOW + timedelta(microseconds=1)).isoformat(), None, one, one),
        batch_case("payer_owes_nothing", LATER, None, one, {"expenses": [{"version_id": v1, "paid_by_id": A, "allocations": [[row(0), A, 100]]}], "unavailable_version_ids": []}),
        batch_case("negative_allocation", LATER, None, one, {"expenses": [{"version_id": v1, "paid_by_id": A, "allocations": [[row(0), B, -1]]}], "unavailable_version_ids": []}),
        batch_case(
            "two_expenses_merge_and_sources",
            LATER,
            None,
            one,
            {
                "expenses": [
                    {"version_id": v2, "paid_by_id": A, "allocations": [[row(3), B, 10], [row(4), B, 0], [row(5), C, INT64_MAX]]},
                    {"version_id": v1, "paid_by_id": A, "allocations": [[row(6), C, INT64_MAX], [row(7), B, 5]]},
                    {"version_id": v3, "paid_by_id": B, "allocations": [[row(8), A, 7]]},
                ],
                "unavailable_version_ids": [],
            },
        ),
        batch_case(
            "repeated_participant_row",
            LATER,
            None,
            one,
            {"expenses": [{"version_id": v1, "paid_by_id": A, "allocations": [[row(0), B, 5], [row(1), B, 0], [row(2), C, 0], [row(3), C, 9]]}], "unavailable_version_ids": []},
        ),
    ]

    version = D
    obligations = [[row(10), version, B, A, 30], [row(11), version, C, A, 20], [row(12), version, B, C, 5]]
    batch = {"version_id": version, "status": "frozen", "advancer_acknowledged": True, "obligations": obligations}
    out += [
        publish_case("happy", batch, "personal_link", LATER),
        publish_case("senders_in_uuid_order", dict(batch, obligations=[[row(13), version, HIGH, A, 1], [row(14), version, LOW, A, 2], [row(15), version, HIGH, B, 3]]), "personal_link", LATER),
        publish_case("expired_link", batch, "personal_link", NOW.isoformat()),
        publish_case("expired_beats_gates", dict(batch, advancer_acknowledged=False), "", NOW.isoformat()),
        publish_case("not_acknowledged", dict(batch, advancer_acknowledged=False), "", LATER),
        publish_case("no_delivery_method", batch, "", LATER),
        publish_case("already_published", dict(batch, status="published"), "personal_link", LATER),
        publish_case("unknown_status", dict(batch, status="delivered"), "personal_link", LATER),
        publish_case("crosses_version", dict(batch, obligations=obligations + [[row(16), E, C, B, 4]]), "personal_link", LATER),
        publish_case("duplicate_obligation", dict(batch, obligations=obligations + [obligations[0]]), "personal_link", LATER),
        publish_case("no_obligations", dict(batch, obligations=[]), "personal_link", LATER),
    ]
    for k, (declared, receipts) in enumerate(((100, ()), (100, (40,)), (100, (40, 60)), (100, (101,)), (0, (1,)), (-1, ()), (100, (0,)), (100, (-5,)), (INT64_MAX, (INT64_MAX, INT64_MAX)))):
        out.append(case("confirm_receipt", f"receipt/{k}", {"declared_amount_vnd": declared, "receipt_amounts_vnd": list(receipts)}))
    for k, (me, person) in enumerate(((A, A), (A, B), (HIGH, LOW))):
        out.append(case("person_finance", f"finance/{k}", {"actor_id": me, "person_id": person}))
    trips = [
        dict(outing(), outing_id=A),
        dict(outing(title=" Biển ", headcount=3, split_total=1_000_001, in_progress=True), outing_id=B),
    ]
    out += [
        case("group_budget", "budget/happy", {"outings": trips, "roster": [[A, "active"], [B, "invited"], [C, "active"]], "candidate_per_person_vnd": 450_000}),
        case("group_budget", "budget/invalid", {"outings": [dict(outing(title=" "), outing_id=A)], "roster": [], "candidate_per_person_vnd": None}),
        case("group_budget", "budget/past_int64", {"outings": [dict(outing(split_total=2**66), outing_id=A)], "roster": active(A), "candidate_per_person_vnd": 0}),
    ]
    return out


def batch_fuzz_case(rng: random.Random, i: int) -> dict:
    """One create_batch fuzz case; the caller redraws a case too long to commit."""
    people = rng.sample(PEOPLE, rng.randint(1, 4))
    versions = [random_uuid(rng) for _ in range(rng.randint(0, 3))]
    expenses = []
    for version in versions:
        payer = rng.choice(people)
        rows = []
        for person in rng.sample(people, rng.randint(0, len(people))) + ([rng.choice(people)] if rng.random() < 0.1 else []):
            amount = rng.choice((rng.randint(1, 9) * 1000, 0, rng.randint(1, 500_000)))
            if rng.random() < 0.03:
                amount = rng.choice((-1, INT64_MAX))
            rows.append([random_uuid(rng), person, amount])
        expenses.append({"version_id": version, "paid_by_id": payer, "allocations": rows})
    inputs = {"expenses": expenses, "unavailable_version_ids": [random_uuid(rng)] if rng.random() < 0.15 else []}
    named = rng.random() < 0.5
    due = rng.choice((LATER, LATER, LATER, NOW.isoformat(), (NOW - timedelta(days=1)).isoformat()))
    return batch_case(f"fuzz/{i}", due, versions if named else None, None if named else inputs, inputs)


def money_steps_fuzz(seed: int, count: int) -> list[dict]:
    rng = random.Random(seed * 1000 + 9)
    proposals, confirmations, bills, assignments, splits, batches, publications, receipts, budgets = split_count(count, (800, 1200, 600, 600, 1500, 1200, 800, 600, 600))
    out = []
    for i in range(proposals):
        out.append(propose_case(f"fuzz/{i}", uuid_expense(rng)))
    for i in range(confirmations):
        e = uuid_expense(rng)
        recorded_by = rng.choice(e["participants"] + [HIGH]) if e["participants"] else A
        actor_id = rng.choice((e["advancer_id"], A, B))
        roles = rng.choice((("member",), ("member", "advancer"), ("member", "advancer", "recipient")))
        roster = roster_of(rng, e["participants"] + [e["advancer_id"], recorded_by])
        out.append(confirm_case(f"fuzz/{i}", e, roster, recorded_by, expected_for(rng, e), rng.random() < 0.4, actor_id, roles))
    for i in range(bills):
        lines = [
            {"line_total_vnd": rng.choice((fuzz_amount(rng), INT64_MAX)), "suggested_participant_ids": rng.sample(PEOPLE, rng.choice((0, 0, 1, 2)))}
            for _ in range(rng.choice((0, 1, 2, 3, 5)))
        ]
        total = sum(line["line_total_vnd"] for line in lines)
        items_total = min(INT64_MAX, max(0, total + rng.choice((0, 0, 0, 1, -1)))) if total <= INT64_MAX else rng.choice((0, INT64_MAX))
        roster = roster_of(rng, rng.sample(PEOPLE, rng.randint(0, 8)))
        out.append(create_bill_case(f"fuzz/{i}", items_total, lines, roster))
    for i in range(assignments):
        items = [
            {"item_key": key, "sources": [rng.choice(("confirmed", "confirmed", "ai_suggested")) for _ in range(rng.choice((0, 1, 1, 2, 3)))]}
            for key in rng.sample(ENTITY_POOL, rng.choice((0, 1, 2, 3, 5)))
        ]
        out.append(case("bill_assignment", f"fuzz/{i}", {"items": items}))
    for i in range(splits):
        members = rng.sample(PEOPLE, rng.randint(0, 6))
        sources = ("confirmed", "confirmed", "confirmed", "ai_suggested", "AI")
        items = [
            {
                "item_key": key,
                "line_total_vnd": rng.choice((fuzz_amount(rng), 0, -1)) if rng.random() < 0.05 else fuzz_amount(rng),
                "shares": [[rng.choice(members + [HIGH]) if members else HIGH, rng.choice(sources)] for _ in range(rng.choice((0, 1, 1, 2, 3)) if rng.random() < 0.97 else 0)],
            }
            for key in rng.sample(ENTITY_POOL, rng.choice((0, 1, 1, 2, 3)))
        ]
        surcharges = [{"surcharge_key": f"s{k}", "kind": rng.choice(KINDS), "amount_vnd": fuzz_amount(rng), "mode": rng.choice(("even", "proportional"))} for k in range(rng.choice((0, 0, 1)))]
        discounts = [{"discount_key": "g", "amount_vnd": rng.randint(1, 9), "scope": "global_proportional", "target_item_key": None}] if rng.random() < 0.2 else []
        listed = sum(x["line_total_vnd"] for x in items) + sum(s["amount_vnd"] for s in surcharges) - sum(d["amount_vnd"] for d in discounts)
        printed = None if rng.random() < 0.5 else rng.choice((listed, listed, listed + 1, 0))
        spec = {"printed_total_vnd": printed, "items": items, "surcharges": surcharges, "discounts": discounts}
        out.append(split_case(f"fuzz/{i}", spec, rng.random() < 0.5, rng.choice(members + [None, HIGH]), roster_of(rng, members)))
    for i in range(batches):
        while True:
            built = batch_fuzz_case(rng, i)
            if len(json.dumps(built, ensure_ascii=True, separators=(",", ":"))) <= 3900:
                break
        out.append(built)
    for i in range(publications):
        version = random_uuid(rng)
        senders = rng.sample(PEOPLE, rng.randint(1, 4))
        obligations = [
            [random_uuid(rng), version if rng.random() < 0.97 else random_uuid(rng), rng.choice(senders), rng.choice(PEOPLE), rng.randint(1, 10**6)]
            for _ in range(rng.randint(0, 5))
        ]
        if obligations and rng.random() < 0.03:
            obligations.append(list(obligations[0]))
        batch = {
            "version_id": version,
            "status": rng.choice(("frozen", "frozen", "frozen", "accruing", "published", "")),
            "advancer_acknowledged": rng.random() < 0.9,
            "obligations": obligations,
        }
        out.append(publish_case(f"fuzz/{i}", batch, rng.choice(("personal_link", "personal_link", "")), rng.choice((LATER, LATER, NOW.isoformat()))))
    for i in range(receipts):
        declared = rng.choice((rng.randint(1, 10**6), INT64_MAX, 0, -1))
        receipts = [rng.choice((rng.randint(1, 10**6), declared if declared > 0 else 1, INT64_MAX, 0)) for _ in range(rng.choice((0, 1, 2, 3)))]
        out.append(case("confirm_receipt", f"fuzz/{i}", {"declared_amount_vnd": declared, "receipt_amounts_vnd": receipts}))
    for i in range(budgets):
        outings = [
            dict(
                outing(
                    title=rng.choice(("Đà Lạt", " x ", " ")) if rng.random() < 0.1 else "Đà Lạt",
                    headcount=rng.choice((0, 1, 3, 4, 12)),
                    budget_vnd=rng.choice((0, 300_000, rng.randint(0, 10**7))),
                    split_total=rng.choice((0, rng.randint(0, 10**7), rng.randint(0, 2**66))),
                    in_progress=rng.random() < 0.4,
                ),
                outing_id=random_uuid(rng),
            )
            for _ in range(rng.choice((0, 1, 2, 4)))
        ]
        roster = [[random_uuid(rng), rng.choice(("active", "active", "invited", "left"))] for _ in range(rng.randint(0, 6))]
        candidate = None if rng.random() < 0.3 else rng.choice((0, rng.randint(0, 10**6), INT64_MAX))
        out.append(case("group_budget", f"fuzz/{i}", {"outings": outings, "roster": roster, "candidate_per_person_vnd": candidate}))
    return out




FUNCTIONS = {
    "allocate": allocator.allocate,
    "apportion": allocator._apportion,
    "allocator_input_from_bill": bill.allocator_input_from_bill,
    "build_group_budget": lambda outings, active_member_count, candidate_per_person_vnd: budget.build_group_budget(
        outings, active_member_count=active_member_count, candidate_per_person_vnd=candidate_per_person_vnd
    ),
    "component_rollups": expense.component_rollups,
    "transition": collection.transition,
    "unmet_freeze_requirements": collection.unmet_freeze_requirements,
    "unmet_publish_gates": collection.unmet_publish_gates,
    "terminal_state_for": collection.terminal_state_for,
    "progress": collection.progress,
    "is_stale": lambda now, due_at, last_meaningful_activity_at: collection.is_stale(iso(now), iso(due_at), iso(last_meaningful_activity_at)),
    "counts_toward_collection_rate": collection.counts_toward_collection_rate,
    "capability_scope": capability.capability_scope,
    "confirmed_total": ledger.confirmed_total,
    "obligation_status": ledger.obligation_status,
    "settlement_suggestions": ledger.settlement_suggestions,
    "propose_expense": run_propose,
    "confirm_expense": run_confirm,
    "create_bill": run_create_bill,
    "bill_assignment": run_bill_assignment,
    "split_bill": lambda **kw: run_split(kw["bill"], kw["for_ledger"], kw["paid_by_id"], kw["roster"]),
    "create_batch": lambda **kw: run_create_batch(kw["due_at"], kw["expense_version_ids"], kw["all"], kw["selected"]),
    "publish_batch": run_publish,
    "confirm_receipt": run_receipt,
    "person_finance": run_finance,
    "group_budget": run_group_budget,
}

#: module -> (Go package path, target, constants, edges, fuzz,
#:            cases in the committed sample, cases a live oracle run draws)
MODULES = {
    "allocator": ("internal/domain/allocator", allocator, allocator_constants, allocator_edges, allocator_fuzz, 600, 21000),
    "apportion": ("internal/domain/allocator", allocator, apportion_constants, apportion_edges, apportion_fuzz, 200, 3000),
    "billdraft": ("internal/domain/billdraft", bill, billdraft_constants, billdraft_edges, billdraft_fuzz, 200, 2500),
    "budget": ("internal/domain/budget", budget, budget_constants, budget_edges, budget_fuzz, 150, 2500),
    "expense": ("internal/domain/expense", expense, expense_constants, expense_edges, expense_fuzz, 150, 1500),
    "collection": ("internal/domain/collection", collection, collection_constants, collection_edges, collection_fuzz, 300, 3800),
    "capability": ("internal/domain/capability", capability, capability_constants, capability_edges, capability_fuzz, 150, 1500),
    "ledger_status": ("internal/domain/ledger", ledger, ledger_status_constants, ledger_status_edges, ledger_status_fuzz, 300, 2900),
    "money_steps": ("internal/domain/moneysteps", api_service, money_steps_constants, money_steps_edges, money_steps_fuzz, 300, 7900),
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
        "generator": "scripts/render_domain_w4_goldens.py",
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
            lines.append(f" {json.dumps(key)}: {json.dumps(value, ensure_ascii=True, separators=(',', ':'))},")
    cases = [json.dumps(item, ensure_ascii=True, separators=(",", ":")) for item in document["cases"]]
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
    if len(argv) == 4 and argv[0] == "--live" and argv[1] in MODULES and argv[2].isdigit() and argv[3].isdigit():
        sys.stdout.write(serialize(render_live(argv[1], int(argv[2]), int(argv[3]))))
        return 0
    if len(argv) != 1 or argv[0] not in table:
        print(f"{USAGE}; modes: {' '.join(table)}; live modules: {' '.join(MODULES)}", file=sys.stderr)
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
