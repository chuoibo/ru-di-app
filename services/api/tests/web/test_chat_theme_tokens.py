"""Every chat theme slug the server accepts has a full palette in both schemes,
and every palette clears the contrast floor (ADR-0021 §2.4).

Three places name the five slugs: the domain tuple, the CHECK in the migration,
and `chatTheme` in `packages/shared/tokens.json`. The client's copy is checked
at the repo root (`tests/test_chat_theme_matches_tokens.py`); this file owns
the server ↔ tokens half and the colour arithmetic, because the contrast
helpers live next door in `test_contrast_floor.py`.
"""

from __future__ import annotations

import json
import pathlib
import re
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[2]))

from app.domain.chat_theme import DEFAULT_THEME, THEMES  # noqa: E402


def _relative_luminance(value: str) -> float:
    """WCAG 2.2 relative luminance of a `#rrggbb` colour (same arithmetic as
    `test_contrast_floor.py`, restated here because `tests/web` is not a
    package and cannot import a sibling)."""
    channels = [int(value[i : i + 2], 16) / 255 for i in (1, 3, 5)]
    linear = [
        c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4 for c in channels
    ]
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]


def contrast(foreground: str, background: str) -> float:
    lighter = max(_relative_luminance(foreground), _relative_luminance(background))
    darker = min(_relative_luminance(foreground), _relative_luminance(background))
    return (lighter + 0.05) / (darker + 0.05)


REPO = pathlib.Path(__file__).resolve().parents[4]
TOKENS = json.loads((REPO / "packages/shared/tokens.json").read_text(encoding="utf-8"))
MIGRATION = (
    REPO
    / "services/api/app/db/migrations/versions/"
    / "5c1a7e3d9b42_them_sticker_tra_loi_xoa_tin_va_theme_nhom.py"
)
HEX = re.compile(r"^#[0-9a-f]{6}$")
ROLES = ("bubble", "bubbleInk", "accent")


def _slugs_in_migration() -> tuple[str, ...]:
    text = MIGRATION.read_text(encoding="utf-8")
    match = re.search(r"theme IN \(([^)]*)\)", text)
    assert match, "không thấy CHECK theme trong migration"
    return tuple(re.findall(r"'([a-z-]+)'", match.group(1)))


class ChatThemeTokens(unittest.TestCase):
    def test_tokens_domain_and_migration_name_the_same_slugs(self):
        self.assertEqual(tuple(TOKENS["chatTheme"].keys()), THEMES)
        self.assertEqual(_slugs_in_migration(), THEMES)

    def test_every_slug_has_both_schemes_with_the_three_roles_as_hex(self):
        for slug in THEMES:
            for scheme in ("light", "dark"):
                with self.subTest(slug=slug, scheme=scheme):
                    palette = TOKENS["chatTheme"][slug][scheme]
                    self.assertEqual(tuple(palette.keys()), ROLES)
                    for role in ROLES:
                        self.assertRegex(palette[role], HEX)

    def test_ink_on_bubble_clears_the_text_contrast_floor(self):
        for slug in THEMES:
            for scheme in ("light", "dark"):
                palette = TOKENS["chatTheme"][slug][scheme]
                with self.subTest(slug=slug, scheme=scheme):
                    ratio = contrast(palette["bubbleInk"], palette["bubble"])
                    self.assertGreaterEqual(ratio, 4.5, f"{slug}/{scheme}: {ratio:.2f}")

    def test_the_default_theme_is_the_brand_accent_so_old_screenshots_hold(self):
        for scheme in ("light", "dark"):
            palette = TOKENS["chatTheme"][DEFAULT_THEME][scheme]
            base = TOKENS["color"][scheme]
            self.assertEqual(palette["bubble"], base["accent"])
            self.assertEqual(palette["bubbleInk"], base["accentInk"])
            self.assertEqual(palette["accent"], base["accent"])

    def test_chat_theme_lives_outside_the_colour_palettes(self):
        """`test_shared_tokens` mirrors every `color.*` key into guest.css; the
        guest page draws no chat bubble, so the theme key must not live there."""
        self.assertNotIn("chatTheme", TOKENS["color"]["light"])
        self.assertNotIn("chatTheme", TOKENS["color"]["dark"])


if __name__ == "__main__":
    unittest.main()
