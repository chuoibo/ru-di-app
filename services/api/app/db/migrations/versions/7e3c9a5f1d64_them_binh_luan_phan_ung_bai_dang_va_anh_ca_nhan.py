"""Bình luận, phản ứng trên bài đăng và ảnh cá nhân (L3, ADR-0022 §2.1–2.2).

Hai bảng mới theo đúng mẫu của kỷ niệm (`memory_reactions`, `memory_comments`):

- `post_reactions(post_id, person_id, kind)` — `kind` là sáu loại của tin nhắn,
  UNIQUE `(post_id, person_id, kind)`; không có cột đếm lưu sẵn (bài học «3 tim
  × 2 bình luận không ra 6»: đếm bằng hai GROUP BY riêng).
- `post_comments(post_id, author_id, body)` — thân không rỗng, xếp «cũ trước».
  Cả hai cascade khi bài bị xoá.

Ba thay đổi trên bảng đang có:

- `people.wall_comment_policy ∈ ('readers', 'friends', 'nobody')`, mặc định
  `readers`: chủ tường quyết ai được bình luận trên bài của mình.
- `uploaded_images.purpose ∈ ('group', 'avatar', 'personal')`: backfill
  `owner_person_id IS NOT NULL → 'avatar'` TRƯỚC khi thêm CHECK, vì cột mặc
  định `group` sẽ làm mọi ảnh đại diện đang có sai CHECK «purpose = group ⇔ có
  context_id». Không có `purpose`, ảnh vừa đăng bài sẽ thành ảnh đại diện
  (`get_latest_avatar` lấy ảnh mới nhất của người).
- `posts.image_url` có index một phần: luật đọc ảnh cá nhân hỏi «có bài nào
  trỏ tới ảnh này mà người đọc đọc được không».

Xuống được: gỡ index, CHECK và cột; hai bảng bị xoá cùng dữ liệu của chúng.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "7e3c9a5f1d64"
down_revision: str | Sequence[str] | None = "6d2b8f4e0c53"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

_REACTION_KINDS = "kind IN ('heart', 'haha', 'like', 'wow', 'sad', 'fire')"
_POLICIES = "wall_comment_policy IN ('readers', 'friends', 'nobody')"
_PURPOSES = "purpose IN ('group', 'avatar', 'personal')"
_PURPOSE_OWNER = "(purpose = 'group') = (context_id IS NOT NULL)"


def upgrade() -> None:
    # --- post_reactions -----------------------------------------------------
    op.create_table(
        "post_reactions",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("post_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("kind", sa.String(length=16), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            _REACTION_KINDS, name=op.f("ck_post_reactions_post_reaction_kind_known")
        ),
        sa.ForeignKeyConstraint(
            ["post_id"], ["posts.id"], name="fk_post_reactions_post", ondelete="CASCADE"
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_post_reactions_person"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_post_reactions")),
        sa.UniqueConstraint(
            "post_id", "person_id", "kind", name="uq_post_reactions_one_per_kind"
        ),
    )
    op.create_index("ix_post_reactions_post", "post_reactions", ["post_id"])

    # --- post_comments ------------------------------------------------------
    op.create_table(
        "post_comments",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("post_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("author_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("body", sa.Text(), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            "body <> ''", name=op.f("ck_post_comments_post_comment_body_not_blank")
        ),
        sa.ForeignKeyConstraint(
            ["author_id"], ["people.id"], name="fk_post_comments_author"
        ),
        sa.ForeignKeyConstraint(
            ["post_id"], ["posts.id"], name="fk_post_comments_post", ondelete="CASCADE"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_post_comments")),
    )
    op.create_index(
        "ix_post_comments_post", "post_comments", ["post_id", "created_at", "id"]
    )

    # --- people: wall_comment_policy ----------------------------------------
    op.add_column(
        "people",
        sa.Column(
            "wall_comment_policy",
            sa.String(length=8),
            nullable=False,
            server_default="readers",
        ),
    )
    op.create_check_constraint(
        op.f("ck_people_wall_comment_policy_known"), "people", _POLICIES
    )

    # --- uploaded_images: purpose (backfill BEFORE the CHECK) ---------------
    op.add_column(
        "uploaded_images",
        sa.Column(
            "purpose", sa.String(length=8), nullable=False, server_default="group"
        ),
    )
    op.execute(
        "UPDATE uploaded_images SET purpose = 'avatar' "
        "WHERE owner_person_id IS NOT NULL"
    )
    op.create_check_constraint(
        op.f("ck_uploaded_images_image_purpose_known"), "uploaded_images", _PURPOSES
    )
    op.create_check_constraint(
        op.f("ck_uploaded_images_image_purpose_matches_owner"),
        "uploaded_images",
        _PURPOSE_OWNER,
    )
    op.drop_index("ix_uploaded_images_avatar", table_name="uploaded_images")
    op.create_index(
        "ix_uploaded_images_avatar",
        "uploaded_images",
        ["owner_person_id", sa.text("created_at DESC")],
        postgresql_where=sa.text("owner_person_id IS NOT NULL AND purpose = 'avatar'"),
    )
    op.create_index(
        "ix_uploaded_images_personal",
        "uploaded_images",
        ["owner_person_id", sa.text("created_at DESC")],
        postgresql_where=sa.text("purpose = 'personal'"),
    )

    # --- posts: the lookup the personal-photo gate makes --------------------
    op.create_index(
        "ix_posts_image_url",
        "posts",
        ["image_url"],
        postgresql_where=sa.text("image_url IS NOT NULL"),
    )


def downgrade() -> None:
    op.drop_index("ix_posts_image_url", table_name="posts")
    op.drop_index("ix_uploaded_images_personal", table_name="uploaded_images")
    op.drop_index("ix_uploaded_images_avatar", table_name="uploaded_images")
    op.create_index(
        "ix_uploaded_images_avatar",
        "uploaded_images",
        ["owner_person_id", sa.text("created_at DESC")],
        postgresql_where=sa.text("owner_person_id IS NOT NULL"),
    )
    op.drop_constraint(
        op.f("ck_uploaded_images_image_purpose_matches_owner"),
        "uploaded_images",
        type_="check",
    )
    op.drop_constraint(
        op.f("ck_uploaded_images_image_purpose_known"),
        "uploaded_images",
        type_="check",
    )
    op.drop_column("uploaded_images", "purpose")
    op.drop_constraint(
        op.f("ck_people_wall_comment_policy_known"), "people", type_="check"
    )
    op.drop_column("people", "wall_comment_policy")
    op.drop_index("ix_post_comments_post", table_name="post_comments")
    op.drop_table("post_comments")
    op.drop_index("ix_post_reactions_post", table_name="post_reactions")
    op.drop_table("post_reactions")
