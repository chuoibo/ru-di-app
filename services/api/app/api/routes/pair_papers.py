"""The sheet of paper two people pass each other.

## Why every command answers the same four fields

A command here replies `{id, state, version, outing_id?}` and never content.
A retry that lost its answer on the way back replays that body, and a body with
no content in it can never replay something the caller has since stopped being
allowed to read (ADR-0027 §3). Content is read through `GET /papers/{id}`,
where the permission is checked at the moment of reading.

## Why the version is in the body

Sending, answering and withdrawing all pin the version they were looking at. A
client that read v1, waited while the other person proposed v2, and then
pressed «Ừ» is agreeing to a sheet that no longer exists; it gets 409
`paper_version_stale` rather than an agreement recorded against the wrong
evening.

The three POSTs with no fields to pin -- skip, done, keeps -- take no version
because they are about the week rather than about a version of it.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Body, Depends, Response, status

from app.api.deps import Actor, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    ErrorResponse,
    PaperCommandResponse,
    PaperDraftEditRequest,
    PaperKeepRequest,
    PaperKeepResponse,
    PaperListResponse,
    PaperResponse,
    PaperResponseRequest,
    PaperSendRequest,
    PaperWithdrawRequest,
)
from app.api.service import ApiService

router = APIRouter(tags=["pair-papers"])
ERRORS = {
    403: {"model": ErrorResponse},
    404: {"model": ErrorResponse},
    409: {"model": ErrorResponse},
    422: {"model": ErrorResponse},
}


@router.get(
    "/contexts/{context_id}/papers",
    response_model=PaperListResponse,
    responses=ERRORS,
)
def list_pair_papers(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperListResponse:
    """Every sheet this person may see. A draft belongs to whoever started it,
    so the other person's list does not mention one."""

    return ApiService(repository).list_pair_papers(context_id, actor)


@router.post(
    "/contexts/{context_id}/papers/draft",
    response_model=PaperCommandResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def draft_pair_paper(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """Ask the notebook for a sheet, pre-filled with a shape to edit.

    A command rather than a read with a side effect: opening the screen twice
    must not leave two sheets.
    """

    return ApiService(repository).draft_pair_paper(context_id, actor)


@router.get("/papers/{paper_id}", response_model=PaperResponse, responses=ERRORS)
def read_pair_paper(
    paper_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperResponse:
    """The sheet with everything written on it, as this reader may see it."""

    return ApiService(repository).pair_paper(paper_id, actor)


@router.patch(
    "/papers/{paper_id}/draft",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def edit_pair_draft(
    paper_id: UUID,
    request: PaperDraftEditRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """Rewrite one's own draft, while it is still a draft."""

    return ApiService(repository).edit_pair_draft(paper_id, request, actor)


@router.post(
    "/papers/{paper_id}/send",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def send_pair_paper(
    paper_id: UUID,
    request: PaperSendRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """Hand it over. Pressing send is agreeing to what was sent."""

    return ApiService(repository).send_pair_paper(paper_id, request, actor)


@router.post(
    "/papers/{paper_id}/versions/{version}/viewed",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def mark_pair_paper_viewed(
    paper_id: UUID,
    version: int,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """«Đã xem», recorded once and only for the person who received it."""

    ApiService(repository).mark_pair_paper_viewed(paper_id, version, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.post(
    "/papers/{paper_id}/versions/{version}/responses",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def respond_pair_paper(
    paper_id: UUID,
    version: int,
    request: Annotated[PaperResponseRequest, Body()],
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """«Ừ» or «đề nghị sửa». The second is a new version, sent by whoever
    proposed it and agreed to by them in the same breath."""

    return ApiService(repository).respond_pair_paper(paper_id, version, request, actor)


@router.post(
    "/papers/{paper_id}/withdraw",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def withdraw_pair_paper(
    paper_id: UUID,
    request: PaperWithdrawRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """Take it back, while the other person has neither opened nor answered."""

    return ApiService(repository).withdraw_pair_paper(paper_id, request, actor)


@router.post(
    "/papers/{paper_id}/skip",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def skip_pair_week(
    paper_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """«Tuần này nghỉ», which either of them may say."""

    return ApiService(repository).skip_pair_week(paper_id, actor)


@router.post(
    "/papers/{paper_id}/done",
    response_model=PaperCommandResponse,
    responses=ERRORS,
)
def record_pair_outing_done(
    paper_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperCommandResponse:
    """«Đã đi rồi», with the name of whoever says so and never inferred from
    the date."""

    return ApiService(repository).record_pair_outing_done(paper_id, actor)


@router.post(
    "/papers/{paper_id}/keeps",
    response_model=PaperKeepResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def keep_pair_paper_line(
    paper_id: UUID,
    request: PaperKeepRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PaperKeepResponse:
    """One line kept afterwards. The first one closes the sheet for good."""

    return ApiService(repository).keep_pair_paper_line(paper_id, request, actor)
