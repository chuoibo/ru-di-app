"""Put the invented seed catalogue into the `places` table (M9, ADR-0017).

`app/places/catalog.py` and `app/places/details.py` stop being the catalogue
and become **seed material**: twelve invented rows and their invented prose,
loaded into the table for demos and tests so every flow, fixture and Maestro
assertion that names `p-tiem-nuong-xom-lao` keeps working. A production
database runs the OpenStreetMap import instead, or as well.

Idempotent: running twice changes nothing. Written as a function on a Session
rather than a script so `make demo`, the e2e stack and the Postgres tests can
all call the same code path.
"""

from __future__ import annotations

from typing import Any

from sqlalchemy import select
from sqlalchemy.orm import Session

from app.db.models import Destination, Place
from app.places.activities import hoat_dong_theo_dong
from app.places.catalog import PLACES
from app.places.destinations_vn import DESTINATIONS_VN
from app.places.details import find_detail

# One list of destinations, not two. This module used to carry its own copy of
# two of them, and the copy had drifted: its bounding box for Ho Chi Minh City
# was four times the area of the one in `destinations_vn`, which is the box the
# OpenStreetMap importer actually queries with. All twelve seed rows sit inside
# the `destinations_vn` boxes, so the wider copy bought nothing and cost a
# second answer to the question "where is this city".
#
# Seeding all fifteen also closes a measured hole: a stack with only two
# destinations cannot answer a request for Hoi An, and a flow that asked for one
# failed on an empty screen rather than on a missing destination.
SEED_DESTINATIONS: list[dict[str, Any]] = DESTINATIONS_VN


def _destination_for(place: dict[str, Any]) -> str:
    """Which seeded city a seed row belongs to, from its own address string.

    A lookup by address rather than a hand-written map: the addresses are in
    the same file as the rows, so a thirteenth seed row lands in the right city
    without anybody remembering to edit a second list.
    """
    address = str(place.get("address", ""))
    if "Đà Lạt" in address:
        return "d-da-lat"
    return "d-tphcm"


def seed_place_catalog(session: Session) -> tuple[int, int]:
    """Insert any missing seed destinations and places. Returns (dests, places).

    Existing rows are left exactly as they are: this function is for filling an
    empty catalogue, not for republishing over one somebody has since imported.
    """
    da_co_dd = set(session.scalars(select(Destination.id)).all())
    them_dd = 0
    for row in SEED_DESTINATIONS:
        if row["id"] in da_co_dd:
            continue
        session.add(Destination(**row))
        them_dd += 1
    session.flush()

    da_co = set(session.scalars(select(Place.id)).all())
    them = 0
    for place in PLACES:
        if place["id"] in da_co:
            continue
        prose = find_detail(place["id"])
        session.add(
            Place(
                id=place["id"],
                destination_id=_destination_for(place),
                name=place["name"],
                category=place["category"],
                kinds=list(place["kinds"]),
                address=place["address"],
                lat=place["lat"],
                lng=place["lng"],
                # A seed row's point is a specific spot rather than the
                # centre of an area, so it claims the precision it has.
                # The catalogue refuses a point that will not say.
                geo_precision="rooftop",
                rating=place["rating"],
                rating_count=place["rating_count"],
                price_min_vnd=place["price_min_vnd"],
                price_max_vnd=place["price_max_vnd"],
                open_hours=place["open_hours"],
                open_now=place["open_now"],
                travel_minutes=place["travel_minutes"],
                distance_km=place["distance_km"],
                photo_count=place["photo_count"],
                traits=list(place["traits"]),
                group_fit=dict(place["group_fit"]),
                # «Nên làm gì ở đây» (M12). The seed rows have no OSM tags, so
                # the phrases are read off the columns they do have -- same
                # rule, applied to the vocabulary this file is written in.
                activities=hoat_dong_theo_dong(place),
                flag=place["flag"],
                description=None if prose is None else prose["description"],
                reviews=None if prose is None else list(prose["reviews"]),
                source="seed",
                source_ref=None,
                license=None,
            )
        )
        them += 1
    session.flush()
    return them_dd, them


def main() -> int:
    """`python3 -m app.places.seed_catalog` -- fill an empty catalogue.

    Used by the e2e stack and the demo box, which both have a fresh database
    and no import of their own. Reads `MOBILE_DATABASE_URL`, like every other
    script that talks to the database directly.
    """
    import os

    from sqlalchemy import create_engine

    url = os.environ.get("MOBILE_DATABASE_URL", "").strip()
    if not url:
        print("MOBILE_DATABASE_URL chưa đặt", flush=True)
        return 2
    engine = create_engine(url)
    with Session(engine) as session:
        dests, places = seed_place_catalog(session)
        session.commit()
    print(f"danh mục seed: thêm {dests} điểm đến, {places} địa điểm", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
