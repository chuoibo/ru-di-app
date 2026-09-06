"""F39 posts and the F42 audiences that decide who reads them.

## Why these paths carry no context id

`/contexts/{id}/memories` puts the group in the path so the membership check
can run before the row is looked up. A post cannot do that: three of its four
audiences address *people* rather than a group, and an `only_me` post lives in
no group at all. There is no container to gate on.

So the gate moves. `ApiService` runs `app.domain.post_audience.can_read` for
the actor over every row before it is serialised, and a post the actor may not
read comes back 404 rather than 403 -- see `read_post` for why the difference
matters. The rule is the same one the SQL in
`SqlAlchemyApiRepository._readable_by` refuses to fetch by; these routes state
neither and decide nothing.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Query, Response, status

from app.api.deps import Actor, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    ErrorResponse,
    PersonPostListResponse,
    PostCommentCreateRequest,
    PostCommentListResponse,
    PostCommentResponse,
    PostCreateRequest,
    PostListResponse,
    PostReactionKind,
    PostReactionRequest,
    PostReactionsResponse,
    PostResponse,
)
from app.api.service import ApiService

router = APIRouter(tags=["posts"])
ERRORS = {
    403: {"model": ErrorResponse},
    404: {"model": ErrorResponse},
    422: {"model": ErrorResponse},
}


@router.post(
    "/posts",
    response_model=PostResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def create_post(
    request: PostCreateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PostResponse:
    """F39. Say something, as yourself, to one of four audiences.

    The body has no `author_id` and no recipient list. Both are absent by
    design and not by omission: the writer is the actor the gateway proved,
    and the audience is a word rather than a list of people nobody verified
    the caller may name.
    """

    return ApiService(repository).create_post(request, actor)


@router.get("/posts", response_model=PostListResponse, responses=ERRORS)
def list_posts(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    limit: Annotated[int, Query(ge=1, le=100)] = 50,
) -> PostListResponse:
    """Everything this actor may read, newest first.

    There is no `viewer_id` query parameter and there will not be one. The
    reader is the actor; a parameter naming somebody else would be a route for
    reading the feed through another person's eyes.
    """

    return ApiService(repository).list_posts(actor, limit=limit)


@router.get(
    "/people/{person_id}/posts",
    response_model=PersonPostListResponse,
    responses=ERRORS,
)
def list_person_posts(
    person_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    limit: Annotated[int, Query(ge=1, le=100)] = 50,
) -> PersonPostListResponse:
    """One person's wall, already narrowed to what the caller may see.

    Answers 200 with an empty list for a person the caller shares nothing
    with. Telling "this person has nothing for you" apart from "no such
    person" would make the route a directory of who holds an account.
    """

    return ApiService(repository).list_person_posts(person_id, actor, limit=limit)


@router.get("/posts/{post_id}", response_model=PostResponse, responses=ERRORS)
def read_post(
    post_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PostResponse:
    """One post, or 404 -- including when it exists and is not for you."""

    return ApiService(repository).read_post(post_id, actor)


# --- reactions and comments (ADR-0022 §2.2) ----------------------------------
#
# Every route below goes through `_readable_post_or_404` first: a post the
# actor may not read answers 404 here exactly as `read_post` does, so none of
# these becomes the oracle `read_post` refuses to be.


@router.post(
    "/posts/{post_id}/reactions",
    response_model=PostReactionsResponse,
    responses=ERRORS,
)
def react_to_post(
    post_id: UUID,
    request: PostReactionRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PostReactionsResponse:
    """One reaction of one kind; the same tap twice is one row. 200 with the
    recounted totals both times."""

    return ApiService(repository).react_to_post(post_id, request, actor)


@router.delete(
    "/posts/{post_id}/reactions/{kind}",
    response_model=PostReactionsResponse,
    responses=ERRORS,
)
def unreact_to_post(
    post_id: UUID,
    kind: PostReactionKind,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PostReactionsResponse:
    """Take back one's own reaction. 200 with the totals rather than 204, so
    the screen redraws from the server's count and not from its own."""

    return ApiService(repository).unreact_to_post(post_id, kind, actor)


@router.get(
    "/posts/{post_id}/comments",
    response_model=PostCommentListResponse,
    responses=ERRORS,
)
def list_post_comments(
    post_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    limit: Annotated[int, Query(ge=1, le=100)] = 50,
    after: Annotated[str | None, Query()] = None,
) -> PostCommentListResponse:
    """Oldest first, one page; `after` continues forward."""

    return ApiService(repository).list_post_comments(
        post_id, actor, limit=limit, after=after
    )


@router.post(
    "/posts/{post_id}/comments",
    response_model=PostCommentResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def post_comment(
    post_id: UUID,
    request: PostCommentCreateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PostCommentResponse:
    """Say something under a post one may read. 403 `comments_closed` when the
    wall owner's policy shuts the composer for this reader."""

    return ApiService(repository).post_comment(post_id, request, actor)


@router.delete(
    "/posts/{post_id}/comments/{comment_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def delete_post_comment(
    post_id: UUID,
    comment_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """The comment's author or the post's author; nobody else."""

    ApiService(repository).delete_post_comment(post_id, comment_id, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
