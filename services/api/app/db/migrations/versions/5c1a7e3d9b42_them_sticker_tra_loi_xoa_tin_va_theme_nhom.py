"""Sticker, trả lời, xoá tin và theme nhóm (L1, ADR-0021 §2.1–2.4).

Bốn thay đổi trên hai bảng, không thay đổi nào chạm tiền:

- `messages.kind` nhận thêm `sticker` (thân là id sticker, slug ASCII) và
  `deleted` (thân, ảnh, thẻ đều NULL). CHECK `payload_matches_kind` được
  dựng lại với hai nhánh mới; CHECK `message_kind` (do enum không-native
  sinh) dựng lại với năm giá trị. Cách «drop rồi create» là bắt buộc: một
  CHECK không sửa tại chỗ được.
- `messages.deleted_at` đi cùng CHECK `(kind = 'deleted') = (deleted_at IS
  NOT NULL)` — cùng hình với `left_state_matches_timestamp` của membership:
  trạng thái và mốc giờ không được rời nhau.
- `messages.reply_to_id` là khoá ngoại KÉP `(reply_to_id, context_id) →
  messages(id, context_id)`, kèm UNIQUE `(id, context_id)` để tham chiếu
  được. Cơ sở dữ liệu tự chặn trả lời chéo nhóm; service không phải tin lời
  khai của client về `context_id` của tin gốc.
- `contexts.theme` là một slug trong bộ đóng năm giá trị; mặc định
  `mac-dinh` để mọi nhóm đang có giữ đúng màu hiện tại.

`human_kinds_have_author` giữ nguyên: tin `deleted` vẫn có tác giả, vì chỉ
`text|image|sticker` của chính mình mới xoá được (ADR-0021 §2.3).

Xuống được: tin `sticker`/`deleted` phải được đưa về dạng cũ trước khi CHECK
cũ quay lại — ở đây chọn xoá hàng `deleted` (nội dung đã không còn) và đổi
`sticker` thành `text` với thân là id, để không mất một tin nào có nội dung.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op

revision: str = "5c1a7e3d9b42"
down_revision: str | Sequence[str] | None = "e1f2a3b4c5d6"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_OLD_KIND = "kind IN ('text', 'image', 'ai_card')"
_NEW_KIND = "kind IN ('text', 'image', 'ai_card', 'sticker', 'deleted')"

_OLD_PAYLOAD = (
    "(kind = 'text' AND body IS NOT NULL AND image_url IS NULL "
    "AND card IS NULL) OR "
    "(kind = 'image' AND image_url IS NOT NULL AND card IS NULL) OR "
    "(kind = 'ai_card' AND card IS NOT NULL AND image_url IS NULL "
    "AND body IS NULL)"
)
_NEW_PAYLOAD = (
    "(kind = 'text' AND body IS NOT NULL AND image_url IS NULL "
    "AND card IS NULL) OR "
    "(kind = 'image' AND image_url IS NOT NULL AND card IS NULL) OR "
    "(kind = 'ai_card' AND card IS NOT NULL AND image_url IS NULL "
    "AND body IS NULL) OR "
    "(kind = 'sticker' AND body ~ '^[a-z0-9-]{1,32}$' AND image_url IS NULL "
    "AND card IS NULL) OR "
    "(kind = 'deleted' AND body IS NULL AND image_url IS NULL "
    "AND card IS NULL)"
)

_THEMES = "theme IN ('mac-dinh', 'hoang-hon', 'bien-dem', 'rung-thong', 'ruc-ro')"


def upgrade() -> None:
    # --- messages: kinds ---------------------------------------------------
    op.drop_constraint(op.f("ck_messages_message_kind"), "messages", type_="check")
    op.create_check_constraint(op.f("ck_messages_message_kind"), "messages", _NEW_KIND)
    op.drop_constraint(
        op.f("ck_messages_payload_matches_kind"), "messages", type_="check"
    )
    op.create_check_constraint(
        op.f("ck_messages_payload_matches_kind"), "messages", _NEW_PAYLOAD
    )

    # --- messages: deletion -------------------------------------------------
    op.add_column(
        "messages",
        sa.Column("deleted_at", sa.DateTime(timezone=True), nullable=True),
    )
    op.create_check_constraint(
        op.f("ck_messages_deleted_state_matches_timestamp"),
        "messages",
        "(kind = 'deleted') = (deleted_at IS NOT NULL)",
    )

    # --- messages: reply ----------------------------------------------------
    op.add_column("messages", sa.Column("reply_to_id", sa.UUID(), nullable=True))
    op.create_unique_constraint(
        op.f("uq_messages_id_context"), "messages", ["id", "context_id"]
    )
    op.create_foreign_key(
        "fk_messages_reply_to",
        "messages",
        "messages",
        ["reply_to_id", "context_id"],
        ["id", "context_id"],
    )
    op.create_index(
        "ix_messages_reply_to",
        "messages",
        ["reply_to_id"],
        unique=False,
        postgresql_where=sa.text("reply_to_id IS NOT NULL"),
    )

    # --- contexts: theme ----------------------------------------------------
    op.add_column(
        "contexts",
        sa.Column(
            "theme",
            sa.String(length=16),
            nullable=False,
            server_default="mac-dinh",
        ),
    )
    op.create_check_constraint(
        op.f("ck_contexts_context_theme_known"), "contexts", _THEMES
    )


def downgrade() -> None:
    op.drop_constraint(
        op.f("ck_contexts_context_theme_known"), "contexts", type_="check"
    )
    op.drop_column("contexts", "theme")

    op.drop_index("ix_messages_reply_to", table_name="messages")
    op.drop_constraint("fk_messages_reply_to", "messages", type_="foreignkey")
    op.drop_constraint(op.f("uq_messages_id_context"), "messages", type_="unique")
    op.drop_column("messages", "reply_to_id")

    # Rows the old CHECK cannot hold: a deleted message has no content left to
    # keep; a sticker keeps its words by becoming text that names the id.
    op.execute("DELETE FROM messages WHERE kind = 'deleted'")
    op.execute("UPDATE messages SET kind = 'text' WHERE kind = 'sticker'")
    op.drop_constraint(
        op.f("ck_messages_deleted_state_matches_timestamp"), "messages", type_="check"
    )
    op.drop_column("messages", "deleted_at")

    op.drop_constraint(
        op.f("ck_messages_payload_matches_kind"), "messages", type_="check"
    )
    op.create_check_constraint(
        op.f("ck_messages_payload_matches_kind"), "messages", _OLD_PAYLOAD
    )
    op.drop_constraint(op.f("ck_messages_message_kind"), "messages", type_="check")
    op.create_check_constraint(op.f("ck_messages_message_kind"), "messages", _OLD_KIND)
