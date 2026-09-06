"""Story 24 giờ (L4, ADR-0022 §2.3).

Hai bảng mới:

- `stories(author_id, image_url, caption, audience, created_at, expires_at)` —
  một ảnh cá nhân (`/people/{id}/photos/{id}`) với chú thích ≤ 200 ký tự, chỉ
  cho bạn bè (`audience IN ('friends')`), hết hạn sau 24 giờ. `expires_at`
  KHÔNG có `server_default`: kỳ hạn do domain
  (`app/domain/story_visibility.py::expires_at_for`) tính từ `created_at` rồi
  ghi xuống; cơ sở dữ liệu không có đồng hồ riêng cho luật này.
  CHECK `expires_at > created_at` là cách viết thứ hai của cùng luật.
  Index `(author_id, expires_at DESC)` cho «story còn hạn của những người là
  bạn tôi»; index `image_url` cho cửa ảnh cá nhân («có story còn hạn nào trỏ
  tới ảnh này mà người đọc xem được không»).
- `story_views(story_id, viewer_id, seen_at)` — khoá chính kép: một người xem
  một story là một hàng, xem lại không thêm hàng. Cascade khi story bị xoá.

Xuống được: hai bảng bị xoá cùng dữ liệu của chúng.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "8f4d0b6a2e75"
down_revision: str | Sequence[str] | None = "7e3c9a5f1d64"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_AUDIENCES = "audience IN ('friends')"
_EXPIRES_AFTER_CREATED = "expires_at > created_at"
_CAPTION_LENGTH = "caption IS NULL OR length(caption) <= 200"


def upgrade() -> None:
    # --- stories ------------------------------------------------------------
    op.create_table(
        "stories",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("author_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("image_url", sa.Text(), nullable=False),
        sa.Column("caption", sa.Text(), nullable=True),
        sa.Column(
            "audience", sa.String(length=8), nullable=False, server_default="friends"
        ),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        # No server default on purpose: see the module docstring.
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint(_AUDIENCES, name=op.f("ck_stories_story_audience_known")),
        sa.CheckConstraint(
            _EXPIRES_AFTER_CREATED, name=op.f("ck_stories_story_expires_after_created")
        ),
        sa.CheckConstraint(
            _CAPTION_LENGTH, name=op.f("ck_stories_story_caption_length")
        ),
        sa.ForeignKeyConstraint(["author_id"], ["people.id"], name="fk_stories_author"),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_stories")),
    )
    op.create_index(
        "ix_stories_author_live",
        "stories",
        ["author_id", sa.text("expires_at DESC")],
    )
    op.create_index("ix_stories_image_url", "stories", ["image_url"])

    # --- story_views --------------------------------------------------------
    op.create_table(
        "story_views",
        sa.Column("story_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("viewer_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("seen_at", sa.DateTime(timezone=True), nullable=False),
        sa.ForeignKeyConstraint(
            ["story_id"],
            ["stories.id"],
            name="fk_story_views_story",
            ondelete="CASCADE",
        ),
        sa.ForeignKeyConstraint(
            ["viewer_id"], ["people.id"], name="fk_story_views_viewer"
        ),
        sa.PrimaryKeyConstraint("story_id", "viewer_id", name=op.f("pk_story_views")),
    )
    op.create_index("ix_story_views_viewer", "story_views", ["viewer_id"])


def downgrade() -> None:
    op.drop_index("ix_story_views_viewer", table_name="story_views")
    op.drop_table("story_views")
    op.drop_index("ix_stories_image_url", table_name="stories")
    op.drop_index("ix_stories_author_live", table_name="stories")
    op.drop_table("stories")
