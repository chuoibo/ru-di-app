"""Mỗi khoản chi thuộc nhiều nhất một kèo (ADR-0054, QA UI-149).

Trước bản này khoản chi được gán vào kèo theo ngày: kèo nào phủ ngày của khoản
chi thì «đã chia» của kèo đó tính khoản ấy. Hai kèo trùng ngày cùng tính một
bữa tối, và hero quyết toán ghi 27.411.356đ cho một sổ chỉ có 13.705.678đ.

Bản này thêm `expenses.outing_id`:

- khoá ngoại ghép `(outing_id, context_id) -> outings(id, context_id)`: một
  khoản chi không thể thuộc kèo của nhóm khác;
- không `ON DELETE`: kèo đang giữ khoản chi thì không xoá được, tiền không mất
  hay đi theo một kèo bị xoá;
- `uq_outings_id_context_id`: đích của khoá ghép.

Backfill theo đúng luật 2 của ADR-0054 §2.2, áp cho phiên bản mới nhất của mỗi
khoản chi: kèo DUY NHẤT của nhóm phủ ngày Việt Nam của `occurred_at`. Hai kèo
trở lên hoặc không kèo nào thì để NULL -- không đoán. Khoản chi chưa xác nhận
(chưa có phiên bản) để NULL; lần xác nhận đầu áp cùng luật.

Xuống được: bỏ cột, index và hai ràng buộc. Quy thuộc mất theo, và recap lại
đọc theo ngày ở mã cũ.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "d5e1a7c3b902"
down_revision: str | Sequence[str] | None = "f4b8d1c6e2a7"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# The ledger's calendar, as `_wall_clock_date` and Go's `wallClockDate`.
VUNG_GIO = "Asia/Ho_Chi_Minh"

# ADR-0054 §2.2 rule 2 on what is already in the ledger: each expense's newest
# version, the one trip of its group covering that Vietnam day, else none. A
# constant so tests/postgres can run the very statement the upgrade runs.
BACKFILL = f"""
        WITH moi_nhat AS (
            SELECT DISTINCT ON (v.expense_id)
                   v.expense_id,
                   CAST(timezone('{VUNG_GIO}', v.occurred_at) AS DATE) AS ngay
              FROM expense_versions v
             ORDER BY v.expense_id, v.version_number DESC
        ), phu AS (
            SELECT e.id AS expense_id,
                   min(o.id::text) AS outing_id,
                   count(*) AS so_keo
              FROM expenses e
              JOIN moi_nhat m ON m.expense_id = e.id
              JOIN outings o
                ON o.context_id = e.context_id
               AND m.ngay BETWEEN o.starts_on AND o.ends_on
             GROUP BY e.id
        )
        UPDATE expenses e
           SET outing_id = phu.outing_id::uuid
          FROM phu
         WHERE phu.expense_id = e.id
           AND phu.so_keo = 1
"""


def upgrade() -> None:
    op.create_unique_constraint(
        "uq_outings_id_context_id", "outings", ["id", "context_id"]
    )
    op.add_column(
        "expenses", sa.Column("outing_id", postgresql.UUID(as_uuid=True), nullable=True)
    )
    op.create_foreign_key(
        "fk_expenses_outing_context",
        "expenses",
        "outings",
        ["outing_id", "context_id"],
        ["id", "context_id"],
    )
    op.create_index("ix_expenses_outing_id", "expenses", ["outing_id"])
    op.execute(BACKFILL)


def downgrade() -> None:
    op.drop_index("ix_expenses_outing_id", table_name="expenses")
    op.drop_constraint("fk_expenses_outing_context", "expenses", type_="foreignkey")
    op.drop_column("expenses", "outing_id")
    op.drop_constraint("uq_outings_id_context_id", "outings", type_="unique")
