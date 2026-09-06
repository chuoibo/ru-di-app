"""24-hour stories (L4, ADR-0022 §2.3).

## Why these paths carry no person id

A story is addressed to the author's friends, and «who are your friends» is a
fact only the server holds. So, as with posts, the gate moves into
`ApiService`: `app.domain.story_visibility.can_view` runs for the actor over
every row before it is serialised, the feed is fetched by the same rule
spelled in SQL, and a story the actor may not see answers 404 -- never 403,
for the reason `read_post` gives. These routes state nothing and decide
nothing.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Response, status

from app.api.deps import Actor, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    ErrorResponse,
    StoryCreateRequest,
    StoryFeedResponse,
    StoryResponse,
    StorySeenResponse,
)
from app.api.service import ApiService

router = APIRouter(tags=["stories"])
ERRORS = {
    403: {"model": ErrorResponse},
    404: {"model": ErrorResponse},
    422: {"model": ErrorResponse},
}


@router.post(
    "/stories",
    response_model=StoryResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def create_story(
    request: StoryCreateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> StoryResponse:
    """One of one's own photographs, to one's friends, for 24 hours.

    The body names a photo and a caption; the author is the actor and the
    deadline is computed here. Neither is a field a caller could set.
    """

    return ApiService(repository).create_story(request, actor)


@router.get("/stories", response_model=StoryFeedResponse, responses=ERRORS)
def list_stories(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> StoryFeedResponse:
    """Every live story this actor may see, grouped by author: one's own
    first, then authors with something unseen, then the rest."""

    return ApiService(repository).list_stories(actor)


@router.post(
    "/stories/{story_id}/seen",
    response_model=StorySeenResponse,
    responses=ERRORS,
)
def mark_story_seen(
    story_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> StorySeenResponse:
    """«I have seen this.» Idempotent: the first look is the one recorded."""

    return ApiService(repository).mark_story_seen(story_id, actor)


@router.delete(
    "/stories/{story_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def delete_story(
    story_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """The author takes their story down early; nobody else may."""

    ApiService(repository).delete_story(story_id, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
