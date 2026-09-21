#!/usr/bin/env python3
"""Oracle for the Go port of three pure domain modules (ADR-0029 §2.4, W1).

`app.domain.interests`, `app.domain.preferences` and `app.domain.reports`
become `services/core/internal/domain/{interests,preferences,reports}`. Go must
answer exactly as Python answers, so instead of restating the rules this script
calls the real functions inside the pinned API image and records what each one
returns or raises. The Go tests replay every case through both the typed entry
points and the `...Value` ones that take Python-shaped objects.

Each invocation renders one file, chosen by MODE:

    docker run --rm -i --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d - MODE < scripts/render_domain_w1_goldens.py \\
      > services/core/internal/domain/PKG/testdata/python_MODE.json

`--list` prints every MODE and its target path, tab separated, so the whole set
regenerates with:

    IMAGE=mobile-parity-api:7bf58e3d
    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_domain_w1_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_domain_w1_goldens.py > "$path"
    done

Modes: `interests`, `preferences` and `reports` hold the module constants and
the named edge cases (every Python domain test's inputs, plus the traps found
while porting); `preferences-scores` walks the score arithmetic over a table of
counts; `<module>-fuzz-<k>` is shard k of that module's seeded fuzz. Shards are
contiguous slices of one fixed case list, so one shard rendered alone gives the
same bytes.

Non-ASCII characters whose identity matters (combining marks, zero-width and
spacing characters) are spelled with `chr()` below: the formatter turns `\\u`
escapes into literal characters, and a literal U+0301 cannot be reviewed.

## Encoding

JSON cannot tell `3` from `3.0` or a tuple from a list, and a float written in
decimal either loses bits or grows digit runs the repository guard refuses. So:

* None, bool, int with at most eight digits, and a str not starting with `$`
  are plain JSON; a JSON object whose keys do not start with `$` is a dict;
* `"$i:<hex>"` is a larger int, in hex with `_` between every four digits
  so that no decimal digit run can trip the guard's number or phone rules;
  `"$f:<float.hex()>"` is a float and `"$o:<type name>"` an object of any
  other type;
* `{"$tuple": list}` is a tuple;
* `{"$runs": [[item, count], ...]}` is a list and `{"$str": [[unit, count],
  ...]}` a str, both expanded by repetition. They keep each file under one MiB
  at `indent=1`, and the second also carries any str that starts with `$`.

A result is `{"ok": value}` or `{"raised": {"type", "message", "code"}}`, where
`code` is the exception's `.code` attribute or null.
"""

from __future__ import annotations

import dataclasses
import json
import platform
import random
import re
import sys

sys.path.insert(0, "/srv")

from app.domain import interests, preferences, reports  # noqa: E402

SEED = 29
FUZZ_CASES = 2400
SMALL_INT = 10**8
LONG_STR = 48
TARGET = "services/core/internal/domain/{package}/testdata/python_{mode}.json"
FUZZ_SHARDS = {"interests": 2, "preferences": 4, "reports": 2}

#: Copies of the digit rules in scripts/repo_guard.py. A rendered line that
#: matches one could not be committed, so rendering stops instead.
GUARD_RULES = (
    re.compile(r"(?<![A-Za-z0-9])\d(?:[ .-]?\d){8,63}(?![A-Za-z0-9])"),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:3|5|7|8|9)"
        r"(?:[ ().-]*\d){8}(?!\d)"
    ),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*2"
        r"(?:[ ().-]*\d){8,9}(?!\d)"
    ),
)

ZERO_WIDTH_SPACE = chr(0x200B)
ZERO_WIDTH_JOINER = chr(0x200D)
WORD_JOINER = chr(0x2060)
BYTE_ORDER_MARK = chr(0xFEFF)
MONGOLIAN_VOWEL_SEPARATOR = chr(0x180E)
SOFT_HYPHEN = chr(0xAD)
COMBINING_ACUTE = chr(0x301)
COMBINING_DOT_BELOW = chr(0x323)
COMBINING_BREVE = chr(0x306)
NO_BREAK_SPACE = chr(0xA0)
IDEOGRAPHIC_SPACE = chr(0x3000)
HYPHEN = chr(0x2010)
EN_DASH = chr(0x2013)
HALFWIDTH_FULL_STOP = chr(0xFF61)
REPLACEMENT_CHARACTER = chr(0xFFFD)
E_ACUTE = chr(0xE9)
E_DOT_CIRCUMFLEX = chr(0x1EC7)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)
STEAMING_BOWL = chr(0x1F35C)
CJK_EXTENSION_B = chr(0x20000)

WHITESPACE = tuple(chr(c) for c in range(sys.maxunicode + 1) if chr(c).isspace())
#: Characters that read as spacing or nothing but are not `str.isspace()`.
LOOKALIKES = (
    ZERO_WIDTH_SPACE,
    ZERO_WIDTH_JOINER,
    WORD_JOINER,
    BYTE_ORDER_MARK,
    MONGOLIAN_VOWEL_SEPARATOR,
    SOFT_HYPHEN,
)
MARKS = (COMBINING_ACUTE, COMBINING_DOT_BELOW, COMBINING_BREVE)
WIDE = (
    GRINNING_FACE,
    STEAMING_BOWL,
    CJK_EXTENSION_B,
    HALFWIDTH_FULL_STOP,
    REPLACEMENT_CHARACTER,
    E_DOT_CIRCUMFLEX,
)
NON_STRINGS = (None, True, False, 0, 7, 1.0, -0.0, b"cafe", ("cafe",), ["cafe"])


