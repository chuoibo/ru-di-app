"""Opt-in real graph checks. Public landmark coordinates contain no person data."""

import os

import pytest

from app.journey.routing import ValhallaProvider

pytestmark = pytest.mark.skipif(
    not os.environ.get("MOBILE_TEST_VALHALLA_URL"),
    reason="requires a self-hosted Vietnam routing graph",
)


@pytest.mark.parametrize("mode", ["motorbike", "car", "walk"])
@pytest.mark.parametrize(
    "points",
    [
        # Public central Da Lat landmarks, on both sides of the lake.
        [
            {"lat": 11.9404, "lon": 108.4383},
            {"lat": 11.9407, "lon": 108.4447},
            {"lat": 11.9459, "lon": 108.4488},
        ],
        # Central Hanoi, around Hoan Kiem Lake.
        [
            {"lat": 21.0308, "lon": 105.8524},
            {"lat": 21.0249, "lon": 105.8531},
            {"lat": 21.0285, "lon": 105.8553},
        ],
    ],
)
def test_real_vietnam_graph_modes_matrix_and_complete_route(mode, points):
    provider = ValhallaProvider(
        os.environ["MOBILE_TEST_VALHALLA_URL"],
        os.environ["MOBILE_ROUTING_GRAPH_VERSION"],
        timeout=30,
    )
    matrix = provider.matrix(points, mode)
    assert len(matrix) == len(points)
    # A directed road snap can require a loop even back to the same input:
    # Hoan Kiem's second landmark has a 630m motor-vehicle loop in this graph.
    # The planner never traverses the matrix diagonal; do not falsify its cost.
    assert all(row[index] is not None for index, row in enumerate(matrix))
    assert all(matrix[a][b] is not None for a, b in ((0, 1), (1, 2), (2, 0)))
    legs = provider.route(points + points[:1], mode)
    assert len(legs) == 3
    assert all(
        leg["distance_meters"] > 0
        and leg["duration_seconds"] > 0
        and len(leg["geometry"]) >= 2
        for leg in legs
    )
    assert all(leg["source"] == "valhalla" for leg in legs)
