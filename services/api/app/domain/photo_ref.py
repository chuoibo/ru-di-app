"""Where an `image_url` points (ADR-0022 §2.1).

Two shapes exist on the wire, and they are kept apart on purpose:

    /contexts/{context_id}/photos/{photo_id}   a group's photograph
    /people/{person_id}/photos/{photo_id}      a person's own photograph

A group photograph is read behind a membership check; a personal one behind
«is there a post (or, later, a live story) pointing at it that this reader may
read». Memories, chat and albums accept only the first shape; a post accepts
either, with a different rule for each. This module only parses: it says which
owner and which photo a url names, and refuses anything else with one code,
so the layer above answers 422 rather than crashing on a malformed body.
"""

from __future__ import annotations

import uuid
from dataclasses import dataclass

OWNER_CONTEXT = "context"
OWNER_PERSON = "person"

_SEGMENT_TO_OWNER = {"contexts": OWNER_CONTEXT, "people": OWNER_PERSON}
_OWNER_TO_SEGMENT = {OWNER_CONTEXT: "contexts", OWNER_PERSON: "people"}


class PhotoUrlError(ValueError):
    """A url that names no photograph this product serves. One code: the
    caller has nothing to gain from knowing which part was wrong."""

    def __init__(self, code: str = "MALFORMED"):
        super().__init__(code)
        self.code = code


@dataclass(frozen=True, slots=True)
class PhotoRef:
    owner_kind: str
    owner_id: str
    photo_id: str

    @property
    def url(self) -> str:
        """The canonical spelling: lower-case hyphenated ids. What gets stored
        on a post, so a later `image_url = :url` lookup compares like with
        like whatever form the client typed the ids in."""
        return f"/{_OWNER_TO_SEGMENT[self.owner_kind]}/{self.owner_id}/photos/{self.photo_id}"


def parse_photo_url(image_url: object) -> PhotoRef:
    """`PhotoRef` for a well-formed url; `PhotoUrlError` for anything else.

    Strict on shape: exactly five segments, a known owner word, two ids that
    parse. `..`, schemes, query strings and empty segments all fail the
    segment count or the id parse and never reach a filesystem or a query.
    """
    if not isinstance(image_url, str):
        raise PhotoUrlError()
    parts = image_url.split("/")
    # ["", "contexts" | "people", <owner id>, "photos", <photo id>]
    if len(parts) != 5 or parts[0] != "" or parts[3] != "photos":
        raise PhotoUrlError()
    owner_kind = _SEGMENT_TO_OWNER.get(parts[1])
    if owner_kind is None:
        raise PhotoUrlError()
    try:
        owner_id = uuid.UUID(parts[2])
        photo_id = uuid.UUID(parts[4])
    except ValueError as exc:
        raise PhotoUrlError() from exc
    return PhotoRef(
        owner_kind=owner_kind, owner_id=str(owner_id), photo_id=str(photo_id)
    )


def person_photo_url(person_id: str, photo_id: str) -> str:
    return PhotoRef(OWNER_PERSON, person_id, photo_id).url


def context_photo_url(context_id: str, photo_id: str) -> str:
    return PhotoRef(OWNER_CONTEXT, context_id, photo_id).url
