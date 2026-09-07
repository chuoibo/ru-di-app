"""Generate guest.css and contrast documentation from shared tokens.json.

The shared token file is the source; this script never replaces its palette
with a stale built-in copy. Re-run after any colour change.
"""

from __future__ import annotations

import json
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[1]
TOKENS = ROOT / "packages/shared/tokens.json"
CSS = ROOT / "services/api/app/web/static/guest.css"
DESIGN = ROOT / "DESIGN.md"
DJSON = ROOT / ".impeccable/design.json"

ORDER = [
    "ground",
    "card",
    "line",
    "lineStrong",
    "ink",
    "inkSoft",
    "inkFaint",
    "accent",
    "accentEnd",
    "accentInk",
    "accentSoft",
    "split",
    "splitInk",
    "splitSoft",
    "ai",
    "aiInk",
    "aiSoft",
    "warn",
    "cover",
    "coverInk",
    "coverInkSoft",
    "coverLine",
    "coverLineStrong",
]
PALETTE = json.loads(TOKENS.read_text(encoding="utf-8"))["color"]
LIGHT = PALETTE["light"]
DARK = PALETTE["dark"]
assert list(LIGHT) == ORDER and list(DARK) == ORDER

TEXT_PAIRS = [
    ("ink", "ground", "Chữ thân trên nền trang"),
    ("ink", "card", "Chữ thân trên thẻ"),
    ("inkSoft", "card", "Chữ phụ trên thẻ"),
    ("inkSoft", "ground", "Chữ phụ trên nền"),
    ("inkFaint", "card", "Chú thích trên thẻ"),
    ("inkFaint", "ground", "Chú thích trên nền"),
    ("accent", "card", "Cam trên thẻ"),
    ("accent", "ground", "Cam trên nền"),
    ("accentInk", "accent", "Nhãn trên nút cam"),
    ("accent", "accentSoft", "Cam trên chip cam nhạt"),
    ("split", "card", "Teal trên thẻ"),
    ("split", "ground", "Teal trên nền"),
    ("splitInk", "split", "Nhãn trên nút teal"),
    ("split", "splitSoft", "Teal trên chip teal nhạt"),
    ("ai", "card", "Tím trên thẻ"),
    ("ai", "ground", "Tím trên nền"),
    ("aiInk", "ai", "Nhãn trên nút tím"),
    ("ai", "aiSoft", "Tím trên chip tím nhạt"),
    ("warn", "card", "Cảnh báo trên thẻ"),
    ("warn", "ground", "Cảnh báo trên nền"),
    ("ink", "accentSoft", "Chữ thân trên chip cam"),
    ("ink", "splitSoft", "Chữ thân trên chip teal"),
    ("ink", "aiSoft", "Chữ thân trên chip tím"),
    ("coverInk", "cover", "Chữ trên bìa sổ"),
    ("coverInkSoft", "cover", "Chữ phụ trên bìa sổ"),
]
# Action text on cover must use coverInk; light accent is below the text floor.
NONTEXT = [
    ("lineStrong", "ground", "Viền control trên nền trang", True),
    ("lineStrong", "card", "Viền control trên thẻ", True),
    ("coverLineStrong", "cover", "Viền control trên bìa sổ", True),
    ("line", "ground", "Cạnh thẻ trên nền trang", False),
    ("line", "card", "Đường kẻ trong thẻ", False),
    ("coverLine", "cover", "Đường kẻ trên bìa", False),
]


def lum(h):
    c = [int(h[i : i + 2], 16) / 255 for i in (1, 3, 5)]
    lin = [x / 12.92 if x <= 0.03928 else ((x + 0.055) / 1.055) ** 2.4 for x in c]
    return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]


def cr(a, b):
    la, lb = lum(a), lum(b)
    return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)


def tier(r):
    return "AAA" if r >= 7 else "AA"


def table_text(pal):
    rows = ["| Cặp | Vai trò | Tỉ lệ | Ngưỡng |", "|---|---|---|---|"]
    for fg, bg, role in TEXT_PAIRS:
        r = cr(pal[fg], pal[bg])
        assert r >= 4.5, (fg, bg, r)
        rows.append(
            f"| `{fg}` {pal[fg]} trên `{bg}` {pal[bg]} | {role} | **{r:.2f}:1** | {tier(r)} |"
        )
    return "\n".join(rows)


def table_nontext(pal):
    rows = ["| Cặp | Vai trò | Tỉ lệ | Ngưỡng |", "|---|---|---|---|"]
    for fg, bg, role, control in NONTEXT:
        r = cr(pal[fg], pal[bg])
        if control:
            assert r >= 3, (fg, bg, r)
        rows.append(
            f"| `{fg}` {pal[fg]} trên `{bg}` {pal[bg]} | {role} | **{r:.2f}:1** | {'1.4.11' if control else 'trang trí'} |"
        )
    return "\n".join(rows)


def css_name(k):
    return "--" + re.sub(r"([A-Z])", lambda m: "-" + m.group(1).lower(), k)


