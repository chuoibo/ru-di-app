"""F32 and F33, the suggestion cards: served by Go since ADR-0052.

The prompt builders and the model call moved to `services/core`
(internal/routes/suggestions_wai.go, internal/aiharness/goiy). What stays here
is each route's declaration: the Go front door still takes its route order,
its request contract and the limiter it shares from this app's table
(services/core/ownership/routes.json, `python: frozen`, state PY-DELETED).
Reached directly, each says where it went.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Request

from app.api.deps import Actor, get_actor, get_repository
from app.api.errors import ApiProblem
from app.api.repository import ApiRepository
from app.api.schemas import (
    ContextualSuggestionResponse,
    ErrorResponse,
    GroupSuggestionResponse,
)
from app.api.search_rate_limit import FixedWindowLimiter

router = APIRouter(tags=["suggestions"])


def get_suggestion_limiter(request: Request) -> FixedWindowLimiter:
    """Resolve the one suggestion limiter owned by this application instance.

    Read off the application rather than constructed here: a limiter built per
    request counts to one and forgets, which is a limiter-shaped object that
    limits nothing.
    """

    return request.app.state.suggestion_limiter


def get_contextual_suggestion_limiter(request: Request) -> FixedWindowLimiter:
    """The F33 window, read off the application for the same reason."""

    return request.app.state.contextual_suggestion_limiter


@router.get(
    "/contexts/{context_id}/suggestion",
    response_model=GroupSuggestionResponse,
    responses={
        403: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
        429: {"model": ErrorResponse},
    },
)
def read_group_suggestion(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    limiter: Annotated[FixedWindowLimiter, Depends(get_suggestion_limiter)],
) -> GroupSuggestionResponse:
    """Declaration only: the Go core serves this route (ADR-0052)."""

    del context_id, actor, repository, limiter
    raise ApiProblem(
        410, "served_by_go", "GET /contexts/{id}/suggestion do core Go phục vụ."
    )


@router.get(
    "/contexts/{context_id}/contextual-suggestion",
    response_model=ContextualSuggestionResponse,
    responses={
        403: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
        429: {"model": ErrorResponse},
    },
)
def read_contextual_suggestion(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    limiter: Annotated[FixedWindowLimiter, Depends(get_contextual_suggestion_limiter)],
) -> ContextualSuggestionResponse:
    """Declaration only: the Go core serves this route (ADR-0052)."""

    del context_id, actor, repository, limiter
    raise ApiProblem(
        410,
        "served_by_go",
        "GET /contexts/{id}/contextual-suggestion do core Go phục vụ.",
    )
