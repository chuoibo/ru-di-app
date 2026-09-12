"""Exercise the real HTTP adapter against a local synthetic provider boundary."""

import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread

import pytest

from app.journey.routing import RoutingUnavailable, ValhallaProvider, _decode_shape


@pytest.fixture
def server():
    requests = []
    replies = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            requests.append(
                (
                    self.path,
                    json.loads(self.rfile.read(int(self.headers["Content-Length"]))),
                )
            )
            status, payload, headers = replies.pop(0)
            self.send_response(status)
            for key, value in headers.items():
                self.send_header(key, value)
            self.end_headers()
            self.wfile.write(json.dumps(payload).encode())

        def log_message(self, *args):
            pass

    httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    yield (
        ValhallaProvider(f"http://127.0.0.1:{httpd.server_port}", "synthetic-graph"),
        requests,
        replies,
    )
    httpd.shutdown()
    httpd.server_close()
    thread.join()


@pytest.mark.parametrize(
    ("mode", "costing"),
    [("motorbike", "motor_scooter"), ("car", "auto"), ("walk", "pedestrian")],
)
def test_mode_directed_matrix_units_and_null_unreachable(server, mode, costing):
    provider, requests, replies = server
    replies.append(
        (
            200,
            {
                "sources_to_targets": [
                    [{"time": 0, "distance": 0}, {"time": 12.2, "distance": 1.25}],
                    [{"time": None, "distance": None}, {"time": 0, "distance": 0}],
                ]
            },
            {},
        )
    )
    result = provider.matrix(
        [{"lat": 11.94, "lon": 108.43}, {"lat": 11.95, "lon": 108.44}], mode
    )
    assert result == [[(0, 0), (13, 1250)], [None, (0, 0)]]
    assert requests[0][0] == "/sources_to_targets"
    assert requests[0][1]["costing"] == costing
    assert "date_time" not in requests[0][1]


def test_route_uses_post_break_points_and_decodes_polyline6(server):
    provider, requests, replies = server
    # Google's documented polyline5 example interpreted at Valhalla precision6.
    shape = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
    replies.append(
        (
            200,
            {
                "trip": {
                    "status": 0,
                    "units": "kilometers",
                    "legs": [
                        {"summary": {"time": 12.3, "length": 0.567}, "shape": shape}
                    ],
                }
            },
            {},
        )
    )
    result = provider.route(
        [{"lat": 3.85, "lon": -12.02}, {"lat": 4.3252, "lon": -12.6453}], "car"
    )
    assert result[0]["geometry"] == [
        [-12.02, 3.85],
        [-12.095, 4.07],
        [-12.6453, 4.3252],
    ]
    assert result[0]["distance_meters"] == 567
    assert result[0]["duration_seconds"] == 13
    assert requests[0][1]["locations"][0]["type"] == "break"


def test_redirect_never_forwards_private_coordinates(server):
    provider, requests, replies = server
    replies.append((307, {}, {"Location": "https://example.com/private-pins"}))
    with pytest.raises(RoutingUnavailable, match="^routing_unavailable$"):
        provider.matrix([{"lat": 11.94, "lon": 108.43}], "car")
    assert len(requests) == 1


@pytest.mark.parametrize(
    "payload",
    [
        {"sources_to_targets": []},
        {"sources_to_targets": [[{"time": -1, "distance": 5}]]},
        {"sources_to_targets": [[{"time": True, "distance": 5}]]},
        {"sources_to_targets": [[{"time": float("nan"), "distance": 5}]]},
        {"error_code": 171, "error": "request coordinates intentionally not returned"},
    ],
)
def test_malformed_provider_response_is_sanitized(server, payload):
    provider, _, replies = server
    replies.append((200, payload, {}))
    with pytest.raises(RoutingUnavailable, match="^routing_unavailable$"):
        provider.matrix([{"lat": 11.94, "lon": 108.43}], "walk")


@pytest.mark.parametrize("shape", ["", "_", "????_", "x" * 40])
def test_truncated_or_overflow_shape_refused(shape):
    with pytest.raises(ValueError):
        _decode_shape(shape)


@pytest.mark.parametrize("value", [1e308, 10**400])
@pytest.mark.parametrize("action", ["matrix", "route"])
def test_extreme_provider_cost_never_escapes_as_overflow(server, value, action):
    provider, _, replies = server
    payload = (
        {"sources_to_targets": [[{"time": 1, "distance": value}]]}
        if action == "matrix"
        else {
            "trip": {
                "status": 0,
                "legs": [{"summary": {"time": 1, "length": value}, "shape": "????"}],
            }
        }
    )
    replies.append((200, payload, {}))
    points = [{"lat": 11.94, "lon": 108.43}]
    with pytest.raises(RoutingUnavailable, match="^routing_unavailable$"):
        if action == "matrix":
            provider.matrix(points, "walk")
        else:
            provider.route(points * 2, "walk")
