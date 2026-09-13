"""Sổ hai người và tờ giấy có phiên bản (ADR-0027, spec «Nếp truyền giấy»).

Mười ba bảng mới, tất cả treo vào MỘT context `kind = 'pair'`, và đúng một
thay đổi trên bảng dùng chung:

- `contexts` thêm `UNIQUE (id, kind)` — **đích** của khoá ngoại ghép. Bảng con
  chỉ mang `context_id` cộng CHECK `context_kind = 'pair'` thì không chứng minh
  được gì: CHECK nói về cột của chính bảng con, nên một hàng vẫn có thể trỏ vào
  một NHÓM rồi viết 'pair' bên cạnh. Trỏ `(context_id, context_kind)` vào
  `(id, kind)` mới là cái database từ chối được. Unique này dư theo định nghĩa
  (`id` đã là khoá chính) và vô hại đúng vì thế.
- `pair_notebooks` — một sổ cho một cuộc trò chuyện, `context_id` UNIQUE.
- `pair_notebook_cycles` — một quãng «sổ này đang mở»; **partial unique** cho
  «nhiều nhất một chu kỳ chưa đóng mỗi sổ». Đóng sổ không xoá (§7.6); mở lại là
  chu kỳ MỚI, nên đồng ý cho năm ngoái không tự cho phép sổ mở lại năm nay.
- `pair_cycle_participants` — hai người của chu kỳ, ghi xuống chứ không suy từ
  membership: membership đổi được, còn đồng ý phải giữ nguyên tên người đã cho.
- `pair_consent_proposals` + `pair_consents` — thang đồng ý là (mục đích × người
  × phiên bản điều khoản × chu kỳ). `expires_at` **không có server default**:
  con số là của domain, viết lần hai bằng SQL là viết ở chỗ không ca nào đứng ở
  biên chạm tới (cùng lập trường `stories.expires_at`). Thu hồi ghi mốc chứ
  không xoá hàng: «đã đồng ý rồi thôi» khác «chưa từng đồng ý».
- `active_couple_members` — khoá chính là **NGƯỜI** (K2), nên «một người chỉ có
  một sổ đôi» là điều database từ chối, không phải điều service nhớ kiểm.
- `pair_papers` — một tờ một tuần; **partial unique** cho «một tờ đang mở mỗi
  cuộc trò chuyện» (§15.1), vì luật chỉ sống trong service thì đúng lúc hai
  request tới cùng nhau là lúc nó thôi đúng. `da_di` mang người ghi nhận và mốc
  (§3.1), CHECK buộc hai cột đi cùng nhau.
- `pair_paper_versions` — khoá `(paper_id, version)` vì mọi thứ khác trỏ vào
  đúng cặp ấy; một id thay thế sẽ cho phép trỏ sang phiên bản của tờ khác mà
  database không thấy. Trigger T1 làm phiên bản ĐÃ GỬI bất biến.
- `pair_paper_views` (nhìn ≠ trả lời, §3.3 luật 4), `pair_paper_responses`
  (**partial unique** một «ừ» mỗi người mỗi phiên bản), `pair_paper_outings`
  (khoá chính là TỜ — K3: khoá theo cặp tờ-phiên bản vẫn sinh được hai outing),
  `pair_paper_keeps` (CHECK dòng không rỗng), `pair_shared_constraints`.

Bốn trigger, mỗi cái có một ca ĐỎ trong `tests/postgres` — trigger không có ca
đỏ là trang trí:

- `pair_paper_versions_immutable` (T1) — UPDATE/DELETE một phiên bản đã gửi bị
  từ chối. **Không có cửa thoát** `current_setting`: kế hoạch có đề xuất một
  cái cho đường xoá, nhưng không đường nào của lát 1 cần (xoá tài khoản GIỮ
  phiên bản, xem `account_lifecycle.ERASURE`), và một cửa thoát không ai dùng
  là một lỗ không có lý do.
- `pair_paper_outing_link_valid` (T2, DEFERRED) — tờ phải `chot`, phiên bản
  phải là phiên bản hiện tại, outing phải thuộc đúng cuộc trò chuyện ấy. Hoãn
  tới cuối giao dịch để service ghi link và đổi trạng thái theo thứ tự nào cũng
  được trong MỘT giao dịch.
- `active_couple_needs_two_consents` (T3, DEFERRED) — hàng «đang là một đôi»
  chỉ tồn tại khi cả hai người của chu kỳ đang giữ đồng ý `bat_doi` còn hiệu
  lực. Đếm theo `DISTINCT person_id`: hai đồng ý của cùng một người không phải
  là hai người.
- `pair_paper_response_is_participant` (T4) — người trả lời phải đang ở trong
  cuộc trò chuyện ấy.

Xuống được: bốn trigger và bốn hàm bị bỏ, mười ba bảng bị xoá ngược thứ tự khoá
ngoại (dữ liệu trong chúng mất), và `uq_contexts_id_kind` rời khỏi `contexts`.
"""

