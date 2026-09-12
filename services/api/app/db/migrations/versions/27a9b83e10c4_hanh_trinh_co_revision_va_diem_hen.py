"""Preserve itinerary identity, schedule constraints and group meeting points."""

import sqlalchemy as sa
from alembic import op
from sqlalchemy.dialects import postgresql

revision = "27a9b83e10c4"
down_revision = "9a5e1c7b3f86"
branch_labels = None
depends_on = None


def upgrade() -> None:
    op.add_column(
        "outings",
        sa.Column(
            "timeline_revision", sa.Integer(), nullable=False, server_default="0"
        ),
    )
    op.add_column(
        "outings",
        sa.Column(
            "itinerary_version", sa.Integer(), nullable=False, server_default="1"
        ),
    )
    op.add_column(
        "outings",
        sa.Column(
            "itinerary_days",
            postgresql.JSONB(),
            nullable=False,
            server_default=sa.text("'[]'::jsonb"),
        ),
    )
    op.create_check_constraint(
        "timeline_revision_nonnegative", "outings", "timeline_revision >= 0"
    )
    op.create_check_constraint(
        "itinerary_version_known", "outings", "itinerary_version IN (1, 2)"
    )
    op.create_check_constraint(
        "itinerary_days_array", "outings", "jsonb_typeof(itinerary_days) = 'array'"
    )
    op.add_column("outing_stops", sa.Column("day", sa.Date(), nullable=True))
    op.add_column(
        "outing_stops", sa.Column("duration_minutes", sa.Integer(), nullable=True)
    )
    op.add_column(
        "outing_stops",
        sa.Column(
            "time_locked", sa.Boolean(), nullable=False, server_default=sa.text("true")
        ),
    )
    op.add_column("outing_stops", sa.Column("meeting_lat", sa.Float(), nullable=True))
    op.add_column("outing_stops", sa.Column("meeting_lng", sa.Float(), nullable=True))
    op.add_column("outing_stops", sa.Column("meeting_label", sa.Text(), nullable=True))
    op.create_check_constraint(
        "duration_in_day",
        "outing_stops",
        "duration_minutes IS NULL OR duration_minutes BETWEEN 0 AND 1440",
    )
    op.create_check_constraint(
        "meeting_point_valid",
        "outing_stops",
        "(meeting_lat IS NULL AND meeting_lng IS NULL AND meeting_label IS NULL) OR (meeting_lat IS NOT NULL AND meeting_lng IS NOT NULL AND meeting_label IS NOT NULL AND meeting_lat BETWEEN -90 AND 90 AND meeting_lng BETWEEN -180 AND 180 AND length(trim(meeting_label)) > 0 AND place_id IS NULL)",
    )
    op.execute(
        "UPDATE outing_stops AS s SET day = o.starts_on FROM outings AS o WHERE s.outing_id = o.id AND o.starts_on = o.ends_on"
    )


def downgrade() -> None:
    op.drop_constraint(
        op.f("ck_outing_stops_meeting_point_valid"), "outing_stops", type_="check"
    )
    op.drop_constraint(
        op.f("ck_outing_stops_duration_in_day"), "outing_stops", type_="check"
    )
    for column in (
        "meeting_label",
        "meeting_lng",
        "meeting_lat",
        "time_locked",
        "duration_minutes",
        "day",
    ):
        op.drop_column("outing_stops", column)
    op.drop_constraint(
        op.f("ck_outings_itinerary_days_array"), "outings", type_="check"
    )
    op.drop_constraint(
        op.f("ck_outings_itinerary_version_known"), "outings", type_="check"
    )
    op.drop_constraint(
        op.f("ck_outings_timeline_revision_nonnegative"), "outings", type_="check"
    )
    for column in ("itinerary_days", "itinerary_version", "timeline_revision"):
        op.drop_column("outings", column)
