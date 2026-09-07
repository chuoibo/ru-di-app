"""The route that gives a person id a name.

`contexts.created_by_id` and `memberships.person_id` are foreign keys into
`people`, and no HTTP surface ever wrote that table. `POST /contexts` therefore
answered 500 for every caller -- `ForeignKeyViolation` on
`fk_contexts_created_by` -- and no request existed that would have fixed it.

The same gap reached the reader: `get_guest_envelope` had no names to join, so
the guest page said "Phần của a5b2c277-9b99-4699-a875-ed324e886237" to somebody
being asked for money.

PUT rather than POST because the caller already holds the id. Participant ids
are minted client-side and used in expenses, obligations and envelopes long
before anybody types a name; a server-minted id here would name a person no
expense refers to.
"""

from __future__ import annotations

from typing import Annotated
from uuid import UUID

from fastapi import APIRouter, Depends, Response, status

from app.api.deps import Actor, get_actor, get_photo_storage, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import (
    AccountDeleteRequest,
    BlockedListResponse,
    BlockResponse,
    ContextSummary,
    ErrorResponse,
    PersonContextListResponse,
    PersonRegistrationRequest,
    PersonResponse,
    ProfileResponse,
    ProfileUpdateRequest,
    PublicPersonResponse,
    SavedPlacesResponse,
    SavedPlaceSummary,
)
from app.api.service import ApiService
from app.media.storage import PhotoStorage

router = APIRouter(tags=["people"])


