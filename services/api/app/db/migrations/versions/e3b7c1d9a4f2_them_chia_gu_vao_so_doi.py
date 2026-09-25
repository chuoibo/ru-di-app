"""Add the per-person `chia_gu` purpose to the pair notebook (ADR-0034 §2.1).

Raw SQL on purpose: the pair tables are still Alembic's (the Python oracle
needs the same schema for parity), and the change is one CHECK.
"""

from alembic import op

revision = "e3b7c1d9a4f2"
down_revision = "d6a2f93b81e7"
branch_labels = None
depends_on = None

_NAME = "ck_pair_consent_proposals_consent_purpose_known"


def upgrade() -> None:
    op.execute(f"ALTER TABLE pair_consent_proposals DROP CONSTRAINT {_NAME}")
    op.execute(
        f"ALTER TABLE pair_consent_proposals ADD CONSTRAINT {_NAME} "
        "CHECK (purpose IN ('lap_so', 'bat_doi', 'doc_chat', 'chia_gu'))"
    )


def downgrade() -> None:
    # A `chia_gu` row cannot survive the narrower CHECK; removing them is
    # what taking the switch away means.
    op.execute("DELETE FROM pair_consents WHERE proposal_id IN (SELECT id FROM pair_consent_proposals WHERE purpose = 'chia_gu')")
    op.execute("DELETE FROM pair_consent_proposals WHERE purpose = 'chia_gu'")
    op.execute(f"ALTER TABLE pair_consent_proposals DROP CONSTRAINT {_NAME}")
    op.execute(
        f"ALTER TABLE pair_consent_proposals ADD CONSTRAINT {_NAME} "
        "CHECK (purpose IN ('lap_so', 'bat_doi', 'doc_chat'))"
    )
