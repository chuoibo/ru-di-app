#!/usr/bin/env python3
"""Oracle for the Go port of `app.places.taste` and `app.places.scoring`.

ADR-0029 §2.4: Go must answer exactly as Python answers, so this script does not
restate the rules. It calls the real functions inside the pinned API image and
records what each one returns or raises; `services/core/internal/domain/taste`
and `.../scoring` replay every case through their typed entry points.

Each invocation renders one file, chosen by MODE:

    docker run --rm -i --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d - MODE < scripts/render_places_taste_goldens.py \\
      > services/core/internal/domain/PKG/testdata/python_MODE.json

`--list` prints every MODE and its target path, tab separated:

    IMAGE=mobile-parity-api:7bf58e3d
    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_places_taste_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_places_taste_goldens.py > "$path"
    done

Modes: `taste` and `scoring` hold the module constants and the named edge
cases; `scoring-grid` walks every combination of a small table of terms so that
exact ties at one half appear on both sides of even; `<module>-fuzz-<k>` is
shard k of that module's seeded fuzz. Shards are contiguous slices of one fixed
case list, so one shard rendered alone gives the same bytes.

## Inputs are the shapes the service passes

`place` is `PlaceRecord.to_row()`: `category` a str, `kinds` and `traits`
JSONB lists, prices BigInteger or None, `distance_km` a double or None,
`travel_minutes` an Integer or None, `group_fit` JSONB holding
`min_people`/`max_people` or None. Fuzz also drops keys and puts `{}` in
`group_fit`, and a few list elements that are not str; the Go replay maps an
absent key to None, `{}` to None and a non-str element to "" (which `_tokens`
drops exactly as it drops the element), and every such case is replayed, so
that mapping is itself checked. `profile` is a `TasteProfile`; its ints stay
inside int64, the range the Go types hold.

## Encoding

JSON cannot tell `3` from `3.0` or a tuple from a list, and long digit runs
trip the repository guard's number and phone rules. So:

* None, bool, an int with at most eight digits and a str with at most four
  ASCII digits that does not start with `$` are plain JSON;
* `"$i:<decimal>"` is a larger int and `"$f:<repr>"` a float, both with `_`
  after every fourth digit (Go deletes every `_` before parsing);
  `"$q:<numerator>/<denominator>"` is a Fraction, grouped the same way;
* `{"$cat": [piece, ...]}` is a str cut into pieces of at most four digits;
* `{"$tuple": list}`, `{"$set": sorted list}` and `{"$profile": dict}` are a
  tuple, a set and a `TasteProfile` (its fields plus `cache_key` and `known`).

A call result is `{"ok": value}` or `{"raised": {"type", "message"}}`.
"""

from __future__ import annotations

import json
import math
import platform
import random
import re
import struct
import sys
import typing
import unicodedata
from fractions import Fraction

sys.path.insert(0, "/srv")

from app.domain import interests  # noqa: E402
from app.places import scoring, taste  # noqa: E402

SEED = 1511
FUZZ_CASES = 2400
SMALL_INT = 10**8
INT64_MIN = -(2**63)
INT64_MAX = 2**63 - 1
MAX_BYTES = 1 << 20
TARGET = "services/core/internal/domain/{package}/testdata/python_{mode}.json"
FUZZ_SHARDS = {"taste": 7, "scoring": 6}

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
BYTE_ORDER_MARK = chr(0xFEFF)
SOFT_HYPHEN = chr(0xAD)
NO_BREAK_SPACE = chr(0xA0)
IDEOGRAPHIC_SPACE = chr(0x3000)
COMBINING_ACUTE = chr(0x301)
LONG_S = chr(0x17F)
KELVIN_SIGN = chr(0x212A)
CAPITAL_I_DOT = chr(0x130)
DOTLESS_I = chr(0x131)
SHARP_S = chr(0xDF)
CAPITAL_SHARP_S = chr(0x1E9E)
LIGATURE_FI = chr(0xFB01)
LIGATURE_ST = chr(0xFB06)
MICRO_SIGN = chr(0xB5)
CHEROKEE_SMALL_A = chr(0xAB70)
BULLET = chr(0x2022)
KATAKANA_MIDDLE_DOT = chr(0x30FB)

WHITESPACE = tuple(chr(c) for c in range(sys.maxunicode + 1) if chr(c).isspace())
LOOKALIKES = (ZERO_WIDTH_SPACE, ZERO_WIDTH_JOINER, BYTE_ORDER_MARK, SOFT_HYPHEN)
CASEFOLD_TRAPS = (
    LONG_S,
    KELVIN_SIGN,
    CAPITAL_I_DOT,
    DOTLESS_I,
    SHARP_S,
    CAPITAL_SHARP_S,
    LIGATURE_FI,
    LIGATURE_ST,
    MICRO_SIGN,
    CHEROKEE_SMALL_A,
)

VOCAB = interests.INTEREST_IDS
BAND_IDS = interests.BUDGET_BAND_IDS
CATEGORIES = ("quan-an-local", "cafe", "di-choi-dem", "vui-choi")
EVIDENCE_WORDS = tuple(
    dict.fromkeys(
        word
        for evidence in taste.EVIDENCE.values()
        for word in (*evidence.categories, *evidence.traits, *evidence.kinds)
    )
)
NOISE_WORDS = (
    "Chill",
    "View đẹp",
    "BBQ",
    "Rooftop",
    "Trà trộn",
    "Cafe",
    "Bar",
    "Nhóm đông",
    "Park view",
    "Local food",
    "",
    "   ",
)
STALE_BANDS = ("sang-chanh", "", " vua-phai", "VUA-PHAI", "tiet_kiem", "thoai-mai ")
MIDPOINTS = tuple(taste._midpoint_vnd(band) for band in BAND_IDS)


