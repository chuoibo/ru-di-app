"""The two-person notebook: consent, the two shared lines, and closing it.

## Why the notebook answers 404 rather than 403

Every path here hangs off a context id, and three different facts leave by the
same door with the same sentence: there is no such context, it is a group
rather than a pair, and the caller is not one of the two people in it. A 403
would answer «do these two people have a notebook together» for anybody
holding an id, and that is the one question a two-person notebook must not
answer. `ApiService._pair_context_or_404` is where that is decided; these
routes state nothing.

## Why closing takes two calls

The preview is a POST because it mints the revision the close must carry
(ADR-0027 §7.6). Reading «3 tờ sẽ khoá» and pressing «Đóng sổ» are separate
moments, and between them the other person may send something; the revision is
what makes the count somebody agreed to and the rows being closed provably the
same rows.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Response, status

from app.api.deps import Actor, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    CloseNotebookRequest,
    ClosePreviewResponse,
    ErrorResponse,
    PairConstraintPutRequest,
    PairConstraintResponse,
    PairNotebookResponse,
    PairProposalCreateRequest,
    PairProposalResponse,
)
from app.api.service import ApiService

router = APIRouter(tags=["pair-notebooks"])
ERRORS = {
    403: {"model": ErrorResponse},
    404: {"model": ErrorResponse},
    409: {"model": ErrorResponse},
    422: {"model": ErrorResponse},
}


@router.get(
    "/contexts/{context_id}/notebook",
    response_model=PairNotebookResponse,
    responses=ERRORS,
)
def read_pair_notebook(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PairNotebookResponse:
    """What the two of them have agreed to, and the one sheet in play."""

    return ApiService(repository).pair_notebook(context_id, actor)


@router.post(
    "/contexts/{context_id}/notebook/proposals",
    response_model=PairProposalResponse,
    status_code=status.HTTP_201_CREATED,
    responses=ERRORS,
)
def propose_pair_consent(
    context_id: UUID,
    request: PairProposalCreateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PairProposalResponse:
    """Ask for one rung of the ladder. Asking is agreeing: the asker's own
    grant is written with the offer."""

    return ApiService(repository).propose_pair_consent(context_id, request, actor)


@router.post(
    "/contexts/{context_id}/notebook/proposals/{proposal_id}/grant",
    response_model=PairProposalResponse,
    responses=ERRORS,
)
def grant_pair_consent(
    context_id: UUID,
    proposal_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PairProposalResponse:
    """The second yes. What it unlocks depends on which rung it was."""

    return ApiService(repository).grant_pair_consent(context_id, proposal_id, actor)


@router.delete(
    "/contexts/{context_id}/notebook/consents/{purpose}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def revoke_pair_consent(
    context_id: UUID,
    purpose: str,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """Take back one's own grant."""

    ApiService(repository).revoke_pair_consent(context_id, purpose, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.put(
    "/contexts/{context_id}/notebook/constraints/{kind}",
    response_model=PairConstraintResponse,
    responses=ERRORS,
)
def put_pair_constraint(
    context_id: UUID,
    kind: str,
    request: PairConstraintPutRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PairConstraintResponse:
    """«Không ăn được» or «Đừng» -- what one says about oneself, in the shared
    area where the other may read it but not rewrite it."""

    return ApiService(repository).put_pair_constraint(context_id, kind, request, actor)


@router.delete(
    "/contexts/{context_id}/notebook/constraints/{kind}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def delete_pair_constraint(
    context_id: UUID,
    kind: str,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    ApiService(repository).delete_pair_constraint(context_id, kind, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.post(
    "/contexts/{context_id}/notebook/close/preview",
    response_model=ClosePreviewResponse,
    responses=ERRORS,
)
def preview_close_pair_notebook(
    context_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> ClosePreviewResponse:
    """Three counts and the revision that pins them."""

    return ApiService(repository).preview_close_pair_notebook(context_id, actor)


@router.post(
    "/contexts/{context_id}/notebook/close",
    status_code=status.HTTP_204_NO_CONTENT,
    responses=ERRORS,
)
def close_pair_notebook(
    context_id: UUID,
    request: CloseNotebookRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    """Closing is closing. Nothing is deleted; sheets in play are cancelled and
    plans that stood stay readable."""

    ApiService(repository).close_pair_notebook(context_id, request, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
