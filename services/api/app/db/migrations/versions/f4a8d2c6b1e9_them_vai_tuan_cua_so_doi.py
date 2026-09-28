"""«Người lo» of a week in a couple's notebook, when the two chose it (ADR-0034 §2.4).

One row per cycle and week, only when somebody chose: the default is inferred
from the notebook every time it is read and never stored. `nguoi_lo_id` NULL
is «Hôm nay mình share».
"""

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision = "f4a8d2c6b1e9"
down_revision = "e3b7c1d9a4f2"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_table(
        "pair_cycle_rhythms",
        sa.Column("cycle_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column("tuan", sa.Date(), nullable=False),
        sa.Column("nguoi_lo_id", postgresql.UUID(as_uuid=True), nullable=True),
        sa.Column("chon_boi_id", postgresql.UUID(as_uuid=True), nullable=False),
        sa.Column(
            "updated_at",
            sa.DateTime(timezone=True),
            server_default=sa.text("now()"),
            nullable=False,
        ),
        sa.ForeignKeyConstraint(
            ["cycle_id"], ["pair_notebook_cycles.id"], name="fk_pair_cycle_rhythms_cycle"
        ),
        sa.ForeignKeyConstraint(
            ["nguoi_lo_id"], ["people.id"], name="fk_pair_cycle_rhythms_nguoi_lo"
        ),
        sa.ForeignKeyConstraint(
            ["chon_boi_id"], ["people.id"], name="fk_pair_cycle_rhythms_chon_boi"
        ),
        sa.PrimaryKeyConstraint("cycle_id", "tuan", name=op.f("pk_pair_cycle_rhythms")),
    )


def downgrade() -> None:
    op.drop_table("pair_cycle_rhythms")
