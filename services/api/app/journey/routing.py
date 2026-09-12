"""Valhalla adapter. Only the configured private service receives coordinates."""

from __future__ import annotations

import json
import math
import os
from typing import Any, Protocol
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener

from app.domain.journey import Matrix

MODES = {"motorbike": "motor_scooter", "car": "auto", "walk": "pedestrian"}
MAX_RESPONSE_BYTES = 8 * 1024 * 1024


class RoutingUnavailable(Exception):
    """A private provider failed; no request details may reach API errors/logs."""


class RoutingProvider(Protocol):
    graph_version: str

    def matrix(self, points: list[dict[str, float]], mode: str) -> Matrix: ...

    def route(
        self, points: list[dict[str, float]], mode: str
    ) -> list[dict[str, Any]]: ...


class _NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RoutingUnavailable("routing_unavailable")


def _number(value: Any) -> float:
    if (
        isinstance(value, bool)
        or not isinstance(value, int | float)
        or value > 1_000_000_000
        or not math.isfinite(value)
        or value < 0
    ):
        raise ValueError("invalid_cost")
    return value


def _decode_shape(encoded: str) -> list[list[float]]:
    """Decode Valhalla's polyline6 as GeoJSON-order coordinate pairs."""
    cursor = 0
    lat = lng = 0
    result = []
    while cursor < len(encoded):
        coordinates = []
        for _ in range(2):
            bits = shift = 0
            while True:
                if cursor >= len(encoded) or shift > 30:
                    raise ValueError("invalid_shape")
                byte = ord(encoded[cursor]) - 63
                cursor += 1
                if not 0 <= byte <= 63:
                    raise ValueError("invalid_shape")
                bits |= (byte & 31) << shift
                shift += 5
                if byte < 32:
                    break
            coordinates.append(~(bits >> 1) if bits & 1 else bits >> 1)
        lat += coordinates[0]
        lng += coordinates[1]
        if abs(lat) > 90_000_000 or abs(lng) > 180_000_000:
            raise ValueError("invalid_shape")
        result.append([lng / 1_000_000, lat / 1_000_000])
    if len(result) < 2:
        raise ValueError("invalid_shape")
    return result


class ValhallaProvider:
    def __init__(self, base_url: str, graph_version: str, *, timeout: float = 12):
        parsed = urlsplit(base_url)
        if (
            parsed.scheme not in {"http", "https"}
            or not parsed.hostname
            or parsed.username
            or parsed.password
            or parsed.query
            or parsed.fragment
        ):
            raise ValueError("invalid_routing_configuration")
        if not graph_version.strip():
            raise ValueError("missing_graph_version")
        self.base_url = base_url.rstrip("/")
        self.graph_version = graph_version
        self.timeout = timeout
        # Do not forward private pins through a host proxy or follow a redirect.
        self._opener = build_opener(ProxyHandler({}), _NoRedirect())

    def _post(self, action: str, payload: dict[str, Any]) -> dict[str, Any]:
        request = Request(
            self.base_url + "/" + action,
            data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with self._opener.open(request, timeout=self.timeout) as response:
                raw = response.read(MAX_RESPONSE_BYTES + 1)
            if len(raw) > MAX_RESPONSE_BYTES:
                raise ValueError("response_too_large")
            result = json.loads(raw)
            if not isinstance(result, dict) or "error_code" in result:
                raise ValueError("routing_failed")
            return result
        except (HTTPError, URLError, TimeoutError, OSError, ValueError):
            raise RoutingUnavailable("routing_unavailable") from None

    def matrix(self, points: list[dict[str, float]], mode: str) -> Matrix:
        payload = {
            "sources": points,
            "targets": points,
            "costing": MODES[mode],
            "units": "kilometers",
            "verbose": True,
        }
        response = self._post("sources_to_targets", payload)
        try:
            rows = response["sources_to_targets"]
            if len(rows) != len(points) or any(len(row) != len(points) for row in rows):
                raise ValueError("invalid_matrix")
            return [
                [
                    None
                    if c.get("time") is None or c.get("distance") is None
                    else (
                        math.ceil(_number(c["time"])),
                        round(_number(c["distance"]) * 1000),
                    )
                    for c in row
                ]
                for row in rows
            ]
        except (KeyError, TypeError, ValueError, AttributeError):
            raise RoutingUnavailable("routing_unavailable") from None

    def route(self, points: list[dict[str, float]], mode: str) -> list[dict[str, Any]]:
        if len(points) < 2:
            return []
        payload = {
            "locations": [{**point, "type": "break"} for point in points],
            "costing": MODES[mode],
            "units": "kilometers",
            "shape_format": "polyline6",
            "directions_options": {"units": "kilometers"},
        }
        response = self._post("route", payload)
        try:
            trip = response["trip"]
            legs = trip["legs"]
            if (
                trip["status"] != 0
                or len(legs) != len(points) - 1
                or trip.get("units", "kilometers") != "kilometers"
            ):
                raise ValueError("invalid_route")
            return [
                {
                    "distance_meters": round(_number(leg["summary"]["length"]) * 1000),
                    "duration_seconds": math.ceil(_number(leg["summary"]["time"])),
                    "geometry": _decode_shape(leg["shape"]),
                    "source": "valhalla",
                }
                for leg in legs
            ]
        except (KeyError, TypeError, ValueError, AttributeError):
            raise RoutingUnavailable("routing_unavailable") from None


def configured_provider() -> RoutingProvider | None:
    url = os.environ.get("MOBILE_VALHALLA_URL", "")
    version = os.environ.get("MOBILE_ROUTING_GRAPH_VERSION", "")
    if not url or not version:
        return None
    try:
        return ValhallaProvider(url, version)
    except ValueError:
        return None