# ---------------------------------------------------------------------------
# Encoding
# ---------------------------------------------------------------------------


def group_digits(text: str) -> str:
    """`_` after every fourth ASCII digit of a run; a `.` does not end a run."""

    out: list[str] = []
    run = 0
    for char in text:
        if "0" <= char <= "9":
            if run and run % 4 == 0:
                out.append("_")
            run += 1
        elif char != ".":
            run = 0
        out.append(char)
    return "".join(out)


def enc_str(text: str) -> object:
    if any(0xD800 <= ord(char) <= 0xDFFF for char in text):
        raise ValueError("a surrogate cannot cross into Go strings")
    digits = sum("0" <= char <= "9" for char in text)
    if digits <= 4 and not text.startswith("$"):
        return text
    pieces = [""]
    count = 0
    for char in text:
        if count == 4:
            pieces.append("")
            count = 0
        pieces[-1] += char
        if "0" <= char <= "9":
            count += 1
    return {"$cat": pieces}


def enc(value: object) -> object:
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, int):
        if -SMALL_INT < value < SMALL_INT:
            return value
        return f"$i:{group_digits(str(value))}"
    if isinstance(value, float):
        return f"$f:{group_digits(repr(value))}"
    if isinstance(value, Fraction):
        numerator = group_digits(str(value.numerator))
        return f"$q:{numerator}/{group_digits(str(value.denominator))}"
    if isinstance(value, str):
        return enc_str(value)
    if isinstance(value, list):
        return [enc(item) for item in value]
    if isinstance(value, tuple):
        return {"$tuple": [enc(item) for item in value]}
    if isinstance(value, set | frozenset):
        return {"$set": [enc(item) for item in sorted(value)]}
    if isinstance(value, taste.TasteProfile):
        return {
            "$profile": {
                "basis": value.basis,
                "interests": enc(value.interests),
                "budget_per_person_vnd": enc(value.budget_per_person_vnd),
                "size": enc(value.size),
                "people": enc(value.people),
                "people_answered": enc(value.people_answered),
                "cache_key": enc(value.cache_key),
                "known": value.known,
            }
        }
    if isinstance(value, dict):
        if not all(isinstance(key, str) and not key.startswith("$") for key in value):
            raise TypeError("dict keys must be str not starting with $")
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"no encoding for {type(value).__name__}")


def outcome(function, *args: object) -> dict:
    try:
        value = function(*args)
    except Exception as exc:  # every refusal is data for the oracle
        return {"raised": {"type": type(exc).__name__, "message": enc_str(str(exc))}}
    return {"ok": enc(value)}


def midpoint_of_band(band: dict) -> int | None:
    """`_midpoint_vnd` over a band the real table does not hold.

    The three real bands never reach the open-top branch or a negative bound.
    `taste` reads `budget_band` as a module global at call time, so the
    arithmetic is exercised by swapping that one name for the length of a call.
    """

    synthetic = interests.BudgetBand(
        "synthetic", "synthetic", band["min_vnd"], band["max_vnd"]
    )
    original = taste.budget_band
    taste.budget_band = lambda band_id: synthetic if band_id == "synthetic" else None
    try:
        return taste._midpoint_vnd("synthetic")
    finally:
        taste.budget_band = original


TASTE_CALLS = {
    "covers": (taste.covers, ("tag",)),
    "uncovered": (taste.uncovered, ("tags",)),
    "matches": (taste.matches, ("tag", "place")),
    "_tokens": (taste._tokens, ("values",)),
    "_midpoint_vnd": (taste._midpoint_vnd, ("band_id",)),
    "_midpoint_vnd/band": (midpoint_of_band, ("band",)),
    "profile_for_person": (taste.profile_for_person, ("interests", "band_id")),
    "profile_for_group": (taste.profile_for_group, ("members",)),
}

SCORING_CALLS = {
    "_exact": (scoring._exact, ("value",)),
    "budget_fit": (scoring.budget_fit, ("place", "profile")),
    "taste_fit": (scoring.taste_fit, ("place", "profile")),
    "distance_fit": (scoring.distance_fit, ("place",)),
    "group_size_fit": (scoring.group_size_fit, ("place", "profile")),
    "score_place": (scoring.score_place, ("place", "profile")),
}


def make_case(table: dict, name: str, fns: tuple[str, ...], **inputs: object) -> dict:
    calls: dict[str, dict] = {}
    for fn in fns:
        function, names = table[fn]
        calls[fn] = outcome(function, *(inputs[key] for key in names))
    used = {key for fn in fns for key in table[fn][1]}
    return {
        "name": enc_str(name),
        "in": {key: enc(inputs[key]) for key in sorted(used)},
        "calls": calls,
    }


def taste_case(name: str, *fns: str, **inputs: object) -> dict:
    return make_case(TASTE_CALLS, name, fns, **inputs)


def scoring_case(name: str, *fns: str, **inputs: object) -> dict:
    return make_case(SCORING_CALLS, name, fns or tuple(SCORING_CALLS), **inputs)


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


def fullwidth(text: str) -> str:
    return "".join(
        chr(ord(char) + 0xFEE0) if "!" <= char <= "~" else char for char in text
    )


