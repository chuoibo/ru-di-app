"""Where a person gets a session, and where they give one back.

Two things about this module are deliberate.

**No actor dependency on the way in.** Everywhere else in this package a
handler starts by naming who is asking. This one cannot: it is the route where
that answer is obtained. The same shape, and the same reason, as the identity
route that mints a person id.

**Nothing in the request says who the caller is.** The body carries one
secret and no `person_id`. Which person the session belongs to is read from
the invitation row, where a member wrote it. A route that accepted both a token
and a name would be back to trusting the client about identity, with a longer
credential -- which is the hole ADR-0014 exists to close, not a smaller one.

Logout is here rather than beside it in the mobile client for the ordinary
reason: a session that only the phone forgets is still a live credential on the
server, and a stolen phone is exactly when that matters.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Header, Response, status

from app.api.deps import Actor, bearer_token, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    ErrorResponse,
    SessionBootstrapRequest,
    SessionListResponse,
    SessionResponse,
)
from app.api.service import ApiService

router = APIRouter(tags=["sessions"])


@router.post(
    "/sessions",
    response_model=SessionResponse,
    status_code=status.HTTP_201_CREATED,
    responses={
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
    },
)
def create_session(
    request: SessionBootstrapRequest,
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> SessionResponse:
    return ApiService(repository).bootstrap_session_from_invite(request.invite_token)


@router.get(
    "/sessions",
    response_model=SessionListResponse,
    responses={401: {"model": ErrorResponse}},
)
def list_sessions(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    authorization: Annotated[str | None, Header(alias="Authorization")] = None,
) -> SessionListResponse:
    """Where this account is signed in (ADR-0023 §2.5).

    The header is read again here, next to the actor, for one reason: the
    screen has to be able to say which row is «phiên này», and the actor
    carries a person, not a session.
    """
    return ApiService(repository).list_account_sessions(
        actor, current_token=bearer_token(authorization) if authorization else None
    )


@router.delete(
    "/sessions/{session_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses={401: {"model": ErrorResponse}, 404: {"model": ErrorResponse}},
)
def revoke_session(
    session_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """Sign one other device out. Somebody else's session answers 404: a 403
    would confirm that the id names a real session. Declared after
    `/sessions/current` so the literal is never parsed as an id."""
    ApiService(repository).revoke_account_session(session_id, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.delete(
    "/sessions/current",
    status_code=status.HTTP_204_NO_CONTENT,
    responses={401: {"model": ErrorResponse}},
)
def revoke_current_session(
    repository: Annotated[ApiRepository, Depends(get_repository)],
    authorization: Annotated[str | None, Header(alias="Authorization")] = None,
) -> Response:
    # Written `-> Response` rather than `-> None` on purpose: FastAPI refuses
    # to build a router where a 204 promises a body, and the two library
    # versions this repo runs on disagree about which annotation counts as a
    # promise. See `tests/api/test_bodyless_status_declarations.py`.
    ApiService(repository).revoke_session_token(bearer_token(authorization))
    return Response(status_code=status.HTTP_204_NO_CONTENT)
