"""Indexes for the reads every signed-in request and every feed makes.

No statement changes: the queries stay exactly what the parity oracle compares,
and the planner picks these up on its own.

- `memberships (person_id)`: the actor lookup on EVERY authenticated request
  reads `WHERE person_id = $1` with no `left_at` condition, and the only
  person index is partial (`WHERE left_at IS NULL`), which the planner cannot
  use for it -- a CHECK constraint does not let it infer the predicate. Without
  this the lookup is a scan of every membership on the platform.
- `memberships (context_id, state)`: roster reads and the per-group active
  counts filter on `state`, not `left_at`, for the same reason.
- `stories (expires_at)`: the feed keeps stories with `expires_at > now`, a
  24-hour window; the only story index leads with `author_id`, so the feed
  otherwise reads every story ever posted.
"""

from alembic import op

revision = "e8c4d2a7f913"
down_revision = "b3f19c7d2a04"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.create_index("ix_memberships_person", "memberships", ["person_id"])
    op.create_index(
        "ix_memberships_context_state", "memberships", ["context_id", "state"]
    )
    op.create_index("ix_stories_live", "stories", ["expires_at"])


def downgrade() -> None:
    op.drop_index("ix_stories_live", table_name="stories")
    op.drop_index("ix_memberships_context_state", table_name="memberships")
    op.drop_index("ix_memberships_person", table_name="memberships")
