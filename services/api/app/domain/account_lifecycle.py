"""How an account ends (ADR-0023 §2.1).

«Xoá tài khoản» in this product is not `DELETE FROM people`. It cannot be: a
person's id is a foreign key in the money ledger, and invariant 3 says a
balance is recomputable from that ledger forever. Deleting the row would
either fail on a constraint or, worse, cascade a debt out of existence.

So the account ends in four different ways at once, and this module is the
closed map of which way each table takes:

- **delete** -- what the person wrote *about themselves* and what only they
  could see: posts, the comments and reactions under them, stories and the
  views of them, their own photographs, tastes, saved places, every friend
  edge in every state, the proofs that let them sign in, read marks, and
  (ADR-0024) devices and notifications.
- **revoke** -- sessions. Every one, not just the caller's.
- **leave** -- memberships become `left`. Not deleted: a group's history says
  who was in it, and a deleted membership would make an old message look like
  it came from a stranger.
- **anonymise** -- the `people` row keeps its id and loses everything that
  names a person.
- **keep** -- messages, memories and their comments, check-ins, votes,
  outings, group photographs, `contexts.created_by_id`, and **every money
  table**, byte for byte.
- **untouched** -- tables that name no person at all (the catalogue, the
  idempotency ledger, OTP challenges). Listed so that «every table has an
  answer» is checkable rather than assumed.

The map is data, not prose, so a test can walk it and a reviewer can read the
whole policy in one screen. `ERASURE` names tables; the repository turns the
names into statements, and `tests/postgres/test_delete_account_postgres.py`
proves the money tables were not touched by comparing their md5 before and
after.

Pure: no I/O, no ORM, no clock of its own -- `now` arrives as an argument.
"""

from __future__ import annotations

from datetime import datetime

__all__ = [
    "ANONYMOUS_DISPLAY_NAME",
    "CONFIRMATION_REQUIRED",
    "ERASURE",
    "AccountLifecycleError",
    "anonymised_person",
    "check_confirmation",
    "tables_for",
]

#: What the rest of the product shows where the name used to be. One sentence,
#: not a blank: a message from an empty name reads like a bug, and «Người dùng
#: đã rời» is the true thing to say to a group that still has their messages.
ANONYMOUS_DISPLAY_NAME = "Người dùng đã rời"

#: The body must say so in as many words. A DELETE with an empty body is one
#: mistyped route away from being an accident.
CONFIRMATION_REQUIRED = "confirm_required"

#: The closed map. Keys are what happens; values are the tables it happens to.
#: `keep` is listed even though it is a no-op, because a table missing from
#: this map entirely is the bug this shape exists to make visible.
ERASURE: dict[str, tuple[str, ...]] = {
    "delete": (
        "posts",
        "post_comments",
        "post_reactions",
        "stories",
        "story_views",
        "uploaded_images",
        "person_interests",
        "saved_places",
        "friend_requests",
        "account_identities",
        "context_read_marks",
    ),
    "revoke": ("account_sessions",),
    "leave": ("memberships",),
    "anonymise": ("people",),
    "keep": (
        "messages",
        "message_reactions",
        "memories",
        "memory_comments",
        "memory_reactions",
        "outing_stop_checkins",
        "votes",
        "vote_options",
        "vote_ballots",
        "outings",
        "outing_stops",
        "outing_invites",
        "contexts",
        "expenses",
        "expense_versions",
        "expense_items",
        "expense_item_shares",
        "expense_discounts",
        "expense_surcharges",
        "confirmed_allocations",
        "collection_batches",
        "collection_batch_versions",
        "collection_obligations",
        "collection_obligation_sources",
        "collection_envelopes",
        "payment_reports",
        "receipt_confirmations",
        "guest_links",
        # A report is evidence for whoever reads the table, and most reports
        # are about somebody else. Deleting one's account must not delete what
        # one reported; `reporter_id` still resolves, to the anonymised row.
        "reports",
        "bills",
        "bill_items",
        "bill_item_shares",
        "bill_discounts",
        "bill_surcharges",
        "audit_events",
    ),
    #: Tables that say nothing about any particular person: the place
    #: catalogue, the idempotency ledger keyed by a header, and OTP challenges
    #: keyed by a phone digest that expires within minutes. Named anyway, so
    #: «is every table accounted for» stays a question a test can answer.
    "untouched": (
        "places",
        "place_photos",
        "destinations",
        "idempotency_keys",
        "otp_challenges",
    ),
}


class AccountLifecycleError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def tables_for(action: str) -> tuple[str, ...]:
    """The tables one action touches. Raises on an action the map has no
    opinion about, so a typo cannot silently mean «nothing happens»."""
    if action not in ERASURE:
        raise AccountLifecycleError("UNKNOWN_ERASURE_ACTION")
    return ERASURE[action]


def check_confirmation(confirm: object) -> None:
    """`{"confirm": true}` and nothing else counts."""
    if confirm is not True:
        raise AccountLifecycleError(CONFIRMATION_REQUIRED.upper())


def anonymised_person(person: dict, now: datetime) -> dict:
    """The `people` row after the account ends.

    Keeps `id` and `created_at` -- the id because the ledger points at it, the
    join date because «a person was here from then» is true and says nothing
    about who. Everything else that could name somebody goes, and two settings
    are forced closed rather than left at whatever the person last chose:
    nobody looks this account up by telephone number again, and nobody
    comments on a wall that has no owner.
    """
    if now.tzinfo is None or now.utcoffset() is None:
        raise AccountLifecycleError("NAIVE_DATETIME")
    return {
        **person,
        "display_name": ANONYMOUS_DISPLAY_NAME,
        "bio": None,
        "city": None,
        "budget_band": None,
        "discoverable_by_phone": False,
        "wall_comment_policy": "nobody",
        "deleted_at": now,
    }