def casefold_trap(rng: random.Random, word: str) -> str:
    """Swap one letter for a character whose casefold lands on (or near) it."""

    swaps = {
        "s": (LONG_S, SHARP_S),
        "k": (KELVIN_SIGN,),
        "K": (KELVIN_SIGN,),
        "i": (CAPITAL_I_DOT, DOTLESS_I),
        "I": (CAPITAL_I_DOT,),
        "a": (CHEROKEE_SMALL_A,),
    }
    spots = [index for index, char in enumerate(word) if char in swaps]
    if not spots:
        return word + rng.choice(CASEFOLD_TRAPS)
    index = rng.choice(spots)
    return word[:index] + rng.choice(swaps[word[index]]) + word[index + 1 :]


def variant(rng: random.Random, word: str) -> str:
    """`word`, or one of the ways an importer, a seed file or a paste spells it."""

    kind = rng.randrange(14)
    if kind <= 2:
        return word
    if kind == 3:
        return word.upper()
    if kind == 4:
        return word.lower()
    if kind == 5:
        return word.title()
    if kind == 6:
        return word.swapcase()
    if kind == 7:
        return pad(rng) + word + pad(rng)
    if kind == 8:
        return rng.choice(LOOKALIKES) + word
    if kind == 9:
        return unicodedata.normalize(rng.choice(("NFD", "NFKD")), word)
    if kind == 10:
        return casefold_trap(rng, word)
    if kind == 11:
        return fullwidth(word)
    if kind == 12:
        return word.replace(
            " ", rng.choice(("  ", NO_BREAK_SPACE, "")), 1
        ) + rng.choice(("", " trộn", "s"))
    return word[: rng.randrange(max(len(word), 1))]


def pad(rng: random.Random, most: int = 3) -> str:
    return "".join(rng.choice(WHITESPACE) for _ in range(rng.randrange(most + 1)))


def near_miss(rng: random.Random, word: str) -> str:
    kind = rng.randrange(8)
    if kind == 0:
        return word.upper()
    if kind == 1:
        return word.capitalize()
    if kind == 2:
        return rng.choice(WHITESPACE) + word
    if kind == 3:
        return word + rng.choice(LOOKALIKES)
    if kind == 4:
        return word.replace("-", rng.choice(("_", " ", "")), 1)
    if kind == 5:
        return word[: rng.randrange(len(word))]
    if kind == 6:
        return word + COMBINING_ACUTE
    return rng.choice(("", "du-thuyen", "Cafe", "Ăn uống", "quan-an-local"))


def random_tag(rng: random.Random) -> str:
    roll = rng.random()
    if roll < 0.7:
        return rng.choice(VOCAB)
    if roll < 0.9:
        return near_miss(rng, rng.choice(VOCAB))
    return rng.choice(EVIDENCE_WORDS + NOISE_WORDS)


def random_tag_list(rng: random.Random) -> list[str]:
    roll = rng.random()
    if roll < 0.1:
        return []
    if roll < 0.15:
        return list(VOCAB)
    return [random_tag(rng) for _ in range(rng.randrange(1, 10))]


def random_band_id(rng: random.Random) -> str | None:
    roll = rng.random()
    if roll < 0.3:
        return None
    if roll < 0.8:
        return rng.choice(BAND_IDS)
    return rng.choice(STALE_BANDS)


def random_int64(rng: random.Random) -> int:
    roll = rng.random()
    if roll < 0.3:
        return rng.choice(
            (INT64_MIN, INT64_MIN + 1, INT64_MAX, INT64_MAX - 1, -1, 0, 1)
        )
    return rng.randint(INT64_MIN, INT64_MAX)


def random_synthetic_band(rng: random.Random) -> dict:
    roll = rng.random()
    if roll < 0.2:
        low = rng.randrange(0, 1_000_000)
        return {"min_vnd": low, "max_vnd": None}
    if roll < 0.55:
        low = rng.randrange(0, 1_000_000)
        return {"min_vnd": low, "max_vnd": low + rng.randrange(0, 1_000_001)}
    if roll < 0.8:
        return {
            "min_vnd": rng.randint(-1001, 1001),
            "max_vnd": rng.randint(-1001, 1001),
        }
    top = None if rng.random() < 0.2 else random_int64(rng)
    return {"min_vnd": random_int64(rng), "max_vnd": top}


def random_word_list(rng: random.Random) -> list:
    out: list = []
    for _ in range(rng.randrange(6)):
        roll = rng.random()
        if roll < 0.45:
            out.append(variant(rng, rng.choice(EVIDENCE_WORDS)))
        elif roll < 0.95:
            out.append(variant(rng, rng.choice(NOISE_WORDS)))
        else:
            out.append(rng.choice((None, 0, 7, True)))
    return out


def random_place_words(rng: random.Random) -> dict:
    place: dict[str, object] = {"id": "p-mau-fuzz"}
    roll = rng.random()
    if roll < 0.6:
        place["category"] = rng.choice(CATEGORIES)
    elif roll < 0.75:
        place["category"] = near_miss(rng, rng.choice(CATEGORIES))
    elif roll < 0.9:
        place["category"] = rng.choice(EVIDENCE_WORDS + ("",))
    for key in ("kinds", "traits"):
        roll = rng.random()
        if roll < 0.08:
            continue
        place[key] = None if roll < 0.12 else random_word_list(rng)
    return place


# ---------------------------------------------------------------------------
# taste
# ---------------------------------------------------------------------------