def fullwidth(text: str) -> str:
    return "".join(chr(ord(char) + 0xFEE0) for char in text)


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
    if isinstance(value, list):
        return enc_items(value)
    if isinstance(value, tuple):
        return {"$tuple": enc_items(value)}
    if isinstance(value, dict):
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    if dataclasses.is_dataclass(value) and not isinstance(value, type):
        fields = dataclasses.fields(value)
        return enc({field.name: getattr(value, field.name) for field in fields})
    return f"$o:{type(value).__name__}"


def enc_items(items: list | tuple) -> object:
    encoded = [enc(item) for item in items]
    runs: list[list] = []
    for item in encoded:
        key = json.dumps(item, sort_keys=True)
        if runs and runs[-1][0] == key:
            runs[-1][2] += 1
        else:
            runs.append([key, item, 1])
    if len(encoded) >= 4 and 2 * len(runs) <= len(encoded):
        return {"$runs": [[item, count] for _key, item, count in runs]}
    return encoded


def enc_str(text: str) -> object:
    if any(0xD800 <= ord(char) <= 0xDFFF for char in text):
        raise ValueError("a surrogate cannot cross into Go strings")
    if len(text) <= LONG_STR:
        return {"$str": [[text, 1]]} if text.startswith("$") else text
    runs: list[list] = []
    index = 0
    while index < len(text):
        unit, count = text[index], 1
        for width in (1, 2, 3, 4):
            candidate = text[index : index + width]
            if len(candidate) < width:
                break
            repeats = 1
            while text.startswith(candidate, index + repeats * width):
                repeats += 1
            if repeats >= 2 and width * repeats > len(unit) * count:
                unit, count = candidate, repeats
        if count == 1 and runs and runs[-1][1] == 1:
            runs[-1][0] += unit
        else:
            runs.append([unit, count])
        index += len(unit) * count
    if "".join(unit * count for unit, count in runs) != text:
        raise AssertionError("run encoding does not round-trip")
    if 4 * len(runs) < len(text) or text.startswith("$"):
        return {"$str": runs}
    return text


def outcome(function, *args: object, **kwargs: object) -> dict:
    try:
        value = function(*args, **kwargs)
    except Exception as exc:  # every refusal is data for the oracle
        return {
            "raised": {
                "type": type(exc).__name__,
                "message": str(exc),
                "code": getattr(exc, "code", None),
            }
        }
    return {"ok": enc(value)}


FUNCTIONS = {
    "normalise_interests": interests.normalise_interests,
    "normalise_budget_band": interests.normalise_budget_band,
    "budget_band": interests.budget_band,
    "build_preference_profile": preferences.build_preference_profile,
}


def case(fn: str, name: str, *args: object) -> dict:
    return {
        "fn": fn,
        "name": name,
        "args": [enc(arg) for arg in args],
        "result": outcome(FUNCTIONS[fn], *args),
    }


def report_case(name: str, **kwargs: object) -> dict:
    return {
        "fn": "validate_report",
        "name": name,
        "kwargs": {key: enc(value) for key, value in kwargs.items()},
        "result": outcome(reports.validate_report, **kwargs),
    }


def code_point_ranges(chars: tuple[str, ...]) -> list[list[int]]:
    ranges: list[list[int]] = []
    for point in sorted(ord(char) for char in chars):
        if ranges and ranges[-1][1] == point - 1:
            ranges[-1][1] = point
        else:
            ranges.append([point, point])
    return ranges


# ---------------------------------------------------------------------------
# Shared random pieces
# ---------------------------------------------------------------------------


def near_miss(rng: random.Random, word: str) -> str:
    """A string that is almost `word`: the ways a client or a paste goes wrong."""

    kind = rng.randrange(10)
    if kind == 0:
        return word.upper()
    if kind == 1:
        return word.capitalize()
    if kind == 2:
        return rng.choice(WHITESPACE) + word
    if kind == 3:
        return word + rng.choice(WHITESPACE)
    if kind == 4:
        return word + rng.choice(LOOKALIKES)
    if kind == 5:
        return word + rng.choice(MARKS)
    if kind == 6:
        if "-" in word:
            return word.replace("-", rng.choice((HYPHEN, EN_DASH, "_", " ", "")), 1)
        return word + "s"
    if kind == 7:
        return word[: rng.randrange(len(word))]
    if kind == 8:
        return word + rng.choice(WIDE)
    return rng.choice(("", "du-thuyen", "sang-chanh", "Ăn uống", "Cafe"))


def pad(rng: random.Random, most: int = 3) -> str:
    return "".join(rng.choice(WHITESPACE) for _ in range(rng.randrange(most + 1)))


# ---------------------------------------------------------------------------
# interests
# ---------------------------------------------------------------------------


def interests_constants() -> dict:
    return {
        "interest_tags": [[tag.id, tag.label] for tag in interests.INTEREST_TAGS],
        "interest_ids": list(interests.INTEREST_IDS),
        "max_interests": interests.MAX_INTERESTS,
        "budget_bands": [enc(band) for band in interests.BUDGET_BANDS],
        "budget_band_ids": list(interests.BUDGET_BAND_IDS),
    }


