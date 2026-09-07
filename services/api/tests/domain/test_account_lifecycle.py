"""`account_lifecycle` (L5, ADR-0023 §2.1): the closed map, and what a row
looks like after the account ends."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest

from app.domain.account_lifecycle import (
    ANONYMOUS_DISPLAY_NAME,
    ERASURE,
    AccountLifecycleError,
    anonymised_person,
    check_confirmation,
    tables_for,
)

NOW = datetime(2026, 9, 6, 12, tzinfo=UTC)
PERSON = {
    "id": "person-1",
    "display_name": "Minh",
    "bio": "thích cà phê",
    "city": "Đà Lạt",
    "budget_band": "vua-phai",
    "discoverable_by_phone": True,
    "wall_comment_policy": "readers",
    "created_at": NOW,
    "deleted_at": None,
}


def test_no_table_is_named_twice_and_money_is_kept():
    named = [table for group in ERASURE.values() for table in group]
    assert len(named) == len(set(named)), "một bảng hai chỗ là hai lệnh mâu thuẫn"
    for money in (
        "expenses",
        "expense_versions",
        "confirmed_allocations",
        "collection_batches",
        "collection_obligations",
        "payment_reports",
        "receipt_confirmations",
    ):
        assert money in ERASURE["keep"], money


def test_what_gets_deleted_is_the_persons_own_writing_about_themselves():
    for table in ("posts", "stories", "friend_requests", "account_identities"):
        assert table in ERASURE["delete"], table
    # Messages stay: a group's history is the group's.
    assert "messages" in ERASURE["keep"]
    assert "memberships" in ERASURE["leave"]
    assert ERASURE["anonymise"] == ("people",)


def test_an_unknown_action_is_refused_rather_than_read_as_nothing():
    assert tables_for("revoke") == ("account_sessions",)
    with pytest.raises(AccountLifecycleError) as refused:
        tables_for("purge")
    assert refused.value.code == "UNKNOWN_ERASURE_ACTION"


@pytest.mark.parametrize("confirm", [None, False, "true", 1, "", {}])
def test_only_a_literal_true_confirms(confirm):
    with pytest.raises(AccountLifecycleError) as refused:
        check_confirmation(confirm)
    assert refused.value.code == "CONFIRM_REQUIRED"


def test_a_true_confirmation_passes():
    assert check_confirmation(True) is None


def test_the_row_keeps_its_id_and_loses_everything_that_names_somebody():
    after = anonymised_person(PERSON, NOW)
    assert after["id"] == PERSON["id"], "sổ tiền trỏ vào id này"
    assert after["created_at"] == PERSON["created_at"]
    assert after["display_name"] == ANONYMOUS_DISPLAY_NAME
    assert after["bio"] is None and after["city"] is None
    assert after["budget_band"] is None
    assert after["discoverable_by_phone"] is False
    assert after["wall_comment_policy"] == "nobody"
    assert after["deleted_at"] == NOW
    assert PERSON["display_name"] == "Minh", "hàm thuần: không sửa dict của người gọi"


def test_a_naive_clock_is_refused():
    with pytest.raises(AccountLifecycleError) as refused:
        anonymised_person(PERSON, datetime(2026, 9, 6, 12))
    assert refused.value.code == "NAIVE_DATETIME"