def write_css():
    s = CSS.read_text(encoding="utf-8")

    def rewrite(block_text, pal, indent):
        for k, v in pal.items():
            n = css_name(k)
            if re.search(rf"{re.escape(n)}:\s*#[0-9a-fA-F]{{6}};", block_text):
                block_text = re.sub(
                    rf"({re.escape(n)}:\s*)#[0-9a-fA-F]{{6}};",
                    rf"\g<1>{v};",
                    block_text,
                )
            else:
                # append after --warn
                block_text = re.sub(
                    r"(--warn:\s*#[0-9a-fA-F]{6};\n)",
                    rf"\g<1>{indent}{n}: {v};\n",
                    block_text,
                    count=1,
                )
        return block_text

    m = re.search(r"(:root\s*\{)(.*?)(\})", s, re.S)
    assert m
    s = s[: m.start(2)] + rewrite(m.group(2), LIGHT, "  ") + s[m.end(2) :]
    m = re.search(
        r"(prefers-color-scheme: dark\s*\)\s*\{\s*:root\s*\{)(.*?)(\})", s, re.S
    )
    assert m
    s = s[: m.start(2)] + rewrite(m.group(2), DARK, "    ") + s[m.end(2) :]
    CSS.write_text(s, encoding="utf-8")


def splice(s, start_marker, end_marker, new_body):
    a = s.index(start_marker) + len(start_marker)
    b = s.index(end_marker, a)
    return s[:a] + "\n\n" + new_body + "\n" + s[b:]


def write_design():
    s = DESIGN.read_text(encoding="utf-8")
    sec = s.index("## Màu, kèm số đo tương phản")
    nt = s.index("## Sàn phi-chữ 3:1")
    head, rest = s[:sec], s[sec:]
    ratios = [cr(p[fg], p[bg]) for p in (LIGHT, DARK) for fg, bg, _ in TEXT_PAIRS]
    intro = (
        f"## Màu, kèm số đo tương phản\n\n{len(ratios)} cặp chữ trên nền mà hệ này thật sự dùng đều được đo, cả trang giấy lẫn bìa sổ. "
        f"Thấp nhất **{min(ratios):.2f}:1**, cao nhất **{max(ratios):.2f}:1**, không cặp nào dưới ngưỡng AA 4.5:1.\n\n"
        'Bảng này chỉ đo **chữ**. Ranh giới của thành phần giao diện đi theo ngưỡng khác và nằm ở mục "Sàn phi-chữ 3:1" bên dưới. '
        "Đọc thiếu mục đó là cách lỗi viền nút 1.21:1 đã lọt qua một lần.\n\n"
        f"### Chế độ sáng\n\n{table_text(LIGHT)}\n\n### Chế độ tối\n\n{table_text(DARK)}\n\n"
    )
    rest = intro + s[nt:]
    s = head + rest
    # non-text tables: replace the two tables under "## Sàn phi-chữ"
    a = s.index("## Sàn phi-chữ 3:1")
    b = s.index("### Tầng thương hiệu", a)
    seg = s[a:b]
    i = seg.index("### Chế độ sáng")
    seg = (
        seg[:i]
        + f"### Chế độ sáng\n\n{table_nontext(LIGHT)}\n\n### Chế độ tối\n\n{table_nontext(DARK)}\n\n"
        "Số của `line` và `coverLine` ghi ra ở đây **chính vì chúng không đạt 3:1**. Người sau đọc bảng này phải thấy ngay chúng đứng ở đâu, thay vì thấy một token không có số rồi dùng nó cho một cái nút. `coverLineStrong` là viền của control đặt trên bìa sổ (Welcome, Login), đo trên cả hai scheme.\n\n"
    )
    s = s[:a] + seg + s[b:]
    DESIGN.write_text(s, encoding="utf-8")


def write_design_json():
    d = json.loads(DJSON.read_text(encoding="utf-8"))
    t = json.loads(TOKENS.read_text(encoding="utf-8"))
    for k in ("color", "brand", "type", "motion", "radius", "space"):
        d[k] = t[k]
    light = [
        {
            "fg": fg,
            "bg": bg,
            "fgHex": LIGHT[fg],
            "bgHex": LIGHT[bg],
            "ratio": round(cr(LIGHT[fg], LIGHT[bg]), 2),
        }
        for fg, bg, _ in TEXT_PAIRS
    ]
    dark = [
        {
            "fg": fg,
            "bg": bg,
            "fgHex": DARK[fg],
            "bgHex": DARK[bg],
            "ratio": round(cr(DARK[fg], DARK[bg]), 2),
        }
        for fg, bg, _ in TEXT_PAIRS
    ]
    allr = [x["ratio"] for x in light + dark]
    d["contrast"].update(
        {
            "pairsChecked": len(allr),
            "min": min(allr),
            "max": max(allr),
            "allPassAA": min(allr) >= 4.5,
            "light": light,
            "dark": dark,
        }
    )
    d["contrast"]["nonText"]["controls"] = {
        m: {f"{fg}/{bg}": round(cr(p[fg], p[bg]), 2) for fg, bg, _, c in NONTEXT if c}
        for m, p in (("light", LIGHT), ("dark", DARK))
    }
    d["measuredFrom"] = (
        "packages/shared/tokens.json là nguồn; script chỉ sinh mirror và số đo. "
        "Các tỷ lệ token không thay thế bằng chứng native của từng component."
    )
    DJSON.write_text(
        json.dumps(d, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


if __name__ == "__main__":
    # Validate all pairs before writing any mirror.
    for palette in (LIGHT, DARK):
        table_text(palette)
        table_nontext(palette)
    write_css()
    write_design()
    write_design_json()
    print("guest.css, DESIGN.md contrast tables, design.json mirrored from tokens.json")