def taste_constants() -> dict:
    folds = [
        [point, chr(point).casefold()]
        for point in range(sys.maxunicode + 1)
        if not 0xD800 <= point <= 0xDFFF and chr(point).casefold() != chr(point)
    ]
    return {
        "interest_ids": list(VOCAB),
        "evidence": [
            [tag, list(e.categories), list(e.traits), list(e.kinds)]
            for tag, e in taste.EVIDENCE.items()
        ],
        "basis": list(typing.get_args(taste.Basis)),
        "unknown": enc(taste.UNKNOWN),
        "casefold": [[point, enc_str(folded)] for point, folded in folds],
        "isspace": code_point_ranges(WHITESPACE),
        "unicode": unicodedata.unidata_version,
    }


def place_with(**fields: object) -> dict:
    place: dict[str, object] = {"id": "p-mau", "category": "vui-choi"}
    place.update(fields)
    return {key: value for key, value in place.items() if value is not ...}


def taste_edges() -> list[dict]:
    out: list[dict] = []
    tags = (*VOCAB, "", "Cafe", "CAFE", " cafe", "cafe ", "du-thuyen", "quan-an-local")
    for tag in tags:
        out.append(taste_case(f"covers/{tag!r}", "covers", tag=tag))

    # matches: every word of the evidence table, on the part it is cited for and
    # on the two parts it is not.
    for tag, evidence in taste.EVIDENCE.items():
        for category in (*evidence.categories, *CATEGORIES):
            out.append(
                taste_case(
                    f"matches/{tag}/category/{category}",
                    "matches",
                    tag=tag,
                    place=place_with(category=category, traits=[], kinds=[]),
                )
            )
        for word in (*evidence.categories, *evidence.traits, *evidence.kinds):
            spellings = {
                "exact": word,
                "upper": word.upper(),
                "casefold": word.casefold(),
                "title": word.title(),
                "swapcase": word.swapcase(),
                "nfd": unicodedata.normalize("NFD", word),
                "fullwidth": fullwidth(word),
                "nbsp_pad": NO_BREAK_SPACE + word + IDEOGRAPHIC_SPACE,
                "zwsp_pad": ZERO_WIDTH_SPACE + word,
                "suffix": word + " trộn",
            }
            for how, spelling in spellings.items():
                for part in ("traits", "kinds"):
                    out.append(
                        taste_case(
                            f"matches/{tag}/{part}/{word}/{how}",
                            "matches",
                            "_tokens",
                            tag=tag,
                            place=place_with(**{part: [spelling]}),
                            values=[spelling],
                        )
                    )
    for char in WHITESPACE:
        spelling = char + "Park" + char
        out.append(
            taste_case(
                f"matches/strip/U+{ord(char):04X}",
                "matches",
                "_tokens",
                tag="outdoor",
                place=place_with(kinds=[spelling]),
                values=[spelling],
            )
        )
    traps = {
        "long_s": ("cafe", "Trà " + LONG_S + "ữa"),
        "sharp_s": ("cafe", "Trà " + SHARP_S + "ữa"),
        "kelvin": ("outdoor", "Par" + KELVIN_SIGN),
        "capital_i_dot": ("mon-local", "V" + CAPITAL_I_DOT + "ỆT"),
        "dotless_i": ("mon-local", "V" + DOTLESS_I + "ệt"),
        "plain_capital_i": ("mon-local", "VIỆT"),
        "bullet_for_middle_dot": ("mon-local", "Mì " + BULLET + " bún"),
        "katakana_middle_dot": ("mon-local", "Mì " + KATAKANA_MIDDLE_DOT + " bún"),
        "double_space": ("game", "Theme  park"),
        "nbsp_inside": ("game", "Theme" + NO_BREAK_SPACE + "park"),
        "ligature_st": ("game", "Bowling alley" + LIGATURE_ST),
        "cherokee": ("cafe", "Tr" + CHEROKEE_SMALL_A),
        "category_word_as_kind": ("cafe", "cafe"),
        "trait_word_as_kind": ("outdoor", "Ngoài trời"),
        "substring": ("cafe", "Trà trộn"),
        "empty": ("cafe", ""),
        "only_space": ("cafe", "   "),
    }
    for name, (tag, spelling) in traps.items():
        for part in ("traits", "kinds"):
            out.append(
                taste_case(
                    f"matches/trap/{name}/{part}",
                    "matches",
                    "_tokens",
                    tag=tag,
                    place=place_with(**{part: [spelling]}),
                    values=[spelling],
                )
            )
    everything = place_with(
        category="cafe",
        traits=list(EVIDENCE_WORDS),
        kinds=list(EVIDENCE_WORDS),
    )
    for tag in ("", "CAFE", "Cafe", "du-thuyen", "shopping", "karaoke", *VOCAB):
        out.append(
            taste_case(
                f"matches/everything/{tag!r}", "matches", tag=tag, place=everything
            )
        )
    shapes = {
        "no_keys": {"id": "p-mau"},
        "none_lists": {"id": "p-mau", "category": None, "traits": None, "kinds": None},
        "empty_lists": {"id": "p-mau", "category": "", "traits": [], "kinds": []},
        "non_str_elements": {"id": "p-mau", "traits": [None, 7, True], "kinds": [0]},
        "non_str_then_hit": {"id": "p-mau", "kinds": [None, "park"]},
    }
    for name, place in shapes.items():
        for tag in ("outdoor", "cafe"):
            out.append(
                taste_case(
                    f"matches/shape/{name}/{tag}",
                    "matches",
                    "_tokens",
                    tag=tag,
                    place=place,
                    values=place.get("kinds"),
                )
            )

    lists = {
        "empty": [],
        "all": list(VOCAB),
        "all_reversed": list(reversed(VOCAB)),
        "shopping": ["shopping"],
        "karaoke_then_shopping": ["karaoke", "shopping"],
        "duplicates": ["karaoke", "karaoke", "shopping", "shopping"],
        "covered_only": ["cafe", "outdoor"],
        "case": ["Shopping", "KARAOKE", " shopping"],
        "unknown": ["du-thuyen", ""],
    }
    for name, value in lists.items():
        out.append(taste_case(f"uncovered/{name}", "uncovered", tags=value))

    for band_id in (None, *BAND_IDS, *STALE_BANDS):
        out.append(
            taste_case(f"midpoint/{band_id!r}", "_midpoint_vnd", band_id=band_id)
        )
    synthetic = (
        (0, None),
        (250_000, None),
        (0, 100_000),
        (100_000, 250_000),
        (1, 2),
        (0, 1),
        (-3, 0),
        (-1, -2),
        (-1, 0),
        (-1001, 0),
        (5, -8),
        (INT64_MAX, INT64_MAX),
        (INT64_MIN, INT64_MIN),
        (INT64_MIN, INT64_MAX),
        (INT64_MAX, INT64_MAX - 2),
        (INT64_MIN, None),
        (INT64_MAX, None),
    )
    for low, high in synthetic:
        out.append(
            taste_case(
                f"midpoint/band/{low}/{high}",
                "_midpoint_vnd/band",
                band={"min_vnd": low, "max_vnd": high},
            )
        )

    person_interests = {
        "none": [],
        "cafe": ["cafe"],
        "unknown_only": ["du-thuyen"],
        "empty_word": [""],
        "duplicates_reversed": ["game", "cafe", "cafe", "an-uong"],
        "all": list(VOCAB),
    }
    for name, chosen in person_interests.items():
        for band_id in (None, *BAND_IDS, "sang-chanh", ""):
            out.append(
                taste_case(
                    f"person/{name}/{band_id!r}",
                    "profile_for_person",
                    interests=chosen,
                    band_id=band_id,
                )
            )

    groups = {
        "nobody": [],
        "one_silent": [([], None)],
        "one_stale_band": [([], "sang-chanh")],
        "one_empty_band": [([], "")],
        "unknown_word_only": [(["du-thuyen"], None)],
        "floor_of_three": [
            ([], "tiet-kiem"),
            (["cafe"], "tiet-kiem"),
            ([], "vua-phai"),
        ],
        "two_bands_exact": [([], "tiet-kiem"), ([], "vua-phai")],
        "silent_members_not_zero": [
            (["cafe"], "thoai-mai"),
            ([], None),
            ([], None),
        ],
        "union_in_vocabulary_order": [
            (["game", "cafe"], None),
            (["karaoke", "an-uong"], "vua-phai"),
            (["cafe"], "sang-chanh"),
        ],
        "six": [(["outdoor"], "vua-phai")] * 6,
    }
    for name, members in groups.items():
        out.append(taste_case(f"group/{name}", "profile_for_group", members=members))
    return out


