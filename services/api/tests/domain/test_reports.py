"""`reports` (L5, ADR-0023 §2.4): two closed vocabularies and one length."""

from __future__ import annotations

import pytest

from app.domain.reports import (
    MAX_NOTE_LENGTH,
    REASONS,
    TARGET_TYPES,
    ReportError,
    validate_report,
)


def test_the_vocabularies_are_the_ones_the_adr_names():
    assert TARGET_TYPES == ("person", "post", "message", "comment", "story")
    assert REASONS == ("spam", "harassment", "inappropriate", "impersonation", "other")


@pytest.mark.parametrize("target_type", TARGET_TYPES)
@pytest.mark.parametrize("reason", REASONS)
def test_every_declared_combination_is_accepted(target_type, reason):
    assert validate_report(target_type=target_type, reason=reason) == {
        "target_type": target_type,
        "reason": reason,
        "note": None,
    }


@pytest.mark.parametrize(
    ("field", "value", "code"),
    [
        ("target_type", "trip", "UNKNOWN_TARGET_TYPE"),
        ("target_type", None, "UNKNOWN_TARGET_TYPE"),
        ("reason", "vi phạm", "UNKNOWN_REASON"),
        ("reason", 7, "UNKNOWN_REASON"),
    ],
)
def test_a_word_outside_the_vocabulary_is_refused(field, value, code):
    body = {"target_type": "post", "reason": "spam", field: value}
    with pytest.raises(ReportError) as refused:
        validate_report(**body)
    assert refused.value.code == code


def test_the_note_is_optional_trimmed_and_bounded():
    assert validate_report(target_type="post", reason="spam", note="  ")["note"] is None
    assert (
        validate_report(target_type="post", reason="spam", note=" nội dung xấu ")[
            "note"
        ]
        == "nội dung xấu"
    )
    longest = "x" * MAX_NOTE_LENGTH
    assert (
        validate_report(target_type="post", reason="other", note=longest)["note"]
        == longest
    )
    with pytest.raises(ReportError) as too_long:
        validate_report(
            target_type="post", reason="other", note="x" * (MAX_NOTE_LENGTH + 1)
        )
    assert too_long.value.code == "NOTE_TOO_LONG"
    with pytest.raises(ReportError) as not_text:
        validate_report(target_type="post", reason="other", note=12)
    assert not_text.value.code == "NOTE_NOT_TEXT"
