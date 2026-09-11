"""Reproduce the proposed spec's opaque sRGB contrast pairs at its Git revision."""

import json
import subprocess


REVISION = "9c0d6f98a32361d220b3f5cb0e5c8cc8d94242a8"


def luminance(value):
    channels = [int(value[index:index + 2], 16) / 255 for index in (1, 3, 5)]
    linear = [
        channel / 12.92 if channel <= 0.04045 else ((channel + 0.055) / 1.055) ** 2.4
        for channel in channels
    ]
    return sum(channel * weight for channel, weight in zip(linear, (0.2126, 0.7152, 0.0722)))


def contrast(first, second):
    high, low = sorted((luminance(first), luminance(second)), reverse=True)
    return (high + 0.05) / (low + 0.05)


tokens = json.loads(subprocess.check_output(
    ["git", "show", f"{REVISION}:packages/shared/tokens.json"], text=True,
))
rows = []
for scheme in ("light", "dark"):
    colors = tokens["color"][scheme]
    pairs = [
        ("ink/paper", colors["ink"], colors["paper"], 7),
        ("inkFaint/paper", colors["inkFaint"], colors["paper"], 4.5),
        ("paperShade/paper", colors["paperShade"], colors["paper"], 1.25),
        ("inkSoft/paper (repair candidate)", colors["inkSoft"], colors["paper"], 4.5),
    ]
    if scheme == "dark":
        pairs.append(("paper/spec background", colors["paper"], "#1c1f36", 1.9))
    for name, foreground, background, threshold in pairs:
        ratio = contrast(foreground, background)
        rows.append({
            "scheme": scheme, "pair": name, "foreground": foreground,
            "background": background, "ratio": round(ratio, 6),
            "spec_threshold": threshold, "meets_threshold": ratio >= threshold,
        })
print(json.dumps({
    "revision": REVISION,
    "scope": "Opaque sRGB token arithmetic; background #1c1f36 is supplied by the spec, not newly sampled. No native capture or behavioral evidence.",
    "rows": rows,
}, ensure_ascii=False, indent=2))
