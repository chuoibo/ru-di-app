"""What a block means, everywhere it means something (ADR-0023 §2.3).

Blocking is not its own table. It is the `blocked` state of the one friend
edge two people share, so «are these two blocked» is a question about a row
that already exists and already has a unique index keeping it single. What
this module adds is the *consequences*, written once as pure functions so the
service, the SQL and the tests all read the same sentences:

- reading somebody's posts and stories stops **both** ways. A block is not a
  mute: the person who blocked does not keep watching.
- a group both people are in keeps working. The group is the group's, not
  either member's, and a block that emptied a shared trip of one person's
  messages would rewrite what the rest of the group already read.
- a private conversation that already exists does not vanish, it stops
  accepting messages -- with ONE code and ONE sentence shared with «the other
  person deleted their account». That is a weak oracle, taken deliberately:
  somebody typing into a dead conversation needs to be told it is dead, and
  two causes behind one sentence is the least that can be said while still
  being useful (ADR-0023 §2.3.2).

`is_blocked` takes the edge dict the repository already returns rather than
two ids: whether an edge exists is a question for a table, and this module
does not read tables. Pure; no I/O, no ORM.
"""

from __future__ import annotations

__all__ = [
    "DIRECT_MESSAGE_UNAVAILABLE",
    "blocker_of",
    "dm_allowed",
    "hidden_between",
    "is_blocked",
]

#: The one code both causes answer with. See the module docstring.
DIRECT_MESSAGE_UNAVAILABLE = "direct_message_unavailable"


def is_blocked(edge: dict | None) -> bool:
    """True when this pair's live edge is a block, whichever way it points."""
    return edge is not None and edge.get("state") == "blocked"


def blocker_of(edge: dict | None) -> str | None:
    """Who did the blocking, or None when this edge is not a block.

    `decided_by_id` and not `requester_id`: a block from an ACCEPTED
    friendship is decided by whichever party ended it, and that party may be
    the person who originally asked or the person who originally answered.
    Only this person may lift it (§2.3.3).
    """
    if not is_blocked(edge):
        return None
    return None if edge is None else edge.get("decided_by_id")


def hidden_between(edge: dict | None) -> bool:
    """Whether posts and stories stop crossing between these two.

    Symmetric on purpose. The same expression is spelled in SQL as a two-way
    `NOT EXISTS` inside `_readable_by`; change either and change both.
    """
    return is_blocked(edge)


def dm_allowed(edge: dict | None, *, other_deleted: bool) -> bool:
    """Whether a private conversation may still carry a new message.

    Both refusals collapse here rather than at the call site, so no caller can
    accidentally answer them differently and turn the shared sentence back
    into an oracle for which of the two happened.
    """
    return not is_blocked(edge) and not other_deleted