def taste_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 1)
    out: list[dict] = []
    for index in range(FUZZ_CASES):
        place = random_place_words(rng)
        members = [
            (random_tag_list(rng), random_band_id(rng))
            for _ in range(rng.choice((0, 1, 1, 2, 3, 4, 6)))
        ]
        out.append(
            taste_case(
                f"fuzz/{index}",
                *TASTE_CALLS,
                tag=random_tag(rng),
                place=place,
                values=place.get(rng.choice(("kinds", "traits"))),
                tags=random_tag_list(rng),
                interests=random_tag_list(rng),
                band_id=random_band_id(rng),
                members=members,
                band=random_synthetic_band(rng),
            )
        )
    return out


# ---------------------------------------------------------------------------
# scoring
# ---------------------------------------------------------------------------

NHOM = taste.TasteProfile(
    basis="nhom",
    interests=("an-uong", "cafe", "nightlife", "mon-local", "outdoor"),
    budget_per_person_vnd=250_000,
    size=6,
    people=6,
    people_answered=6,
)
TOI = taste.TasteProfile(
    basis="ca-nhan",
    interests=("an-uong", "cafe"),
    budget_per_person_vnd=175_000,
    people=1,
    people_answered=1,
)
BASE_PLACE = {
    "id": "p-mau-1",
    "category": "quan-an-local",
    "kinds": ["BBQ", "Local"],
    "traits": ["Chill", "Ngoài trời"],
    "price_min_vnd": 200_000,
    "price_max_vnd": 250_000,
    "distance_km": 1.2,
    "travel_minutes": 25,
    "group_fit": {"min_people": 4, "max_people": 10, "relation": "Bạn bè"},
}


def scoring_constants() -> dict:
    return {
        "weights": [
            scoring.WEIGHT_BUDGET,
            scoring.WEIGHT_TASTE,
            scoring.WEIGHT_DISTANCE,
            scoring.WEIGHT_GROUP_SIZE,
        ],
        "far_km": enc(scoring.FAR_KM),
        "labels": [[key, label] for key, label in scoring._LABELS.items()],
    }


def placed(**changes: object) -> dict:
    place = dict(BASE_PLACE)
    for key, value in changes.items():
        if value is ...:
            place.pop(key, None)
        else:
            place[key] = value
    return place


