"""Replying to and deleting a chat message (ADR-0021 §2.2–2.3).

Pure: dicts in, dicts out. The service reads the rows and hands them here;
this module says what may happen and what the row must look like afterwards.
The database says the same thing twice more (the payload CHECK and the
`(kind = 'deleted') = (deleted_at IS NOT NULL)` CHECK), and the wire model a
third time, so a deletion that keeps its text cannot survive any layer.

Why a deletion is a new `kind` and not a removed row: `reply_to_id` of other
messages and `context_read_marks.last_read_message_id` point at it. A removed
row breaks a reply quote and a read mark; a `deleted` row reads as «Tin nhắn
đã bị xoá» wherever it is shown.
"""

from __future__ import annotations

from datetime import datetime

__all__ = [
    "DELETABLE_KINDS",
    "REPLYABLE_KINDS",
    "MessageEditError",
    "check_deletable",
    "check_reply_target",
    "deleted_shape",
]

#: What a person may take back: their own words, pictures and stickers. An
#: `ai_card` is left alone -- a poll card has a vote behind it and an expense
#: draft card has a ledger entry that may quote it.
DELETABLE_KINDS: tuple[str, ...] = ("text", "image", "sticker")

#: What may be quoted. Cards are not quoted: their preview is the card itself.
REPLYABLE_KINDS: tuple[str, ...] = ("text", "image", "sticker")


class MessageEditError(ValueError):
    """`code` is a closed vocabulary the service maps to HTTP."""

    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def check_deletable(message: dict, actor_id: str) -> None:
    """Refuse unless `actor_id` wrote `message` and it is still a live human kind.

    Order matters for what a caller learns: an already-deleted message says
    so before ownership is examined, because that state is visible to every
    member anyway; ownership is checked before kind so a stranger to the
    message never learns whether it was a card.
    """
    if message.get("kind") == "deleted":
        raise MessageEditError("ALREADY_DELETED")
    author = message.get("author_id")
    if author is None or str(author) != str(actor_id):
        raise MessageEditError("NOT_AUTHOR")
    if message.get("kind") not in DELETABLE_KINDS:
        raise MessageEditError("KIND_NOT_DELETABLE")


def deleted_shape(message: dict, now: datetime) -> dict:
    """The row after deletion: kind flipped, payload gone, author kept.

    The author stays because `human_kinds_have_author` still holds for the
    deleted kind, and because «ai đã xoá tin của mình» is a fact the thread
    may state. Reactions are the repository's to clear.
    """
    if now.tzinfo is None:
        raise MessageEditError("NAIVE_DATETIME")
    return {
        **message,
        "kind": "deleted",
        "body": None,
        "image_url": None,
        "card": None,
        "deleted_at": now,
    }


def check_reply_target(target: dict | None, context_id: str) -> None:
    """A reply may quote a live human message in the same group.

    Absent and cross-group are the SAME refusal on purpose: a guessed id must
    not learn that a message exists in another group. A deleted target and a
    card are refused with their own codes because the caller already sees
    those rows in their own thread.
    """
    if target is None or str(target.get("context_id")) != str(context_id):
        raise MessageEditError("REPLY_NOT_FOUND")
    if target.get("kind") == "deleted":
        raise MessageEditError("REPLY_TO_DELETED")
    if target.get("kind") not in REPLYABLE_KINDS:
        raise MessageEditError("REPLY_TO_CARD")