def interests_edges() -> list[dict]:
    ids = list(interests.INTEREST_IDS)
    out: list[dict] = []

    def tags(name: str, value: object) -> None:
        out.append(case("normalise_interests", name, value))

    order = "test_order_is_the_vocabularys_and_not_the_callers"
    tags(f"{order}/first", ["cafe", "an-uong"])
    tags(f"{order}/second", ["an-uong", "cafe"])
    tags("test_repeats_collapse", ["cafe", "cafe", "cafe"])
    tags("test_empty_is_a_real_answer", [])
    tags("test_an_unknown_word_is_refused_rather_than_dropped", ["cafe", "du-thuyen"])
    tags("test_a_non_string_is_refused", ["cafe", 7])
    tags("test_a_bare_string_is_not_a_list_of_tags", "cafe")
    tags("test_choosing_everything_is_allowed", ids)
    tags("empty_tuple", ())
    tags("max_reversed", ids[::-1])
    tags("max_plus_one_with_duplicate", [*ids, ids[0]])
    tags("every_id_twice_interleaved", [t for p in zip(ids, ids[::-1]) for t in p])
    tags("tuple_every_id_reversed", tuple(ids[::-1]))
    tags("twenty_repeats", ["game"] * 20)
    for tag in ids:
        tags(f"one/{tag}", [tag])
    tags("unknown_before_non_string", ["du-thuyen", 7])
    tags("non_string_before_unknown", [7, "du-thuyen"])
    tags("known_then_none", ["cafe", None])
    for label, value in (
        ("none", None),
        ("dict", {"cafe": True}),
        ("int", 7),
        ("float", 1.0),
        ("bool", True),
        ("bytes", b"cafe"),
        ("set", {"cafe"}),
        ("empty_string", ""),
    ):
        tags(f"not_a_list/{label}", value)
    for label, item in (
        ("none", None),
        ("true", True),
        ("zero", 0),
        ("float", 1.0),
        ("bytes", b"cafe"),
        ("nested_list", ["cafe"]),
        ("nested_tuple", ("cafe",)),
        ("dict", {"cafe": 1}),
    ):
        tags(f"not_a_string/{label}", ["an-uong", item])
    for label, word in (
        ("upper", "Cafe"),
        ("shout", "CAFE"),
        ("leading_space", " cafe"),
        ("trailing_space", "cafe "),
        ("no_break_space", "cafe" + NO_BREAK_SPACE),
        ("ideographic_space", IDEOGRAPHIC_SPACE + "cafe"),
        ("unit_separator", "cafe\x1f"),
        ("zero_width_space", "ca" + ZERO_WIDTH_SPACE + "fe"),
        ("byte_order_mark", BYTE_ORDER_MARK + "cafe"),
        ("precomposed_accent", "caf" + E_ACUTE),
        ("combining_accent", "cafe" + COMBINING_ACUTE),
        ("unicode_hyphen", "an" + HYPHEN + "uong"),
        ("en_dash", "an" + EN_DASH + "uong"),
        ("underscore", "an_uong"),
        ("fullwidth", fullwidth("cafe")),
        ("nul", "cafe\x00"),
        ("astral", "cafe" + GRINNING_FACE),
        ("empty", ""),
        ("label_not_id", "Ăn uống"),
        ("prefix", "caf"),
        ("dollar", "$cafe"),
    ):
        tags(f"near_miss/{label}", ["cafe", word])

    def band(name: str, value: object) -> None:
        out.append(case("normalise_budget_band", name, value))

    def lookup(name: str, value: object) -> None:
        out.append(case("budget_band", name, value))

    skipped = "test_a_skipped_budget_is_none_and_not_the_cheapest_band"
    band(skipped, None)
    lookup(skipped, None)
    lookup("test_a_band_this_build_dropped_reads_as_no_answer", "sang-chanh")
    band("test_an_unknown_band_is_refused_on_the_way_in", "sang-chanh")
    for band_id in interests.BUDGET_BAND_IDS:
        band(f"test_band_ids_resolve_to_themselves/{band_id}", band_id)
        lookup(f"test_band_ids_resolve_to_themselves/{band_id}", band_id)
    for label, word in (
        ("empty", ""),
        ("title_case", "Vua-Phai"),
        ("leading_space", " vua-phai"),
        ("ideographic_space", "vua-phai" + IDEOGRAPHIC_SPACE),
        ("file_separator", "vua-phai\x1c"),
        ("label_not_id", "100K" + EN_DASH + "250K"),
        ("interest_id", "cafe"),
        ("underscore", "tiet_kiem"),
        ("combining_mark", "tiet-kiem" + COMBINING_ACUTE),
        ("astral", "thoai-mai" + GRINNING_FACE),
    ):
        band(f"near_miss/{label}", word)
        lookup(f"near_miss/{label}", word)
    for label, value in (
        ("zero", 0),
        ("one", 1),
        ("float", 1.0),
        ("true", True),
        ("false", False),
        ("list", ["vua-phai"]),
        ("tuple", ("vua-phai",)),
        ("dict", {"id": "vua-phai"}),
        ("bytes", b"vua-phai"),
    ):
        band(f"not_a_string/{label}", value)
    return out