from collections.abc import Sequence

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision: str = "c4f27a90d1e3"
down_revision: str | Sequence[str] | None = "9a5e1c7b3f86"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

# Chép nguyên văn từ `app/db/models.py`, và models chép nguyên văn từ
# `app/domain/pair_paper.py` / `pair_notebook.py`. Ba cách đánh vần một từ vựng.
_PAPER_STATES = (
    "state IN ('nhap', 'da_gui', 'da_xem', 'de_nghi_sua', 'dong_y', 'chot', "
    "'da_di', 'da_giu', 'nghi_tuan', 'het_han', 'rut', 'bo', 'huy')"
)
_PAPER_OPEN = "state IN ('nhap', 'da_gui', 'da_xem', 'de_nghi_sua', 'dong_y')"
_AUTHOR_TYPES = "author_type IN ('human', 'nep')"
_CYCLE_STATES = "state IN ('pending', 'active', 'closed')"
_PURPOSES = "purpose IN ('lap_so', 'bat_doi', 'doc_chat')"
_RESPONSE_KINDS = "kind IN ('dong_y', 'de_nghi_sua')"
_CONSTRAINT_KINDS = "kind IN ('khong_an_duoc', 'dung')"

# Không có hàm phụ dựng cột trong file này, dù nó gọn hơn nhiều: cổng parity
# (`tests/db/test_migration_matches_models.py`) đọc file này bằng AST và lấy tên
# cột từ đối số đầu của `sa.Column(...)`. Một hàm phụ giấu tên cột khỏi cổng, và
# cổng ấy là thứ duy nhất giữ migration và models không trôi khỏi nhau. Bản đầu
# của migration này có hàm phụ và cổng đỏ đúng mười ba bảng.


