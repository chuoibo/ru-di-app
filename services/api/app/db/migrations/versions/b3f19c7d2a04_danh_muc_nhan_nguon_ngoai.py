"""Let the catalogue hold places from an external feed, and say what it knows.

The feed being taken on carries no coordinates for roughly a quarter of its
rows and, today, for all of them. A NOT NULL `lat` would have meant either
refusing that data or inventing a point for it, and an invented point on a map
is worse than an absent one: somebody drives to it.

So coordinates become optional and gain a precision, and every other thing the
feed cannot supply -- opening hours, a price band, a rating -- keeps the null
the catalogue already allowed. `geo_precision` is required whenever a point is
present, because a rooftop match and a model's guess must not look alike.
"""

import sqlalchemy as sa
from alembic import op

revision = "b3f19c7d2a04"
down_revision = "d6a2f93b81e7"
branch_labels = None
depends_on = None

GEO_PRECISION = (
    "rooftop",
    "street",
    "ward_centroid",
    "province_centroid",
    "suy_luan",
    "none",
)
PLACE_STATUS = ("active", "hidden", "superseded", "stale")


def _in_list(column: str, values: tuple[str, ...]) -> str:
    joined = ", ".join(f"'{value}'" for value in values)
    return f"{column} IN ({joined})"


def upgrade() -> None:
    # --- places: coordinates become optional, and carry their provenance ----
    op.alter_column("places", "lat", existing_type=sa.Float(), nullable=True)
    op.alter_column("places", "lng", existing_type=sa.Float(), nullable=True)
    op.add_column("places", sa.Column("geo_precision", sa.Text(), nullable=True))
    op.add_column("places", sa.Column("geo_evidence", sa.Text(), nullable=True))
    op.add_column("places", sa.Column("province_code", sa.SmallInteger(), nullable=True))
    op.add_column(
        "places", sa.Column("source_updated_at", sa.DateTime(timezone=True), nullable=True)
    )
    op.add_column("places", sa.Column("source_kind", sa.Text(), nullable=True))
    op.add_column("places", sa.Column("confidence", sa.Float(), nullable=True))
    op.add_column("places", sa.Column("evidence_posts", sa.Integer(), nullable=True))
    op.add_column(
        "places",
        sa.Column(
            "status", sa.Text(), nullable=False, server_default="active"
        ),
    )
    op.add_column("places", sa.Column("superseded_by", sa.Text(), nullable=True))

    # Every row that exists today carries a specific point rather than an area
    # centroid, so the constraint below can be added without exceptions.
    op.execute("UPDATE places SET geo_precision = 'rooftop' WHERE lat IS NOT NULL")

    op.create_check_constraint(
        "place_geo_precision_known",
        "places",
        f"geo_precision IS NULL OR {_in_list('geo_precision', GEO_PRECISION)}",
    )
    # A latitude without a longitude is not a partial answer, it is a broken row.
    op.create_check_constraint(
        "place_point_is_whole", "places", "num_nonnulls(lat, lng) <> 1"
    )
    # The rule this migration exists for: a point must say how it was arrived at.
    op.create_check_constraint(
        "place_point_states_its_precision",
        "places",
        "lat IS NULL OR geo_precision IS NOT NULL",
    )
    op.create_check_constraint(
        "place_status_known", "places", _in_list("status", PLACE_STATUS)
    )
    # A superseded row must name its successor, or a merge left readers nowhere
    # to follow -- and saved places point at rows like these.
    op.create_check_constraint(
        "place_superseded_names_successor",
        "places",
        "status <> 'superseded' OR superseded_by IS NOT NULL",
    )
    op.create_check_constraint(
        "place_confidence_range",
        "places",
        "confidence IS NULL OR (confidence >= 0 AND confidence <= 1)",
    )
    op.create_check_constraint(
        "place_evidence_posts_sane",
        "places",
        "evidence_posts IS NULL OR evidence_posts >= 0",
    )
    op.create_foreign_key(
        "fk_places_superseded_by", "places", "places", ["superseded_by"], ["id"]
    )
    op.create_index(
        "ix_places_province", "places", ["province_code", "category", "id"]
    )
    op.create_index("ix_places_status", "places", ["status"])

    # A fourth source. The imported row must still point back at what produced
    # it, which is what makes a second delivery an update instead of a copy.
    op.drop_constraint("place_source_known", "places", type_="check")
    op.create_check_constraint(
        "place_source_known",
        "places",
        _in_list("source", ("seed", "osm", "curated", "vnlocal")),
    )
    op.create_check_constraint(
        "place_vnlocal_row_cites_its_source",
        "places",
        "source <> 'vnlocal' OR source_ref IS NOT NULL",
    )

    # --- place_photos: attribution is recorded where it exists --------------
    op.alter_column("place_photos", "author", existing_type=sa.Text(), nullable=True)
    op.alter_column("place_photos", "license", existing_type=sa.Text(), nullable=True)
    op.drop_constraint("place_photo_cites_its_source", "place_photos", type_="check")
    # Where the photograph came from stays mandatory; who made it and under what
    # terms are recorded when they are known and left null when they are not.
    # A blank string is not a weaker answer than null, it is a false one.
    op.create_check_constraint(
        "place_photo_cites_its_source",
        "place_photos",
        "length(btrim(source_url)) > 0 "
        "AND (author IS NULL OR length(btrim(author)) > 0) "
        "AND (license IS NULL OR length(btrim(license)) > 0)",
    )
    op.add_column("place_photos", sa.Column("platform", sa.Text(), nullable=True))
    op.add_column("place_photos", sa.Column("post_id", sa.Text(), nullable=True))
    op.add_column("place_photos", sa.Column("frame_second", sa.Float(), nullable=True))
    op.add_column("place_photos", sa.Column("score", sa.Float(), nullable=True))
    op.add_column("place_photos", sa.Column("subject", sa.Text(), nullable=True))
    op.create_check_constraint(
        "place_photo_platform_known",
        "place_photos",
        "platform IS NULL OR platform IN ('tiktok', 'threads')",
    )

    # --- content digests: a non-unique index, on purpose --------------------
    # Duplicate bytes are worth knowing about; they are not worth collapsing.
    # Making the digest the storage key would break the unique constraint on it
    # the first time two people upload the same picture, and would weaken the
    # answer given to somebody who asks for their photographs to be deleted.
    # Written out per table rather than looped: the gate that keeps this file
    # and the models from drifting reads this source with an AST parser, and a
    # loop variable is not a table name it can resolve.
    op.add_column(
        "uploaded_images", sa.Column("content_sha256", sa.Text(), nullable=True)
    )
    op.create_check_constraint(
        "uploaded_images_content_sha256_shape",
        "uploaded_images",
        "content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$'",
    )
    op.create_index(
        "ix_uploaded_images_content_sha256", "uploaded_images", ["content_sha256"]
    )
    op.add_column(
        "place_photos", sa.Column("content_sha256", sa.Text(), nullable=True)
    )
    op.create_check_constraint(
        "place_photos_content_sha256_shape",
        "place_photos",
        "content_sha256 IS NULL OR content_sha256 ~ '^[0-9a-f]{64}$'",
    )
    op.create_index(
        "ix_place_photos_content_sha256", "place_photos", ["content_sha256"]
    )


