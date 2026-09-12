"""Read-only orchestration: compare complete routes, never scale geodesic costs."""

from __future__ import annotations

import math
import os
from threading import BoundedSemaphore
from typing import Any

from app.domain.journey import MAX_STOPS, issue, minute, schedule, suggest_order
from app.journey.routing import (
    MODES,
    RoutingProvider,
    RoutingUnavailable,
    configured_provider,
)

_CAPACITY = BoundedSemaphore(4)


def _route(
    stops: list[dict[str, Any]], settings: dict[str, Any], provider: RoutingProvider
) -> dict[str, Any]:
    points = [{"lat": stop["lat"], "lon": stop["lng"]} for stop in stops]
    ids = [stop["id"] for stop in stops]
    if settings.get("return_to_start") and len(points) > 1:
        points.append(points[0])
        ids.append(ids[0])
    legs = provider.route(points, settings["transport_mode"])
    if len(legs) != max(0, len(points) - 1):
        raise RoutingUnavailable("routing_unavailable")
    costs = [(leg["duration_seconds"], leg["distance_meters"]) for leg in legs]
    result = schedule(stops, costs, settings)
    result.update(
        {
            "segments": [
                {**leg, "from_stop_id": ids[i], "to_stop_id": ids[i + 1]}
                for i, leg in enumerate(legs)
            ],
            "distance_meters": sum(cost[1] for cost in costs),
            "duration_seconds": sum(cost[0] for cost in costs),
        }
    )
    return result


def preview_itinerary(
    draft: dict[str, Any], provider: RoutingProvider | None = None
) -> dict[str, Any]:
    """Caller authorizes outing, resolves locations and derives check-ins first."""
    provider = provider or configured_provider()
    result: dict[str, Any] = {
        "revision": draft["expected_revision"],
        "day": draft["day"],
        "status": "incomplete",
        "source": {
            "engine": "valhalla",
            "graph_version": provider.graph_version if provider else None,
            "traffic": "none",
        },
        "current": None,
        "suggestion": None,
        "savings": None,
        "issues": [],
    }
    issues = result["issues"]
    all_stops = draft["stops"]
    if len(all_stops) > MAX_STOPS or len({s["id"] for s in all_stops}) != len(
        all_stops
    ):
        issues.append(
            issue(
                "invalid_stops",
                "Lịch trình cần ID riêng cho từng chặng và tối đa 50 chặng.",
            )
        )
        return result
    settings = next((d for d in draft["days"] if d["day"] == draft["day"]), None)
    if settings is None or settings.get("transport_mode") not in MODES:
        issues.append(
            issue(
                "missing_day_settings",
                "Chọn phương tiện và giờ xuất phát cho ngày này.",
            )
        )
        return result
    stops = [s for s in all_stops if s.get("day") == draft["day"]]
    if not stops:
        issues.append(issue("empty_day", "Thêm điểm hẹn cho ngày này."))
        return result
    for stop in all_stops:
        if stop.get("day") is None:
            issues.append(
                issue(
                    "unassigned_day",
                    "Chọn ngày cho chặng này trước khi so sánh.",
                    stop["id"],
                )
            )
    try:
        minute(settings["start_at"])
        for stop in stops:
            minute(stop["at"])
            dwell = stop.get("duration_minutes")
            if dwell is not None and (type(dwell) is not int or not 0 <= dwell <= 1440):
                raise ValueError("invalid_duration")
    except (KeyError, ValueError, TypeError):
        issues.append(issue("invalid_schedule", "Giờ hoặc thời lượng chưa hợp lệ."))
        return result
    missing_location = False
    for stop in stops:
        lat, lng = stop.get("lat"), stop.get("lng")
        if (
            any(
                isinstance(v, bool)
                or not isinstance(v, int | float)
                or not math.isfinite(v)
                for v in (lat, lng)
            )
            or not -90 <= lat <= 90
            or not -180 <= lng <= 180
        ):
            issues.append(
                issue("missing_location", "Chọn vị trí cho điểm hẹn này.", stop["id"])
            )
            missing_location = True
    if missing_location:
        return result
    for key, actual in (
        ("start_stop_id", stops[0]["id"]),
        ("end_stop_id", stops[-1]["id"]),
    ):
        if settings.get(key) and settings[key] != actual:
            issues.append(
                issue(
                    "anchor_mismatch",
                    "Đặt điểm đầu/cuối đã chọn đúng vị trí trong lịch trình.",
                    settings[key],
                )
            )
    if provider is None:
        result["status"] = "unavailable"
        issues.append(
            issue(
                "routing_unavailable",
                "Chưa kết nối được dữ liệu đường đi. Thử lại sau.",
            )
        )
        return result
    if not _CAPACITY.acquire(blocking=False):
        result["status"] = "unavailable"
        issues.append(
            issue("routing_busy", "Đang có nhiều yêu cầu tính đường. Thử lại sau.")
        )
        return result
    try:
        current = _route(stops, settings, provider)
        result["current"] = current
        issues.extend(current["issues"])
        if issues:
            current["feasible"] = False
        result["status"] = "ready" if not issues else "incomplete"
        if os.environ.get("MOBILE_JOURNEY_SUGGESTIONS_ENABLED", "1") == "0":
            if draft.get("include_suggestion"):
                issues.append(
                    issue(
                        "suggestions_disabled",
                        "Đề xuất đang tạm nghỉ. Bạn vẫn có thể xem và sửa lịch trình.",
                    )
                )
            return result
        # Missing facts cannot support a safe recommendation. A late appointment
        # alone can be repaired by a genuinely feasible alternate road ordering.
        if not draft.get("include_suggestion") or any(
            i["code"] != "late_fixed_stop" for i in issues
        ):
            return result
        points = [{"lat": s["lat"], "lon": s["lng"]} for s in stops]
        matrix = provider.matrix(points, settings["transport_mode"])
        order = suggest_order(stops, matrix, settings)
        if order == list(range(len(stops))):
            return result
        candidate = _route([stops[i] for i in order], settings, provider)
        # Matrix search is only a candidate generator. Whole-route costs decide.
        improves = (candidate["duration_seconds"], candidate["distance_meters"]) < (
            current["duration_seconds"],
            current["distance_meters"],
        )
        if candidate["feasible"] and (improves or not current["feasible"]):
            result["suggestion"] = candidate
            result["savings"] = {
                "distance_meters": current["distance_meters"]
                - candidate["distance_meters"],
                "duration_seconds": current["duration_seconds"]
                - candidate["duration_seconds"],
            }
        return result
    except RoutingUnavailable:
        # Preserve a verified current route if only recommendation lookup failed.
        result["status"] = "unavailable"
        issues.append(
            issue(
                "routing_unavailable", "Chưa tính đủ đường đi để so sánh. Thử lại sau."
            )
        )
        return result
    finally:
        _CAPACITY.release()
