"""Chat themes (ADR-0021 §2.4): five closed slugs, never a free colour.

The slug is all the server stores (`contexts.theme`, CHECKed against this
tuple). The colours live in `packages/shared/tokens.json` under `chatTheme`
for both schemes, and a web-tier test proves every slug here has a full
light and dark palette that clears the contrast floor. The server never sees
a hex value; a colour that is not in tokens cannot be chosen.
"""

from __future__ import annotations

__all__ = ["DEFAULT_THEME", "THEMES", "is_theme"]

THEMES: tuple[str, ...] = ("mac-dinh", "hoang-hon", "bien-dem", "rung-thong", "ruc-ro")
DEFAULT_THEME = "mac-dinh"


def is_theme(value: object) -> bool:
    return isinstance(value, str) and value in THEMES
