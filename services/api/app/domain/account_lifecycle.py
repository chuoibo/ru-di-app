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
        # Sổ hai người (ADR-0027). Three of its thirteen tables are the
        # person's own, and they go with the account:
        #   * a shared constraint is what somebody said about THEMSELVES
        #     («không ăn được», «đừng»), the same shape as `person_interests`;
        #   * a view mark is a private note about their own reading, the same
        #     shape as `context_read_marks`;
        #   * a couple row is a LIVE state, not history, and a person whose
        #     account has ended is not in a couple.
        # The other ten are the conversation's history and stay -- see «keep».
        "pair_shared_constraints",
        "pair_paper_views",
        "active_couple_members",
    ),
    "revoke": ("account_sessions",),
    "leave": ("memberships",),
    "anonymise": ("people",),
    "keep": (
        # Sổ hai người (ADR-0027): the notebook is a conversation BETWEEN two
        # people, so its history is the other person's memory too. A sheet, the
        # versions of it, who agreed to which one, the outing it became and the
        # line somebody kept about that evening all read like `messages` and
        # `memories`, and they stay for the same reason: deleting them would
        # take the surviving person's own evening away from them. Nothing here
        # keeps a NAME -- the `people` row is anonymised and only its id
        # remains, exactly as it does for a group.
        #
        # Nợ đã biết: a cycle whose participant has ended stays `active` until
        # somebody closes it. Slice 1 has no closer; the surviving person can
        # close the notebook by hand and the pair already refuses new messages
        # (ADR-0023 §2.3.2). Đóng tự động là quyết định của lát sau.
        "pair_notebooks",
        "pair_notebook_cycles",
        "pair_cycle_participants",
        "pair_consent_proposals",
        "pair_consents",
        "pair_papers",
        "pair_paper_versions",
        "pair_paper_responses",
        "pair_paper_outings",
        "pair_paper_keeps",
        # ADR-0034: a week's chosen «Người lo» belongs to the cycle, like its sheets.
        "pair_cycle_rhythms",
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


#: Bảng mà một lần xoá tài khoản KHÔNG được chạm tới, dù chỉ một byte.
#:
#: Danh sách này là một sự thật về sản phẩm chứ không phải một chi tiết của
#: test, nên nó sống ở đây và có HAI người đọc: một ca thuần khẳng định mọi tên
#: dưới đây nằm ở nhánh «keep» của bản đồ, và ca Postgres so md5 của đúng những
#: bảng này trước và sau khi xoá.
#:
#: Vì sao cần cả hai. Bản đồ `ERASURE` trước đây chỉ bị gác bởi một ca «mọi
#: bảng thật đều có tên trong bản đồ». Ca ấy đếm sự CÓ MẶT, không đọc NHÁNH,
#: nên chuyển một bảng từ «keep» sang «delete» đi qua sạch sẽ — hai người đo
#: độc lập (agy QA và reviewer PR #581) cùng dựng đúng đột biến ấy cho
#: `reports` và cả hai đều thấy cổng xanh. Lỗ ấy đi hai bước: xếp nhầm nhánh
#: (im lặng), rồi thêm một `wipe()` cho nó (nằm trong giới hạn cũ), và md5 mù
#: vì bảng ấy không có trong danh sách viết tay.
#:
#: `reports` nằm đây không phải vì nó mang tiền: nó mang bằng chứng của NGƯỜI
#: KHÁC. `guest_links` cũng vậy — nó là cửa vào một nghĩa vụ, và mất một hàng
#: ở đấy là một envelope khách không mở được nữa.
MONEY_TABLES: tuple[str, ...] = (
    "expenses",
    "expense_versions",
    "expense_items",
    "expense_item_shares",
    "expense_surcharges",
    "expense_discounts",
    "confirmed_allocations",
    "collection_batches",
    "collection_batch_versions",
    "collection_obligations",
    "collection_obligation_sources",
    "collection_envelopes",
    "payment_reports",
    "receipt_confirmations",
    "guest_links",
    "bills",
    "bill_items",
    "bill_item_shares",
    "bill_surcharges",
    "bill_discounts",
)

#: Bảng mang lời của người KHÁC, hoặc lời của mình trong cuộc trò chuyện của
#: người khác. Xoá tài khoản của mình không được viết lại thứ người khác đã đọc.
OTHERS_KEEP_TABLES: tuple[str, ...] = (
    # Sổ hai người: what the OTHER person wrote, or agreed to, or kept. Listed
    # here so that moving one of them into «delete» later fails a test rather
    # than quietly taking a surviving person's evening away.
    "pair_papers",
    "pair_paper_versions",
    "pair_paper_responses",
    "pair_paper_keeps",
    "messages",
    "message_reactions",
    "memories",
    "memory_comments",
    "memory_reactions",
    "reports",
    "contexts",
    "outings",
    "audit_events",
)


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
