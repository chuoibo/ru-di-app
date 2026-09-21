"""Material gate for the dark scheme (review 11/09, A3).

    python3 do-chat-lieu.py <screenshot.png> <hierarchy.xml> [--nen HEX] [--dem-mau HEX --hop x0,y0,x1,y1]

Picks a text-free square patch of the ground from the uiautomator dump (no node
with text/content-desc overlaps it, below the status bar; with --nen, the patch
whose mean is closest to the ground token, so a card or a picture is never
measured as «the ground») and reports its grey-level stddev (grain) and CIELAB
L*. With --dem-mau/--hop it counts the pixels inside the box within ±8 of a hex:
does the paper tone appear where a drawn sheet should be? Numbers only; the
verdict is the reader's.
"""

from __future__ import annotations

import argparse
import re
import sys

import numpy as np
from PIL import Image


def hop(s: str) -> tuple[int, int, int, int]:
    a = [int(v) for v in s.split(",")]
    if len(a) != 4:
        raise SystemExit("hộp cần 4 số x0,y0,x1,y1")
    return a[0], a[1], a[2], a[3]


def mau(hex6: str) -> np.ndarray:
    return np.array([int(hex6[i : i + 2], 16) for i in (1, 3, 5)], dtype=float)


def nodes_co_chu(xml: str) -> list[tuple[int, int, int, int]]:
    ra = []
    for m in re.finditer(r'<node [^>]*?bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', xml):
        node = m.group(0)
        if re.search(r'(text|content-desc)="[^"]+"', node):
            ra.append(tuple(int(v) for v in m.groups()))
    return ra


def vung_trong(
    arr: np.ndarray,
    cam: list[tuple[int, int, int, int]],
    nen: np.ndarray | None,
    canh: int,
    tren: int = 150,
    buoc: int = 20,
):
    """A canh×canh patch touching no text node, below the status bar; closest to `nen` when given."""
    h, w = arr.shape[:2]
    best = None
    best_d = None
    for y in range(tren, h - canh, buoc):
        for x in range(0, w - canh, buoc):
            if any(
                not (x + canh < a or x > c or y + canh < b or y > d)
                for a, b, c, d in cam
            ):
                continue
            if nen is None:
                return (x, y, x + canh, y + canh)
            d = float(
                np.abs(
                    arr[y : y + canh, x : x + canh].reshape(-1, 3).mean(axis=0) - nen
                ).sum()
            )
            if best_d is None or d < best_d:
                best, best_d = (x, y, x + canh, y + canh), d
    return best


def lab_L(rgb: np.ndarray) -> float:
    c = rgb / 255.0
    lin = np.where(c <= 0.04045, c / 12.92, ((c + 0.055) / 1.055) ** 2.4)
    y = 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]
    f = y ** (1 / 3) if y > 0.008856 else 7.787 * y + 16 / 116
    return float(116 * f - 16)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("png")
    ap.add_argument("xml")
    ap.add_argument(
        "--nen", help="hex of the ground token: pick the text-free patch closest to it"
    )
    ap.add_argument("--canh", type=int, default=200)
    ap.add_argument("--dem-mau", help="hex; count pixels within ±8 of it inside --hop")
    ap.add_argument("--hop", help="x0,y0,x1,y1 for --dem-mau")
    a = ap.parse_args()
    im = Image.open(a.png).convert("RGB")
    arr = np.asarray(im, dtype=float)
    xml = open(a.xml, encoding="utf-8", errors="ignore").read()
    vung = vung_trong(arr, nodes_co_chu(xml), mau(a.nen) if a.nen else None, a.canh)
    if vung is None:
        print("không tìm được vùng nền không chữ", file=sys.stderr)
        return 2
    x0, y0, x1, y1 = vung
    patch = arr[y0:y1, x0:x1]
    grey = patch.mean(axis=2)
    print(
        f"nền [{x0},{y0}]→[{x1},{y1}]  mean={grey.mean():.1f}  stddev={grey.std():.2f}  L*={lab_L(patch.reshape(-1, 3).mean(axis=0)):.1f}"
    )
    if a.dem_mau and a.hop:
        h = hop(a.hop)
        vung_hop = arr[max(h[1], 0) : h[3], max(h[0], 0) : h[2]]
        if vung_hop.size == 0:
            print(f"hộp {h} rỗng", file=sys.stderr)
            return 2
        gan = (np.abs(vung_hop - mau(a.dem_mau)) <= 8).all(axis=2)
        print(
            f"pixel gần {a.dem_mau} trong hộp {h}: {int(gan.sum())} / {gan.size} ({100 * gan.mean():.1f}%)"
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
