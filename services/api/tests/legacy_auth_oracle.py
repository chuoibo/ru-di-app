"""Test-only ADR-0014/0016 HTTP oracle; never imported by a runtime.

The retired handlers keep historical wire/SQL evidence. New account behavior
is tested through Go and PostgreSQL, and production retirement separately.
"""

from app.api.main import create_app as create_runtime_app
from app.api.routes import auth, friends, identity, sessions
from app.api.schemas import PersonMatchResponse, SessionResponse


def create_app(*args, **kwargs):
    application = create_runtime_app(*args, **kwargs)
    application.include_router(auth.router)
    application.include_router(identity.router)
    application.add_api_route(
        "/friends/lookup",
        friends.find_person_by_phone,
        methods=["POST"],
        response_model=PersonMatchResponse,
    )
    application.add_api_route(
        "/sessions",
        sessions.create_session,
        methods=["POST"],
        response_model=SessionResponse,
        status_code=201,
    )
    return application