def random_tags(rng: random.Random) -> object:
    ids = interests.INTEREST_IDS
    roll = rng.random()
    if roll < 0.05:
        others = (None, "cafe", "", 7, 1.5, True, {"cafe": True}, b"cafe", frozenset())
        return rng.choice(others)
    items: list[object] = [rng.choice(ids) for _ in range(rng.randrange(11))]
    style = rng.random()
    if items and style < 0.3:
        items[rng.randrange(len(items))] = near_miss(rng, rng.choice(ids))
    elif items and style < 0.42:
        items[rng.randrange(len(items))] = rng.choice(NON_STRINGS)
    elif len(items) >= 2 and style < 0.5:
        first, second = rng.sample(range(len(items)), 2)
        items[first] = near_miss(rng, rng.choice(ids))
        items[second] = rng.choice(NON_STRINGS)
    return tuple(items) if roll > 0.9 else items


def random_band(rng: random.Random, *, strings_only: bool) -> object:
    words = interests.BUDGET_BAND_IDS + interests.INTEREST_IDS
    roll = rng.random()
    if roll < 0.1:
        return None
    if roll < 0.55:
        return rng.choice(interests.BUDGET_BAND_IDS)
    if roll < 0.92 or strings_only:
        return near_miss(rng, rng.choice(words))
    return rng.choice(NON_STRINGS)


def interests_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 1)
    out = [
        case("normalise_interests", f"fuzz/{index}", random_tags(rng))
        for index in range(FUZZ_CASES)
    ]
    out += [
        case(
            "normalise_budget_band",
            f"fuzz/{index}",
            random_band(rng, strings_only=False),
        )
        for index in range(FUZZ_CASES)
    ]
    out += [
        case("budget_band", f"fuzz/{index}", random_band(rng, strings_only=True))
        for index in range(FUZZ_CASES)
    ]
    return out


# ---------------------------------------------------------------------------
# preferences
# ---------------------------------------------------------------------------

FOOD = "quan-an-local"
LABELS = (
    "BBQ",
    "Lẩu",
    "Local",
    "Lào",
    "Cà phê",
    "Chill",
    "Outdoor",
    "a",
    "B",
    "b",
    E_ACUTE,
    "e" + COMBINING_ACUTE,
    HALFWIDTH_FULL_STOP,
    GRINNING_FACE,
    "Zeta",
    "Alpha",
)


def visit(category: object, *kinds: object) -> dict:
    return {"category": category, "kinds": list(kinds)}


def profile_case(name: str, visits: list, trips: list) -> dict:
    return case("build_preference_profile", name, visits, trips)


def preferences_constants() -> dict:
    return {
        "section_of_category": [
            [category, section]
            for category, section in preferences.SECTION_OF_CATEGORY.items()
        ],
        "section_order": list(preferences.SECTION_ORDER),
        "max_tastes_per_section": preferences.MAX_TASTES_PER_SECTION,
        "max_label": preferences.MAX_LABEL,
        "isspace": code_point_ranges(WHITESPACE),
    }


