#!/usr/bin/env python3
"""Oracle for the Go port of the place geometry modules (ADR-0029 §2.4).

`app.places.areas` (`haversine_km`, `nearest_area`, `area_of_address`),
`app.places.meeting` and `app.places.social_map` gain Go twins under
`services/core/internal/domain/{areas,meeting,socialmap}`. Go must answer
exactly as Python answers, so this script calls the real functions inside the
pinned API image and records what each returns or raises; the Go tests replay
every case.

Each invocation renders one file, chosen by MODE:

    docker run --rm -i --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d - MODE < scripts/render_places_geo_goldens.py \\
      > services/core/internal/domain/PKG/testdata/python_MODE.json

`--list` prints every MODE and its target path, tab separated, so the whole set
regenerates with:

    IMAGE=mobile-parity-api:7bf58e3d
    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_places_geo_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_places_geo_goldens.py > "$path"
    done

Modes: `geo`, `meeting` and `socialmap` hold constants and the named edge
cases; `<module>-fuzz-<k>` is shard k of that module's seeded fuzz. Shards are
contiguous slices of one fixed case list, so one shard rendered alone gives the
same bytes.

## Floats are compared bit for bit

`math.sin`, `math.cos`, `math.asin` and float `**` are glibc's, and on x86-64
glibc picks an FMA-compiled variant when the CPU has FMA and AVX2. The Go port
reproduces those variants, so every file records the host it was rendered on
and the Go tests refuse a file rendered elsewhere. The `libm` rows exercise
every branch and every table interval of those routines directly.

## Encoding

A float is its `repr` with `_` after every fourth digit of a digit run
(`0.12345` is written `0.1234_5`), so no run of digits can trip the
repository guard's number or phone rules; Go deletes the underscores and
parses. Row-shaped values are one string: `|` separates fields, `;` separates
list items, and a field that is exactly `~` is None. The fuzz alphabets contain
no `|` and no field equal to `~`. A call's outcome is `ok:<value>` or
`raise:<exception type>:<message>`.

Non-ASCII characters whose identity matters (spaces, digits of other scripts,
combining marks) are spelled with `chr()` below: the formatter turns `\\u`
escapes into literal characters, and a literal U+00A0 cannot be reviewed.
"""

from __future__ import annotations

import json
import math
import platform
import random
import re
import struct
import sys
import unicodedata

from app.places import areas as areas_module
from app.places.areas import (
    AREAS,
    MAX_AREA_RADIUS_KM,
    area_of_address,
    area_summary,
    haversine_km,
    nearest_area,
)
from app.places.catalog import PLACES
from app.places.meeting import (
    MAX_ORIGIN_AREAS,
    MIN_ORIGIN_AREAS,
    rank_meeting_points,
)
from app.places.social_map import (
    PRIVATE_CHECKIN_FIELDS,
    heatmap_rows,
    trending_layer,
    unknown_area_count,
    visited_layer,
)

CORE = "services/core/internal/domain/"
TARGETS = {"geo": "areas", "meeting": "meeting", "socialmap": "socialmap"}
SHARDS = {"geo": 2, "meeting": 3, "socialmap": 3}
FUZZ_CASES = 2400

_GROUPS = re.compile(r"(\d{4})(?=\d)")
_GUARD_RUN = re.compile(r"\d(?:[ ().-]*\d){4}")


# -- encoding ----------------------------------------------------------------


def enc_float(value: float) -> str:
    if type(value) is not float:
        raise TypeError(f"expected a float, got {type(value).__name__}")
    return _GROUPS.sub(r"\1_", repr(value))


def enc_int(value: int) -> str:
    if type(value) is not int:
        raise TypeError(f"expected an int, got {type(value).__name__}")
    return _GROUPS.sub(r"\1_", repr(value))


def enc_str(value: str) -> str:
    if type(value) is not str:
        raise TypeError(f"expected a str, got {type(value).__name__}")
    if "|" in value or value == "~":
        raise ValueError(f"unencodable string {value!r}")
    return value


def opt(value, encode) -> str:
    return "~" if value is None else encode(value)


def row(*fields: str) -> str:
    return "|".join(fields)


def raised(exc: BaseException) -> str:
    return f"raise:{type(exc).__name__}:{exc}"


def scalar_outcome(call, encode) -> str:
    try:
        value = call()
    except Exception as exc:  # noqa: BLE001 - the oracle records every raise
        return raised(exc)
    return "ok:" + encode(value)


def host() -> dict:
    flags: set[str] = set()
    try:
        with open("/proc/cpuinfo", encoding="utf-8") as handle:
            for line in handle:
                if line.startswith("flags"):
                    flags = set(line.split(":", 1)[1].split())
                    break
    except OSError:
        pass
    return {
        "machine": platform.machine(),
        "libc": " ".join(platform.libc_ver()),
        "python": platform.python_version(),
        "fma_avx2": {"fma", "avx2"} <= flags,
    }


def from_bits(word: int) -> float:
    return struct.unpack(">d", word.to_bytes(8, "big"))[0]


# -- areas -------------------------------------------------------------------

LIBM = {
    "sin": math.sin,
    "cos": math.cos,
    "asin": math.asin,
    "pow2": lambda value: value**2,
    "sqrt": math.sqrt,
    "radians": math.radians,
}

SPECIALS = [0.0, -0.0, 5e-324, -5e-324, math.inf, -math.inf, math.nan, 1.0, -1.0]