@router.get(
    "/people/me/contexts",
    response_model=PersonContextListResponse,
    responses={401: {"model": ErrorResponse}},
)
def list_my_contexts(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PersonContextListResponse:
    """The caller's groups, invited and active, with the newest message and an
    unread count. `me` on purpose: a session already says who is asking, and a
    person id in the path would be either redundant or a claim about somebody
    else. Declared before any `/people/{person_id}/...` route in this module so
    a literal `me` is never parsed as an id."""
    return ApiService(repository).list_my_contexts(actor)


@router.get(
    "/people/me",
    response_model=ProfileResponse,
    responses={401: {"model": ErrorResponse}, 404: {"model": ErrorResponse}},
)
def get_my_profile(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> ProfileResponse:
    """The caller's own profile with server-counted numbers (M2)."""
    return ApiService(repository).get_my_profile(actor)


@router.patch(
    "/people/me",
    response_model=ProfileResponse,
    responses={
        401: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
    },
)
def update_my_profile(
    request: ProfileUpdateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> ProfileResponse:
    """Partial update: at least one of `display_name`, `bio`, `city`."""
    return ApiService(repository).update_my_profile(request, actor)


@router.get(
    "/people/me/saved-places",
    response_model=SavedPlacesResponse,
    responses={401: {"model": ErrorResponse}},
)
def list_saved_places(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> SavedPlacesResponse:
    return ApiService(repository).list_saved_places(actor)


@router.put(
    "/people/me/saved-places/{place_id}",
    response_model=SavedPlaceSummary,
    status_code=status.HTTP_201_CREATED,
    responses={
        200: {"model": SavedPlaceSummary},
        401: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
    },
)
def save_place(
    place_id: str,
    response: Response,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> SavedPlaceSummary:
    """Bookmark a catalogue place. 201 the first time, 200 when it already was."""
    summary, created = ApiService(repository).save_place(place_id, actor)
    response.status_code = status.HTTP_201_CREATED if created else status.HTTP_200_OK
    return summary


@router.delete(
    "/people/me/saved-places/{place_id}",
    status_code=status.HTTP_204_NO_CONTENT,
    responses={401: {"model": ErrorResponse}, 404: {"model": ErrorResponse}},
)
def unsave_place(
    place_id: str,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> Response:
    ApiService(repository).unsave_place(place_id, actor)
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.get(
    "/people/me/blocked",
    response_model=BlockedListResponse,
    responses={401: {"model": ErrorResponse}},
)
def list_blocked(
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> BlockedListResponse:
    """Who the caller is blocking, so they can lift it. Never who is blocking
    them: that list is the one thing `BLOCKED_IS_SILENT` withholds."""
    return ApiService(repository).list_blocked_people(actor)


@router.delete(
    "/people/me",
    status_code=status.HTTP_204_NO_CONTENT,
    responses={
        401: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
    },
)
def delete_my_account(
    request: AccountDeleteRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
    photo_storage: Annotated[PhotoStorage, Depends(get_photo_storage)],
) -> Response:
    """End this account (ADR-0023 §2.1). The body must say `confirm: true`.

    `me` and not an id: a route that took somebody else's id would be a route
    for deleting somebody else, however carefully it checked afterwards.
    """
    ApiService(repository, photo_storage=photo_storage).delete_own_account(
        request, actor
    )
    return Response(status_code=status.HTTP_204_NO_CONTENT)


@router.post(
    "/people/{person_id}/block",
    response_model=BlockResponse,
    responses={
        401: {"model": ErrorResponse},
        403: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
    },
)
def block_person(
    person_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> BlockResponse:
    """Block somebody. Pressing it twice is the same wall, answered the same
    way -- an error there would only tell the caller what they already did."""
    return ApiService(repository).block_person(person_id, actor)


@router.delete(
    "/people/{person_id}/block",
    response_model=BlockResponse,
    responses={
        401: {"model": ErrorResponse},
        403: {"model": ErrorResponse},
        409: {"model": ErrorResponse},
    },
)
def unblock_person(
    person_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> BlockResponse:
    """Lift a block. Only whoever put it up, and the edge comes back
    `declined` rather than as a restored friendship (§2.3.3)."""
    return ApiService(repository).unblock_person(person_id, actor)


@router.post(
    "/people/{person_id}/dm",
    response_model=ContextSummary,
    status_code=status.HTTP_201_CREATED,
    responses={
        200: {"model": ContextSummary},
        401: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
    },
)
def open_direct_message(
    person_id: UUID,
    response: Response,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> ContextSummary:
    """Open, or find, the caller's private conversation with a friend
    (ADR-0021 §2.5). 201 when the pair was just created, 200 when it already
    existed -- same body either way, shaped like one row of
    `GET /people/me/contexts` so the client can open the chat at once. Every
    refusal is the same 404 with the same sentence; the door says nothing
    about why."""
    summary, created = ApiService(repository).open_direct_message(person_id, actor)
    response.status_code = status.HTTP_201_CREATED if created else status.HTTP_200_OK
    return summary


@router.get(
    "/people/{person_id}",
    response_model=PublicPersonResponse,
    responses={
        401: {"model": ErrorResponse},
        403: {"model": ErrorResponse},
        404: {"model": ErrorResponse},
    },
)
def get_person_profile(
    person_id: UUID,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PublicPersonResponse:
    """Somebody's public profile, for a friend or a groupmate. Declared after
    every literal `/people/me/...` route so `me` is never parsed as an id."""
    return ApiService(repository).get_person_profile(person_id, actor)


@router.put(
    "/people/{person_id}",
    response_model=PersonResponse,
    status_code=status.HTTP_201_CREATED,
    responses={
        200: {"model": PersonResponse},
        403: {"model": ErrorResponse},
        409: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
    },
)
def register_person(
    person_id: UUID,
    request: PersonRegistrationRequest,
    response: Response,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> PersonResponse:
    record, created = ApiService(repository).register_person(
        person_id, request.display_name, actor
    )
    # 201 when this id became a person, 200 when it already was one. A client
    # retrying a lost response needs to be able to tell those apart without
    # the answer changing under it.
    response.status_code = status.HTTP_201_CREATED if created else status.HTTP_200_OK
    return PersonResponse(
        id=record.id,
        display_name=record.display_name,
        created_at=record.created_at,
    )
