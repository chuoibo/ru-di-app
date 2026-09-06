"""Five chat themes, one default, nothing else (ADR-0021 §2.4)."""

from __future__ import annotations

from typing import get_args

from app.api.schemas import ChatTheme, ContextUpdateRequest
from app.domain.chat_theme import DEFAULT_THEME, THEMES, is_theme


def test_the_default_is_one_of_the_themes():
    assert DEFAULT_THEME in THEMES
    assert is_theme(DEFAULT_THEME)


def test_is_theme_accepts_exactly_the_five_slugs():
    for slug in THEMES:
        assert is_theme(slug)
    assert not is_theme("hong")
    assert not is_theme("#c93900"), "một mã màu không bao giờ là theme"
    assert not is_theme(None)


def test_the_wire_literal_spells_the_same_five_slugs_as_the_domain():
    """Two spellings of one list: the OpenAPI enum and the domain tuple."""
    assert tuple(get_args(ChatTheme)) == THEMES


def test_an_update_needs_at_least_one_field():
    try:
        ContextUpdateRequest()
    except ValueError as exc:
        assert "ít nhất một trường" in str(exc)
    else:
        raise AssertionError("body rỗng phải bị từ chối")
