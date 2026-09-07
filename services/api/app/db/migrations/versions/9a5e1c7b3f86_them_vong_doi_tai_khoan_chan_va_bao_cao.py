"""Vòng đời tài khoản, chặn và báo cáo (L5, ADR-0023).

Ba thay đổi trên `people` và một bảng mới:

- `discoverable_by_phone BOOLEAN NOT NULL DEFAULT true` — tắt thì tra theo số
  trả cùng câu 404 với «không có ai dùng số này» (§2.5).
- `deleted_at TIMESTAMPTZ NULL` — dấu «tài khoản đã kết thúc». Hàng KHÔNG bị
  xoá: `people.id` là khoá ngoại của sổ tiền và bất biến 3 nói số dư phải tính
  lại được mãi mãi (§2.1). Index một phần cho «những ai còn sống», vì mọi
  truy vấn danh tính đều lọc theo nó.
- `notify_prefs JSONB NOT NULL DEFAULT '{}'` — chỗ cho ADR-0024; L5 chỉ tạo
  cột để lát thông báo không phải sửa `people` lần nữa.
- `reports(reporter_id, target_type, target_id, reason, note, created_at)` —
  hai từ vựng đóng bằng CHECK (gương của `app/domain/reports.py`), `note` ≤
  500. Không có bảng trạng thái xử lý: v1 không có màn quản trị (§2.4).

Chặn KHÔNG có bảng riêng: nó là trạng thái `blocked` của cạnh
`friend_requests` đã có (§2.3). Cái duy nhất chặn cần ở tầng này là một index
cho câu hỏi «hai người này có cạnh nào không» theo cả hai chiều — đã có
`ix_friend_requests_requester`/`_addressee` phủ, nên migration này không thêm.

Xuống được: bảng `reports` bị xoá cùng dữ liệu; ba cột rời khỏi `people`.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "9a5e1c7b3f86"
down_revision: str | Sequence[str] | None = "8f4d0b6a2e75"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_TARGET_TYPES = "target_type IN ('person', 'post', 'message', 'comment', 'story')"
_REASONS = "reason IN ('spam', 'harassment', 'inappropriate', 'impersonation', 'other')"
_NOTE_LENGTH = "note IS NULL OR length(note) <= 500"


def upgrade() -> None:
    op.add_column(
        "people",
        sa.Column(
            "discoverable_by_phone",
            sa.Boolean(),
            nullable=False,
            server_default=sa.text("true"),
        ),
    )
    op.add_column(
        "people", sa.Column("deleted_at", sa.DateTime(timezone=True), nullable=True)
    )
    op.add_column(
        "people",
        sa.Column(
            "notify_prefs",
            postgresql.JSONB(astext_type=sa.Text()),
            nullable=False,
            server_default=sa.text("'{}'::jsonb"),
        ),
    )
    op.create_index(
        "ix_people_live",
        "people",
        ["id"],
        postgresql_where=sa.text("deleted_at IS NULL"),
    )

    op.create_table(
        "reports",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("reporter_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("target_type", sa.String(length=16), nullable=False),
        sa.Column("target_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("reason", sa.String(length=24), nullable=False),
        sa.Column("note", sa.Text(), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(_TARGET_TYPES, name=op.f("ck_reports_report_target_known")),
        sa.CheckConstraint(_REASONS, name=op.f("ck_reports_report_reason_known")),
        sa.CheckConstraint(_NOTE_LENGTH, name=op.f("ck_reports_report_note_length")),
        sa.ForeignKeyConstraint(
            ["reporter_id"], ["people.id"], name="fk_reports_reporter"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_reports")),
    )
    op.create_index("ix_reports_target", "reports", ["target_type", "target_id"])
    op.create_index("ix_reports_reporter", "reports", ["reporter_id"])


def downgrade() -> None:
    op.drop_index("ix_reports_reporter", table_name="reports")
    op.drop_index("ix_reports_target", table_name="reports")
    op.drop_table("reports")
    op.drop_index("ix_people_live", table_name="people")
    op.drop_column("people", "notify_prefs")
    op.drop_column("people", "deleted_at")
    op.drop_column("people", "discoverable_by_phone")
