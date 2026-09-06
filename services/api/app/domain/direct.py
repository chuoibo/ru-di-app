"""Nhắn riêng: a two-person context (ADR-0021 §2.5).

A direct message is not a second kind of conversation with tables of its own.
It is a `context` whose `kind` is `pair`, so messages, photos, reactions, read
marks, the companion and the theme all work on it unchanged. What differs is
who may open one and which doors are closed once it exists, and both of those
rules live here so the service, the SQL and the tests read one spelling.

Pure: strings and booleans in, strings and booleans out. The friendship module
already knows how to order two ids; this module reuses that ordering so the
unique key in the database and `uq_friend_edge_live` agree on which of two
people comes first.
"""

from __future__ import annotations

from collections.abc import Iterable

from app.domain.friendship import pair_key as _ordered_pair

KIND_GROUP = "group"
KIND_PAIR = "pair"
#: Every value `contexts.kind` may hold; the CHECK constraint is the second
#: spelling of this tuple.
KINDS = (KIND_GROUP, KIND_PAIR)

#: What a pair is called when nobody's name is available to call it by. A
#: word, never an id: two unnamed people must not read as one hexadecimal.
ANONYMOUS_COUNTERPART = "Thành viên"

#: Doors that only make sense against a roster somebody curates. A pair has
#: exactly two members for as long as it exists, so each of these answers
#: `not_a_group` there (§2.5.5). The list is closed on purpose and read by the
#: tests that walk every door: money, outings, votes and memories are absent
#: because splitting a bill between two friends is a real thing to do.
ROSTER_ONLY_DOORS = (
    "invite_context_member",
    "accept_context_membership",
    "leave_context",
    "set_context_member_role",
    "create_outing_invite",
    "rename_context",
)


def pair_key(a: str, b: str) -> str:
    """The value `uq_contexts_pair_key` guards: both ids, ordered, joined by `:`.

    Ordered by `friendship.pair_key`, so A opening a conversation with B and B
    opening one with A compute the same key and land on the same row. Raises
    `FriendshipError("SELF_EDGE")` for one person twice and `PERSON_REQUIRED`
    for an empty id -- the same refusals a friend request gives.
    """
    first, second = _ordered_pair(a, b)
    return f"{first}:{second}"


def is_pair(kind: str | None) -> bool:
    return kind == KIND_PAIR


def is_kind(value: object) -> bool:
    return isinstance(value, str) and value in KINDS


def can_open(
    *, is_friend: bool, other_exists: bool = True, other_deleted: bool = False
) -> bool:
    """Whether one person may open (or re-open) a pair with another.

    Friendship is the consent step (§2.5.2): there is no «accept conversation»
    because accepting the friend request already was that. A person who does
    not exist, or whose account is gone (ADR-0023, L5), cannot be written to.
    The caller collapses every `False` into one 404 -- this function says
    *whether*, never *why*, so it cannot become an oracle by accident.
    """
    return bool(is_friend) and bool(other_exists) and not bool(other_deleted)


def counterpart_of(member_ids: Iterable[str], me: str) -> str | None:
    """The one member of a pair who is not `me`; None when the roster is not a
    pair's roster (zero or several other people)."""
    others = [member for member in member_ids if member != me]
    if len(others) != 1:
        return None
    return others[0]


def display_name_for(
    kind: str | None, stored_name: str, counterpart_name: str | None
) -> str:
    """What the reader sees at the top of the conversation.

    A group is called what its members called it. A pair has no name of its
    own -- `display_name` is stored empty (§2.5.6) -- so the reader sees the
    other person's name, derived on every read and never written back. When
    that name is missing the fallback is a word, not an id.
    """
    if not is_pair(kind):
        return stored_name
    if counterpart_name:
        return counterpart_name
    return ANONYMOUS_COUNTERPART
