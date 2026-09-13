"""Seed a synthetic itinerary into an explicitly selected local demo database."""

import os
import sys
from datetime import date
from pathlib import Path
from uuid import NAMESPACE_URL, uuid5

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "services/api"))

from sqlalchemy import create_engine
from sqlalchemy.engine import make_url
from sqlalchemy.orm import Session

from app.db.models import (
    Context,
    Membership,
    MembershipRole,
    MembershipState,
    Outing,
    OutingStop,
    Person,
)
from app.places.seed_catalog import seed_place_catalog

ACTOR = uuid5(NAMESPACE_URL, "rudi:synthetic-journey:actor")
GROUP = uuid5(NAMESPACE_URL, "rudi:synthetic-journey:group")
OUTING = uuid5(NAMESPACE_URL, "rudi:synthetic-journey:outing")


def main():
    url = make_url(os.environ["MOBILE_JOURNEY_DEMO_DATABASE_URL"])
    if url.host not in {"localhost", "127.0.0.1"} or url.database != "mobile":
        raise SystemExit(
            "Demo seeder requires an explicitly selected local mobile database"
        )
    engine = create_engine(url, hide_parameters=True)
    with Session(engine) as session:
        if session.get(Outing, OUTING):
            print("Synthetic itinerary already exists; nothing changed.")
            return
        seed_place_catalog(session)
        session.add(Person(id=ACTOR, display_name="Người thử hành trình"))
        session.flush()
        session.add(
            Context(id=GROUP, display_name="Hội thử bản đồ", created_by_id=ACTOR)
        )
        session.flush()
        session.add(
            Membership(
                context_id=GROUP,
                person_id=ACTOR,
                role=MembershipRole.MEMBER,
                state=MembershipState.ACTIVE,
            )
        )
        day = date(2030, 10, 17)
        session.add(
            Outing(
                id=OUTING,
                context_id=GROUP,
                created_by_id=ACTOR,
                title="Một ngày quanh hồ · dữ liệu thử",
                starts_on=day,
                ends_on=date(2030, 10, 19),
                headcount=4,
                budget_per_person_vnd=0,
                itinerary_version=2,
                itinerary_days=[
                    {
                        "day": str(day),
                        "transport_mode": "motorbike",
                        "start_at": "08:00",
                        "start_stop_id": None,
                        "end_stop_id": None,
                        "return_to_start": False,
                    }
                ],
            )
        )
        session.flush()
        # Public coordinates; names describe synthetic meeting points, not venues.
        for i, (label, lat, lng, at, locked) in enumerate(
            [
                ("Hẹn nhau bên chợ", 11.9420, 108.4374, 480, True),
                ("Một vòng phía đông hồ", 11.9457, 108.4604, 600, False),
                ("Cà phê gần điểm hẹn", 11.9428, 108.4382, 660, False),
                ("Hẹn ăn trưa", 11.9410, 108.4559, 720, True),
            ]
        ):
            session.add(
                OutingStop(
                    outing_id=OUTING,
                    position=i,
                    minute_of_day=at,
                    label=label,
                    day=day,
                    duration_minutes=30,
                    time_locked=locked,
                    meeting_lat=lat,
                    meeting_lng=lng,
                    meeting_label=label,
                )
            )
        session.commit()
    engine.dispose()
    print(f"Synthetic itinerary: {OUTING}; actor: {ACTOR}; group: {GROUP}")


if __name__ == "__main__":
    main()