def preferences_edges() -> list[dict]:
    out: list[dict] = []

    def add(name: str, visits: list, trips: list | None = None) -> None:
        out.append(profile_case(name, visits, trips or []))

    add("test_top_taste_of_a_section_scores_one", [visit(FOOD, "BBQ")] * 2)
    add(
        "test_score_is_the_share_of_the_busiest_taste_in_the_section",
        [visit(FOOD, "BBQ")] * 4 + [visit(FOOD, "Lẩu")] * 3,
    )
    add(
        "test_sections_are_scored_independently",
        [visit("cafe", "Cà phê")] * 40 + [visit(FOOD, "BBQ")] * 4,
    )
    add(
        "test_count_travels_beside_the_score_so_the_ratio_can_be_checked",
        [visit(FOOD, "BBQ")] * 3 + [visit(FOOD, "Lẩu")],
    )
    add(
        "test_rounding_is_half_up_not_bankers",
        [visit(FOOD, "BBQ")] * 200 + [visit(FOOD, "Lẩu")] * 25,
    )
    add(
        "test_unknown_category_is_skipped_never_defaulted",
        [visit("khong-co-trong-bang", "Gì Đó"), visit(FOOD, "BBQ")],
    )
    add("test_a_visit_with_no_kinds_contributes_nothing", [visit("cafe")])
    add(
        "test_a_group_with_no_checkins_has_no_sections",
        [],
        [{"split_total_vnd": 0, "headcount": 2}],
    )
    add(
        "test_same_rows_render_the_same_way_twice",
        [
            visit("cafe", "Chill"),
            visit("vui-choi", "Outdoor"),
            visit(FOOD, "BBQ", "Local"),
        ],
    )
    ties = "test_ties_break_on_label_not_on_insertion_order"
    add(f"{ties}/forward", [visit(FOOD, "Zeta"), visit(FOOD, "Alpha")])
    add(f"{ties}/backward", [visit(FOOD, "Alpha"), visit(FOOD, "Zeta")])
    add(
        "test_taste_count_reports_the_full_number_when_the_list_is_capped",
        [visit(FOOD, f"Mon{index:02d}") for index in range(10)],
    )
    add(
        "test_average_is_floor_division_over_person_trips",
        [],
        [
            {"split_total_vnd": 1_000_000, "headcount": 3},
            {"split_total_vnd": 500_000, "headcount": 4},
        ],
    )
    add("test_no_trips_means_no_average_rather_than_zero", [])
    money = "test_a_money_value_that_is_not_a_whole_number_of_dong_is_refused"
    for label, bad in (
        ("true", True),
        ("false", False),
        ("str", "200000"),
        ("float", 200_000.0),
        ("none", None),
        ("negative", -1),
    ):
        add(f"{money}/{label}", [], [{"split_total_vnd": bad, "headcount": 2}])
    heads = "test_a_headcount_that_is_not_an_integer_is_refused"
    for label, bad in (("true", True), ("str", "3"), ("float", 3.0), ("none", None)):
        add(f"{heads}/{label}", [], [{"split_total_vnd": 1000, "headcount": bad}])
    add("test_a_visit_that_is_not_a_mapping_is_refused", ["quan-an-local"])
    add("test_a_trip_that_is_not_a_mapping_is_refused", [], ["a trip"])

    for category in preferences.SECTION_OF_CATEGORY:
        add(f"category/{category}", [visit(category, "BBQ")])
    for label, category in (
        ("title_case", "Cafe"),
        ("leading_space", " cafe"),
        ("trailing_space", "quan-an-local "),
        ("prefix", "quan-an"),
        ("none", None),
        ("int", 7),
        ("list", ["cafe"]),
    ):
        add(f"category/skipped/{label}", [visit(category, "BBQ"), visit(FOOD, "Local")])
    add("category/missing_key", [{"kinds": ["BBQ"]}, visit(FOOD, "Local")])
    add("visit/extra_keys_ignored", [{"category": "cafe", "kinds": ["BBQ"], "x": 1}])
    for label, kinds in (
        ("none", None),
        ("string", "BBQ"),
        ("int", 7),
        ("dict", {"BBQ": 1}),
        ("empty_string_only", [""]),
        ("whitespace_only", ["   "]),
    ):
        add(f"kinds/skipped/{label}", [{"category": "cafe", "kinds": kinds}])
    add("kinds/missing_key", [{"category": "cafe"}, visit("cafe", "Chill")])
    add("kinds/tuple", [{"category": "cafe", "kinds": ("BBQ", "Lẩu")}])
    add(
        "kinds/non_strings_ignored",
        [visit("cafe", None, 7, True, 1.0, b"BBQ", ["BBQ"], ("BBQ",), "BBQ")],
    )
    add("kinds/only_non_strings", [visit("cafe", None, 7, ["BBQ"])])
    add(
        "kinds/duplicates_count_twice",
        [visit("cafe", "BBQ", "BBQ", "BBQ"), visit("cafe", "Chill")],
    )

    for char in WHITESPACE:
        add(
            f"strip/U+{ord(char):04X}",
            [
                visit("cafe", f"{char}BBQ{char}"),
                visit("cafe", char * 2),
                visit("cafe", "BBQ"),
            ],
        )
    for char in LOOKALIKES:
        add(
            f"no_strip/U+{ord(char):04X}",
            [visit("cafe", f"{char}BBQ"), visit("cafe", char), visit("cafe", "BBQ")],
        )
    add("strip/interior_space_kept", [visit("cafe", " Cà  phê ")])
    add("strip/nul_is_not_space", [visit("cafe", "\x00BBQ\x00", "BBQ")])

    combining_e = "e" + COMBINING_ACUTE
    for label, kinds in (
        ("ascii_40", ["x" * 40]),
        ("ascii_41", ["x" * 41]),
        ("vietnamese_40", [E_DOT_CIRCUMFLEX * 40]),
        ("vietnamese_41", [E_DOT_CIRCUMFLEX * 41]),
        ("combining_pairs_40", [combining_e * 20]),
        ("combining_mark_cut_at_40", ["a" + combining_e * 20]),
        ("astral_40", [GRINNING_FACE * 40]),
        ("astral_41", [GRINNING_FACE * 41]),
        ("astral_at_40_kept_whole", ["x" * 39 + GRINNING_FACE + "y"]),
        ("strip_then_truncate", ["  " + "x" * 41 + IDEOGRAPHIC_SPACE]),
        ("truncation_keeps_trailing_space", ["x" * 39 + " yz"]),
        ("merge_after_truncation", ["x" * 40 + "1", "x" * 40 + "2", "x" * 40]),
        ("file_separator_padding_then_40", ["\x1c" + "x" * 40 + "\x1f"]),
        ("dollar_prefix", ["$BBQ", "$f:0x1.0p+0"]),
    ):
        add(f"label/{label}", [visit("cafe", *kinds)])

    for label, kinds in (
        ("local_before_lao", ["Lào", "Local"]),
        ("upper_before_lower", ["b", "B", "a", "A"]),
        ("precomposed_after_combining", ["f", E_ACUTE, combining_e, "e"]),
        ("astral_after_halfwidth", [GRINNING_FACE, HALFWIDTH_FULL_STOP, "z"]),
        ("prefix_first", ["BBQ", "BB", "B"]),
        ("digits_and_punctuation", ["a", "1", "A", "_", "~"]),
        ("nul_suffix", ["a\x00", "a"]),
    ):
        add(f"sort/{label}", [visit(FOOD, *kinds)])
    add("sort/count_then_label", [visit(FOOD, "c", "b", "b", "a", "a", "d", "d", "d")])
    add("cap/exactly_six", [visit(FOOD, *"abcdef")])
    add("cap/seven_last_tie_cut", [visit(FOOD, *"gfedcba")])
    add(
        "cap/both_sections", [visit(FOOD, *"abcdefgh"), visit("vui-choi", *"ABCDEFGHI")]
    )
    add("order/activity_rows_first", [visit("cafe", "Chill"), visit(FOOD, "BBQ")])
    add("order/activity_only", [visit("di-choi-dem", "Bar")])
    add("score/zero", [visit("cafe", *(["T"] * 201), "u")])
    add("score/one_third", [visit("cafe", "T", "T", "T", "u")])

    one_trip = {"split_total_vnd": 1, "headcount": 1}
    add("precedence/visit_before_trip", ["x"], ["y"])
    add("precedence/second_trip_bad", [visit("cafe", "BBQ")], [one_trip, "bad"])
    add("trips/headcount_zero", [], [{"split_total_vnd": 5, "headcount": 0}])
    add("trips/floor", [], [{"split_total_vnd": 7, "headcount": 2}])
    add("trips/zero_zero", [], [{"split_total_vnd": 0, "headcount": 0}])
    add("trips/beyond_float", [], [{"split_total_vnd": 2**62 + 1, "headcount": 3}])
    add("trips/near_int64", [], [{"split_total_vnd": 2**61 + 7, "headcount": 1}] * 3)
    add("trips/missing_split", [], [{"headcount": 2}])
    add("trips/missing_headcount", [], [{"split_total_vnd": 2}])
    add("trips/float_zero", [], [{"split_total_vnd": 0.0, "headcount": 1}])
    add("trips/false_headcount", [], [{"split_total_vnd": 1, "headcount": False}])
    add("trips/extra_keys", [], [{"split_total_vnd": 9, "headcount": 2, "id": "o"}])
    add("trips/tuple_not_mapping", [], [("split_total_vnd", 1)])
    return out


