"""Which catalogue coordinates may be used as a point.

A fed place carries `geo_precision` next to its `lat`/`lng`. Only `rooftop`
and `street` say where the place is. A ward or province centroid is the middle
of an area and `suy_luan` is a model's guess: they are "has coordinates", not
"is here", and the source contract (vnlocal HANDOFF-GEO §2) allows them for
area filtering only -- never a pin, a distance, a route or a stored point.

The same line is drawn by the ingest (`MappablePoint`, services/core), by the
Go catalogue (`repo.Place.MappablePoint`) and by the phone (`veDuocLenBanDo`).
A NULL precision is a row that predates the column (the seed, OSM imports):
the database refuses a new point without one, so NULL with a point is a
trusted legacy point.

Pure: no database, no HTTP, no framework imports.
"""

from __future__ import annotations

from collections.abc import Mapping

MAPPABLE_PRECISIONS = frozenset({"rooftop", "street"})


def mappable_point(place: Mapping[str, object]) -> tuple[float, float] | None:
    """`(lat, lng)` when the place's point may be drawn or measured, else None."""
    lat, lng = place.get("lat"), place.get("lng")
    if lat is None or lng is None:
        return None
    precision = place.get("geo_precision")
    if precision is not None and precision not in MAPPABLE_PRECISIONS:
        return None
    return float(lat), float(lng)