def upgrade() -> None:
    op.create_unique_constraint("uq_contexts_id_kind", "contexts", ["id", "kind"])

    op.create_table(
        "pair_notebooks",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("context_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "context_kind", sa.String(length=8), server_default="pair", nullable=False
        ),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            "context_kind = 'pair'",
            name=op.f("ck_pair_notebooks_notebook_context_is_pair"),
        ),
        sa.ForeignKeyConstraint(
            ["context_id", "context_kind"],
            ["contexts.id", "contexts.kind"],
            name="fk_pair_notebooks_context",
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_notebooks")),
        sa.UniqueConstraint("context_id", name="uq_pair_notebooks_context"),
    )

    op.create_table(
        "pair_notebook_cycles",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("notebook_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "state", sa.String(length=8), server_default="pending", nullable=False
        ),
        sa.Column(
            "terms_version", sa.Integer(), server_default=sa.text("1"), nullable=False
        ),
        sa.Column("opened_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("closed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            _CYCLE_STATES, name=op.f("ck_pair_notebook_cycles_cycle_state_known")
        ),
        sa.CheckConstraint(
            "(state = 'closed') = (closed_at IS NOT NULL)",
            name=op.f("ck_pair_notebook_cycles_cycle_closed_matches_timestamp"),
        ),
        sa.ForeignKeyConstraint(
            ["notebook_id"],
            ["pair_notebooks.id"],
            name="fk_pair_notebook_cycles_notebook",
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_notebook_cycles")),
    )
    op.create_index(
        "uq_pair_cycles_open_per_notebook",
        "pair_notebook_cycles",
        ["notebook_id"],
        unique=True,
        postgresql_where=sa.text("state <> 'closed'"),
    )

    op.create_table(
        "pair_cycle_participants",
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"],
            ["pair_notebook_cycles.id"],
            name="fk_pair_cycle_participants_cycle",
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_pair_cycle_participants_person"
        ),
        sa.PrimaryKeyConstraint(
            "cycle_id", "person_id", name=op.f("pk_pair_cycle_participants")
        ),
    )

    op.create_table(
        "pair_consent_proposals",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("purpose", sa.String(length=16), nullable=False),
        sa.Column("proposed_by_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "terms_version", sa.Integer(), server_default=sa.text("1"), nullable=False
        ),
        sa.Column("completed_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        # Không server default: con số là của domain. Xem docstring.
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint(
            _PURPOSES, name=op.f("ck_pair_consent_proposals_consent_purpose_known")
        ),
        sa.CheckConstraint(
            "expires_at > created_at",
            name=op.f("ck_pair_consent_proposals_consent_expires_after_created"),
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"],
            ["pair_notebook_cycles.id"],
            name="fk_pair_consent_proposals_cycle",
        ),
        sa.ForeignKeyConstraint(
            ["proposed_by_id"],
            ["people.id"],
            name="fk_pair_consent_proposals_proposed_by",
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_consent_proposals")),
    )
    op.create_index(
        "ix_pair_consent_proposals_cycle",
        "pair_consent_proposals",
        ["cycle_id", "purpose"],
    )

    op.create_table(
        "pair_consents",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("proposal_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("granted_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("revoked_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            "revoked_at IS NULL OR granted_at IS NOT NULL",
            name=op.f("ck_pair_consents_consent_revoke_needs_grant"),
        ),
        sa.ForeignKeyConstraint(
            ["proposal_id"],
            ["pair_consent_proposals.id"],
            name="fk_pair_consents_proposal",
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_pair_consents_person"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_consents")),
        sa.UniqueConstraint(
            "proposal_id", "person_id", name="uq_pair_consents_proposal"
        ),
    )

    op.create_table(
        "active_couple_members",
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "since",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"],
            ["pair_notebook_cycles.id"],
            name="fk_active_couple_members_cycle",
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_active_couple_members_person"
        ),
        sa.PrimaryKeyConstraint("person_id", name=op.f("pk_active_couple_members")),
    )
    op.create_index(
        "ix_active_couple_members_cycle", "active_couple_members", ["cycle_id"]
    )

    op.create_table(
        "pair_papers",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("context_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "context_kind", sa.String(length=8), server_default="pair", nullable=False
        ),
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column(
            "is_temporary",
            sa.Boolean(),
            server_default=sa.text("false"),
            nullable=False,
        ),
        sa.Column("draft_owner_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("state", sa.String(length=16), server_default="nhap", nullable=False),
        sa.Column(
            "current_version", sa.Integer(), server_default=sa.text("1"), nullable=False
        ),
        sa.Column("tuan", sa.Date(), nullable=False),
        sa.Column("done_recorded_by_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("done_recorded_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.Column("expires_at", sa.DateTime(timezone=True), nullable=False),
        sa.CheckConstraint(
            "context_kind = 'pair'", name=op.f("ck_pair_papers_paper_context_is_pair")
        ),
        sa.CheckConstraint(
            _PAPER_STATES, name=op.f("ck_pair_papers_paper_state_known")
        ),
        sa.CheckConstraint(
            "current_version >= 1", name=op.f("ck_pair_papers_paper_version_positive")
        ),
        sa.CheckConstraint(
            "(cycle_id IS NULL) = is_temporary",
            name=op.f("ck_pair_papers_paper_temporary_has_no_cycle"),
        ),
        sa.CheckConstraint(
            "(done_recorded_by_id IS NULL) = (done_recorded_at IS NULL)",
            name=op.f("ck_pair_papers_paper_done_has_recorder"),
        ),
        sa.ForeignKeyConstraint(
            ["context_id", "context_kind"],
            ["contexts.id", "contexts.kind"],
            name="fk_pair_papers_context",
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"], ["pair_notebook_cycles.id"], name="fk_pair_papers_cycle"
        ),
        sa.ForeignKeyConstraint(
            ["draft_owner_id"], ["people.id"], name="fk_pair_papers_draft_owner"
        ),
        sa.ForeignKeyConstraint(
            ["done_recorded_by_id"],
            ["people.id"],
            name="fk_pair_papers_done_recorded_by",
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_papers")),
    )
    op.create_index(
        "uq_pair_papers_open_per_context",
        "pair_papers",
        ["context_id"],
        unique=True,
        postgresql_where=sa.text(_PAPER_OPEN),
    )
    op.create_index(
        "ix_pair_papers_context_week",
        "pair_papers",
        ["context_id", sa.text("tuan DESC")],
    )

    op.create_table(
        "pair_paper_versions",
        sa.Column("paper_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column(
            "content",
            postgresql.JSONB(astext_type=sa.Text()),
            server_default=sa.text("'{}'::jsonb"),
            nullable=False,
        ),
        sa.Column("ly_do", sa.Text(), nullable=True),
        sa.Column(
            "nguon",
            postgresql.JSONB(astext_type=sa.Text()),
            server_default=sa.text("'{}'::jsonb"),
            nullable=False,
        ),
        sa.Column(
            "author_type", sa.String(length=8), server_default="human", nullable=False
        ),
        sa.Column("sent_at", sa.DateTime(timezone=True), nullable=True),
        sa.Column("sent_by", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            _AUTHOR_TYPES, name=op.f("ck_pair_paper_versions_version_author_known")
        ),
        sa.CheckConstraint(
            "version >= 1", name=op.f("ck_pair_paper_versions_version_positive")
        ),
        sa.CheckConstraint(
            "(sent_at IS NULL) = (author_type = 'human' AND sent_by IS NULL)",
            name=op.f("ck_pair_paper_versions_version_sent_by_human_has_sender"),
        ),
        sa.ForeignKeyConstraint(
            ["paper_id"], ["pair_papers.id"], name="fk_pair_paper_versions_paper"
        ),
        sa.ForeignKeyConstraint(
            ["sent_by"], ["people.id"], name="fk_pair_paper_versions_sent_by"
        ),
        sa.PrimaryKeyConstraint(
            "paper_id", "version", name=op.f("pk_pair_paper_versions")
        ),
    )

    op.create_table(
        "pair_paper_views",
        sa.Column("paper_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "seen_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.ForeignKeyConstraint(
            ["paper_id", "version"],
            ["pair_paper_versions.paper_id", "pair_paper_versions.version"],
            name="fk_pair_paper_views_version",
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_pair_paper_views_person"
        ),
        sa.PrimaryKeyConstraint(
            "paper_id", "version", "person_id", name=op.f("pk_pair_paper_views")
        ),
    )

    op.create_table(
        "pair_paper_responses",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("paper_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("kind", sa.String(length=16), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            _RESPONSE_KINDS, name=op.f("ck_pair_paper_responses_response_kind_known")
        ),
        sa.ForeignKeyConstraint(
            ["paper_id", "version"],
            ["pair_paper_versions.paper_id", "pair_paper_versions.version"],
            name="fk_pair_paper_responses_version",
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_pair_paper_responses_person"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_paper_responses")),
    )
    op.create_index(
        "uq_pair_responses_agree_per_person",
        "pair_paper_responses",
        ["paper_id", "version", "person_id"],
        unique=True,
        postgresql_where=sa.text("kind = 'dong_y'"),
    )

    op.create_table(
        "pair_paper_outings",
        sa.Column("paper_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("version", sa.Integer(), nullable=False),
        sa.Column("outing_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "linked_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.ForeignKeyConstraint(
            ["paper_id", "version"],
            ["pair_paper_versions.paper_id", "pair_paper_versions.version"],
            name="fk_pair_paper_outings_version",
        ),
        sa.ForeignKeyConstraint(
            ["outing_id"], ["outings.id"], name="fk_pair_paper_outings_outing"
        ),
        sa.PrimaryKeyConstraint("paper_id", name=op.f("pk_pair_paper_outings")),
        sa.UniqueConstraint("outing_id", name="uq_pair_paper_outings_outing"),
    )

    op.create_table(
        "pair_paper_keeps",
        sa.Column("id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("paper_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("person_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("line", sa.Text(), nullable=False),
        sa.Column(
            "created_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            # `btrim` mặc định chỉ cắt dấu cách, nên `length(btrim(line)) > 0`
            # vẫn cho một dòng toàn xuống dòng đi qua.
            "line ~ '[^[:space:]]'",
            name=op.f("ck_pair_paper_keeps_keep_line_not_blank"),
        ),
        sa.ForeignKeyConstraint(
            ["paper_id"], ["pair_papers.id"], name="fk_pair_paper_keeps_paper"
        ),
        sa.ForeignKeyConstraint(
            ["person_id"], ["people.id"], name="fk_pair_paper_keeps_person"
        ),
        sa.PrimaryKeyConstraint("id", name=op.f("pk_pair_paper_keeps")),
    )
    op.create_index("ix_pair_paper_keeps_paper", "pair_paper_keeps", ["paper_id"])

    op.create_table(
        "pair_shared_constraints",
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("owner_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("kind", sa.String(length=16), nullable=False),
        sa.Column("content", sa.Text(), nullable=False),
        sa.Column("version", sa.Integer(), server_default=sa.text("1"), nullable=False),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.CheckConstraint(
            _CONSTRAINT_KINDS,
            name=op.f("ck_pair_shared_constraints_constraint_kind_known"),
        ),
        sa.CheckConstraint(
            "length(content) <= 200",
            name=op.f("ck_pair_shared_constraints_constraint_content_length"),
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"],
            ["pair_notebook_cycles.id"],
            name="fk_pair_shared_constraints_cycle",
        ),
        sa.ForeignKeyConstraint(
            ["owner_id"], ["people.id"], name="fk_pair_shared_constraints_owner"
        ),
        sa.PrimaryKeyConstraint(
            "cycle_id", "owner_id", "kind", name=op.f("pk_pair_shared_constraints")
        ),
    )

    _create_triggers()


def _create_triggers() -> None:
    """Bốn luật database nói được mà CHECK không nói được."""
    # T1. Một phiên bản đã gửi là cái người kia đã đọc; UPDATE nó là viết lại
    # điều họ đã đồng ý, SAU khi họ đồng ý.
    op.execute(
        sa.text(
            """
            CREATE FUNCTION pair_reject_sent_version_change()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $$
            BEGIN
                IF OLD.sent_at IS NOT NULL THEN
                    RAISE EXCEPTION 'pair_paper_versions is immutable once sent'
                        USING ERRCODE = '55000';
                END IF;
                IF TG_OP = 'DELETE' THEN
                    RETURN OLD;
                END IF;
                RETURN NEW;
            END;
            $$
            """
        )
    )
    op.execute(
        sa.text(
            """
            CREATE TRIGGER pair_paper_versions_immutable
            BEFORE UPDATE OR DELETE ON pair_paper_versions
            FOR EACH ROW
            EXECUTE FUNCTION pair_reject_sent_version_change()
            """
        )
    )

    # T2. Một tờ chốt sinh đúng một outing, của đúng cuộc trò chuyện ấy, theo
    # đúng phiên bản hai người vừa đồng ý. Hoãn tới cuối giao dịch vì service
    # ghi link và đổi trạng thái trong cùng một giao dịch.
    op.execute(
        sa.text(
            """
            CREATE FUNCTION pair_check_outing_link()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $$
            DECLARE
                paper_state text;
                paper_current integer;
                paper_context uuid;
                outing_context uuid;
            BEGIN
                SELECT state, current_version, context_id
                  INTO paper_state, paper_current, paper_context
                  FROM pair_papers WHERE id = NEW.paper_id;
                IF paper_state IS DISTINCT FROM 'chot' THEN
                    RAISE EXCEPTION 'outing link needs a chot paper'
                        USING ERRCODE = '55000';
                END IF;
                IF NEW.version IS DISTINCT FROM paper_current THEN
                    RAISE EXCEPTION 'outing link must name the current version'
                        USING ERRCODE = '55000';
                END IF;
                SELECT context_id INTO outing_context
                  FROM outings WHERE id = NEW.outing_id;
                IF outing_context IS DISTINCT FROM paper_context THEN
                    RAISE EXCEPTION 'outing belongs to another conversation'
                        USING ERRCODE = '55000';
                END IF;
                RETURN NEW;
            END;
            $$
            """
        )
    )
    op.execute(
        sa.text(
            """
            CREATE CONSTRAINT TRIGGER pair_paper_outing_link_valid
            AFTER INSERT OR UPDATE ON pair_paper_outings
            DEFERRABLE INITIALLY DEFERRED
            FOR EACH ROW
            EXECUTE FUNCTION pair_check_outing_link()
            """
        )
    )

    # T3. «Đang là một đôi» chỉ đúng khi CẢ HAI đang giữ đồng ý `bat_doi`.
    # DISTINCT vì hai đồng ý của một người không phải là hai người.
    op.execute(
        sa.text(
            """
            CREATE FUNCTION pair_check_couple_consent()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $$
            DECLARE
                so_nguoi integer;
            BEGIN
                SELECT count(DISTINCT p.person_id) INTO so_nguoi
                  FROM pair_cycle_participants p
                  JOIN pair_consent_proposals pr
                    ON pr.cycle_id = p.cycle_id AND pr.purpose = 'bat_doi'
                  JOIN pair_consents c
                    ON c.proposal_id = pr.id AND c.person_id = p.person_id
                 WHERE p.cycle_id = NEW.cycle_id
                   AND c.granted_at IS NOT NULL
                   AND c.revoked_at IS NULL;
                IF so_nguoi < 2 THEN
                    RAISE EXCEPTION 'a couple needs both consents'
                        USING ERRCODE = '55000';
                END IF;
                RETURN NEW;
            END;
            $$
            """
        )
    )
    op.execute(
        sa.text(
            """
            CREATE CONSTRAINT TRIGGER active_couple_needs_two_consents
            AFTER INSERT OR UPDATE ON active_couple_members
            DEFERRABLE INITIALLY DEFERRED
            FOR EACH ROW
            EXECUTE FUNCTION pair_check_couple_consent()
            """
        )
    )

    # T4. Người trả lời phải đang ở trong cuộc trò chuyện. Không hoãn: membership
    # có trước lời đáp, nên không có thứ tự hợp lệ nào cần chờ cuối giao dịch.
    op.execute(
        sa.text(
            """
            CREATE FUNCTION pair_check_response_participant()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $$
            DECLARE
                o_trong boolean;
            BEGIN
                SELECT EXISTS (
                    SELECT 1
                      FROM pair_papers pp
                      JOIN memberships m
                        ON m.context_id = pp.context_id
                       AND m.person_id = NEW.person_id
                       AND m.left_at IS NULL
                     WHERE pp.id = NEW.paper_id
                ) INTO o_trong;
                IF NOT o_trong THEN
                    RAISE EXCEPTION 'responder is not in this conversation'
                        USING ERRCODE = '55000';
                END IF;
                RETURN NEW;
            END;
            $$
            """
        )
    )
    op.execute(
        sa.text(
            """
            CREATE TRIGGER pair_paper_response_is_participant
            BEFORE INSERT ON pair_paper_responses
            FOR EACH ROW
            EXECUTE FUNCTION pair_check_response_participant()
            """
        )
    )


def downgrade() -> None:
    for trigger, table in (
        ("pair_paper_response_is_participant", "pair_paper_responses"),
        ("active_couple_needs_two_consents", "active_couple_members"),
        ("pair_paper_outing_link_valid", "pair_paper_outings"),
        ("pair_paper_versions_immutable", "pair_paper_versions"),
    ):
        op.execute(sa.text(f"DROP TRIGGER IF EXISTS {trigger} ON {table}"))
    for function in (
        "pair_check_response_participant",
        "pair_check_couple_consent",
        "pair_check_outing_link",
        "pair_reject_sent_version_change",
    ):
        op.execute(sa.text(f"DROP FUNCTION IF EXISTS {function}()"))
    op.drop_table("pair_shared_constraints")
    op.drop_table("pair_paper_keeps")
    op.drop_table("pair_paper_outings")
    op.drop_table("pair_paper_responses")
    op.drop_table("pair_paper_views")
    op.drop_table("pair_paper_versions")
    op.drop_table("pair_papers")
    op.drop_table("active_couple_members")
    op.drop_table("pair_consents")
    op.drop_table("pair_consent_proposals")
    op.drop_table("pair_cycle_participants")
    op.drop_table("pair_notebook_cycles")
    op.drop_table("pair_notebooks")
    op.drop_constraint("uq_contexts_id_kind", "contexts", type_="unique")
