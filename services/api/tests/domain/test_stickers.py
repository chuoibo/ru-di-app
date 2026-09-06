"""The sticker vocabulary is closed, well-formed, and the only thing `is_sticker`
accepts (ADR-0021 §2.1)."""

from __future__ import annotations

import re

from app.domain.stickers import STICKER_ID_PATTERN, STICKER_IDS, STICKERS, is_sticker


def test_every_sticker_id_is_an_ascii_slug_the_database_check_accepts():
    for sticker in STICKERS:
        assert STICKER_ID_PATTERN.match(sticker.id), sticker.id
        assert re.match(r"^[a-z0-9-]{1,32}$", sticker.id), sticker.id


def test_ids_are_unique_and_labels_are_not_blank():
    ids = [sticker.id for sticker in STICKERS]
    assert len(ids) == len(set(ids)) == len(STICKER_IDS)
    for sticker in STICKERS:
        assert sticker.label.strip(), sticker.id
        assert "—" not in sticker.label, "không dùng gạch dài trong nhãn"


def test_no_id_looks_like_a_number_the_repo_guard_could_mistake_for_an_account():
    for sticker in STICKERS:
        assert not re.search(r"\d{4,}", sticker.id), sticker.id


def test_is_sticker_accepts_the_vocabulary_and_nothing_else():
    for sticker in STICKERS:
        assert is_sticker(sticker.id)
    assert not is_sticker("khong-co-trong-bo")  # well-formed but unknown
    assert not is_sticker("DI-THOI")  # case is part of the id
    assert not is_sticker("")
    assert not is_sticker(None)
    assert not is_sticker(42)