def downgrade() -> None:
    op.drop_index("ix_place_photos_content_sha256", table_name="place_photos")
    op.drop_constraint(
        "place_photos_content_sha256_shape", "place_photos", type_="check"
    )
    op.drop_column("place_photos", "content_sha256")
    op.drop_index("ix_uploaded_images_content_sha256", table_name="uploaded_images")
    op.drop_constraint(
        "uploaded_images_content_sha256_shape", "uploaded_images", type_="check"
    )
    op.drop_column("uploaded_images", "content_sha256")

    op.drop_constraint("place_photo_platform_known", "place_photos", type_="check")
    for column in ("subject", "score", "frame_second", "post_id", "platform"):
        op.drop_column("place_photos", column)
    op.drop_constraint("place_photo_cites_its_source", "place_photos", type_="check")
    op.execute("UPDATE place_photos SET author = '' WHERE author IS NULL")
    op.execute("UPDATE place_photos SET license = '' WHERE license IS NULL")
    op.create_check_constraint(
        "place_photo_cites_its_source",
        "place_photos",
        "length(btrim(author)) > 0 AND length(btrim(license)) > 0 "
        "AND length(btrim(source_url)) > 0",
    )
    op.alter_column("place_photos", "license", existing_type=sa.Text(), nullable=False)
    op.alter_column("place_photos", "author", existing_type=sa.Text(), nullable=False)

    op.drop_constraint("place_vnlocal_row_cites_its_source", "places", type_="check")
    op.drop_constraint("place_source_known", "places", type_="check")
    op.create_check_constraint(
        "place_source_known", "places", _in_list("source", ("seed", "osm", "curated"))
    )
    op.drop_index("ix_places_status", table_name="places")
    op.drop_index("ix_places_province", table_name="places")
    op.drop_constraint("fk_places_superseded_by", "places", type_="foreignkey")
    for name in (
        "place_evidence_posts_sane",
        "place_confidence_range",
        "place_superseded_names_successor",
        "place_status_known",
        "place_point_states_its_precision",
        "place_point_is_whole",
        "place_geo_precision_known",
    ):
        op.drop_constraint(name, "places", type_="check")
    for column in (
        "superseded_by",
        "status",
        "evidence_posts",
        "confidence",
        "source_kind",
        "source_updated_at",
        "province_code",
        "geo_evidence",
        "geo_precision",
    ):
        op.drop_column("places", column)
    op.alter_column("places", "lng", existing_type=sa.Float(), nullable=False)
    op.alter_column("places", "lat", existing_type=sa.Float(), nullable=False)