def libm_inputs() -> dict[str, list[float]]:
    rng = random.Random(29)
    out: dict[str, list[float]] = {name: list(SPECIALS) for name in LIBM}

    trig = []
    for index in range(111):
        for frac in (-0.4, 0.0, 0.4):
            trig.append((index + frac) / 128)
    for _ in range(200):
        trig.append(rng.uniform(0.855, 2.43))
    for quadrant in range(1, 9):
        for _ in range(12):
            trig.append(quadrant * math.pi / 2 + rng.uniform(-0.8, 0.8))
    for exponent in range(27, 1024, 7):
        trig.append(math.ldexp(1 + rng.random(), exponent))
    for edge in (2**-27, 2**-26, 0.126, 0.855469, 2.426265, 1.0541435e8):
        trig.extend([edge, math.nextafter(edge, 0), math.nextafter(edge, 9e9)])
    for _ in range(150):
        trig.append(math.ldexp(1 + rng.random(), rng.randint(-60, 30)))
    signed = [value if rng.random() < 0.5 else -value for value in trig]
    out["sin"] += signed
    out["cos"] += [-value for value in signed]

    asin = []
    for index in range(32):
        asin.append(from_bits(((0x3FC00000 + (index << 15) + 0x4000) << 32) | 29))
    for index in range(64):
        asin.append(from_bits(((0x3FD00000 + (index << 14) + 0x2000) << 32) | 29))
    for index in range(120):
        asin.append(from_bits(((0x3FE00000 + (index << 13) + 0x1000) << 32) | 29))
    for index in range(128):
        exponent = 0x3F8 if index >> 6 else 0x3F7
        for shift in (0, 10, 40):
            high = ((exponent - 2 * (shift // 2)) << 20) | ((index & 63) << 14) | 0x2000
            z = from_bits((high << 32) | 29)
            asin.append(1.0 - 2.0 * z)
    for edge in (0.125, 0.25, 0.5, 0.75, 0.921875, 0.953125, 0.96875, 1.0, 2**-26):
        asin.extend([edge, math.nextafter(edge, 0), math.nextafter(edge, 2)])
    for _ in range(300):
        asin.append(rng.uniform(0, 1))
    for _ in range(60):
        asin.append(1.0 - rng.randint(1, 1 << rng.randint(1, 40)) * 2.0**-53)
    out["asin"] += [value if rng.random() < 0.5 else -value for value in asin]

    powers = []
    offset = 0x3FE6955500000000
    for index in range(128):
        for exponent in (0, -1, -9, -300, -520, -537, -600, 200, 511, 512):
            word = offset + (index << 45) + (1 << 44)
            word = (word & ~(0xFFF << 52)) | ((0x3FE + exponent) << 52)
            if 0 < (word >> 52) < 0x7FF:
                powers.append(from_bits(word))
    powers += [2.0 ** (-step / 256) for step in range(1, 257)]
    powers += [2.0 ** (step / 256) for step in range(1, 129)]
    powers += [
        math.ldexp(1 + rng.random(), -rng.randint(1050, 1074)) for _ in range(20)
    ]
    powers += [2.0**-537, 2.0**-538, 2.0**-540, 2.0**512, 2.0**511.9]
    out["pow2"] += [value if rng.random() < 0.5 else -value for value in powers]

    out["sqrt"] += [-1.0, 2.0, 1e-300, 0.25, math.nextafter(1.0, 2)]
    out["radians"] += [180.0, 90.0, 1e308, -1e308, 106.7009, 10.7769]
    return out


def libm_rows() -> list[str]:
    rows = []
    for name, values in libm_inputs().items():
        for value in values:
            outcome = scalar_outcome(lambda v=value: LIBM[name](v), enc_float)
            rows.append(row(name, enc_float(value), outcome))
    return rows


def haversine_row(args: tuple[float, float, float, float]) -> str:
    outcome = scalar_outcome(lambda: haversine_km(*args), enc_float)
    return row(";".join(enc_float(value) for value in args), outcome)


def nearest_row(lat: float, lng: float) -> str:
    def call():
        area = nearest_area(lat, lng)
        return None if area is None else area["id"]

    outcome = scalar_outcome(call, lambda area_id: opt(area_id, enc_str))
    return row(enc_float(lat), enc_float(lng), outcome)


def haversine_edges() -> list[str]:
    cases = [
        (10.77, 106.70, 10.77, 106.70),
        (0.0, 0.0, 0.0, 1.0),
        (11.94, 108.43, 10.77, 106.70),
        (10.77, 106.70, 11.94, 108.43),
        (11.9433, 108.4370, 10.7769, 106.7009),
        (90.0, 0.0, -90.0, 0.0),
        (0.0, 0.0, 0.0, 180.0),
        (0.0, 0.0, 0.0, -180.0),
        (0.0, 90.0, 0.0, -90.0),
        (10.0, 106.7, -10.0, 106.7),
        (-0.0, -0.0, 0.0, 0.0),
        (1e308, 0.0, -1e308, 0.0),
        (math.inf, 0.0, 0.0, 0.0),
        (0.0, 0.0, 0.0, -math.inf),
        (math.nan, 0.0, 0.0, 0.0),
        (5e-324, 0.0, 0.0, 0.0),
        (0.0, 5e-324, 0.0, -5e-324),
        (1e-160, 1e-160, 0.0, 0.0),
        (360.0, 720.0, -360.0, -720.0),
        (1e9, 1e9, -1e9, 2e9),
    ]
    for area in AREAS:
        lat, lng = area["lat"], area["lng"]
        cases.append((lat, lng, lat, lng))
        cases.append((lat, lng, -lat, lng - 180.0))
        cases.append((lat, lng, -lat, lng + 180.0))
        cases.append((-lat, lng - 180.0, lat, lng))
        cases.append((lat + 1e-9, lng, lat, lng))
        cases.append((lat, lng - 1e-9, lat, lng))
    return [haversine_row(case) for case in cases]


def radius_crossings(rng: random.Random) -> tuple[list[str], list[str]]:
    """Points on either side of the 25 km radius, and points exactly on it."""

    near_rows: list[str] = []
    exact_rows: list[str] = []
    for area in sorted(AREAS, key=lambda item: item["id"]):
        lat0, lng0 = area["lat"], area["lng"]
        for step in range(72):
            theta = math.radians(step * 5)
            dlat, dlng = math.cos(theta), math.sin(theta)

            def km(t, dlat=dlat, dlng=dlng, lat0=lat0, lng0=lng0):
                return haversine_km(lat0 + t * dlat, lng0 + t * dlng, lat0, lng0)

            low, high = 0.0, 1.0
            while True:
                mid = (low + high) / 2
                if mid in (low, high):
                    break
                if km(mid) < MAX_AREA_RADIUS_KM:
                    low = mid
                else:
                    high = mid
            lat, lng = lat0 + low * dlat, lng0 + low * dlng
            if step % 9 == 0:
                for t in (low, high, low - 1e-9, high + 1e-9):
                    near_rows.append(nearest_row(lat0 + t * dlat, lng0 + t * dlng))
            probe = lat
            for _ in range(160):
                probe = math.nextafter(probe, -1e9)
            for _ in range(320):
                if haversine_km(probe, lng, lat0, lng0) == MAX_AREA_RADIUS_KM:
                    exact_rows.append(nearest_row(probe, lng))
                probe = math.nextafter(probe, 1e9)
    rng.shuffle(exact_rows)
    return near_rows, exact_rows


def distance_ties() -> list[str]:
    """Points exactly as far from two areas, where the smaller id must win."""

    hcm = sorted((a for a in AREAS if a["id"].startswith("hcm")), key=lambda a: a["id"])
    found: list[str] = []
    for i, first in enumerate(hcm):
        for second in hcm[i + 1 :]:
            du = second["lat"] - first["lat"]
            dv = second["lng"] - first["lng"]
            norm = math.hypot(du, dv)
            ux, uy = du / norm, dv / norm
            mid_lat = (first["lat"] + second["lat"]) / 2
            mid_lng = (first["lng"] + second["lng"]) / 2
            for offset in range(-150, 151):
                base_lat = mid_lat - uy * offset * 0.002
                base_lng = mid_lng + ux * offset * 0.002

                def gap(s, base_lat=base_lat, base_lng=base_lng, ux=ux, uy=uy):
                    lat, lng = base_lat + s * ux, base_lng + s * uy
                    return haversine_km(
                        lat, lng, first["lat"], first["lng"]
                    ) - haversine_km(lat, lng, second["lat"], second["lng"])

                low, high = -0.05, 0.05
                if gap(low) >= 0 or gap(high) <= 0:
                    continue
                while True:
                    mid = (low + high) / 2
                    if mid in (low, high):
                        break
                    if gap(mid) < 0:
                        low = mid
                    else:
                        high = mid
                lat, lng = base_lat + low * ux, base_lng + low * uy
                probe = lat
                for _ in range(200):
                    probe = math.nextafter(probe, -1e9)
                for _ in range(400):
                    to_first = haversine_km(probe, lng, first["lat"], first["lng"])
                    to_second = haversine_km(probe, lng, second["lat"], second["lng"])
                    if to_first == to_second and to_first < MAX_AREA_RADIUS_KM:
                        others = [
                            haversine_km(probe, lng, a["lat"], a["lng"])
                            for a in AREAS
                            if a["id"] not in (first["id"], second["id"])
                        ]
                        if min(others) > to_first:
                            found.append(nearest_row(probe, lng))
                    probe = math.nextafter(probe, 1e9)
    return found


NBSP = chr(0xA0)
FILE_SEP = chr(0x1C)
UNIT_SEP = chr(0x1F)
NEL = chr(0x85)
LINE_SEP = chr(0x2028)
IDEO_SPACE = chr(0x3000)
ZWSP = chr(0x200B)
MONGOLIAN_VS = chr(0x180E)
BOM = chr(0xFEFF)
ARABIC_ONE = chr(0x661)
DEVANAGARI_FOUR = chr(0x96A)
FULLWIDTH_THREE = chr(0xFF13)
MATH_BOLD_ONE = chr(0x1D7CF)
SUPERSCRIPT_ONE = chr(0xB9)
SUPERSCRIPT_TWO = chr(0xB2)
CIRCLED_ONE = chr(0x2460)
ROMAN_FOUR = chr(0x2163)
COMBINING_ACUTE = chr(0x301)
COMBINING_DOT_BELOW = chr(0x323)
COMBINING_CIRCUMFLEX = chr(0x302)
DOTTED_CAPITAL_I = chr(0x130)
CAPITAL_SIGMA = chr(0x3A3)
QUAN = "Qu" + chr(0x1EAD) + "n"
QUAN_UPPER = "QU" + chr(0x1EAC) + "N"
QUAN_HAT = "Qu" + chr(0xE2) + "n"
QUAN_NFD = "Qua" + COMBINING_DOT_BELOW + COMBINING_CIRCUMFLEX + "n"
HCM_FULL = "H" + chr(0x1ED3) + " Ch" + chr(0xED) + " Minh"
HCM_UPPER = "H" + chr(0x1ED2) + " CH" + chr(0xCD) + " MINH"
HCM_DOTTED = "H" + chr(0x1ED2) + " CH" + chr(0xCD) + " M" + DOTTED_CAPITAL_I + "NH"
DA_LAT = chr(0x110) + chr(0xE0) + " L" + chr(0x1EA1) + "t"
DA_LAT_UPPER = chr(0x110) + chr(0xC0) + " L" + chr(0x1EA0) + "T"
PHU_NHUAN = "Ph" + chr(0xFA) + " Nhu" + chr(0x1EAD) + "n"
BINH_THANH = "B" + chr(0xEC) + "nh Th" + chr(0x1EA1) + "nh"
BINH_DOTTED = "B" + DOTTED_CAPITAL_I + "NH TH" + chr(0x1EA0) + "NH"
THU_DUC = "Th" + chr(0x1EE7) + " " + chr(0x110) + chr(0x1EE9) + "c"


def address_edges() -> list[str]:
    texts = [
        "",
        " ",
        "27/1 Yersin, P.10, TP. "
        + DA_LAT
        + ", L"
        + chr(0xE2)
        + "m "
        + chr(0x110)
        + "ồng",
        "220 V" + chr(0x129) + "nh Kh" + chr(0xE1) + "nh, P.9, " + QUAN + " 4, TP.HCM",
        "180 Nam K" + chr(0x1EF3) + ", P.6, " + QUAN + " 3, TP.HCM",
        "330 Phan, P.1, Q. " + PHU_NHUAN + ", TP.HCM",
        "1 Tr"
        + chr(0xE0)
        + "ng Ti"
        + chr(0x1EC1)
        + "n, H"
        + chr(0xE0)
        + " N"
        + chr(0x1ED9)
        + "i",
        QUAN + " 1, HCM",
        QUAN + " 7 hcm",
        QUAN + " 2, TP.HCM",
        QUAN + " 2, " + THU_DUC + ", TP.HCM",
        QUAN + " 10, HCM",
        QUAN + " 01, HCM",
        QUAN + " 1a, HCM",
        QUAN + " 1_, HCM",
        QUAN + " 1-, HCM",
        QUAN + " 1" + COMBINING_ACUTE + ", HCM",
        QUAN + " 1" + SUPERSCRIPT_TWO + " HCM",
        QUAN + " 1" + CIRCLED_ONE + " HCM",
        QUAN + " 1" + ROMAN_FOUR + " HCM",
        QUAN + " " + SUPERSCRIPT_ONE + " " + PHU_NHUAN + " HCM",
        QUAN + " " + ARABIC_ONE + " HCM",
        QUAN + " " + DEVANAGARI_FOUR + " HCM",
        QUAN + " " + FULLWIDTH_THREE + " HCM",
        QUAN + " " + MATH_BOLD_ONE + " HCM",
        QUAN + NBSP + "4 HCM",
        QUAN + FILE_SEP + "4 HCM",
        QUAN + UNIT_SEP + "4 HCM",
        QUAN + NEL + "4 HCM",
        QUAN + LINE_SEP + "4 HCM",
        QUAN + IDEO_SPACE + "4 HCM",
        QUAN + ZWSP + "4 HCM",
        QUAN + MONGOLIAN_VS + "4 HCM",
        QUAN + BOM + "4 HCM",
        QUAN + "4 HCM",
        QUAN + " \t\n 7 HCM",
        QUAN_UPPER + " 4 HCM",
        QUAN.lower() + " 4 hcm",
        QUAN_HAT + " 3 hcm",
        QUAN_HAT.upper() + " 3 hcm",
        "Quan 4 hcm",
        QUAN_NFD + " 4 hcm",
        QUAN + " 4 " + QUAN + " 3 hcm",
        QUAN + " 10a " + QUAN + " 3 hcm",
        "Q" + QUAN + " 1 hcm",
        QUAN + " 1 " + HCM_FULL,
        QUAN + " 7 " + HCM_UPPER,
        QUAN + " 7 " + HCM_DOTTED,
        HCM_DOTTED + ", " + PHU_NHUAN,
        HCM_FULL + ", " + BINH_DOTTED,
        "HCM, " + BINH_DOTTED,
        DA_LAT_UPPER,
        DA_LAT + ", " + QUAN + " 1, HCM",
        PHU_NHUAN + ", " + BINH_THANH + ", HCM",
        THU_DUC + " " + BINH_THANH + " hcm",
        THU_DUC.upper() + " HCM",
        PHU_NHUAN + " (no city)",
        CAPITAL_SIGMA + "HCM " + PHU_NHUAN,
        "hcm" + CAPITAL_SIGMA + " " + THU_DUC,
        "H" + DOTTED_CAPITAL_I + "CM " + PHU_NHUAN,
    ]
    texts += [place["address"] for place in PLACES if place.get("address")]
    return texts


def address_case(text: str) -> dict:
    guard_check(text)
    area = area_of_address(text)
    return {"address": text, "result": "~" if area is None else area["id"]}


def guard_check(text: str) -> None:
    if _GUARD_RUN.search(text):
        raise ValueError(f"digit run would trip the repository guard: {text!r}")


def ranges(predicate) -> list[str]:
    out: list[str] = []
    start = None
    for code in range(0x110000):
        ok = not 0xD800 <= code <= 0xDFFF and predicate(chr(code))
        if ok and start is None:
            start = code
        elif not ok and start is not None:
            out.append(f"{start:x}:{code - 1:x}")
            start = None
    if start is not None:
        out.append(f"{start:x}:10ffff")
    return out


def unicode_tables() -> dict:
    ignore = re.IGNORECASE
    return {
        "unidata_version": unicodedata.unidata_version,
        "digit": ranges(re.compile(r"\d").fullmatch),
        "space": ranges(re.compile(r"\s").fullmatch),
        "word": ranges(re.compile(r"\w").fullmatch),
        "atom_q": ranges(re.compile("Q", ignore).fullmatch),
        "atom_u": ranges(re.compile("u", ignore).fullmatch),
        "atom_n": ranges(re.compile("n", ignore).fullmatch),
        "atom_class": ranges(
            re.compile("[" + chr(0xE2) + chr(0x1EAD) + "]", ignore).fullmatch
        ),
        "lower": [
            f"{code:x}>" + ",".join(f"{ord(ch):x}" for ch in chr(code).lower())
            for code in range(0x110000)
            if not 0xD800 <= code <= 0xDFFF and chr(code).lower() != chr(code)
        ],
    }


def geo_constants() -> dict:
    pattern = areas_module._HCM_DISTRICT
    return {
        "max_area_radius_km": enc_float(MAX_AREA_RADIUS_KM),
        "earth_radius_km": enc_float(areas_module._EARTH_RADIUS_KM),
        "radians_of_one": enc_float(math.radians(1.0)),
        "hcm_district_pattern": pattern.pattern,
        "hcm_district_ignorecase": bool(pattern.flags & re.IGNORECASE),
        "named_hcm": [
            row(needle, area_id) for needle, area_id in areas_module._NAMED_HCM
        ],
        "summary_keys": list(area_summary(AREAS[0])),
    }


def geo_edges() -> dict:
    rng = random.Random(4)
    near, exact = radius_crossings(rng)
    return {
        "libm": libm_rows(),
        "haversine": haversine_edges(),
        "nearest": [nearest_row(a["lat"], a["lng"]) for a in AREAS]
        + [nearest_row(p["lat"], p["lng"]) for p in PLACES]
        + [
            nearest_row(*point)
            for point in (
                (21.0278, 105.8342),
                (0.0, 0.0),
                (math.nan, 106.7),
                (10.77, math.nan),
                (math.inf, 106.7),
                (-10.7769, -73.2991),
                (5e-324, -5e-324),
            )
        ]
        + near,
        "nearest_exact_radius": exact[:40],
        "nearest_ties": distance_ties()[:40],
        "address": [address_case(text) for text in address_edges()],
    }


def random_address(rng: random.Random) -> str:
    vocabulary = [
        QUAN,
        QUAN_UPPER,
        QUAN_HAT,
        QUAN_HAT.upper(),
        QUAN.lower(),
        "Quan",
        QUAN_NFD,
        "Q",
        "u",
        "n",
        " ",
        "  ",
        NBSP,
        FILE_SEP,
        UNIT_SEP,
        NEL,
        LINE_SEP,
        IDEO_SPACE,
        ZWSP,
        MONGOLIAN_VS,
        BOM,
        "\t",
        "1",
        "3",
        "4",
        "7",
        "2",
        "10",
        "04",
        ARABIC_ONE,
        DEVANAGARI_FOUR,
        FULLWIDTH_THREE,
        MATH_BOLD_ONE,
        SUPERSCRIPT_ONE,
        SUPERSCRIPT_TWO,
        CIRCLED_ONE,
        ROMAN_FOUR,
        "a",
        "Z",
        "_",
        "-",
        ",",
        ".",
        COMBINING_ACUTE,
        chr(0xE9),
        "hcm",
        "HCM",
        "Hcm",
        "TP.HCM",
        HCM_FULL,
        HCM_UPPER,
        HCM_DOTTED,
        PHU_NHUAN,
        PHU_NHUAN.upper(),
        BINH_THANH,
        BINH_DOTTED,
        THU_DUC,
        THU_DUC.upper(),
        DA_LAT,
        DA_LAT_UPPER,
        "Da Lat",
        CAPITAL_SIGMA,
        DOTTED_CAPITAL_I,
        "P.9",
        "Yersin",
    ]
    district = [QUAN, QUAN_UPPER, QUAN_HAT, QUAN.lower(), QUAN_NFD, "Quan"]
    spaces = [" ", "  ", NBSP, FILE_SEP, NEL, IDEO_SPACE, ZWSP, "\t", ""]
    numbers = [
        "1",
        "3",
        "4",
        "7",
        "2",
        "10",
        ARABIC_ONE,
        MATH_BOLD_ONE,
        SUPERSCRIPT_ONE,
    ]
    tails = ["", ",", " ", "a", "_", COMBINING_ACUTE, SUPERSCRIPT_TWO, ".", "-"]
    while True:
        parts = [rng.choice(vocabulary) for _ in range(rng.randint(0, 6))]
        if rng.random() < 0.6:
            parts.insert(
                rng.randint(0, len(parts)),
                rng.choice(district)
                + rng.choice(spaces)
                + rng.choice(numbers)
                + rng.choice(tails),
            )
        if rng.random() < 0.5:
            parts.insert(
                rng.randint(0, len(parts)),
                rng.choice(["hcm", "HCM", HCM_FULL, HCM_DOTTED]),
            )
        text = rng.choice(["", " ", ", "]).join(parts)
        if not _GUARD_RUN.search(text):
            return text


def random_point(rng: random.Random) -> tuple[float, float]:
    choice = rng.random()
    area = rng.choice(AREAS)
    if choice < 0.35:
        return area["lat"] + rng.gauss(0, 0.15), area["lng"] + rng.gauss(0, 0.15)
    if choice < 0.55:
        return rng.uniform(8, 24), rng.uniform(102, 110)
    if choice < 0.7:
        return rng.uniform(-90, 90), rng.uniform(-180, 180)
    if choice < 0.8:
        return (
            -area["lat"] + rng.uniform(-1, 1) * 10 ** rng.randint(-13, -1),
            area["lng"] - 180 + rng.uniform(-1, 1) * 10 ** rng.randint(-13, -1),
        )
    if choice < 0.9:
        return area["lat"] + rng.choice([0.2248, -0.2248, 0.0]), area[
            "lng"
        ] + rng.choice([0.2289, -0.2289, 0.0])
    specials = [math.nan, math.inf, -math.inf, 1e308, -0.0, 5e-324, 1e-200]
    return rng.choice(specials + [area["lat"]]), rng.choice(specials + [area["lng"]])


def geo_fuzz() -> list[dict]:
    rng = random.Random(2029)
    cases: list[dict] = []
    for index in range(FUZZ_CASES):
        lat1, lng1 = random_point(rng)
        if rng.random() < 0.6:
            area = rng.choice(AREAS)
            lat2, lng2 = area["lat"], area["lng"]
        else:
            lat2, lng2 = random_point(rng)
        lat, lng = random_point(rng)
        cases.append(
            {
                "haversine": haversine_row((lat1, lng1, lat2, lng2)),
                "nearest": nearest_row(lat, lng),
                "address": address_case(random_address(rng)),
                "index": index,
            }
        )
    return cases


# -- meeting -----------------------------------------------------------------

WEST = {"id": "west", "label": "T" + chr(0xE2) + "y", "lat": 0.0, "lng": 0.0}
EAST = {"id": "east", "label": chr(0x110) + chr(0xF4) + "ng", "lat": 0.0, "lng": 1.0}


def area_row(area: dict) -> str:
    return row(
        enc_str(area["id"]),
        enc_str(area["label"]),
        enc_float(area["lat"]),
        enc_float(area["lng"]),
    )


def place_row(place: dict) -> str:
    return row(
        enc_str(place["id"]),
        enc_str(place["name"]),
        enc_str(place["category"]),
        opt(place["address"], enc_str),
        enc_float(place["lat"]),
        enc_float(place["lng"]),
    )


CANDIDATE_KEYS = [
    "place_id",
    "place_name",
    "category",
    "address",
    "lat",
    "lng",
    "fairness",
    "travel",
]
FAIRNESS_KEYS = ["worst_km", "total_km", "spread_km"]
LEG_KEYS = ["id", "label", "lat", "lng", "km"]


def meeting_case(
    name: str, origins: list[dict], places: list[dict], limit: int
) -> dict:
    case = {
        "name": name,
        "origins": [area_row(area) for area in origins],
        "places": [place_row(place) for place in places],
        "limit": limit,
    }
    try:
        ranked = rank_meeting_points(origins, places, limit=limit)
    except Exception as exc:  # noqa: BLE001 - the oracle records every raise
        case["raise"] = raised(exc)
        return case
    rows = []
    for candidate in ranked:
        if (
            list(candidate) != CANDIDATE_KEYS
            or list(candidate["fairness"]) != FAIRNESS_KEYS
        ):
            raise AssertionError(f"candidate shape changed: {list(candidate)}")
        for leg, origin in zip(candidate["travel"], origins, strict=True):
            if list(leg) != LEG_KEYS or {
                k: leg[k] for k in LEG_KEYS[:4]
            } != area_summary(origin):
                raise AssertionError(f"travel leg no longer echoes its origin: {leg}")
        fairness = candidate["fairness"]
        rows.append(
            row(
                enc_str(candidate["place_id"]),
                enc_str(candidate["place_name"]),
                enc_str(candidate["category"]),
                opt(candidate["address"], enc_str),
                enc_float(candidate["lat"]),
                enc_float(candidate["lng"]),
                enc_float(fairness["worst_km"]),
                enc_float(fairness["total_km"]),
                enc_float(fairness["spread_km"]),
                ";".join(enc_float(leg["km"]) for leg in candidate["travel"]),
            )
        )
    case["candidates"] = rows
    return case


def point(place_id: str, lng: float, lat: float = 0.0, name: str | None = None) -> dict:
    return {
        "id": place_id,
        "name": name or place_id,
        "category": "cafe",
        "address": "somewhere",
        "lat": lat,
        "lng": lng,
    }


def catalogue_places() -> list[dict]:
    return [
        {
            key: place.get(key)
            for key in ("id", "name", "category", "address", "lat", "lng")
        }
        for place in PLACES
    ]


def by_id(area_id: str) -> dict:
    return next(area for area in AREAS if area["id"] == area_id)


def meeting_edges() -> list[dict]:
    line = [
        point("p-a-near-west", 0.05),
        point("p-b-off-centre", 0.35),
        point("p-c-midpoint", 0.5),
    ]
    pair = [point("p-z-west", 0.4), point("p-a-east", 0.6)]
    catalogue = catalogue_places()
    saigon = [
        by_id(i) for i in ("hcm-quan-1", "hcm-quan-3", "hcm-quan-7", "hcm-thu-duc")
    ]
    antipode = point("p-antipode", AREAS[1]["lng"] - 180.0, -AREAS[1]["lat"])
    duplicates = [
        point("p-same", 0.5, name="first"),
        point("p-same", 0.5, name="second"),
        point("p-same", 0.5, name="third"),
    ]
    cases = [
        meeting_case("line_two_origins", [WEST, EAST], line, 5),
        meeting_case("pair_tie_even", [WEST, EAST], pair, 5),
        meeting_case("pair_tie_weighted", [WEST, WEST, WEST, EAST], pair, 5),
        meeting_case("lopsided_line", [WEST, WEST, WEST, EAST], line, 5),
        meeting_case("every_origin_gets_a_leg", [WEST, WEST, EAST], [line[2]], 5),
        meeting_case("limit_two", [WEST, EAST], line, 2),
        meeting_case("no_places", [WEST, EAST], [], 5),
        meeting_case("no_origins", [], line, 5),
        meeting_case("one_origin", [WEST], line, 5),
        meeting_case("saigon_catalogue", saigon, catalogue, 5),
        meeting_case("every_area", list(AREAS), catalogue, 5),
        meeting_case(
            "twelve_origins",
            [AREAS[i % 8] for i in range(MAX_ORIGIN_AREAS)],
            catalogue,
            5,
        ),
        meeting_case(
            "thirteen_origins",
            [AREAS[i % 8] for i in range(MAX_ORIGIN_AREAS + 1)],
            catalogue,
            5,
        ),
        meeting_case(
            "cross_city", [by_id("da-lat"), by_id("hcm-quan-1")], catalogue, 5
        ),
        meeting_case(
            "antipode_raises",
            [by_id("hcm-quan-1"), by_id("hcm-quan-3")],
            [line[0], antipode],
            5,
        ),
        meeting_case("duplicate_ids_stay_stable", [WEST, EAST], duplicates + line, 5),
        meeting_case("limit_zero", [WEST, EAST], line, 0),
        meeting_case("limit_negative_one", [WEST, EAST], line, -1),
        meeting_case("limit_very_negative", [WEST, EAST], line, -100),
        meeting_case("limit_large", [WEST, EAST], line, 100),
        meeting_case(
            "place_on_origin",
            [WEST, EAST],
            [point("p-on-west", 0.0), point("p-on-east", 1.0)],
            5,
        ),
        meeting_case(
            "unicode_ids_order",
            [WEST, EAST],
            [
                point(chr(0xE9), 0.5),
                point("z", 0.5),
                point("Z", 0.5),
                point("", 0.5),
                point("e" + COMBINING_ACUTE, 0.5),
            ],
            5,
        ),
        meeting_case(
            "null_address", [WEST, EAST], [dict(point("p-null", 0.5), address=None)], 5
        ),
        meeting_case(
            "infinite_place_raises",
            [WEST, EAST],
            [line[0], point("p-inf", math.inf), line[1]],
            5,
        ),
        meeting_case(
            "infinite_place_without_origins", [], [point("p-inf", math.inf)], 5
        ),
        meeting_case(
            "many_duplicates_stay_stable",
            [WEST, EAST],
            [point("p-twin", 0.5, name=f"twin-{n}") for n in range(24)] + line,
            30,
        ),
    ]
    return cases


def round_rows() -> list[str]:
    rng = random.Random(7)
    values = [
        0.125,
        0.375,
        0.625,
        2.675,
        1.005,
        0.005,
        0.015,
        0.025,
        -0.125,
        -0.005,
        0.0,
        -0.0,
        5e-324,
        -5e-324,
        math.nextafter(0.005, 0.0),
        1e16,
        1e22,
        sys.float_info.max,
        123.455,
        999.995,
        math.nextafter(25.005, 0.0),
        111.195,
        math.nan,
        math.inf,
        -math.inf,
        -0.001,
        math.nextafter(0.995, 0.0),
        0.995,
    ]
    for _ in range(300):
        tie = (2 * rng.randint(0, 200000) + 1) / 200
        values += [tie, math.nextafter(tie, -1e9), math.nextafter(tie, 1e9)]
    for _ in range(200):
        values.append(rng.uniform(0, 3000))
    return [
        row(enc_float(v), scalar_outcome(lambda v=v: round(v, 2), enc_float))
        for v in values
    ]


def sum_rows() -> list[str]:
    rng = random.Random(8)
    lists = [
        [1e16, 1.0, -1e16],
        [0.1] * 10,
        [1e308, 1e308],
        [math.inf, 1.0],
        [math.inf, -math.inf],
        [math.nan, 1.0],
        [-0.0],
        [-0.0, -0.0],
        [1.0, 1e100, 1.0, -1e100],
        [5e-324, 5e-324],
    ]
    for _ in range(300):
        size = rng.randint(1, 13)
        lists.append(
            [
                rng.choice(
                    [rng.uniform(0, 30), rng.uniform(0, 3000), 10 ** rng.uniform(-6, 6)]
                )
                for _ in range(size)
            ]
        )
    return [
        row(
            ";".join(enc_float(v) for v in items),
            scalar_outcome(lambda i=items: sum(i), enc_float),
        )
        for items in lists
    ]


def random_origin(rng: random.Random, pool: list[dict]) -> dict:
    if rng.random() < 0.75:
        return rng.choice(AREAS)
    return rng.choice(pool)


def meeting_fuzz() -> list[dict]:
    rng = random.Random(45)
    names = [
        "cafe",
        "bar",
        "an",
        "p",
        "P",
        "p-1",
        "p1",
        "p10",
        "p2",
        chr(0xE9),
        "z",
        "",
    ]
    categories = ["cafe", "food", "bar", "Cafe"]
    cases = []
    for index in range(FUZZ_CASES):
        pool = [
            {
                "id": rng.choice(["west", "east", "o", "O-1"]),
                "label": rng.choice(["", "T" + chr(0xE2) + "y", "x"]),
                "lat": rng.uniform(-60, 60),
                "lng": rng.uniform(-170, 170),
            }
            for _ in range(3)
        ]
        count = rng.choice([0, 1, 2, 2, 2, 3, 3, 4, 5, 12, 13])
        origins = [random_origin(rng, pool) for _ in range(count)]
        places: list[dict] = []
        for _ in range(rng.randint(0, 6)):
            choice = rng.random()
            if places and choice < 0.15:
                twin = dict(rng.choice(places))
                twin["name"] = rng.choice(names) + "-twin"
                if rng.random() < 0.5:
                    twin["id"] = rng.choice(names)
                places.append(twin)
                continue
            if origins and choice < 0.3:
                left, right = (
                    rng.sample(origins, 2)
                    if len(origins) > 1
                    else (origins[0], origins[0])
                )
                t = rng.choice([0.25, 0.5, 0.75, 0.0, 1.0])
                lat = left["lat"] + (right["lat"] - left["lat"]) * t
                lng = left["lng"] + (right["lng"] - left["lng"]) * t
            elif choice < 0.4 and origins:
                origin = rng.choice(origins)
                lat, lng = -origin["lat"], origin["lng"] - 180.0
            elif choice < 0.45:
                lat, lng = rng.choice([1e300, -1e300, 400.0]), rng.uniform(-180, 180)
            elif choice < 0.47:
                # CHECK constraints keep these out of the table; the raise
                # happens before any sort, so it is still reproducible.
                lat, lng = rng.choice([(math.inf, 106.7), (10.8, -math.inf)])
            else:
                lat, lng = random_point(rng)
                if not (math.isfinite(lat) and math.isfinite(lng)):
                    lat, lng = 10.8, 106.7
            places.append(
                {
                    "id": rng.choice(names),
                    "name": rng.choice(names),
                    "category": rng.choice(categories),
                    "address": rng.choice([None, "", "somewhere", QUAN + " 1"]),
                    "lat": float(lat),
                    "lng": float(lng),
                }
            )
        limit = rng.choice([5, 5, 5, 0, 1, 2, 3, 10, -1, -2, -100, 100])
        case = meeting_case(f"fuzz_{index}", origins, places, limit)
        cases.append(case)
    return cases


# -- social_map --------------------------------------------------------------


def checkin_row(checkin: dict) -> str:
    return row(
        opt(checkin.get("place_id"), enc_str),
        opt(checkin.get("place_name"), enc_str),
        opt(checkin.get("lat"), enc_float),
        opt(checkin.get("lng"), enc_float),
    )


def trending_place_row(place: dict) -> str:
    return row(
        enc_str(place["id"]),
        enc_str(place["name"]),
        enc_float(place["lat"]),
        enc_float(place["lng"]),
        opt(place.get("rating"), enc_float),
        opt(place["rating_count"], enc_int),
        opt(place.get("flag"), enc_str),
    )


VISITED_KEYS = ["place_id", "place_name", "lat", "lng", "visit_count"]
TRENDING_KEYS = ["place_id", "place_name", "lat", "lng", "rating", "rating_count"]
HEATMAP_KEYS = ["id", "label", "lat", "lng", "visit_count", "share_percent"]


def checked_rows(rows: list[dict], keys: list[str]) -> list[dict]:
    for item in rows:
        if list(item) != keys:
            raise AssertionError(f"row shape changed: {list(item)}")
    return rows


def rows_outcome(call, keys: list[str], encode) -> dict:
    try:
        rows = checked_rows(call(), keys)
    except Exception as exc:  # noqa: BLE001 - the oracle records every raise
        return {"raise": raised(exc)}
    return {"rows": [encode(item) for item in rows]}


def checkin_case(name: str, checkins: list[dict]) -> dict:
    def visited(item: dict) -> str:
        return row(
            enc_str(item["place_id"]),
            enc_str(item["place_name"]),
            enc_float(item["lat"]),
            enc_float(item["lng"]),
            enc_int(item["visit_count"]),
        )

    def heat(item: dict) -> str:
        return row(
            enc_str(item["id"]),
            enc_str(item["label"]),
            enc_float(item["lat"]),
            enc_float(item["lng"]),
            enc_int(item["visit_count"]),
            enc_int(item["share_percent"]),
        )

    return {
        "name": name,
        "checkins": [checkin_row(item) for item in checkins],
        "visited": rows_outcome(lambda: visited_layer(checkins), VISITED_KEYS, visited),
        "heatmap": rows_outcome(lambda: heatmap_rows(checkins), HEATMAP_KEYS, heat),
        "unknown": scalar_outcome(lambda: unknown_area_count(checkins), enc_int),
    }


def trending_case(name: str, places: list[dict]) -> dict:
    def trend(item: dict) -> str:
        return row(
            enc_str(item["place_id"]),
            enc_str(item["place_name"]),
            enc_float(item["lat"]),
            enc_float(item["lng"]),
            opt(item["rating"], enc_float),
            opt(item["rating_count"], enc_int),
        )

    return {
        "name": name,
        "places": [trending_place_row(place) for place in places],
        "trending": rows_outcome(lambda: trending_layer(places), TRENDING_KEYS, trend),
    }


def visit(place: dict, **overrides) -> dict:
    record = {
        "id": "memory",
        "place_id": place["id"],
        "place_name": place["name"],
        "lat": place["lat"],
        "lng": place["lng"],
        "author_id": "author",
        "created_at": "2026-03-14",
        "caption": "hidden",
    }
    record.update(overrides)
    return record


def socialmap_edges() -> dict:
    da_lat, cafe = PLACES[0], PLACES[1]
    saigon = next(
        p for p in PLACES if "Qu" + chr(0x1EAD) + "n 4" in (p.get("address") or "")
    )
    hanoi = {"id": "p-hanoi", "name": "Somewhere", "lat": 21.0278, "lng": 105.8342}
    antipode = {
        "id": "p-antipode",
        "name": "Far",
        "lat": -AREAS[1]["lat"],
        "lng": AREAS[1]["lng"] - 180.0,
    }
    photo = {"place_id": None, "place_name": None, "lat": None, "lng": None}
    checkins = [
        checkin_case("empty", []),
        checkin_case("repeat_visits", [visit(da_lat), visit(da_lat), visit(cafe)]),
        checkin_case("count_then_id", [visit(cafe), visit(da_lat), visit(saigon)]),
        checkin_case("photo_is_not_a_visit", [photo]),
        checkin_case("blank_place_id", [visit(da_lat, place_id="")]),
        checkin_case(
            "blank_name_falls_back",
            [visit(da_lat, place_name=""), visit(cafe, place_name=None)],
        ),
        checkin_case(
            "lat_without_lng", [visit(da_lat, lng=None), visit(cafe, lat=None)]
        ),
        checkin_case(
            "first_snapshot_wins",
            [visit(da_lat), visit(da_lat, place_name="Renamed", lat=11.0, lng=108.0)],
        ),
        checkin_case("heatmap_buckets", [visit(da_lat), visit(cafe), visit(saigon)]),
        checkin_case(
            "hanoi_is_unknown",
            [visit(da_lat), hanoi | {"place_id": "p-hanoi", "place_name": "x"}],
        ),
        checkin_case(
            "only_unknown", [hanoi | {"place_id": "p-hanoi", "place_name": "x"}]
        ),
        checkin_case("antipode_raises", [visit(da_lat), visit(antipode)]),
        checkin_case(
            "nan_coordinates", [visit(da_lat, lat=math.nan), visit(cafe, lng=math.inf)]
        ),
        checkin_case(
            "seven_rows", [visit(saigon)] * 3 + [visit(p) for p in PLACES[:4]]
        ),
        checkin_case(
            "tie_on_count",
            [visit(da_lat)] * 3 + [visit(saigon)] * 3 + [visit(p) for p in PLACES[1:4]],
        ),
        checkin_case("catalogue", [visit(p) for p in PLACES]),
    ]
    hot = [{**p, "rating_count": p.get("rating_count")} for p in PLACES]
    variants = [
        {
            "id": "b",
            "name": "B",
            "lat": 1.0,
            "lng": 2.0,
            "rating": None,
            "rating_count": None,
            "flag": "hot",
        },
        {
            "id": "a",
            "name": "A",
            "lat": 1.0,
            "lng": 2.0,
            "rating": 4.5,
            "rating_count": 3,
            "flag": "HOT",
        },
        {
            "id": "a",
            "name": "A2",
            "lat": 1.0,
            "lng": 2.0,
            "rating": 4.0,
            "rating_count": 0,
            "flag": "hot",
        },
        {
            "id": "a",
            "name": "A3",
            "lat": 1.5,
            "lng": 2.5,
            "rating": 1.0,
            "rating_count": 1,
            "flag": "hot",
        },
        {
            "id": "c",
            "name": "C",
            "lat": 1.0,
            "lng": 2.0,
            "rating": 3.0,
            "rating_count": 1,
            "flag": "hot ",
        },
        {
            "id": "d",
            "name": "D",
            "lat": 1.0,
            "lng": 2.0,
            "rating": 3.0,
            "rating_count": 1,
        },
        {
            "id": chr(0xE9),
            "name": "E",
            "lat": 1.0,
            "lng": 2.0,
            "rating": 3.0,
            "rating_count": 1,
            "flag": "hot",
        },
    ]
    trending = [
        trending_case("catalogue", hot),
        trending_case("empty", []),
        trending_case("flags_and_duplicates", variants),
        trending_case(
            "many_duplicate_ids",
            [
                {
                    **variants[0],
                    "id": "same" if n % 2 == 0 else f"z-{39 - n:02d}",
                    "name": f"twin-{n}",
                    "lat": float(n),
                }
                for n in range(40)
            ],
        ),
    ]
    return {"checkins": checkins, "trending": trending}


def socialmap_fuzz() -> list[dict]:
    rng = random.Random(44)
    ids = [None, "", "p", "p-1", "p1", "p10", "p2", chr(0xE9), "z", "Z"]
    names = [None, "", "Name", "Quán", "~x"]
    cases = []
    for index in range(FUZZ_CASES):
        checkins = []
        for _ in range(rng.randint(0, 10)):
            if rng.random() < 0.08:
                lat, lng = None, None
            else:
                lat, lng = random_point(rng)
                if rng.random() < 0.05:
                    lat = None
                if rng.random() < 0.05:
                    lng = None
            checkins.append(
                {
                    "place_id": rng.choice(ids),
                    "place_name": rng.choice(names),
                    "lat": lat,
                    "lng": lng,
                }
            )
        if checkins and rng.random() < 0.3:
            checkins += [rng.choice(checkins) for _ in range(rng.randint(1, 5))]
        places = []
        for _ in range(rng.randint(0, 7)):
            places.append(
                {
                    "id": rng.choice([i for i in ids if i is not None]),
                    "name": rng.choice(["N", "M", "Quán"]),
                    "lat": rng.uniform(-90, 90),
                    "lng": rng.uniform(-180, 180),
                    "rating": rng.choice([None, 4.5, 3.0, rng.uniform(0, 5)]),
                    "rating_count": rng.choice([None, 0, 1, 250]),
                    "flag": rng.choice([None, "hot", "hot", "HOT", "", "new"]),
                }
            )
        cases.append(
            {
                **checkin_case(f"fuzz_{index}", checkins),
                **trending_case(f"fuzz_{index}", places),
            }
        )
    return cases


# -- files -------------------------------------------------------------------


def modes() -> dict[str, str]:
    out = {}
    for module, package in TARGETS.items():
        out[module] = f"{CORE}{package}/testdata/python_{module}.json"
        for shard in range(SHARDS[module]):
            out[f"{module}-fuzz-{shard}"] = (
                f"{CORE}{package}/testdata/python_{module}_fuzz_{shard}.json"
            )
    return out


def shard(cases: list, module: str, index: int) -> dict:
    total = len(cases)
    size = -(-total // SHARDS[module])
    return {
        "fuzz": {"shard": index, "shards": SHARDS[module], "total": total},
        "cases": cases[index * size : (index + 1) * size],
    }


def render(mode: str) -> dict:
    document: dict = {"mode": mode, "host": host()}
    if mode == "geo":
        document["constants"] = geo_constants()
        document["unicode"] = unicode_tables()
        document["cases"] = geo_edges()
    elif mode == "meeting":
        document["constants"] = {
            "min_origin_areas": MIN_ORIGIN_AREAS,
            "max_origin_areas": MAX_ORIGIN_AREAS,
            "candidate_keys": CANDIDATE_KEYS,
            "fairness_keys": FAIRNESS_KEYS,
            "leg_keys": LEG_KEYS,
        }
        document["round2"] = round_rows()
        document["sum"] = sum_rows()
        document["cases"] = meeting_edges()
    elif mode == "socialmap":
        document["constants"] = {
            "private_checkin_fields": list(PRIVATE_CHECKIN_FIELDS),
            "visited_keys": VISITED_KEYS,
            "trending_keys": TRENDING_KEYS,
            "heatmap_keys": HEATMAP_KEYS,
        }
        document["cases"] = socialmap_edges()
    else:
        module, _, index = mode.rpartition("-fuzz-")
        cases = {"geo": geo_fuzz, "meeting": meeting_fuzz, "socialmap": socialmap_fuzz}[
            module
        ]()
        document.update(shard(cases, module, int(index)))
    return document


def main() -> None:
    if sys.argv[1:] == ["--list"]:
        for mode, path in modes().items():
            sys.stdout.write(f"{mode}\t{path}\n")
        return
    if len(sys.argv) != 2 or sys.argv[1] not in modes():
        raise SystemExit(f"usage: MODE, one of {sorted(modes())} or --list")
    json.dump(render(sys.argv[1]), sys.stdout, ensure_ascii=True, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