def profile(**changes: object) -> taste.TasteProfile:
    fields = {
        "basis": NHOM.basis,
        "interests": NHOM.interests,
        "budget_per_person_vnd": NHOM.budget_per_person_vnd,
        "size": NHOM.size,
        "people": NHOM.people,
        "people_answered": NHOM.people_answered,
    }
    fields.update(changes)
    return taste.TasteProfile(**fields)


def scoring_edges() -> list[dict]:
    out: list[dict] = []

    def add(
        name: str, place: dict, who: taste.TasteProfile, value: float = 5.0
    ) -> None:
        out.append(scoring_case(name, place=place, profile=who, value=value))

    add("base/nhom", BASE_PLACE, NHOM)
    add("base/toi", BASE_PLACE, TOI)
    add("base/unknown", BASE_PLACE, taste.UNKNOWN)
    add("cafe/toi", placed(category="cafe", distance_km=6.1, group_fit=None), TOI)
    add("bare/unknown", {"id": "p-mau"}, taste.UNKNOWN)
    add("bare/nhom", {"id": "p-mau"}, NHOM)
    add(
        "unpriced_unknown_budget",
        placed(price_min_vnd=None),
        profile(budget_per_person_vnd=None),
    )

    budget = NHOM.budget_per_person_vnd
    for name, midpoint in {
        "at": budget,
        "one_over": budget + 1,
        "one_under": budget - 1,
        "double": 2 * budget,
        "double_less_one": 2 * budget - 1,
        "double_plus_one": 2 * budget + 1,
        "ruinous": 10 * budget,
    }.items():
        for spread in (0, 1):
            add(
                f"budget/{name}/spread{spread}",
                placed(
                    price_min_vnd=midpoint - spread,
                    price_max_vnd=midpoint + spread + spread,
                ),
                NHOM,
            )
    for low, high in (
        (None, 250_000),
        (250_000, None),
        (None, None),
        (1, 2),
        (-3, 0),
        (-1, -2),
        (-1001, 0),
        (-999, 0),
        (INT64_MAX, INT64_MAX),
        (INT64_MIN, INT64_MAX),
        (INT64_MIN, INT64_MIN),
        (0, 100_000),
        (100_000, 250_000),
        (250_000, 500_000),
        (500_000, 500_000),
        (500_000, 500_001),
    ):
        for band_midpoint in (None, *MIDPOINTS):
            add(
                f"budget/prices/{low}/{high}/budget{band_midpoint}",
                placed(price_min_vnd=low, price_max_vnd=high),
                profile(budget_per_person_vnd=band_midpoint),
            )
    for who_budget in (0, -1, -250_000, 1, INT64_MAX, INT64_MIN):
        for low, high in (
            (0, 0),
            (0, 1),
            (1, 1),
            (250_000, 250_000),
            (INT64_MAX, INT64_MAX),
        ):
            add(
                f"budget/odd_budget/{who_budget}/{low}/{high}",
                placed(price_min_vnd=low, price_max_vnd=high),
                profile(budget_per_person_vnd=who_budget),
            )
    add(
        "budget/zero_unpriced",
        placed(price_min_vnd=None, price_max_vnd=None),
        profile(budget_per_person_vnd=0),
    )

    far = scoring.FAR_KM
    distances = {
        "none": None,
        "zero": 0.0,
        "negative_zero": -0.0,
        "decimal_1_2": 1.2,
        "decimal_0_1": 0.1,
        "sum_0_3": 0.1 + 0.2,
        "half": 2.5,
        "far": far,
        "far_prev": math.nextafter(far, 0.0),
        "far_next": math.nextafter(far, math.inf),
        "far_minus_1e-9": far - 1e-9,
        "far_plus_1e-9": far + 1e-9,
        "past": 40.0,
        "negative": -1.0,
        "exp_small": 1e-05,
        "exp_edge_fixed": 0.0001,
        "fixed_edge_16": float(1234_5678_9012_3456),
        "exp_edge_17": 1e16,
        "exp_22": 1e22,
        "tiny": 5e-324,
        "huge": sys.float_info.max,
        "negative_huge": -sys.float_info.max,
        "full_digits": 1 / 3,
        "inf": math.inf,
        "negative_inf": -math.inf,
        "nan": math.nan,
    }
    for name, distance in distances.items():
        exact_input = 0.0 if distance is None else distance
        for minutes in (None, 0, 17):
            add(
                f"distance/{name}/minutes{minutes}",
                placed(distance_km=distance, travel_minutes=minutes),
                NHOM,
                exact_input,
            )

    for name, fit in {
        "none": None,
        "empty": {},
        "absent": ...,
        "wide": {"min_people": 1, "max_people": 20, "relation": "x"},
        "tight": {"min_people": 6, "max_people": 6},
        "reversed": {"min_people": 10, "max_people": 4},
        "extreme": {"min_people": INT64_MIN, "max_people": INT64_MAX},
    }.items():
        for size in (None, 0, -1, 1, 4, 5, 6, 10, 11, INT64_MAX, INT64_MIN):
            add(f"size/{name}/{size}", placed(group_fit=fit), profile(size=size))

    for name, chosen in {
        "none": (),
        "one_hit": ("an-uong",),
        "no_hit": ("karaoke", "shopping"),
        "vocabulary_reversed": tuple(reversed(VOCAB)),
        "duplicates": ("an-uong", "an-uong", "karaoke"),
        "unknown": ("du-thuyen", "an-uong"),
        "all": VOCAB,
    }.items():
        add(f"taste/{name}", BASE_PLACE, profile(interests=chosen))
        add(
            f"taste/{name}/no_budget",
            BASE_PLACE,
            profile(interests=chosen, budget_per_person_vnd=None),
        )

    # Who is known: the badge needs something the person said about themself.
    add(
        "known/size_only", BASE_PLACE, profile(interests=(), budget_per_person_vnd=None)
    )
    add(
        "known/budget_only_nothing_on_place",
        {"id": "p-mau", "category": "vui-choi"},
        profile(interests=(), size=None),
    )
    add(
        "known/budget_only_distance_on_place",
        {"id": "p-mau", "category": "vui-choi", "distance_km": 0.0},
        profile(interests=(), size=None),
    )
    add(
        "known/huge_score",
        placed(price_min_vnd=INT64_MAX, price_max_vnd=INT64_MAX),
        profile(budget_per_person_vnd=-1),
    )
    add("known/negative_distance_score", placed(distance_km=-1e300), NHOM)
    return out


