"""What ``create_app`` is responsible for: every router, and both middlewares.

Four branches grew a layer of this function at the same time -- the people
router, the idempotency middleware, the CORS middleware, and ``POST
/receipts/scan``, the step that turns a photograph of a bill into items. All
four edit the same twenty lines, so all four conflict with each other, and a
conflict resolved by keeping the wrong side is silent in a way no other test in
this suite catches: a router that stops being registered takes its own tests
down with it, and every remaining file still passes.

That is not hypothetical. ``/receipts/scan`` was written, reviewed, and then
absent from ``main`` -- the camera could take the photo and the model could read
it, with nothing in between. This file is the inventory to re-read whenever
those twenty lines are touched.

It asserts wiring, and only wiring. Whether each route behaves correctly is the
business of the file named after it.
"""

from __future__ import annotations

from contextlib import contextmanager

import anyio
import pytest

from app.api.cors import PreflightNoContentCORSMiddleware
from app.api.idempotency import IdempotencyMiddleware
from app.api.main import create_app

from .conftest import ASGITestClient
from .test_idempotency import InMemoryIdempotencyStore

# Every step the PoC demo walks, in order, plus the two reads it needs on the
# way. Written out as data rather than derived from the app, because deriving
# the expectation from the thing under test is how an empty list passes.
DEMO_PATH_ROUTES = {
    ("POST", "/contexts"),
    ("POST", "/contexts/{context_id}/members"),
    ("POST", "/memberships/{membership_id}/accept"),
    ("PUT", "/people/{person_id}"),
    ("POST", "/receipts/scan"),
    ("POST", "/expenses"),
    ("POST", "/expenses/{expense_id}/confirm"),
    ("POST", "/batches"),
    ("POST", "/batches/{batch_id}/publish"),
    ("GET", "/batches/{batch_id}/obligations"),
    # The two `/bank-recipients` steps left with the payment rail: the demo
    # walks as far as telling each person their share, and no further.
    ("GET", "/g/{token}"),
    ("POST", "/g/{token}/da-chuyen"),
    ("POST", "/obligations/{obligation_id}/confirm-receipt"),
}


def registered(app) -> set[tuple[str, str]]:
    found = set()
    for route in app.routes:
        for method in getattr(route, "methods", None) or ():
            if method != "HEAD":
                found.add((method, route.path))
    return found


class TestRoutersAreRegistered:
    def test_receipt_scan_exists(self):
        """The link between the camera and the split. Absent from main once."""

        assert ("POST", "/receipts/scan") in registered(create_app())

    def test_no_step_of_the_demo_path_is_missing(self):
        missing = DEMO_PATH_ROUTES - registered(create_app())
        assert missing == set()


class TestMiddlewareOrder:
    """Order is not cosmetic here, so it is asserted rather than assumed.

    ``add_middleware`` prepends, so index 0 is outermost. CORS has to stay
    there: the idempotency layer answers three refusals by itself, before any
    route runs, and an answer that leaves the CORS layer unentered carries no
    allow-origin header. The browser then discards a 409 that said exactly what
    the person needed to hear and reports an opaque network failure instead.
    """

    def test_cors_is_outermost(self):
        assert create_app().user_middleware[0].cls is PreflightNoContentCORSMiddleware

    def test_idempotency_is_inside_cors(self):
        stack = [entry.cls for entry in create_app().user_middleware]
        assert stack.index(IdempotencyMiddleware) > stack.index(
            PreflightNoContentCORSMiddleware
        )


@pytest.fixture
def store():
    return InMemoryIdempotencyStore()


@pytest.fixture
def scan_client(monkeypatch, store):
    """The scan route with the real middleware above it and no database.

    ``create_app()`` would reach for PostgreSQL the moment a key arrives, which
    is a fact about this route worth stating out loud: it stores nothing itself,
    but a caller that sends a key makes it a database-backed request.
    """

    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    # The policy is read when the app is built, so a value left in the
    # developer's environment would otherwise decide what these tests assert.
    monkeypatch.delenv("MOBILE_CORS_ALLOW_ORIGINS", raising=False)

    @contextmanager
    def factory():
        yield store

    return ASGITestClient(create_app(idempotency_store_factory=factory))


class TestTheScanRouteIsReachableFromABrowser:
    """The preflight the web build sends before it uploads anything.

    Answered by the CORS layer ahead of routing, so this passes whether or not
    the route exists -- it is here for the other half of the same conflict. Drop
    ``install_cors`` while resolving it and the browser's preflight reaches the
    router, which answers 405 for a method it has no route for, and the
    photograph never leaves the phone.
    """

    def test_preflight_for_a_scan_is_allowed(self, scan_client):
        response = scan_client.request(
            "OPTIONS",
            "/receipts/scan",
            headers={
                "Origin": "http://localhost:8080",
                "Access-Control-Request-Method": "POST",
                "Access-Control-Request-Headers": "content-type,x-actor-id",
            },
        )

        assert response.status_code == 204
        assert (
            response.headers["access-control-allow-origin"] == "http://localhost:8080"
        )