def preferences_scores() -> list[dict]:
    """The score of every count against a small top, and sampled large tops.

    One visit per case: a repeated kind counts once per repetition, so a top of
    `n` is `n` equal labels and costs a single run in the encoding.
    """

    out: list[dict] = []
    for top in range(1, 46):
        for start in range(1, top + 1, 5):
            counts = range(start, min(start + 5, top + 1))
            kinds = ["T"] * top
            for label, count in zip("pqrsu", counts, strict=False):
                kinds += [label] * count
            name = f"score/top-{top}/from-{start}"
            out.append(profile_case(name, [visit("cafe", *kinds)], []))
    rng = random.Random(SEED * 1000 + 2)
    for index in range(80):
        top = rng.randrange(46, 5000)
        counts = []
        for _ in range(5):
            if rng.random() < 0.5:
                # Next to an exact half hundredth, where the rounding decides.
                half = ((2 * rng.randrange(100) + 1) * top) // 200
                counts.append(max(1, min(top, half + rng.choice((-1, 0, 1)))))
            else:
                counts.append(rng.randrange(1, top + 1))
        kinds = ["T"] * top
        for label, count in zip("pqrsu", counts, strict=True):
            kinds += [label] * count
        out.append(profile_case(f"score/sampled-{index}", [visit("cafe", *kinds)], []))
    return out


def random_label(rng: random.Random) -> object:
    roll = rng.random()
    if roll < 0.78:
        return rng.choice(LABELS)
    if roll < 0.86:
        return pad(rng) + rng.choice(LABELS) + rng.choice(WHITESPACE)
    if roll < 0.89:
        return "".join(rng.choice(WHITESPACE) for _ in range(1 + rng.randrange(3)))
    if roll < 0.92:
        return rng.choice(NON_STRINGS)
    if roll < 0.94:
        return rng.choice(LOOKALIKES) + rng.choice(LABELS)
    unit = rng.choice(
        ("x", E_DOT_CIRCUMFLEX, "e" + COMBINING_ACUTE, GRINNING_FACE, "ab")
    )
    size = rng.randrange(38, 45)
    text = (unit * size)[:size]
    if rng.random() < 0.5:
        text += rng.choice(("1", "2", " z", IDEOGRAPHIC_SPACE))
    return text


def random_visit(rng: random.Random) -> object:
    if rng.random() < 0.01:
        return rng.choice(("quan-an-local", None, ["cafe"], ("cafe", ["BBQ"]), 7))
    categories = tuple(preferences.SECTION_OF_CATEGORY)
    row: dict = {}
    roll = rng.random()
    if roll < 0.82:
        row["category"] = rng.choice(categories)
    elif roll < 0.9:
        row["category"] = near_miss(rng, rng.choice(categories))
    elif roll < 0.95:
        row["category"] = rng.choice((None, 7, ["cafe"], b"cafe"))
    labels = [random_label(rng) for _ in range(rng.choice((0, 1, 1, 2, 2, 3, 4)))]
    roll = rng.random()
    if roll < 0.84:
        row["kinds"] = labels
    elif roll < 0.89:
        row["kinds"] = tuple(labels)
    elif roll < 0.97:
        row["kinds"] = rng.choice((None, "BBQ", 7))
    elif roll < 0.99:
        row["kinds"] = {label: 1 for label in labels if isinstance(label, str)}
    if rng.random() < 0.03:
        row["place_id"] = "p"
    return row


