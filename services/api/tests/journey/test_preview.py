"""Synthetic directed roads expose optimistic matrix and timing errors."""

from copy import deepcopy

import pytest

from app.domain.journey import schedule, suggest_order
from app.journey.preview import preview_itinerary
from app.journey.routing import RoutingUnavailable


def draft():
    return {
        "expected_revision": 7,
        "day": "2026-09-12",
        "include_suggestion": True,
        "stops": [
            {
                "id": key,
                "day": "2026-09-12",
                "at": "08:00",
                "duration_minutes": 30,
                "time_locked": index == 0,
                "checked_in": False,
                "lat": 11.9 + index / 1000,
                "lng": 108.4,
            }
            for index, key in enumerate("ABC")
        ],
        "days": [
            {
                "day": "2026-09-12",
                "transport_mode": "motorbike",
                "start_at": "08:00",
                "start_stop_id": None,
                "end_stop_id": None,
                "return_to_start": False,
            }
        ],
    }


class Roads:
    graph_version = "synthetic-directed-v1"

    def __init__(self, *, worse_candidate=False, unavailable=False):
        self.worse_candidate = worse_candidate
        self.unavailable = unavailable
        self.modes = []

    def matrix(self, points, mode):
        self.modes.append(mode)
        return [
            [(0, 0), (600, 10000), (300, 1000)],
            [(1200, 20000), (0, 0), (660, 11000)],
            [(600, 12000), (300, 1000), (0, 0)],
        ]

    def route(self, points, mode):
        self.modes.append(mode)
        if self.unavailable:
            raise RoutingUnavailable("routing_unavailable")
        indexes = [round((p["lat"] - 11.9) * 1000) for p in points]
        matrix = self.matrix(points, mode)
        costs = [matrix[a][b] for a, b in zip(indexes, indexes[1:], strict=False)]
        if indexes == [0, 2, 1] and self.worse_candidate:
            costs = [(900, 15000), (660, 11000)]
        return [
            {
                "distance_meters": meters,
                "duration_seconds": seconds,
                "geometry": [[108.4, points[i]["lat"]], [108.4, points[i + 1]["lat"]]],
                "source": "valhalla",
            }
            for i, (seconds, meters) in enumerate(costs)
        ]


def test_three_stop_route_can_move_unanchored_last_stop_and_preserves_input():
    data = draft()
    snapshot = deepcopy(data)
    result = preview_itinerary(data, Roads())
    assert data == snapshot
    assert result["revision"] == 7
    assert result["status"] == "ready"
    assert [s["id"] for s in result["suggestion"]["stops"]] == list("ACB")
    assert result["suggestion"]["stops"][1]["at"] == "08:35"
    assert result["savings"] == {"distance_meters": 19000, "duration_seconds": 660}


def test_actual_candidate_route_rejects_optimistic_matrix_saving():
    result = preview_itinerary(draft(), Roads(worse_candidate=True))
    assert result["current"]["distance_meters"] == 21000
    assert result["suggestion"] is None
    assert result["savings"] is None


def test_disable_suggestions_preserves_current_road_and_draft(monkeypatch):
    monkeypatch.setenv("MOBILE_JOURNEY_SUGGESTIONS_ENABLED", "0")
    data = draft()
    before = deepcopy(data)
    result = preview_itinerary(data, Roads())
    assert result["current"]["distance_meters"] == 21000
    assert result["suggestion"] is result["savings"] is None
    assert result["issues"][-1]["code"] == "suggestions_disabled"
    assert data == before


@pytest.mark.parametrize("fixed", ["time_locked", "checked_in"])
def test_fixed_or_checked_in_stop_is_not_moved(fixed):
    data = draft()
    data["stops"][1][fixed] = True
    data["stops"][1]["at"] = "10:00"
    result = preview_itinerary(data, Roads())
    assert result["suggestion"] is None
    assert result["current"]["stops"][1]["wait_minutes"] == 80


def test_explicit_end_anchor_remains_last():
    data = draft()
    data["days"][0]["end_stop_id"] = "C"
    assert preview_itinerary(data, Roads())["suggestion"] is None


def test_wrong_anchor_is_explained_without_silent_reordering():
    data = draft()
    data["days"][0]["start_stop_id"] = "C"
    result = preview_itinerary(data, Roads())
    assert result["status"] == "incomplete"
    assert not result["current"]["feasible"]
    assert result["issues"][0]["code"] == "anchor_mismatch"


