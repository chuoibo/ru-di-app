"""Bounded itinerary search over directed road costs, without IO or frameworks."""

from __future__ import annotations

from math import ceil
from typing import Any

MAX_STOPS = 50
MAX_EVALUATIONS = 2000
Cost = tuple[int, int]  # Travel seconds, distance metres.
Matrix = list[list[Cost | None]]


def issue(code: str, message: str, stop_id: str | None = None) -> dict[str, Any]:
    return {"code": code, "stop_id": stop_id, "message": message}


def minute(value: str) -> int:
    hour, minutes = map(int, value.split(":"))
    if not 0 <= hour <= 23 or not 0 <= minutes <= 59:
        raise ValueError("invalid_time")
    return hour * 60 + minutes


def clock(value: int) -> str | None:
    return f"{value // 60:02}:{value % 60:02}" if 0 <= value < 1440 else None


def schedule(
    stops: list[dict[str, Any]],
    costs: list[Cost | None],
    settings: dict[str, Any],
) -> dict[str, Any]:
    """Evaluate all appointments; never infer an unknown dwell time."""
    issues: list[dict[str, Any]] = []
    rows = []
    now: int | None = minute(settings["start_at"])
    for index, stop in enumerate(stops):
        if index:
            cost = costs[index - 1]
            if cost is None:
                issues.append(
                    issue(
                        "unreachable", "Không tìm được đường đến điểm này.", stop["id"]
                    )
                )
                now = None
            elif now is not None:
                now += ceil(cost[0] / 60)
        arrival = now
        locked = stop.get("time_locked", True) or stop.get("checked_in", False)
        expected = minute(stop["at"])
        wait = max(0, expected - now) if now is not None and locked else 0
        if locked and now is not None:
            if now > expected:
                issues.append(
                    issue("late_fixed_stop", "Không kịp giờ đã ghim.", stop["id"])
                )
            now = max(now, expected)
        visit = now
        dwell = stop.get("duration_minutes")
        if dwell is None:
            issues.append(
                issue("missing_duration", "Xác nhận thời lượng ở lại.", stop["id"])
            )
            now = None
        elif now is not None:
            now += dwell
        if any(value is not None and value >= 1440 for value in (arrival, visit, now)):
            issues.append(
                issue("day_overflow", "Chặng này vượt qua ngày đang lập.", stop["id"])
            )
        rows.append(
            {
                "id": stop["id"],
                "at": clock(visit) if visit is not None else None,
                "arrival_at": clock(arrival) if arrival is not None else None,
                "departure_at": clock(now) if now is not None else None,
                "wait_minutes": wait,
            }
        )
    if settings.get("return_to_start") and len(stops) > 1:
        cost = costs[-1]
        if cost is None:
            issues.append(issue("unreachable_return", "Không tìm được đường quay về."))
        elif now is not None and now + ceil(cost[0] / 60) >= 1440:
            issues.append(
                issue("day_overflow", "Đường quay về vượt qua ngày đang lập.")
            )
    return {"stops": rows, "feasible": not issues, "issues": issues}


def order_costs(order: list[int], matrix: Matrix, returning: bool) -> list[Cost | None]:
    pairs = list(zip(order, order[1:], strict=False))
    if returning and len(order) > 1:
        pairs.append((order[-1], order[0]))
    return [matrix[a][b] for a, b in pairs]


def suggest_order(
    stops: list[dict[str, Any]], matrix: Matrix, settings: dict[str, Any]
) -> list[int]:
    """Keep fixed slots/endpoints, search directed costs, never claim optimality."""
    current = list(range(len(stops)))
    if len(stops) < 3:
        return current
    returning = settings.get("return_to_start", False)
    movable = [
        i
        for i, s in enumerate(stops)
        if i > 0
        and s["id"] != settings.get("end_stop_id")
        and not s.get("time_locked", True)
        and not s.get("checked_in", False)
    ]
    evaluations = 0

    def score(order: list[int]) -> tuple[int, int, int, int]:
        nonlocal evaluations
        evaluations += 1
        costs = order_costs(order, matrix, returning)
        result = schedule([stops[i] for i in order], costs, settings)
        # An infeasible solution is never preferred merely because it is shorter.
        return (
            len(result["issues"]),
            sum(c[0] if c else 10**9 for c in costs),
            sum(c[1] if c else 10**9 for c in costs),
            sum(a != b for a, b in zip(order, current, strict=True)),
        )

    best = current[:]
    best_score = score(best)
    greedy = current[:]
    remaining = set(movable)
    for slot in movable:
        previous = greedy[slot - 1]
        selected = min(
            remaining, key=lambda i: (matrix[previous][i] or (10**9, 10**9), i)
        )
        greedy[slot] = selected
        remaining.remove(selected)
    candidate_score = score(greedy)
    if candidate_score < best_score:
        best, best_score = greedy, candidate_score
    for _ in range(6):
        improved = False
        for a in movable:
            for b in movable:
                if a >= b:
                    continue
                candidates = []
                swapped = best[:]
                swapped[a], swapped[b] = swapped[b], swapped[a]
                candidates.append(swapped)
                # Move within flexible slots, without displacing fixed appointments.
                relocated = best[:]
                slots = [i for i in movable if a <= i <= b]
                values = [relocated[i] for i in slots]
                for slot, value in zip(slots, values[1:] + values[:1], strict=True):
                    relocated[slot] = value
                candidates.append(relocated)
                for candidate in candidates:
                    if evaluations >= MAX_EVALUATIONS:
                        return best
                    candidate_score = score(candidate)
                    if candidate_score < best_score:
                        best, best_score = candidate, candidate_score
                        improved = True
        if not improved:
            break
    return best