def random_trip(rng: random.Random) -> dict:
    split = rng.choice(
        (
            0,
            rng.randrange(1, 10**6),
            rng.randrange(10**6, 10**9),
            rng.randrange(2**40, 2**60),
        )
    )
    headcount = rng.choice((0, 1, 2, 3, rng.randrange(4, 60)))
    return {"split_total_vnd": split, "headcount": headcount}


def spoil_trips(rng: random.Random, trips: list) -> None:
    if not trips:
        trips.append(rng.choice(("a trip", None, 7)))
        return
    index = rng.randrange(len(trips))
    roll = rng.random()
    if roll < 0.2:
        trips[index] = rng.choice(("a trip", None, [("headcount", 1)], ("x",)))
        return
    key = rng.choice(("split_total_vnd", "headcount"))
    trip = dict(trips[index])
    if roll < 0.35:
        del trip[key]
    else:
        bad = (True, False, 1.0, 0.0, "200000", None, -1, [3], -(2**40))
        trip[key] = rng.choice(bad)
    trips[index] = trip


def preferences_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 3)
    out: list[dict] = []
    for index in range(FUZZ_CASES):
        visits = [random_visit(rng) for _ in range(rng.randrange(8))]
        if visits and rng.random() < 0.3:
            visits += [rng.choice(visits)] * rng.randrange(1, 20)
        if rng.random() < 0.2:
            rng.shuffle(visits)
        trips = [random_trip(rng) for _ in range(rng.randrange(4))]
        if rng.random() < 0.1:
            spoil_trips(rng, trips)
        out.append(profile_case(f"fuzz/{index}", visits, trips))
    return out


# ---------------------------------------------------------------------------
# reports
# ---------------------------------------------------------------------------

NOTE_UNITS = (
    "x",
    E_ACUTE,
    "e" + COMBINING_ACUTE,
    GRINNING_FACE,
    "ab",
    CJK_MIDDLE,
    E_DOT_CIRCUMFLEX,
)


def reports_constants() -> dict:
    return {
        "target_types": list(reports.TARGET_TYPES),
        "reasons": list(reports.REASONS),
        "max_note_length": reports.MAX_NOTE_LENGTH,
        "isspace": code_point_ranges(WHITESPACE),
    }