def test_missing_duration_keeps_verified_map_but_not_feasibility_or_eta_schedule():
    data = draft()
    data["stops"][1]["duration_minutes"] = None
    result = preview_itinerary(data, Roads())
    assert result["current"]["distance_meters"] == 21000
    assert result["current"]["stops"][2]["arrival_at"] is None
    assert not result["current"]["feasible"]
    assert result["suggestion"] is None
    assert result["status"] == "incomplete"


@pytest.mark.parametrize("value", [None, float("nan"), float("inf"), True, 200])
def test_missing_or_invalid_coordinate_does_not_join_across_gap(value):
    data = draft()
    data["stops"][1]["lat"] = value
    result = preview_itinerary(data, Roads())
    assert result["current"] is None
    assert result["issues"][0]["code"] == "missing_location"


def test_offline_has_no_fallback_eta_or_saving():
    result = preview_itinerary(draft(), Roads(unavailable=True))
    assert result["status"] == "unavailable"
    assert result["current"] is result["suggestion"] is result["savings"] is None


def test_return_leg_is_in_actual_totals():
    data = draft()
    data["include_suggestion"] = False
    data["days"][0]["return_to_start"] = True
    result = preview_itinerary(data, Roads())
    assert result["current"]["distance_meters"] == 33000
    assert result["current"]["segments"][-1]["to_stop_id"] == "A"


def test_next_day_times_are_null_and_infeasible_not_wrapped():
    data = draft()
    data["days"][0]["start_at"] = "23:30"
    data["stops"][0]["at"] = "23:30"
    result = preview_itinerary(data, Roads())
    assert result["status"] == "incomplete"
    assert result["current"]["stops"][1]["arrival_at"] is None
    assert any(i["code"] == "day_overflow" for i in result["issues"])


def test_unassigned_day_blocks_suggestions_until_confirmed():
    data = draft()
    data["stops"][2]["day"] = None
    result = preview_itinerary(data, Roads())
    assert result["status"] == "incomplete"
    assert result["suggestion"] is None


def test_matrix_is_directed_and_unreachable_is_not_free_travel():
    data = draft()
    matrix = [
        [(0, 0), (100, 10), (1, 1)],
        [(100, 10), (0, 0), (100, 10)],
        [(100, 10), None, (0, 0)],
    ]
    assert suggest_order(data["stops"], matrix, data["days"][0]) == [0, 1, 2]
    assert not schedule(data["stops"], [(100, 10), None], data["days"][0])["feasible"]


def test_one_stop_route_needs_no_road_geometry():
    data = draft()
    data["stops"] = data["stops"][:1]
    result = preview_itinerary(data, Roads())
    assert result["current"]["segments"] == []
    assert result["current"]["distance_meters"] == 0


def test_only_selected_day_routes_and_mode_is_forwarded():
    data = draft()
    data["include_suggestion"] = False
    data["days"][0]["transport_mode"] = "walk"
    data["stops"].append({**data["stops"][0], "id": "other-day", "day": "2026-09-13"})
    roads = Roads()
    result = preview_itinerary(data, roads)
    assert [s["id"] for s in result["current"]["stops"]] == list("ABC")
    assert set(roads.modes) == {"walk"}


def test_distinct_visits_to_same_coordinate_keep_identity():
    data = draft()
    data["include_suggestion"] = False
    data["stops"][2]["lat"] = data["stops"][0]["lat"]
    result = preview_itinerary(data, Roads())
    assert result["status"] == "ready"
    assert result["current"]["segments"][-1]["to_stop_id"] == "C"


@pytest.mark.parametrize("count", [51, 100])
def test_input_limit_before_provider_work(count):
    data = draft()
    data["stops"] = [{**data["stops"][0], "id": str(index)} for index in range(count)]
    result = preview_itinerary(data, Roads(unavailable=True))
    assert result["status"] == "incomplete"
    assert result["issues"][0]["code"] == "invalid_stops"


def test_matrix_failure_keeps_verified_current_without_claiming_savings():
    class BrokenMatrix(Roads):
        def matrix(self, points, mode):
            raise RoutingUnavailable("routing_unavailable")

        def route(self, points, mode):
            return Roads().route(points, mode)

    result = preview_itinerary(draft(), BrokenMatrix())
    assert result["current"]["distance_meters"] == 21000
    assert result["status"] == "unavailable"
    assert result["savings"] is None