def scoring_grid() -> list[dict]:
    """Every combination of a small table of terms, and a check that ties exist.

    The tie check orchestrates the real fit functions; it asserts the grid
    contains exact halves on both sides of even, not what they round to.
    """

    out: list[dict] = []
    ties = {"even": 0, "odd": 0}
    prices = ((None, None), (100_000, 100_000), (150_000, 150_000), (200_000, 200_000))
    tastes = (
        (),
        ("cafe",),
        ("cafe", "karaoke"),
        ("cafe", "karaoke", "game"),
        ("karaoke",),
    )
    distances = (None, 0.0, 1.25, 2.5, 5.0)
    fits = (
        None,
        {"min_people": 1, "max_people": 10},
        {"min_people": 7, "max_people": 10},
    )
    for low, high in prices:
        for chosen in tastes:
            for distance in distances:
                for fit in fits:
                    place = {
                        "id": "p-mau-grid",
                        "category": "cafe",
                        "kinds": [],
                        "traits": [],
                        "price_min_vnd": low,
                        "price_max_vnd": high,
                        "distance_km": distance,
                        "travel_minutes": None,
                        "group_fit": fit,
                    }
                    who = profile(interests=chosen, budget_per_person_vnd=100_000)
                    name = f"grid/{low}/{len(chosen)}/{distance}/{fit and fit['min_people']}"
                    out.append(
                        scoring_case(
                            name, place=place, profile=who, value=distance or 0.0
                        )
                    )
                    terms = [
                        (scoring.WEIGHT_BUDGET, scoring.budget_fit(place, who)),
                        (scoring.WEIGHT_TASTE, scoring.taste_fit(place, who)[0]),
                        (scoring.WEIGHT_DISTANCE, scoring.distance_fit(place)),
                        (scoring.WEIGHT_GROUP_SIZE, scoring.group_size_fit(place, who)),
                    ]
                    known = [(w, v) for w, v in terms if v is not None]
                    if known:
                        total = sum(w for w, _ in known)
                        value = sum(w * v for w, v in known) * 100 / Fraction(total)
                        if value.denominator == 2:
                            ties[
                                "even" if (value.numerator // 2) % 2 == 0 else "odd"
                            ] += 1
    if ties["even"] < 3 or ties["odd"] < 3:
        raise AssertionError(f"grid holds too few exact halves: {ties}")
    return out


def random_double(rng: random.Random) -> float:
    while True:
        value = struct.unpack("<d", rng.getrandbits(64).to_bytes(8, "little"))[0]
        if math.isfinite(value):
            return value


def random_distance(rng: random.Random) -> float:
    far = scoring.FAR_KM
    roll = rng.random()
    if roll < 0.4:
        return round(rng.uniform(0.0, 10.0), rng.choice((0, 1, 1, 2, 3)))
    if roll < 0.55:
        return rng.choice(
            (
                far,
                math.nextafter(far, 0.0),
                math.nextafter(far, math.inf),
                far - 1e-9,
                far + 1e-9,
                0.0,
                -0.0,
                2.5,
                1.25,
                3.75,
            )
        )
    if roll < 0.7:
        return rng.random() * 10.0
    if roll < 0.82:
        return random_double(rng)
    if roll < 0.9:
        return -round(rng.uniform(0.0, 10.0), rng.choice((0, 1, 2)))
    if roll < 0.97:
        return rng.choice((1.0, 1.5, 9.0)) * 10.0 ** rng.randint(-12, 24)
    return rng.choice((math.inf, -math.inf, math.nan))


def random_profile(rng: random.Random) -> taste.TasteProfile:
    roll = rng.random()
    if roll < 0.2:
        chosen: tuple[str, ...] = ()
    elif roll < 0.8:
        chosen = tuple(tag for tag in VOCAB if rng.random() < 0.4)
    elif roll < 0.9:
        chosen = tuple(rng.sample(VOCAB, rng.randrange(1, len(VOCAB) + 1)))
    else:
        chosen = tuple(random_tag(rng) for _ in range(rng.randrange(1, 12)))

    roll = rng.random()
    if roll < 0.2:
        budget = None
    elif roll < 0.5:
        budget = rng.choice(MIDPOINTS + (112_500, 91_666, 200_000, 212_500))
    elif roll < 0.75:
        budget = rng.randrange(1, 1001) * 1000
    elif roll < 0.85:
        budget = rng.randrange(1, 10_000_000)
    elif roll < 0.9:
        budget = 0
    elif roll < 0.95:
        budget = -rng.randrange(1, 1_000_000)
    else:
        budget = random_int64(rng)

    roll = rng.random()
    if roll < 0.25:
        size = None
    elif roll < 0.85:
        size = rng.randrange(1, 21)
    elif roll < 0.9:
        size = 0
    elif roll < 0.95:
        size = -rng.randrange(1, 10)
    else:
        size = random_int64(rng)

    people = rng.randrange(0, 11)
    return taste.TasteProfile(
        basis=rng.choice(typing.get_args(taste.Basis)),
        interests=chosen,
        budget_per_person_vnd=budget,
        size=size,
        people=people,
        people_answered=rng.randrange(0, people + 1),
    )


def random_prices(rng: random.Random, budget: int | None) -> tuple:
    roll = rng.random()
    if roll < 0.15:
        return None, None
    if roll < 0.2:
        return None, rng.randrange(0, 500_001)
    if roll < 0.25:
        return rng.randrange(0, 500_001), None
    if roll < 0.55 and budget is not None and -(2**61) < budget < 2**61:
        target = rng.choice(
            (
                budget,
                budget + 1,
                budget - 1,
                2 * budget,
                2 * budget + 1,
                2 * budget - 1,
                budget // 2,
            )
        )
        spread = rng.randrange(0, 50_001)
        return target - spread, target + spread + rng.randrange(0, 2)
    if roll < 0.75:
        low, high = sorted(
            (rng.randrange(0, 1001) * 1000, rng.randrange(0, 1001) * 1000)
        )
        return low, high
    if roll < 0.85:
        low, high = sorted((rng.randrange(0, 1_000_000), rng.randrange(0, 1_000_000)))
        return low, high
    if roll < 0.92:
        return -rng.randrange(0, 5000), rng.randrange(-5000, 5000)
    return random_int64(rng), random_int64(rng)


def random_scoring_place(rng: random.Random, who: taste.TasteProfile) -> dict:
    place = random_place_words(rng)
    low, high = random_prices(rng, who.budget_per_person_vnd)
    place["price_min_vnd"] = low
    place["price_max_vnd"] = high
    place["distance_km"] = None if rng.random() < 0.2 else random_distance(rng)
    roll = rng.random()
    if roll < 0.3:
        place["travel_minutes"] = None
    elif roll < 0.9:
        place["travel_minutes"] = rng.randrange(0, 121)
    else:
        place["travel_minutes"] = random_int64(rng)
    roll = rng.random()
    if roll < 0.3:
        place["group_fit"] = None
    elif roll < 0.35:
        place["group_fit"] = {}
    elif roll < 0.5 and who.size is not None:
        edge = who.size + rng.choice((-1, 0, 1)) if abs(who.size) < 2**62 else who.size
        low_people, high_people = rng.choice(
            ((edge, edge + 5), (edge - 5, edge), (edge, edge))
        )
        place["group_fit"] = {"min_people": low_people, "max_people": high_people}
    elif roll < 0.95:
        low_people = rng.randrange(1, 11)
        place["group_fit"] = {
            "min_people": low_people,
            "max_people": low_people + rng.randrange(-2, 11),
            "relation": "Bạn bè",
        }
    else:
        place["group_fit"] = {
            "min_people": random_int64(rng),
            "max_people": random_int64(rng),
        }
    if rng.random() < 0.08:
        place.pop(
            rng.choice(
                (
                    "price_min_vnd",
                    "price_max_vnd",
                    "distance_km",
                    "travel_minutes",
                    "group_fit",
                )
            )
        )
    return place


def scoring_fuzz() -> list[dict]:
    rng = random.Random(SEED * 1000 + 2)
    out: list[dict] = []
    for index in range(FUZZ_CASES):
        who = random_profile(rng)
        place = random_scoring_place(rng, who)
        value = place.get("distance_km")
        if value is None or rng.random() < 0.3:
            value = random_distance(rng)
        out.append(scoring_case(f"fuzz/{index}", place=place, profile=who, value=value))
    return out


# ---------------------------------------------------------------------------
# Modes
# ---------------------------------------------------------------------------

MODULES = {
    "taste": ("app.places.taste", taste_constants, taste_edges, taste_fuzz),
    "scoring": ("app.places.scoring", scoring_constants, scoring_edges, scoring_fuzz),
}


def modes() -> dict[str, tuple[str, object]]:
    table: dict[str, tuple[str, object]] = {}
    for package in MODULES:
        table[package] = (package, None)
        if package == "scoring":
            table["scoring-grid"] = (package, "grid")
        for shard in range(FUZZ_SHARDS[package]):
            table[f"{package}-fuzz-{shard}"] = (package, shard)
    return table


def render(mode: str) -> dict:
    package, part = modes()[mode]
    module, constants, edges, fuzz = MODULES[package]
    document: dict[str, object] = {
        "generator": "scripts/render_places_taste_goldens.py",
        "module": module,
        "mode": mode,
        "python": platform.python_version(),
        "seed": SEED,
    }
    if part is None:
        document["constants"] = constants()
        document["cases"] = edges()
    elif part == "grid":
        document["cases"] = scoring_grid()
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
    if len(rendered.encode()) >= MAX_BYTES:
        print(f"{argv[0]} renders {len(rendered)} bytes, over one MiB", file=sys.stderr)
        return 1
    for number, line in enumerate(rendered.splitlines(), 1):
        if any(rule.search(line) for rule in GUARD_RULES):
            print(f"line {number} would trip the repository guard", file=sys.stderr)
            return 1
    sys.stdout.write(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