def reports_edges() -> list[dict]:
    out: list[dict] = []
    longest = reports.MAX_NOTE_LENGTH

    def add(name: str, **kwargs: object) -> None:
        out.append(report_case(name, **kwargs))

    every = "test_every_declared_combination_is_accepted"
    for target_type in reports.TARGET_TYPES:
        for reason in reports.REASONS:
            name = f"{every}/{target_type}/{reason}"
            add(name, target_type=target_type, reason=reason)
    vocab = "test_a_word_outside_the_vocabulary_is_refused"
    for field, value in (
        ("target_type", "trip"),
        ("target_type", None),
        ("reason", "vi phạm"),
        ("reason", 7),
    ):
        body = {"target_type": "post", "reason": "spam", field: value}
        add(f"{vocab}/{field}/{value!r}", **body)
    note = "test_the_note_is_optional_trimmed_and_bounded"
    add(f"{note}/blank", target_type="post", reason="spam", note="  ")
    add(f"{note}/trimmed", target_type="post", reason="spam", note=" nội dung xấu ")
    add(f"{note}/longest", target_type="post", reason="other", note="x" * longest)
    too_long = "x" * (longest + 1)
    add(f"{note}/too_long", target_type="post", reason="other", note=too_long)
    add(f"{note}/not_text", target_type="post", reason="other", note=12)

    add("note/omitted", target_type="story", reason="other")
    add("note/none", target_type="story", reason="other", note=None)
    add("note/empty", target_type="story", reason="other", note="")
    for label, value in (
        ("title_case", "Person"),
        ("trailing_space", "person "),
        ("plural", "posts"),
        ("empty", ""),
        ("tuple", ("person",)),
        ("list", ["person"]),
        ("bytes", b"person"),
        ("true", True),
        ("combining", "person" + COMBINING_ACUTE),
        ("fullwidth", fullwidth("post")),
    ):
        add(f"target_type/{label}", target_type=value, reason="spam")
        add(f"reason/{label}", target_type="post", reason=value)
    add("precedence/target_before_reason", target_type="x", reason="y", note=1)
    add("precedence/reason_before_note", target_type="post", reason="y", note=1)
    add(
        "precedence/reason_before_long_note",
        target_type="post",
        reason="y",
        note="x" * 900,
    )
    for label, value in (
        ("int", 12),
        ("float", 1.5),
        ("true", True),
        ("false", False),
        ("empty_list", []),
        ("list", ["x"]),
        ("tuple", ("x",)),
        ("dict", {"x": 1}),
        ("bytes", b"x"),
    ):
        add(f"note/not_text/{label}", target_type="comment", reason="spam", note=value)

    combining_e = "e" + COMBINING_ACUTE
    for label, text in (
        ("ascii_500_trailing_space", "x" * longest + " "),
        ("ascii_500_padded", "   " + "x" * longest + IDEOGRAPHIC_SPACE),
        ("ascii_499", "x" * (longest - 1)),
        ("precomposed_500", E_ACUTE * longest),
        ("precomposed_501", E_ACUTE * (longest + 1)),
        ("combining_pairs_500", combining_e * (longest // 2)),
        ("combining_pairs_501", combining_e * (longest // 2) + "e"),
        ("astral_500", GRINNING_FACE * longest),
        ("astral_501", GRINNING_FACE * (longest + 1)),
        ("astral_half_ascii_half", GRINNING_FACE * 250 + "x" * 250),
        ("cjk_500", CJK_MIDDLE * longest),
        ("vietnamese_501", E_DOT_CIRCUMFLEX * (longest + 1)),
        ("file_separators_around_500", "\x1c" + "x" * longest + "\x1f"),
        ("unit_separator_after_499", "x" * (longest - 1) + "\x1f"),
        (
            "zero_width_around_499",
            ZERO_WIDTH_SPACE + "x" * (longest - 1) + ZERO_WIDTH_SPACE,
        ),
        ("interior_spaces", "a  " + IDEOGRAPHIC_SPACE + " b"),
        ("nul_kept", "\x00a\x00"),
        ("dollar_prefix", "$f:0x1.0p+0"),
    ):
        add(
            f"note/length/{label}",
            target_type="message",
            reason="harassment",
            note=text,
        )
    for char in WHITESPACE:
        code = f"U+{ord(char):04X}"
        add(
            f"note/strip/{code}",
            target_type="post",
            reason="spam",
            note=f"{char}a{char}",
        )
        add(f"note/strip_only/{code}", target_type="post", reason="spam", note=char * 3)
    for char in LOOKALIKES:
        code = f"U+{ord(char):04X}"
        add(f"note/no_strip/{code}", target_type="post", reason="spam", note=char)
    return out


def vocab_value(rng: random.Random, words: tuple[str, ...]) -> object:
    roll = rng.random()
    if roll < 0.86:
        return rng.choice(words)
    if roll < 0.96:
        return near_miss(rng, rng.choice(words))
    return rng.choice(NON_STRINGS)


def random_note(rng: random.Random) -> str:
    size = rng.choice(
        (
            0,
            1,
            2,
            rng.randrange(3, 60),
            498,
            499,
            500,
            501,
            502,
            rng.randrange(480, 521),
        )
    )
    unit = rng.choice(NOTE_UNITS)
    reps, rest = divmod(size, len(unit))
    core = unit * reps + "y" * rest
    roll = rng.random()
    if core and roll < 0.15:
        cut = rng.randrange(len(core))
        core = core[:cut] + rng.choice(WHITESPACE) + core[cut:]
    elif roll < 0.25:
        core = rng.choice(LOOKALIKES) + core
    return pad(rng) + core + pad(rng)


def reports_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 4)
    out: list[dict] = []
    for index in range(FUZZ_CASES):
        kwargs: dict[str, object] = {
            "target_type": vocab_value(rng, reports.TARGET_TYPES),
            "reason": vocab_value(rng, reports.REASONS),
        }
        roll = rng.random()
        if roll < 0.08:
            pass
        elif roll < 0.14:
            kwargs["note"] = None
        elif roll < 0.19:
            kwargs["note"] = rng.choice(NON_STRINGS)
        else:
            kwargs["note"] = random_note(rng)
        out.append(report_case(f"fuzz/{index}", **kwargs))
    return out


# ---------------------------------------------------------------------------
# Modes
# ---------------------------------------------------------------------------

MODULES = {
    "interests": (interests_constants, interests_edges, interests_fuzz),
    "preferences": (preferences_constants, preferences_edges, preferences_fuzz),
    "reports": (reports_constants, reports_edges, reports_fuzz),
}


def modes() -> dict[str, tuple[str, object]]:
    table: dict[str, tuple[str, object]] = {}
    for package in MODULES:
        table[package] = (package, None)
        for shard in range(FUZZ_SHARDS[package]):
            table[f"{package}-fuzz-{shard}"] = (package, shard)
    table["preferences-scores"] = ("preferences", "scores")
    return table


def render(mode: str) -> dict:
    package, part = modes()[mode]
    constants, edges, fuzz = MODULES[package]
    document: dict[str, object] = {
        "generator": "scripts/render_domain_w1_goldens.py",
        "module": f"app.domain.{package}",
        "mode": mode,
        "python": platform.python_version(),
        "seed": SEED,
    }
    if part is None:
        document["constants"] = constants()
        document["cases"] = edges()
    elif part == "scores":
        document["cases"] = preferences_scores()
    else:
        cases = fuzz()
        shards = FUZZ_SHARDS[package]
        start = part * len(cases) // shards
        stop = (part + 1) * len(cases) // shards
        document["fuzz"] = {"shard": part, "shards": shards, "total": len(cases)}
        document["cases"] = cases[start:stop]
    return document


def main(argv: list[str]) -> int:
    table = modes()
    if argv == ["--list"]:
        for mode, (package, _part) in table.items():
            path = TARGET.format(package=package, mode=mode.replace("-", "_"))
            print(f"{mode}\t{path}")
        return 0
    if len(argv) != 1 or argv[0] not in table:
        print(f"usage: - MODE | --list; modes: {' '.join(table)}", file=sys.stderr)
        return 2
    rendered = json.dumps(render(argv[0]), ensure_ascii=True, indent=1) + "\n"
    for number, line in enumerate(rendered.splitlines(), 1):
        if any(rule.search(line) for rule in GUARD_RULES):
            print(f"line {number} would trip the repository guard", file=sys.stderr)
            return 1
    sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
