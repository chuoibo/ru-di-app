"""`account_lifecycle` (L5, ADR-0023 §2.1): the closed map, and what a row
looks like after the account ends."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest

from app.domain.account_lifecycle import (
    ANONYMOUS_DISPLAY_NAME,
    ERASURE,
    MONEY_TABLES,
    OTHERS_KEEP_TABLES,
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


def test_the_map_pins_the_branch_of_every_table_whose_branch_is_a_decision():
    """Bản đồ `ERASURE` phải nói đúng NHÁNH, không chỉ có mặt.

    Ca «mọi bảng thật đều có tên trong bản đồ» đếm sự CÓ MẶT. Nó không đọc
    nhánh, nên chuyển một bảng từ «keep» sang «delete» đi qua sạch sẽ. Hai
    người đo độc lập (agy QA và reviewer PR #581) cùng dựng đúng đột biến ấy
    cho `reports` và cả hai đều thấy cổng xanh.

    Lỗ ấy đi hai bước, và bước nào cũng im lặng: xếp một bảng tiền nhầm vào
    «delete», rồi thêm một `wipe()` cho nó — lúc ấy phép khẳng định «mọi bảng
    `erase_person` chạm tới đều nằm trong nhánh delete» vẫn đúng, còn phép so
    md5 thì mù vì bảng ấy không có trong danh sách viết tay. Ca này đóng bước
    thứ nhất, và vì `MONEY_TABLES` giờ sống trong chính module domain, ca
    Postgres và ca này đọc CÙNG MỘT danh sách.
    """
    keep = set(ERASURE["keep"])
    delete = set(ERASURE["delete"])

    thieu_tien = sorted(set(MONEY_TABLES) - keep)
    assert thieu_tien == [], (
        f"bảng tiền không nằm ở nhánh «keep»: {thieu_tien}. Xoá tài khoản không "
        "được chạm một byte nào của sổ tiền (ADR-0023 §2.1)."
    )
    thieu_nguoi_khac = sorted(set(OTHERS_KEEP_TABLES) - keep)
    assert thieu_nguoi_khac == [], (
        f"bảng mang lời của người khác không ở nhánh «keep»: {thieu_nguoi_khac}"
    )

    # Và không bảng nào trong hai danh sách ấy được phép có mặt ở «delete»:
    # một bảng nằm cả hai nhánh là một bản đồ tự mâu thuẫn.
    cham = sorted(delete & (set(MONEY_TABLES) | set(OTHERS_KEEP_TABLES)))
    assert cham == [], f"bảng vừa «giữ» vừa «xoá»: {cham}"


def test_this_case_would_notice_a_table_moved_into_delete():
    """Đối chứng dương: phép so ở trên phải phân biệt được hai bản đồ.

    Không có bước này thì ca trên xanh cả khi `MONEY_TABLES` rỗng, và một danh
    sách nguồn rỗng làm cổng tự tháo trong im lặng.
    """
    assert MONEY_TABLES, "danh sách bảng tiền rỗng — ca trên không đo gì cả"
    assert OTHERS_KEEP_TABLES
    gia_lap_xep_nham = set(ERASURE["keep"]) - {"reports"}
    assert "reports" in set(OTHERS_KEEP_TABLES) - gia_lap_xep_nham, (
        "phép so «tên này có ở nhánh keep không» không phân biệt được gì"
    )
