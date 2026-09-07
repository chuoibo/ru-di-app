#!/usr/bin/env python3
"""TalkBack-readiness audit of RuDi screens on an Android emulator (report §13.1).

For each deep link: open it, wait, `uiautomator dump`, and read the tree the
way a screen reader would --

  * every clickable / long-clickable / checkable node must carry a name
    (`text` or `content-desc`, on itself or on a descendant);
  * every such node must be at least 48×48 dp (Material touch target), read
    from `bounds` at the device density;
  * no two clickable nodes on one screen may share the exact same name
    (a reader announcing «Lưu, nút» four times cannot tell which is which),
    unless the name is a row label that legitimately repeats (`--allow-dup`).

Nothing here proves a person using TalkBack can finish a task; it proves the
tree gives them something to work with. Output is one JSON per screen plus a
summary table on stdout; exit 1 when any screen has findings.

    ANDROID_ADB_SERVER_PORT=5038 python3 scripts/a11y_native_audit.py \\
        --serial 127.0.0.1:5561 --out /tmp/a11y \\
        rudi://welcome rudi://settlements/team-da-lat
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from pathlib import Path

MIN_DP = 48


def adb(serial: str, *args: str, timeout: int = 60) -> str:
    out = subprocess.run(["adb", "-s", serial, *args], capture_output=True, text=True, timeout=timeout)
    return out.stdout


def density(serial: str) -> int:
    raw = adb(serial, "shell", "wm", "density")
    m = re.findall(r"density: (\d+)", raw)
    return int(m[-1]) if m else 160


def bounds_of(node: ET.Element) -> tuple[int, int, int, int]:
    m = re.match(r"\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]", node.get("bounds", ""))
    return tuple(int(v) for v in m.groups()) if m else (0, 0, 0, 0)  # type: ignore[return-value]


def name_of(node: ET.Element) -> str:
    """The name a reader announces: own text/desc, else the first descendant's."""
    for n in node.iter():
        t = (n.get("content-desc") or "").strip() or (n.get("text") or "").strip()
        if t:
            return t
    return ""


def tap_text(serial: str, root: ET.Element, label: str) -> bool:
    """Tap the centre of the first node whose text or description is `label`."""
    for n in root.iter("node"):
        if n.get("text") == label or n.get("content-desc") == label:
            l, t, r, b = bounds_of(n)
            adb(serial, "shell", "input", "tap", str((l + r) // 2), str((t + b) // 2))
            return True
    return False


def dump(serial: str, xml_path: Path) -> ET.Element | None:
    adb(serial, "shell", "uiautomator", "dump", "/sdcard/a11y.xml")
    adb(serial, "pull", "/sdcard/a11y.xml", str(xml_path))
    try:
        return ET.parse(xml_path).getroot()
    except (ET.ParseError, FileNotFoundError):
        return None


def qua_dev_launcher(serial: str, xml_path: Path) -> ET.Element | None:
    """The development client answers a deep link with its own «Continue»
    sheet first; a dump taken there measures the launcher, not the app
    (the all-zero audit of 2026-09-07 was exactly that)."""
    root = dump(serial, xml_path)
    for _ in range(3):
        if root is None:
            return None
        if not tap_text(serial, root, "Continue"):
            return root
        time.sleep(2.5)
        root = dump(serial, xml_path)
    return root


def audit(root: ET.Element, dpi: int, allow_dup: set[str]) -> dict:
    scale = dpi / 160
    unnamed, small, dups, cut = [], [], [], []
    seen: dict[str, int] = {}
    # The bottom edge of every scroll container: a target whose box ends
    # there is cut by the fold, not drawn small (five such rows on 2026-09-07).
    edges = {bounds_of(n)[3] for n in root.iter("node") if n.get("scrollable") == "true"}
    edges.add(max((bounds_of(n)[3] for n in root.iter("node")), default=0))
    for n in root.iter("node"):
        interactive = n.get("clickable") == "true" or n.get("long-clickable") == "true" or n.get("checkable") == "true"
        if not interactive:
            continue
        l, t, r, b = bounds_of(n)
        w, h = (r - l) / scale, (b - t) / scale
        name = name_of(n)
        cls = n.get("class", "")
        if not name:
            unnamed.append({"class": cls, "bounds": n.get("bounds")})
        else:
            seen[name] = seen.get(name, 0) + 1
        # A scroll container is clickable to the tree but not a target.
        if name and (w < MIN_DP - 0.5 or h < MIN_DP - 0.5) and "ScrollView" not in cls:
            if b in edges and h < MIN_DP - 0.5:
                cut.append({"name": name[:60], "dp": [round(w), round(h)]})
            else:
                small.append({"name": name[:60], "dp": [round(w), round(h)]})
    for name, count in seen.items():
        if count > 1 and name not in allow_dup:
            dups.append({"name": name[:60], "count": count})
    return {"unnamed": unnamed, "small": small, "duplicates": dups, "cut": cut}


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--serial", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--wait", type=float, default=5.0)
    ap.add_argument("--allow-dup", action="append", default=[], help="tên được phép lặp (hàng cùng nhãn)")
    ap.add_argument("links", nargs="+")
    a = ap.parse_args()
    out = Path(a.out)
    out.mkdir(parents=True, exist_ok=True)
    dpi = density(a.serial)
    rows = []
    red = False
    for link in a.links:
        adb(a.serial, "shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", link, "com.lakiet.rudi")
        time.sleep(a.wait)
        slug = re.sub(r"[^a-z0-9]+", "-", link.split("://", 1)[-1].lower()).strip("-") or "root"
        xml_path = out / f"{slug}.xml"
        root = qua_dev_launcher(a.serial, xml_path)
        if root is None:
            rows.append((link, "KHÔNG DUMP ĐƯỢC", 0, 0, 0))
            red = True
            continue
        # A screen that still shows the launcher, or nothing interactive, is
        # not a measurement: say so instead of printing zeros.
        texts = {(n.get("text") or n.get("content-desc") or "") for n in root.iter("node")}
        if "Runtime version: exposdk:57.0.0" in " ".join(texts) or not any(n.get("clickable") == "true" for n in root.iter("node")):
            rows.append((link, "KHÔNG PHẢI MÀN APP", 0, 0, 0))
            red = True
            continue
        res = audit(root, dpi, set(a.allow_dup))
        (out / f"{slug}.json").write_text(json.dumps({"link": link, **res}, ensure_ascii=False, indent=2))
        n_un, n_sm, n_du, n_cut = len(res["unnamed"]), len(res["small"]), len(res["duplicates"]), len(res["cut"])
        if n_un or n_sm or n_du:
            red = True
        rows.append((link, f"cắt bởi mép: {n_cut}" if n_cut else "", n_un, n_sm, n_du))
    print(f"{'màn':44} {'chưa tên':>8} {'<48dp':>6} {'trùng':>6}")
    for link, note, n_un, n_sm, n_du in rows:
        print(f"{link[:44]:44} {n_un:>8} {n_sm:>6} {n_du:>6} {note}")
    print("density:", dpi, "| chi tiết:", out)
    return 1 if red else 0


if __name__ == "__main__":
    sys.exit(main())
