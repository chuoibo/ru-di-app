"""Nhắn riêng hai người (L2, ADR-0021 §2.5).

`contexts` nhận thêm hai cột và ba ràng buộc; không có bảng mới:

- `kind ∈ ('group', 'pair')`, mặc định `group` để mọi nhóm đang có giữ đúng
  nghĩa cũ mà không cần backfill.
- `pair_key` là hai id người theo thứ tự `friendship.pair_key`, nối bằng `:`.
  UNIQUE, nên hai người chỉ có đúng một cuộc trò chuyện riêng, và cuộc đua
  «hai người cùng mở» giải bằng index rồi đọc lại — không bằng `if exists`.
- CHECK `(kind = 'pair') = (pair_key IS NOT NULL)`: nhóm không mang khoá đôi,
  cặp không thiếu khoá đôi.

Không có bảng tin nhắn riêng thứ hai: tin, ảnh, phản ứng, dấu đọc, Rủ Đi AI
và theme dùng lại nguyên vẹn qua `context_id`. `display_name` của cặp lưu
rỗng; máy chủ điền tên người kia khi trả về (dẫn xuất, không lưu).

Xuống được: cặp trở về nhóm thường mang tên «Nhắn riêng» (tên rỗng là dữ
liệu khó đọc trên bản cũ), rồi ba ràng buộc và hai cột bị gỡ.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "6d2b8f4e0c53"
down_revision: str | Sequence[str] | None = "5c1a7e3d9b42"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_KINDS = "kind IN ('group', 'pair')"
_PAIR_HAS_KEY = "(kind = 'pair') = (pair_key IS NOT NULL)"


def upgrade() -> None:
    op.add_column(
        "contexts",
        sa.Column(
            "kind",
            sa.String(length=8),
            nullable=False,
            server_default="group",
        ),
    )
    op.add_column("contexts", sa.Column("pair_key", sa.Text(), nullable=True))
    op.create_check_constraint(
        op.f("ck_contexts_context_kind_known"), "contexts", _KINDS
    )
    op.create_check_constraint(
        op.f("ck_contexts_context_pair_has_key"), "contexts", _PAIR_HAS_KEY
    )
    op.create_unique_constraint(op.f("uq_contexts_pair_key"), "contexts", ["pair_key"])


def downgrade() -> None:
    op.execute(
        "UPDATE contexts SET display_name = 'Nhắn riêng' "
        "WHERE kind = 'pair' AND display_name = ''"
    )
    op.drop_constraint(op.f("uq_contexts_pair_key"), "contexts", type_="unique")
    op.drop_constraint(
        op.f("ck_contexts_context_pair_has_key"), "contexts", type_="check"
    )
    op.drop_constraint(
        op.f("ck_contexts_context_kind_known"), "contexts", type_="check"
    )
    op.drop_column("contexts", "pair_key")
    op.drop_column("contexts", "kind")
