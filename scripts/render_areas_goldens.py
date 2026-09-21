"""Goldens for services/core/internal/domain/areas.

Reads the real `app.places.areas` inside the pinned API image:

    docker run --rm -i --network none --entrypoint python <api image> - \\
      < scripts/render_areas_goldens.py \\
      > services/core/internal/domain/areas/testdata/python_areas.json

Coordinates are written as Python's shortest repr, which parses back to the
exact float64; hex floats are avoided because their digit runs can read as
phone numbers to the repository guard.
"""

from __future__ import annotations

import json
import sys

from app.places.areas import AREAS, area_summary, find_area


def main() -> None:
    probes = [area["id"] for area in AREAS] + [
        "",
        "DA-LAT",
        " da-lat",
        "da-lat ",
        "hcm-quan-2",
        "quan-1",
        "hcm_quan_1",
    ]
    document = {
        "areas": [
            {
                "id": area["id"],
                "label": area["label"],
                "lat": repr(area["lat"]),
                "lng": repr(area["lng"]),
                "summary_keys": list(area_summary(area)),
            }
            for area in AREAS
        ],
        "find": [
            {
                "id": probe,
                "found": find_area(probe) is not None,
                "index": next(
                    (i for i, area in enumerate(AREAS) if find_area(probe) is area),
                    None,
                ),
            }
            for probe in probes
        ],
    }
    json.dump(document, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
