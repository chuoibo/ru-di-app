"""Stateful fake repository for API tests.

SQLite is deliberately not used: the production schema relies on PostgreSQL
JSONB, regex checks, partial indexes, views, and append-only triggers. SQLite
would turn a green test into a false claim about those guarantees. This fake
tests HTTP/domain orchestration only; static tests cover migration/model parity,
while ``tests/postgres`` executes the production adapter and constraints on a
real PostgreSQL server.
"""

from __future__ import annotations

import dataclasses
import uuid
from dataclasses import dataclass, replace
from datetime import UTC, datetime
from functools import lru_cache

import anyio
import httpx
import pytest

from app.api.deps import get_repository
from app.api.errors import RepositoryConflict
from app.api.limits import OBJECTION_LIMIT, REPORT_LIMIT
from app.api.main import create_app
from app.api.repository import (
    AccountIdentityRecord,
    AccountSessionRecord,
    ActorGrants,
    AllocationRow,
    BatchBoard,
    BatchForPublish,
    BatchInputs,
    BatchObligationRow,
    BillDiscountRecord,
    BillItemRecord,
    BillRecord,
    BillShareRecord,
    BillSurchargeRecord,
    ConfirmationRecord,
    ConfirmedExpense,
    ContextBatchRow,
    ContextRecord,
    DestinationRecord,
    ErasureReport,
    ExpenseIdentity,
    FriendEdgeRecord,
    FrozenBatch,
    FrozenObligation,
    GuestEnvelopeRecord,
    GuestLinkDraft,
    LastMessageRecord,
    MembershipRecord,
    MemoryPage,
    MemoryRecord,
    MessageRecord,
    ObligationDraft,
    OtpChallengeRecord,
    OutingInviteRecord,
    OutingRecord,
    OutingStopRecord,
    PairConsentRecord,
    PairConstraintRecord,
    PairRhythmRecord,
    PairKeepRecord,
    PairNotebookRecord,
    PairPaperRecord,
    PairProposalRecord,
    PairResponseRecord,
    PairVersionRecord,
    PairViewRecord,
    PaymentReportRecord,
    PaymentReportTarget,
    PersonContextSummaryRecord,
    PersonFinanceSummary,
    PersonRecord,
    PlacePhotoRecord,
    PlaceRecord,
    PostCommentRecord,
    PostReactionRecord,
    PostRecord,
    PostSocialCounts,
    ProfileCounts,
    PublishObligation,
    ReadMarkRecord,
    ReceiptRecord,
    ReceiptTarget,
    ReportRecord,
    SavedPlaceRecord,
    StoredGuestLink,
    StoryRecord,
    UploadedImageRecord,
)
from app.domain.account_lifecycle import ANONYMOUS_DISPLAY_NAME
from app.domain.capability import capability_scope
from app.domain.direct import display_name_for
from app.domain.ledger import obligation_status

from .helpers import ADVANCER_ID, CONTEXT_ID, SENDER_ID

FAKE_BATCH_INSTANT = datetime(2026, 1, 1, tzinfo=UTC)


@dataclass(slots=True)
class FakeLink:
    id: uuid.UUID
    sender_id: uuid.UUID
    batch_id: uuid.UUID
    expires_at: datetime
    status: str = "active"


@dataclass(slots=True)
class FakeReport:
    id: uuid.UUID
    link_id: uuid.UUID
    obligation_id: uuid.UUID
    amount_vnd: int
    idempotency_key: uuid.UUID
    #: When the sender said it. Kept because the collection board shows the
    #: claim beside the payment status, so a fake that dropped the timestamp
    #: could not tell "reported at 9:04" from "never reported".
    reported_at: datetime


@dataclass(slots=True)
class FakeReceipt:
    id: uuid.UUID
    obligation_id: uuid.UUID
    confirmed_by_id: uuid.UUID
    amount_vnd: int
    payment_report_id: uuid.UUID | None
    idempotency_key: uuid.UUID


@lru_cache(maxsize=1)
def _seed_place_records() -> tuple[PlaceRecord, ...]:
    """The seed catalogue as repository records, built once per process."""
    from app.places.activities import hoat_dong_theo_dong
    from app.places.catalog import PLACES
    from app.places.details import find_detail
    from app.places.seed_catalog import _destination_for

    out = []
    for place in PLACES:
        prose = find_detail(place["id"])
        out.append(
            PlaceRecord(
                id=place["id"],
                destination_id=_destination_for(place),
                name=place["name"],
                category=place["category"],
                kinds=list(place["kinds"]),
                address=place["address"],
                lat=place["lat"],
                lng=place["lng"],
                rating=place["rating"],
                rating_count=place["rating_count"],
                price_min_vnd=place["price_min_vnd"],
                price_max_vnd=place["price_max_vnd"],
                open_hours=place["open_hours"],
                open_now=place["open_now"],
                travel_minutes=place["travel_minutes"],
                distance_km=place["distance_km"],
                photo_count=place["photo_count"],
                traits=list(place["traits"]),
                group_fit=dict(place["group_fit"]),
                # Mirrors what `seed_catalog.py` writes into the real table
                # (M12): a double that omits it would let a route look empty
                # here and full in production, which is the drift this file
                # exists to avoid.
                activities=hoat_dong_theo_dong(place),
                flag=place["flag"],
                description=None if prose is None else prose["description"],
                reviews=None if prose is None else list(prose["reviews"]),
                source="seed",
                source_ref=None,
                license=None,
            )
        )
    return tuple(out)


@lru_cache(maxsize=1)
def _seed_destination_records() -> tuple[DestinationRecord, ...]:
    from app.places.seed_catalog import SEED_DESTINATIONS

    return tuple(DestinationRecord(**row) for row in SEED_DESTINATIONS)


class SeedCatalogueReads:
    """The four catalogue reads, for a test double that is not `FakeRepository`.

    Since M9 the catalogue is a table, so every repository the service is handed
    has to be able to answer for it -- including the small hand-written doubles
    in individual test modules, which exist to make one path fail in a specific
    way and should not have to grow a catalogue to do it.
    """

    # M11: the catalogue is scored against whoever is asking, so every double
    # the service is handed must be able to answer «what did they say they
    # like». These say «nothing», which is the state every test using this
    # mixin was written in -- a double that really tracks people (like
    # `FakeRepository`) overrides these and wins by MRO.
    # M12: a catalogue row may have licensed photographs. These doubles have
    # none, which is the state every test using this mixin was written in --
    # and «no photograph» is a real answer the card draws as a typographic band.
    def list_place_photos(self, place_id):
        del place_id
        return []

    def get_place_photo(self, place_id, photo_id):
        del place_id, photo_id
        return None

    def photo_covers(self, place_ids):
        del place_ids
        return {}

    def photo_counts(self, place_ids):
        del place_ids
        return {}

    def list_person_interests(self, person_id):
        del person_id
        return []

    def interests_by_person(self, person_ids):
        del person_ids
        return {}

    def budget_bands_by_person(self, person_ids):
        del person_ids
        return {}

    def get_person(self, person_id):
        del person_id
        return None

    def get_context(self, context_id):
        """«An ordinary group», which is what every double using this class is.

        Added when the companion turn started asking what KIND of context it is
        (ADR-0019 addendum): a pair's conversation is not read until both people
        have said so, and «is this a pair» is a question only the context row
        answers. Three hand-written doubles went red at once on
        `AttributeError`, which is the right failure -- a double that cannot say
        what kind of context it stands for cannot stand in for the repository
        on this path any more. Answering `group` here keeps each of them
        testing exactly what it was written to test.
        """
        return ContextRecord(
            id=context_id,
            display_name="Hội bạn",
            created_by_id=context_id,
            created_at=datetime(2030, 8, 27, 12, tzinfo=UTC),
            kind="group",
        )

    def list_places(self, *, destination_id=None, category=None):
        rows = [
            record
            for record in _seed_place_records()
            if (destination_id is None or record.destination_id == destination_id)
            and (category is None or record.category == category)
        ]
        return sorted(rows, key=lambda record: record.id)

    def get_place(self, place_id):
        for record in _seed_place_records():
            if record.id == place_id:
                return record
        return None

    def list_destinations(self):
        return list(_seed_destination_records())

    def get_destination(self, destination_id):
        for record in _seed_destination_records():
            if record.id == destination_id:
                return record
        return None


class FakeRepository(SeedCatalogueReads):
    def __init__(self):
        self.expenses: dict[uuid.UUID, ExpenseIdentity] = {}
        self.confirmed: dict[uuid.UUID, ConfirmedExpense] = {}
        self.version_to_expense: dict[uuid.UUID, uuid.UUID] = {}
        self.version_numbers: dict[uuid.UUID, int] = {}
        self.batched_versions: set[uuid.UUID] = set()
        self.batches: dict[uuid.UUID, BatchForPublish] = {}
        self.obligations: dict[uuid.UUID, PublishObligation] = {}
        self.links: dict[bytes, FakeLink] = {}
        self.reports: dict[uuid.UUID, FakeReport] = {}
        #: ADR-0023 §2.4 -- content reports, a different thing from the
        #: payment `reports` above; named apart so neither shadows the other.
        self.reports_filed: list[dict] = []
        self.objections: list[dict] = []
        self.receipts: dict[uuid.UUID, FakeReceipt] = {}
        self.people: dict[uuid.UUID, PersonRecord] = {}
        self.contexts: dict[uuid.UUID, ContextRecord] = {}
        #: ADR-0021 §2.5: pair_key -> context id. A dict cannot race; the
        #: unique key is proved in tests/postgres/test_direct_message_postgres.py.
        self.pair_contexts: dict[str, uuid.UUID] = {}
        self.messages: dict[uuid.UUID, MessageRecord] = {}
        self.bills: dict[uuid.UUID, BillRecord] = {}
        self.finances: dict[uuid.UUID, PersonFinanceSummary] = {}
        self.active_memberships: set[tuple[uuid.UUID, uuid.UUID]] = set()
        self.outing_invites: dict[uuid.UUID, OutingInviteRecord] = {}
        self.outing_invite_ids_by_digest: dict[bytes, uuid.UUID] = {}
        self.friend_edges: dict[uuid.UUID, dict] = {}
        self.posts: dict[uuid.UUID, PostRecord] = {}
        # ADR-0022: reactions and comments under posts, and the images a
        # person or a group uploaded (with their `purpose`).
        self.post_reactions: dict[uuid.UUID, PostReactionRecord] = {}
        self.post_comments: dict[uuid.UUID, PostCommentRecord] = {}
        self.uploaded_images: dict[uuid.UUID, UploadedImageRecord] = {}
        # ADR-0022 §2.3: stories and who has seen which.
        self.stories: dict[uuid.UUID, StoryRecord] = {}
        self.story_views: dict[tuple[uuid.UUID, uuid.UUID], datetime] = {}
        self.memories: dict[uuid.UUID, MemoryRecord] = {}
        self.account_sessions: dict[uuid.UUID, AccountSessionRecord] = {}
        self.invited_memberships: set[tuple[uuid.UUID, uuid.UUID]] = set()
        self.read_marks: dict[tuple[uuid.UUID, uuid.UUID], ReadMarkRecord] = {}
        self.otp_challenges: dict[uuid.UUID, OtpChallengeRecord] = {}
        self.account_identities: dict[tuple[str, str], AccountIdentityRecord] = {}
        # M2 profile: bookmarks, and the two sources the counts read that this
        # fake did not model before (outing -> context, and check-ins).
        self.saved_places: dict[tuple[uuid.UUID, str], SavedPlaceRecord] = {}
        self.person_interests: dict[uuid.UUID, set[str]] = {}
        self.place_photos: list[PlacePhotoRecord] = []
        self.outings_by_context: dict[uuid.UUID, uuid.UUID] = {}
        self.stop_checkins: set[tuple[uuid.UUID, uuid.UUID]] = set()
        self.account_session_ids_by_digest: dict[bytes, uuid.UUID] = {}
        self.left_memberships: set[tuple[uuid.UUID, uuid.UUID]] = set()
        self.admin_memberships: set[tuple[uuid.UUID, uuid.UUID]] = set()
        # --- Sổ hai người và tờ giấy (ADR-0027) --------------------------
        #
        # Dicts, and therefore blind BY CONSTRUCTION to five things the real
        # schema decides: the composite foreign key that stops a row pointing
        # at a group while writing 'pair' beside it, the partial unique that
        # allows one open sheet per notebook, the primary key that allows one
        # couple per person, the `UNIQUE(paper_id)` that allows one outing per
        # sheet, and all four triggers. Every one of those is proved in
        # tests/postgres/test_pair_*.py. Widening this fake until a race
        # «passes» here is the lie CLAUDE.md names; what it is for is the
        # orchestration above them -- who may do what, in what order.
        self.pair_notebooks: dict[uuid.UUID, dict] = {}
        self.pair_cycles: dict[uuid.UUID, dict] = {}
        self.pair_cycle_participants: dict[uuid.UUID, list[uuid.UUID]] = {}
        self.pair_proposals: dict[uuid.UUID, dict] = {}
        self.pair_consents: dict[tuple[uuid.UUID, uuid.UUID], dict] = {}
        self.active_couple_members: dict[uuid.UUID, uuid.UUID] = {}
        self.pair_constraints: dict[tuple[uuid.UUID, uuid.UUID, str], dict] = {}
        self.pair_rhythms: dict = {}
        self.pair_papers: dict[uuid.UUID, dict] = {}
        self.pair_paper_versions: dict[tuple[uuid.UUID, int], dict] = {}
        self.pair_paper_views: dict[tuple[uuid.UUID, int, uuid.UUID], datetime] = {}
        self.pair_paper_responses: list[dict] = []
        self.pair_paper_keeps: dict[uuid.UUID, list[PairKeepRecord]] = {}
        self.pair_paper_outings: dict[uuid.UUID, tuple[int, uuid.UUID]] = {}
        self.outings: dict[uuid.UUID, OutingRecord] = {}
        self.leak_guest_input = False

    @staticmethod
    def _ordered_bill(bill: BillRecord) -> BillRecord:
        return replace(
            bill,
            items=[
                replace(
                    item,
                    shares=sorted(
                        item.shares,
                        key=lambda share: share.participant_id.bytes,
                    ),
                )
                for item in sorted(
                    bill.items,
                    key=lambda item: (item.position, item.item_key),
                )
            ],
            surcharges=sorted(
                bill.surcharges,
                key=lambda surcharge: surcharge.surcharge_key.encode("utf-8"),
            ),
            discounts=sorted(
                bill.discounts,
                key=lambda discount: discount.discount_key.encode("utf-8"),
            ),
        )

    def get_person(self, person_id):
        return self.people.get(person_id)

    def create_person(self, person_id, display_name):
        # No primary key here, so the double-insert conflict the real table
        # raises cannot happen. That case is covered in tests/postgres.
        record = PersonRecord(
            id=person_id,
            display_name=display_name,
            created_at=datetime(2030, 8, 27, 12, tzinfo=UTC),
        )
        self.people[person_id] = record
        return record

    def rename_person(self, person_id, display_name):
        existing = self.people.get(person_id)
        if existing is None:
            return None
        renamed = PersonRecord(
            id=existing.id,
            display_name=display_name,
            created_at=existing.created_at,
        )
        self.people[person_id] = renamed
        return renamed

    # --- friend graph (F03, F04) ---------------------------------------
    #
    # A dict cannot express `uq_friend_edge_live`, the functional partial
    # unique index that makes (A,B) and (B,A) one edge under concurrency. This
    # fake is blind to that race BY CONSTRUCTION, which is why
    # tests/postgres/test_friend_requests_postgres.py exists. Widening this
    # fake until the race "passes" here would be the lie CLAUDE.md names.

    def _friend_record(self, edge, reader_id):
        other = (
            edge["addressee_id"]
            if edge["requester_id"] == reader_id
            else edge["requester_id"]
        )
        person = self.people.get(other)
        return FriendEdgeRecord(
            id=edge["id"],
            requester_id=edge["requester_id"],
            addressee_id=edge["addressee_id"],
            other_person_id=other,
            other_display_name=(
                person.display_name if person is not None else str(other)
            ),
            state=edge["state"],
            decided_by_id=edge["decided_by_id"],
            created_at=edge["created_at"],
            decided_at=edge["decided_at"],
        )

    def get_friend_edge(self, person_a, person_b):
        pair = frozenset((person_a, person_b))
        for edge in self.friend_edges.values():
            same_pair = frozenset((edge["requester_id"], edge["addressee_id"]))
            if same_pair == pair and edge["state"] != "declined":
                return self._friend_record(edge, person_a)
        return None

    def get_friend_request(self, request_id, reader_id):
        edge = self.friend_edges.get(request_id)
        if edge is None:
            return None
        if reader_id not in (edge["requester_id"], edge["addressee_id"]):
            return None
        return self._friend_record(edge, reader_id)

    def open_friend_request(self, *, requester_id, addressee_id, now):
        edge = {
            "id": uuid.uuid4(),
            "requester_id": requester_id,
            "addressee_id": addressee_id,
            "state": "pending",
            "decided_by_id": None,
            "created_at": now,
            "decided_at": None,
        }
        self.friend_edges[edge["id"]] = edge
        return self._friend_record(edge, requester_id)

    def decide_friend_request(self, *, request_id, state, decided_by_id, now):
        edge = self.friend_edges.get(request_id)
        if edge is None:
            return None
        edge["state"] = state
        edge["decided_by_id"] = decided_by_id
        edge["decided_at"] = now
        return self._friend_record(edge, decided_by_id)

    def list_friend_requests(self, person_id, *, direction):
        side = "addressee_id" if direction == "incoming" else "requester_id"
        return [
            self._friend_record(edge, person_id)
            for edge in self.friend_edges.values()
            if edge[side] == person_id and edge["state"] == "pending"
        ]

    def list_friends(self, person_id):
        return [
            self._friend_record(edge, person_id)
            for edge in self.friend_edges.values()
            if edge["state"] == "accepted"
            and person_id in (edge["requester_id"], edge["addressee_id"])
        ]

    # --- F39 posts, F42 audiences ---------------------------------------
    #
    # This fake re-implements the visibility predicate that
    # `SqlAlchemyApiRepository._readable_by` writes in SQL, so `tests/api` is
    # proving the *service*, not the query. The query has its own live cases in
    # tests/postgres/test_posts_postgres.py, and it has to: a dict cannot show
    # that `only_me` rows are excluded by the SELECT rather than by a Python
    # loop that runs after them, and "excluded by the SELECT" is the claim.

    def create_post(self, *, author_id, audience, context_id, body, image_url, now):
        record = PostRecord(
            id=uuid.uuid4(),
            author_id=author_id,
            audience=audience,
            context_id=context_id,
            body=body,
            image_url=image_url,
            created_at=now,
        )
        self.posts[record.id] = record
        return record

    def get_post(self, post_id):
        return self.posts.get(post_id)

    def _post_visible_to(self, record, reader_id):
        if record.author_id == reader_id:
            return True
        if record.audience == "public":
            return True
        if record.audience == "friends":
            edge = self.get_friend_edge(reader_id, record.author_id)
            return edge is not None and edge.state == "accepted"
        if record.audience == "group":
            return record.context_id is not None and self.is_member(
                record.context_id, reader_id
            )
        return False

    def _posts_newest_first(self):
        return sorted(
            self.posts.values(),
            key=lambda record: (record.created_at, record.id.bytes),
            reverse=True,
        )

    def list_posts_visible_to(self, reader_id, *, limit):
        return tuple(
            record
            for record in self._posts_newest_first()
            if self._post_visible_to(record, reader_id)
        )[:limit]

    def list_person_posts_visible_to(self, person_id, reader_id, *, limit):
        return tuple(
            record
            for record in self._posts_newest_first()
            if record.author_id == person_id
            and self._post_visible_to(record, reader_id)
        )[:limit]

    # --- ADR-0022: reactions and comments under posts, personal photos ----
    #
    # A dict cannot express `uq_post_reactions_one_per_kind` under a race nor
    # the CASCADE from `posts`; tests/postgres/test_post_social_postgres.py
    # exists for those. The visibility predicate here re-implements
    # `_readable_by` by the same hand -- the Postgres gate test is the proof.

    def post_social_counts(self, post_ids, *, viewer_id):
        wanted = set(post_ids)
        reactions: dict = {}
        comments: dict = {}
        mine: dict = {}
        for reaction in self.post_reactions.values():
            if reaction.post_id not in wanted:
                continue
            per_kind = reactions.setdefault(reaction.post_id, {})
            per_kind[reaction.kind] = per_kind.get(reaction.kind, 0) + 1
            if reaction.person_id == viewer_id:
                mine.setdefault(reaction.post_id, set()).add(reaction.kind)
        for comment in self.post_comments.values():
            if comment.post_id in wanted:
                comments[comment.post_id] = comments.get(comment.post_id, 0) + 1
        return PostSocialCounts(
            reactions, comments, {k: frozenset(v) for k, v in mine.items()}
        )

    def add_post_reaction(self, *, post_id, person_id, kind, now):
        for reaction in self.post_reactions.values():
            if (reaction.post_id, reaction.person_id, reaction.kind) == (
                post_id,
                person_id,
                kind,
            ):
                return reaction
        record = PostReactionRecord(
            id=uuid.uuid4(),
            post_id=post_id,
            person_id=person_id,
            kind=kind,
            created_at=now,
        )
        self.post_reactions[record.id] = record
        return record

    def remove_post_reaction(self, *, post_id, person_id, kind):
        for reaction_id, reaction in list(self.post_reactions.items()):
            if (reaction.post_id, reaction.person_id, reaction.kind) == (
                post_id,
                person_id,
                kind,
            ):
                del self.post_reactions[reaction_id]
                return True
        return False

    def create_post_comment(self, *, post_id, author_id, body, now):
        author = self.people.get(author_id)
        record = PostCommentRecord(
            id=uuid.uuid4(),
            post_id=post_id,
            author_id=author_id,
            author_display_name=(
                author.display_name if author is not None else str(author_id)
            ),
            body=body,
            created_at=now,
        )
        self.post_comments[record.id] = record
        return record

    def get_post_comment(self, comment_id):
        return self.post_comments.get(comment_id)

    def delete_post_comment(self, comment_id):
        return self.post_comments.pop(comment_id, None) is not None

    def list_post_comments(self, post_id, *, limit, after=None):
        rows = sorted(
            (c for c in self.post_comments.values() if c.post_id == post_id),
            key=lambda c: (c.created_at, c.id.bytes),
        )
        if after is not None:
            rows = [
                c
                for c in rows
                if (c.created_at, c.id.bytes) > (after[0], after[1].bytes)
            ]
        return tuple(rows[:limit])

    def create_uploaded_image(
        self,
        *,
        storage_key,
        context_id,
        owner_person_id,
        uploaded_by_id,
        content_type,
        byte_size,
        width,
        height,
        now,
        purpose="group",
    ):
        record = UploadedImageRecord(
            id=uuid.uuid4(),
            storage_key=storage_key,
            context_id=context_id,
            owner_person_id=owner_person_id,
            uploaded_by_id=uploaded_by_id,
            content_type=content_type,
            byte_size=byte_size,
            width=width,
            height=height,
            created_at=now,
            purpose=purpose,
        )
        self.uploaded_images[record.id] = record
        return record

    def get_context_image(self, context_id, image_id):
        record = self.uploaded_images.get(image_id)
        if record is None or record.context_id != context_id:
            return None
        return record

    def get_latest_avatar(self, person_id):
        rows = [
            r
            for r in self.uploaded_images.values()
            if r.owner_person_id == person_id and r.purpose == "avatar"
        ]
        return max(rows, key=lambda r: (r.created_at, r.id.bytes), default=None)

    def get_person_image(self, person_id, image_id):
        record = self.uploaded_images.get(image_id)
        if (
            record is None
            or record.owner_person_id != person_id
            or record.purpose != "personal"
        ):
            return None
        return record

    def person_image_visible_to(self, person_id, image_id, reader_id, *, now):
        url = f"/people/{person_id}/photos/{image_id}"
        shown_by_post = any(
            post.image_url == url and self._post_visible_to(post, reader_id)
            for post in self.posts.values()
        )
        shown_by_story = any(
            story.image_url == url and self._story_visible_to(story, reader_id, now)
            for story in self.stories.values()
        )
        return shown_by_post or shown_by_story

    # --- 24-hour stories (ADR-0022 §2.3) ----------------------------------
    #
    # The same hand-written re-implementation of `_story_readable_by` as the
    # post predicate above, for the same reason: tests/api proves the service.
    # The SQL has its own live cases in tests/postgres/test_stories_postgres.py.

    def _story_visible_to(self, record, reader_id, now):
        if record.author_id == reader_id:
            return True
        if record.expires_at <= now:
            return False
        return self.are_friends(reader_id, record.author_id)

    def create_story(self, *, author_id, image_url, caption, audience, now, expires_at):
        person = self.people.get(author_id)
        record = StoryRecord(
            id=uuid.uuid4(),
            author_id=author_id,
            author_display_name="" if person is None else person.display_name,
            image_url=image_url,
            caption=caption,
            audience=audience,
            created_at=now,
            expires_at=expires_at,
            seen=False,
        )
        self.stories[record.id] = record
        return record

    def get_story(self, story_id):
        return self.stories.get(story_id)

    def list_live_stories_for(self, reader_id, *, now):
        rows = [
            record
            for record in self.stories.values()
            if record.expires_at > now
            and self._story_visible_to(record, reader_id, now)
        ]
        rows.sort(key=lambda r: (r.author_id.bytes, r.created_at, r.id.bytes))
        return tuple(
            replace(record, seen=(record.id, reader_id) in self.story_views)
            for record in rows
        )

    def mark_story_seen(self, story_id, viewer_id, *, now):
        return self.story_views.setdefault((story_id, viewer_id), now)

    def delete_story(self, story_id):
        self.stories.pop(story_id, None)
        for key in [key for key in self.story_views if key[0] == story_id]:
            del self.story_views[key]

    def list_memories(
        self,
        context_id,
        *,
        limit,
        before=None,
        kind=None,
        place_id=None,
        viewer_id=None,
    ):
        """Enough of the wall for F38's widget probe to count rows.

        Added for one reason: the leak probe has to be able to see the leak.
        Without a wall in this fake, an outsider whose membership check had
        been broken would receive an *empty* widget, and "no photo because the
        group has none" and "no photo because the gate held" are the same body.
        A probe that cannot tell those apart proves nothing, so the fake has to
        hold at least one photograph for the gate to withhold.

        Ordering, `kind` and the `has_more` flag match
        `SqlAlchemyApiRepository.list_memories`. `before`, `place_id` and
        `viewer_id` are accepted and unused: the widget passes none of them,
        and a fake that pretended to page or to count hearts would be a second
        implementation of behaviour `tests/postgres` already proves.
        """

        rows = sorted(
            (
                record
                for record in self.memories.values()
                if record.context_id == context_id
                and (kind is None or record.kind == kind)
            ),
            key=lambda record: (record.created_at, record.id.bytes),
            reverse=True,
        )
        return MemoryPage(memories=tuple(rows[:limit]), has_more=len(rows) > limit)

    def create_memory(
        self,
        *,
        context_id,
        author_id,
        image_url,
        caption,
        now,
        place_id=None,
        place_name=None,
    ):
        record = MemoryRecord(
            id=uuid.uuid4(),
            context_id=context_id,
            author_id=author_id,
            kind="photo",
            image_url=image_url,
            caption=caption,
            place_id=place_id,
            place_name=place_name,
            lat=None,
            lng=None,
            created_at=now,
        )
        self.memories[record.id] = record
        return record

    def group_photos_at_place(self, place_id, *, viewer_id, limit):
        """The membership gate, in the fake, written as a gate and not a filter.

        `is_member` is asked per row for the same reason the real query puts
        the predicate in SQL: a test that broke the gate must see a *stranger's
        photograph appear*, and that only happens if this fake would otherwise
        have returned it.
        """

        rows = sorted(
            (
                record
                for record in self.memories.values()
                if record.place_id == place_id
                and record.kind == "photo"
                and self.is_member(record.context_id, viewer_id)
            ),
            key=lambda record: (record.created_at, record.id.bytes),
            reverse=True,
        )
        return tuple(rows[:limit])

    def membership_role(self, context_id, person_id):
        if (context_id, person_id) in self.admin_memberships:
            return "admin"
        if (context_id, person_id) in self.active_memberships:
            return "member"
        return None

    def is_member(self, context_id, person_id):
        return (context_id, person_id) in self.active_memberships

    def list_members(self, context_id):
        """`ApiRepository` has always declared this; the fake never had it.

        `ApiService.split_bill` reads it through `getattr(..., None)` and falls
        back to "the participants are whoever the shares name" when it is
        missing -- which, against this fake, was always. That fallback makes
        the allocator's `UNKNOWN_PARTICIPANT` check unreachable by
        construction, so the whole `tests/api` layer was proving a membership
        rule it could not have broken.
        """

        return [
            MembershipRecord(
                id=uuid.uuid5(uuid.NAMESPACE_URL, f"{context}/{person}"),
                context_id=context,
                person_id=person,
                display_name=(
                    self.people[person].display_name
                    if person in self.people
                    else f"Thành viên {str(person)[:8]}"
                ),
                state="active",
                role="member",
                origin="named",
                invited_by_id=None,
                joined_at=datetime(2030, 8, 27, 12, tzinfo=UTC),
                left_at=None,
                created_at=datetime(2030, 8, 27, 12, tzinfo=UTC),
            )
            for context, person in sorted(
                self.active_memberships, key=lambda pair: pair[1].bytes
            )
            if context == context_id
        ]

    def get_context(self, context_id):
        return self.contexts.get(context_id)

    def update_context(self, context_id, *, changes):
        current = self.contexts.get(context_id)
        if current is None:
            return None
        updated = replace(current, **{k: v for k, v in changes.items()})
        self.contexts[context_id] = updated
        return updated

    def get_pair_context(self, pair_key):
        context_id = self.pair_contexts.get(pair_key)
        return None if context_id is None else self.contexts.get(context_id)

    def create_pair_context(self, *, pair_key, member_ids, created_by_id, now):
        if pair_key in self.pair_contexts:
            raise RepositoryConflict("PAIR_EXISTS")
        record = ContextRecord(
            id=uuid.uuid4(),
            display_name="",
            created_by_id=created_by_id,
            created_at=now,
            kind="pair",
            pair_key=pair_key,
        )
        self.contexts[record.id] = record
        self.pair_contexts[pair_key] = record.id
        for person_id in member_ids:
            self.active_memberships.add((record.id, person_id))
        return record

    def get_message(self, message_id):
        return self.messages.get(message_id)

    def get_messages_by_ids(self, message_ids):
        return {m: self.messages[m] for m in message_ids if m in self.messages}

    def soft_delete_message(self, message_id, *, now):
        current = self.messages.get(message_id)
        if current is None:
            return None
        updated = replace(
            current,
            kind="deleted",
            body=None,
            image_url=None,
            card=None,
            deleted_at=now,
        )
        self.messages[message_id] = updated
        return updated

    def create_outing_invite(
        self,
        *,
        outing_id,
        source,
        invited_person_id,
        invited_by_id,
        token_digest,
        expires_at,
        now,
    ):
        record = OutingInviteRecord(
            id=uuid.uuid4(),
            outing_id=outing_id,
            source=source,
            invited_person_id=invited_person_id,
            invited_by_id=invited_by_id,
            accepted_at=None,
            accepted_by_id=None,
            created_at=now,
            expires_at=expires_at,
            revoked_at=None,
        )
        self.outing_invites[record.id] = record
        if token_digest is not None:
            self.outing_invite_ids_by_digest[token_digest] = record.id
        return record

    def find_outing_invite_for_person(self, outing_id, person_id):
        return next(
            (
                invite
                for invite in self.outing_invites.values()
                if invite.outing_id == outing_id
                and invite.invited_person_id == person_id
            ),
            None,
        )

    def get_outing_invite(self, invite_id):
        return self.outing_invites.get(invite_id)

    def get_outing_invite_by_digest(self, token_digest):
        invite_id = self.outing_invite_ids_by_digest.get(token_digest)
        if invite_id is None:
            return None
        return self.outing_invites.get(invite_id)

    def accept_outing_invite(self, *, invite_id, accepted_by_id, now):
        invite = self.outing_invites.get(invite_id)
        if invite is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_FOUND")
        if invite.accepted_at is not None:
            raise RepositoryConflict("OUTING_INVITE_ALREADY_ACCEPTED")
        if invite.revoked_at is not None or invite.expires_at <= now:
            raise RepositoryConflict("OUTING_INVITE_NOT_REDEEMABLE")
        accepted = replace(
            invite,
            accepted_at=now,
            accepted_by_id=accepted_by_id,
        )
        self.outing_invites[invite_id] = accepted
        return accepted

    def revoke_outing_invite(self, *, invite_id, now):
        invite = self.outing_invites.get(invite_id)
        if invite is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_FOUND")
        if invite.accepted_at is not None:
            raise RepositoryConflict("OUTING_INVITE_ALREADY_ACCEPTED")
        if invite.revoked_at is not None:
            return invite
        revoked = replace(invite, revoked_at=now)
        self.outing_invites[invite_id] = revoked
        return revoked

    def rotate_outing_invite_digest(self, *, invite_id, token_digest, expires_at, now):
        del now
        invite = self.outing_invites.get(invite_id)
        if invite is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_FOUND")
        if invite.invited_person_id is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_NAMED")
        if invite.revoked_at is not None:
            raise RepositoryConflict("OUTING_INVITE_NOT_REDEEMABLE")
        self.outing_invite_ids_by_digest = {
            digest: held
            for digest, held in self.outing_invite_ids_by_digest.items()
            if held != invite_id
        }
        self.outing_invite_ids_by_digest[token_digest] = invite_id
        rotated = replace(invite, expires_at=expires_at)
        self.outing_invites[invite_id] = rotated
        return rotated

    def consume_named_invite_secret(
        self, *, invite_id, token_digest, accepted_by_id, now
    ):
        invite = self.outing_invites.get(invite_id)
        if invite is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_FOUND")
        if invite.invited_person_id is None:
            raise RepositoryConflict("OUTING_INVITE_NOT_NAMED")
        if self.outing_invite_ids_by_digest.get(token_digest) != invite_id:
            raise RepositoryConflict("OUTING_INVITE_NOT_REDEEMABLE")
        if invite.revoked_at is not None or invite.expires_at <= now:
            raise RepositoryConflict("OUTING_INVITE_NOT_REDEEMABLE")
        # The digest leaves the table entirely, exactly as the real adapter
        # nulls the column: a spent secret has nothing left to match against.
        del self.outing_invite_ids_by_digest[token_digest]
        if invite.accepted_at is None:
            invite = replace(invite, accepted_at=now, accepted_by_id=accepted_by_id)
            self.outing_invites[invite_id] = invite
        return invite

    def create_account_session(
        self,
        *,
        person_id,
        token_digest,
        issued_from_invite_id,
        expires_at,
        now,
        issued_via=None,
    ):
        if issued_via is None:
            issued_via = "invite" if issued_from_invite_id is not None else "genesis"
        record = AccountSessionRecord(
            id=uuid.uuid4(),
            person_id=person_id,
            issued_from_invite_id=issued_from_invite_id,
            issued_via=issued_via,
            created_at=now,
            expires_at=expires_at,
            revoked_at=None,
        )
        self.account_sessions[record.id] = record
        self.account_session_ids_by_digest[token_digest] = record.id
        return record

    # --- sessions, blocks, reports, erasure (ADR-0023) --------------------

    def list_account_sessions(self, person_id, *, now):
        rows = [
            row
            for row in self.account_sessions.values()
            if row.person_id == person_id
            and row.revoked_at is None
            and row.expires_at > now
        ]
        rows.sort(key=lambda row: (row.created_at, row.id.bytes), reverse=True)
        return rows

    def get_account_session(self, session_id):
        return self.account_sessions.get(session_id)

    def revoke_all_account_sessions(self, person_id, *, now):
        live = [
            row
            for row in self.account_sessions.values()
            if row.person_id == person_id and row.revoked_at is None
        ]
        for row in live:
            self.account_sessions[row.id] = replace(row, revoked_at=now)
        return len(live)

    def _live_edge_between(self, a, b):
        for edge in self.friend_edges.values():
            if {edge["requester_id"], edge["addressee_id"]} == {a, b} and edge[
                "state"
            ] != "declined":
                return edge
        return None

    def open_block_edge(self, *, blocker_id, addressee_id, now):
        edge = self._live_edge_between(blocker_id, addressee_id)
        if edge is None:
            edge = {
                "id": uuid.uuid4(),
                "requester_id": blocker_id,
                "addressee_id": addressee_id,
                "state": "blocked",
                "decided_by_id": blocker_id,
                "created_at": now,
                "decided_at": now,
            }
            self.friend_edges[edge["id"]] = edge
        else:
            edge["state"] = "blocked"
            edge["decided_by_id"] = blocker_id
            edge["decided_at"] = now
        return self._friend_record(edge, blocker_id)

    def lift_block_edge(self, *, blocker_id, addressee_id, now):
        edge = self._live_edge_between(blocker_id, addressee_id)
        if edge is None or edge["state"] != "blocked":
            raise RepositoryConflict("NOT_BLOCKED")
        if edge["decided_by_id"] != blocker_id:
            raise RepositoryConflict("ONLY_BLOCKER_MAY_UNBLOCK")
        edge["state"] = "declined"
        edge["decided_at"] = now
        return self._friend_record(edge, blocker_id)

    def list_blocked(self, person_id):
        rows = [
            edge
            for edge in self.friend_edges.values()
            if edge["state"] == "blocked" and edge["decided_by_id"] == person_id
        ]
        rows.sort(key=lambda edge: (edge["decided_at"] or edge["created_at"]))
        return [self._friend_record(edge, person_id) for edge in reversed(rows)]

    def create_report(self, *, reporter_id, target_type, target_id, reason, note, now):
        record = ReportRecord(id=uuid.uuid4(), created_at=now)
        self.reports_filed.append(
            {
                "id": record.id,
                "reporter_id": reporter_id,
                "target_type": target_type,
                "target_id": target_id,
                "reason": reason,
                "note": note,
                "created_at": now,
            }
        )
        return record

    def erase_person(self, person_id, *, now):
        """The fake's copy of the erasure map. Deliberately literal, and
        deliberately NOT clever: it deletes from the same dicts the rest of
        this fake reads, so a service that forgot one of them shows up here
        rather than only in tests/postgres."""
        person = self.people.get(person_id)
        if person is None:
            raise RepositoryConflict("PERSON_NOT_FOUND")
        counts = {}
        keys = tuple(
            image.storage_key
            for image in self.uploaded_images.values()
            if image.owner_person_id == person_id
        )
        own_posts = {p.id for p in self.posts.values() if p.author_id == person_id}
        own_stories = {s.id for s in self.stories.values() if s.author_id == person_id}

        def drop(table, mapping, keep):
            before = len(mapping)
            for key in [k for k, v in mapping.items() if not keep(v)]:
                del mapping[key]
            counts[table] = before - len(mapping)

        drop(
            "post_comments",
            self.post_comments,
            lambda c: c.author_id != person_id and c.post_id not in own_posts,
        )
        drop(
            "post_reactions",
            self.post_reactions,
            lambda r: r.person_id != person_id and r.post_id not in own_posts,
        )
        counts["story_views"] = 0
        for key in [
            k for k in self.story_views if k[1] == person_id or k[0] in own_stories
        ]:
            del self.story_views[key]
            counts["story_views"] += 1
        drop("posts", self.posts, lambda p: p.author_id != person_id)
        drop("stories", self.stories, lambda s: s.author_id != person_id)
        drop(
            "uploaded_images",
            self.uploaded_images,
            lambda i: i.owner_person_id != person_id,
        )
        counts["person_interests"] = len(self.person_interests.pop(person_id, ()) or ())
        counts["saved_places"] = 0
        for key in [k for k in self.saved_places if k[0] == person_id]:
            del self.saved_places[key]
            counts["saved_places"] += 1
        counts["context_read_marks"] = 0
        for key in [k for k in self.read_marks if k[1] == person_id]:
            del self.read_marks[key]
            counts["context_read_marks"] += 1
        counts["account_identities"] = 0
        for key in [
            k for k, v in self.account_identities.items() if v.person_id == person_id
        ]:
            del self.account_identities[key]
            counts["account_identities"] += 1
        drop(
            "friend_requests",
            self.friend_edges,
            lambda e: person_id not in (e["requester_id"], e["addressee_id"]),
        )
        counts["account_sessions"] = self.revoke_all_account_sessions(
            person_id, now=now
        )
        left = {
            (context_id, member)
            for (context_id, member) in self.active_memberships
            | self.invited_memberships
            if member == person_id
        }
        self.active_memberships -= left
        self.invited_memberships -= left
        self.left_memberships |= left
        counts["memberships"] = len(left)
        self.people[person_id] = replace(
            person,
            display_name=ANONYMOUS_DISPLAY_NAME,
            bio=None,
            city=None,
            budget_band=None,
            discoverable_by_phone=False,
            wall_comment_policy="nobody",
            deleted_at=now,
        )
        return ErasureReport(counts=counts, storage_keys=keys)

    def get_account_session_by_digest(self, token_digest):
        session_id = self.account_session_ids_by_digest.get(token_digest)
        if session_id is None:
            return None
        return self.account_sessions.get(session_id)

    def revoke_account_session(self, *, session_id, now):
        record = self.account_sessions.get(session_id)
        if record is None:
            return None
        if record.revoked_at is None:
            record = replace(record, revoked_at=now)
            self.account_sessions[session_id] = record
        return record

    def actor_grants(self, person_id):
        """The fake's copy of the roster rule, kept deliberately literal.

        It mirrors `SqlAlchemyApiRepository.actor_grants` rather than being
        clever: the four capability roles are granted to any known person
        because every action that names them also proves a predicate, and
        `platform_moderator` is never granted because nothing says who holds it.
        """

        person = self.people.get(person_id)
        if person is None or person.deleted_at is not None:
            # Mirrors the real adapter after ADR-0023 §2.1.4: an ended account
            # is not a person any session may speak as.
            return ActorGrants(
                person_exists=False, roles=frozenset(), context_ids=frozenset()
            )
        # No `group_admin` here on purpose, mirroring the real adapter: it is a
        # fact about one group, and the service derives it per call.
        roles = {"member", "advancer", "recipient", "sender", "creditor"}
        context_ids = {
            context_id
            for context_id, held in self.active_memberships
            if held == person_id
        }
        if any(held == person_id for _, held in self.left_memberships):
            roles.add("former_member")
        return ActorGrants(
            person_exists=True,
            roles=frozenset(roles),
            context_ids=frozenset(context_ids),
        )

    def _membership_id_for(self, context_id, person_id):
        return uuid.uuid5(uuid.NAMESPACE_URL, f"{context_id}/{person_id}")

    def _messages_in(self, context_id):
        return sorted(
            (m for m in self.messages.values() if m.context_id == context_id),
            key=lambda m: (m.created_at, m.id.bytes),
        )

    def count_unread_messages(self, context_id, person_id):
        mark = self.read_marks.get((context_id, person_id))
        total = 0
        for m in self._messages_in(context_id):
            if m.author_id == person_id:
                continue
            if mark is not None and (m.created_at, m.id.bytes) <= (
                mark.last_read_at,
                mark.last_read_message_id.bytes,
            ):
                continue
            total += 1
        return total

    def list_person_context_summaries(self, person_id):
        pairs = {
            (c, p)
            for c, p in self.active_memberships | self.invited_memberships
            if p == person_id and (c, p) not in self.left_memberships
        }
        out = []
        for context_id, _ in pairs:
            context = self.contexts.get(context_id)
            if context is None:
                continue
            state = (
                "active"
                if (context_id, person_id) in self.active_memberships
                else "invited"
            )
            role = self.membership_role(context_id, person_id) or "member"
            other_id = None
            other_name = None
            if context.kind == "pair":
                others = [
                    p
                    for c, p in self.active_memberships
                    if c == context_id and p != person_id
                ]
                other_id = others[0] if others else None
                other = self.people.get(other_id) if other_id else None
                other_name = other.display_name if other else None
            newest = self._messages_in(context_id)
            last = None
            if newest:
                m = newest[-1]
                author = self.people.get(m.author_id) if m.author_id else None
                last = LastMessageRecord(
                    id=m.id,
                    kind=m.kind,
                    preview={
                        "text": (m.body or "")[:80],
                        "image": "[Ảnh]",
                        "sticker": "[Sticker]",
                        "deleted": "Tin nhắn đã bị xoá",
                    }.get(m.kind, "[Rủ Đi AI]"),
                    author_id=m.author_id,
                    author_display_name=author.display_name if author else None,
                    created_at=m.created_at,
                )
            out.append(
                PersonContextSummaryRecord(
                    id=context_id,
                    display_name=display_name_for(
                        context.kind, context.display_name, other_name
                    ),
                    member_count=sum(
                        1 for c, _ in self.active_memberships if c == context_id
                    ),
                    my_role=role,
                    my_state=state,
                    membership_id=self._membership_id_for(context_id, person_id),
                    joined_at=datetime(2030, 8, 27, 12, tzinfo=UTC)
                    if state == "active"
                    else None,
                    last_message=last,
                    unread_count=self.count_unread_messages(context_id, person_id),
                    theme=context.theme,
                    kind=context.kind,
                    counterpart_id=other_id,
                    counterpart_display_name=other_name,
                )
            )
        out.sort(
            key=lambda r: (
                r.last_message is None,
                -(r.last_message.created_at.timestamp()) if r.last_message else 0,
                r.display_name,
            )
        )
        return out

    def get_read_mark(self, context_id, person_id):
        return self.read_marks.get((context_id, person_id))

    def set_read_mark(self, *, context_id, person_id, message, now):
        del now
        current = self.read_marks.get((context_id, person_id))
        if current is None or (message.created_at, message.id.bytes) > (
            current.last_read_at,
            current.last_read_message_id.bytes,
        ):
            current = ReadMarkRecord(
                context_id=context_id,
                person_id=person_id,
                last_read_message_id=message.id,
                last_read_at=message.created_at,
            )
            self.read_marks[(context_id, person_id)] = current
        return current

    def create_otp_challenge(
        self, *, challenge_id, phone_digest, code_digest, expires_at, now
    ):
        record = OtpChallengeRecord(
            id=challenge_id,
            phone_digest=phone_digest,
            code_digest=code_digest,
            created_at=now,
            expires_at=expires_at,
            attempts=0,
            consumed_at=None,
        )
        self.otp_challenges[challenge_id] = record
        return record

    def recent_otp_challenges(self, phone_digest, since):
        return sorted(
            (
                r
                for r in self.otp_challenges.values()
                if r.phone_digest == phone_digest and r.created_at > since
            ),
            key=lambda r: r.created_at,
            reverse=True,
        )

    def get_otp_challenge(self, challenge_id):
        return self.otp_challenges.get(challenge_id)

    def record_otp_attempt(self, *, challenge_id, attempts, consumed, now):
        record = self.otp_challenges.get(challenge_id)
        if record is None:
            return None
        record = replace(
            record,
            attempts=attempts,
            consumed_at=(record.consumed_at or now) if consumed else record.consumed_at,
        )
        self.otp_challenges[challenge_id] = record
        return record

    def get_account_identity(self, provider, subject):
        return self.account_identities.get((provider, subject))

    def create_person_with_identity(
        self, *, person_id, display_name, provider, subject, now
    ):
        if (provider, subject) in self.account_identities:
            raise RepositoryConflict("IDENTITY_ALREADY_BOUND")
        self.create_person(person_id, display_name)
        return self.upsert_account_identity(
            person_id=person_id, provider=provider, subject=subject, now=now
        )

    # --- M2 profile and bookmarks -----------------------------------------

    def update_person_profile(self, person_id, *, changes):
        record = self.people.get(person_id)
        if record is None:
            return None
        record = replace(record, **changes)
        self.people[person_id] = record
        return record

    def profile_counts(self, person_id):
        friends = sum(
            1
            for edge in self.friend_edges.values()
            if edge["state"] == "accepted"
            and person_id in (edge["requester_id"], edge["addressee_id"])
        )
        my_contexts = {
            cid for (cid, pid) in self.active_memberships if pid == person_id
        }
        outings = sum(
            1 for cid in self.outings_by_context.values() if cid in my_contexts
        )
        places = len({stop for (pid, stop) in self.stop_checkins if pid == person_id})
        memories = sum(1 for m in self.memories.values() if m.author_id == person_id)
        groups = {
            cid
            for cid in my_contexts
            if cid not in self.contexts or self.contexts[cid].kind != "pair"
        }
        return ProfileCounts(
            friends=friends,
            contexts=len(groups),
            outings=outings,
            places_checked_in=places,
            memories=memories,
        )

    def list_person_interests(self, person_id):
        return sorted(self.person_interests.get(person_id, set()))

    def set_person_interests(self, person_id, tags, now):
        self.person_interests[person_id] = set(tags)
        return self.list_person_interests(person_id)

    def interests_by_person(self, person_ids):
        return {
            pid: sorted(self.person_interests[pid])
            for pid in person_ids
            if self.person_interests.get(pid)
        }

    def budget_bands_by_person(self, person_ids):
        out = {}
        for pid in person_ids:
            person = self.people.get(pid)
            if person is not None and person.budget_band is not None:
                out[pid] = person.budget_band
        return out

    def list_login_providers(self, person_id):
        return sorted(
            {
                identity.provider
                for identity in self.account_identities.values()
                if identity.person_id == person_id
            }
        )

    def are_friends(self, a, b):
        return any(
            edge["state"] == "accepted"
            and {edge["requester_id"], edge["addressee_id"]} == {a, b}
            for edge in self.friend_edges.values()
        )

    def share_active_context(self, a, b):
        mine = {cid for (cid, pid) in self.active_memberships if pid == a}
        return any(cid in mine for (cid, pid) in self.active_memberships if pid == b)

    def shares_active_context(self, viewer_id, subject_id):
        # The avatar gate asks by this name; same fact as the line above.
        return self.share_active_context(viewer_id, subject_id)

    # --- ảnh địa điểm có giấy phép (M12) ----------------------------------

    def list_place_photos(self, place_id):
        rows = [row for row in self.place_photos if row.place_id == place_id]
        return sorted(rows, key=lambda row: (row.sort_order, str(row.id)))

    def get_place_photo(self, place_id, photo_id):
        for row in self.place_photos:
            if row.id == photo_id and row.place_id == place_id:
                return row
        return None

    def photo_covers(self, place_ids):
        out = {}
        for row in sorted(self.place_photos, key=lambda r: (r.sort_order, str(r.id))):
            if row.place_id in place_ids:
                out.setdefault(row.place_id, row)
        return out

    def photo_counts(self, place_ids):
        out: dict[str, int] = {}
        for row in self.place_photos:
            if row.place_id in place_ids:
                out[row.place_id] = out.get(row.place_id, 0) + 1
        return out

    def list_saved_places(self, person_id):
        rows = [r for (pid, _), r in self.saved_places.items() if pid == person_id]
        return sorted(rows, key=lambda r: (r.created_at, r.id), reverse=True)

    def save_place(self, person_id, place_id, now):
        existing = self.saved_places.get((person_id, place_id))
        if existing is not None:
            return existing, False
        record = SavedPlaceRecord(
            id=uuid.uuid4(), person_id=person_id, place_id=place_id, created_at=now
        )
        self.saved_places[(person_id, place_id)] = record
        return record, True

    def unsave_place(self, person_id, place_id):
        return self.saved_places.pop((person_id, place_id), None) is not None

    def upsert_account_identity(self, *, person_id, provider, subject, now):
        existing = self.account_identities.get((provider, subject))
        if existing is None:
            existing = AccountIdentityRecord(
                id=uuid.uuid4(),
                person_id=person_id,
                provider=provider,
                subject=subject,
                created_at=now,
                last_login_at=now,
            )
        else:
            existing = replace(existing, last_login_at=now)
        self.account_identities[(provider, subject)] = existing
        return existing

    def ensure_invited_membership(
        self, *, context_id, person_id, invited_by_id, origin, now
    ):
        del now
        # The real adapter returns an existing row untouched, which is what
        # keeps an ACTIVE member from being demoted by signing in again.
        state = (
            "active"
            if (context_id, person_id) in self.active_memberships
            else "invited"
        )
        if state == "invited":
            self.invited_memberships.add((context_id, person_id))
        person = self.people.get(person_id)
        return MembershipRecord(
            id=uuid.uuid4(),
            context_id=context_id,
            person_id=person_id,
            display_name="" if person is None else person.display_name,
            state=state,
            role="member",
            origin=origin,
            invited_by_id=invited_by_id,
            joined_at=None,
            left_at=None,
            created_at=datetime(2030, 8, 27, 12, tzinfo=UTC),
        )

    def create_bill(
        self,
        *,
        context_id,
        created_by_id,
        printed_total_vnd,
        items_total_vnd,
        confidence,
        needs_review,
        items,
        surcharges,
        discounts,
        now,
    ):
        # Mirrors the three unique constraints on the bill draft tables so the
        # route's 409 can be exercised without a database. Being taught to
        # refuse is not the same as being unable to accept: what PostgreSQL
        # actually does with these rows is proved in
        # tests/postgres/test_bill_duplicate_item_key_postgres.py.
        for lines, key, code in (
            (items, "item_key", "DUPLICATE_BILL_ITEM_KEY"),
            (surcharges, "surcharge_key", "DUPLICATE_BILL_SURCHARGE_KEY"),
            (discounts, "discount_key", "DUPLICATE_BILL_DISCOUNT_KEY"),
        ):
            keys = [line[key] for line in lines]
            if len(keys) != len(set(keys)):
                raise RepositoryConflict(code)

        bill = BillRecord(
            id=uuid.uuid4(),
            context_id=context_id,
            printed_total_vnd=printed_total_vnd,
            items_total_vnd=items_total_vnd,
            confidence=confidence,
            needs_review=needs_review,
            created_by_id=created_by_id,
            created_at=now,
            items=[
                BillItemRecord(
                    item_key=item["item_key"],
                    name=item["name"],
                    quantity=item["quantity"],
                    unit_price_vnd=item["unit_price_vnd"],
                    line_total_vnd=item["line_total_vnd"],
                    position=item["position"],
                    shares=[
                        BillShareRecord(
                            participant_id=participant_id,
                            source="ai_suggested",
                            decided_by_id=None,
                            decided_at=None,
                        )
                        for participant_id in item["suggested_participant_ids"]
                    ],
                )
                for item in items
            ],
            surcharges=[
                BillSurchargeRecord(
                    surcharge_key=surcharge["surcharge_key"],
                    kind=surcharge["kind"],
                    amount_vnd=surcharge["amount_vnd"],
                    mode=surcharge["mode"],
                )
                for surcharge in surcharges
            ],
            discounts=[
                BillDiscountRecord(
                    discount_key=discount["discount_key"],
                    amount_vnd=discount["amount_vnd"],
                    scope=discount["scope"],
                    target_item_key=discount["target_item_key"],
                )
                for discount in discounts
            ],
        )
        self.bills[bill.id] = bill
        return self._ordered_bill(bill)

    def get_bill(self, bill_id):
        bill = self.bills.get(bill_id)
        return None if bill is None else self._ordered_bill(bill)

    def confirm_bill_assignments(
        self,
        *,
        bill_id,
        assignments,
        decided_by_id,
        now,
    ):
        bill = self.bills.get(bill_id)
        if bill is None:
            raise RepositoryConflict("BILL_NOT_FOUND")

        assignments_by_key = {
            assignment["item_key"]: assignment for assignment in assignments
        }
        item_keys = {item.item_key for item in bill.items}
        if set(assignments_by_key) - item_keys:
            raise RepositoryConflict("UNKNOWN_BILL_ITEM")

        updated_items = []
        for item in bill.items:
            assignment = assignments_by_key.get(item.item_key)
            if assignment is None:
                updated_items.append(item)
                continue
            updated_items.append(
                replace(
                    item,
                    shares=[
                        BillShareRecord(
                            participant_id=participant_id,
                            source="confirmed",
                            decided_by_id=decided_by_id,
                            decided_at=now,
                        )
                        for participant_id in assignment["participant_ids"]
                    ],
                )
            )

        updated = replace(bill, items=updated_items)
        self.bills[bill_id] = updated
        return self._ordered_bill(updated)

    def claim_bill_items(self, *, bill_id, participant_id, item_keys, now):
        """Mirror of the SQL version: this participant's claims, bill-wide.

        Written to the same contract rather than to whatever made the cases
        pass. The one that matters is scope -- shares belonging to other people
        are never touched -- because a fake that dropped them would let the
        route's most damaging bug through the whole API tier.
        """

        bill = self.bills.get(bill_id)
        if bill is None:
            raise RepositoryConflict("BILL_NOT_FOUND")

        requested = dict.fromkeys(item_keys)
        item_keys_present = {item.item_key for item in bill.items}
        if set(requested) - item_keys_present:
            raise RepositoryConflict("UNKNOWN_BILL_ITEM")

        updated_items = []
        for item in bill.items:
            others = [
                share for share in item.shares if share.participant_id != participant_id
            ]
            if item.item_key in requested:
                others.append(
                    BillShareRecord(
                        participant_id=participant_id,
                        source="confirmed",
                        decided_by_id=participant_id,
                        decided_at=now,
                    )
                )
            updated_items.append(replace(item, shares=others))

        updated = replace(bill, items=updated_items)
        self.bills[bill_id] = updated
        return self._ordered_bill(updated)

    def create_expense(self, context_id):
        record = ExpenseIdentity(id=uuid.uuid4(), context_id=context_id)
        self.expenses[record.id] = record
        return record

    def get_expense(self, expense_id):
        return self.expenses.get(expense_id)

    def save_expense_confirmation(
        self,
        *,
        expense_id,
        proposal,
        allocator_expense,
        rollups,
        allocations,
        confirmed_by_id,
        payer_acknowledgement,
        now,
    ):
        del allocator_expense, rollups, confirmed_by_id, now
        version_id = uuid.uuid4()
        number = self.version_numbers.get(expense_id, 0) + 1
        self.version_numbers[expense_id] = number
        rows = tuple(
            AllocationRow(
                id=uuid.uuid4(), participant_id=participant, amount_vnd=amount
            )
            for participant, amount in allocations.items()
        )
        self.confirmed[version_id] = ConfirmedExpense(
            version_id=version_id,
            context_id=proposal.context_id,
            paid_by_id=proposal.paid_by_id,
            payer_acknowledgement=payer_acknowledgement,
            allocations=rows,
        )
        self.version_to_expense[version_id] = expense_id
        return ConfirmationRecord(expense_version_id=version_id, version_number=number)

    def load_batch_inputs(self, context_id, expense_version_ids):
        selected = set(expense_version_ids) if expense_version_ids is not None else None
        records = tuple(
            record
            for version_id, record in self.confirmed.items()
            if record.context_id == context_id
            and (selected is None or version_id not in self.batched_versions)
            and (selected is None or version_id in selected)
        )
        unavailable = (
            tuple(sorted(selected - {record.version_id for record in records}, key=str))
            if selected is not None
            else ()
        )
        return BatchInputs(expenses=records, unavailable_version_ids=unavailable)

    def load_confirmed_receipts(self, context_id):
        batch_version_ids = {
            batch.version_id
            for batch in self.batches.values()
            if batch.context_id == context_id
        }
        totals = {}
        for receipt in self.receipts.values():
            obligation = self.obligations.get(receipt.obligation_id)
            if (
                obligation is None
                or obligation.batch_version_id not in batch_version_ids
                or receipt.confirmed_by_id != obligation.recipient_id
            ):
                continue
            pair = (obligation.sender_id, obligation.recipient_id)
            totals[pair] = totals.get(pair, 0) + receipt.amount_vnd
        return dict(
            sorted(
                totals.items(),
                key=lambda item: (item[0][0].bytes, item[0][1].bytes),
            )
        )

    def save_frozen_batch(
        self,
        *,
        context_id,
        owner_id,
        due_at,
        obligations: tuple[ObligationDraft, ...],
        now,
    ):
        del now
        batch_id = uuid.uuid4()
        version_id = uuid.uuid4()
        frozen = []
        publish = []
        for draft in obligations:
            obligation_id = uuid.uuid4()
            frozen.append(
                FrozenObligation(
                    id=obligation_id,
                    sender_id=draft.sender_id,
                    recipient_id=draft.recipient_id,
                    amount_vnd=draft.amount_vnd,
                    due_at=due_at,
                    source_expense_version_ids=draft.source_expense_version_ids,
                )
            )
            record = PublishObligation(
                id=obligation_id,
                batch_version_id=version_id,
                sender_id=draft.sender_id,
                recipient_id=draft.recipient_id,
                amount_vnd=draft.amount_vnd,
            )
            publish.append(record)
            self.obligations[obligation_id] = record
            self.batched_versions.update(draft.source_expense_version_ids)
        source_versions = {
            source
            for draft in obligations
            for source in draft.source_expense_version_ids
        }
        acknowledged = bool(source_versions) and all(
            self.confirmed[source].payer_acknowledgement == "acknowledged"
            for source in source_versions
        )
        self.batches[batch_id] = BatchForPublish(
            id=batch_id,
            version_id=version_id,
            owner_id=owner_id,
            status="frozen",
            context_id=context_id,
            advancer_acknowledged=acknowledged,
            obligations=tuple(publish),
        )
        return FrozenBatch(
            id=batch_id, version_id=version_id, obligations=tuple(frozen)
        )

    def load_batch_for_publish(self, batch_id):
        return self.batches.get(batch_id)

    def save_published_batch(
        self,
        *,
        batch,
        status,
        links: tuple[GuestLinkDraft, ...],
        actor_id,
        now,
    ):
        del actor_id, now
        self.batches[batch.id] = replace(batch, status=status)
        stored = []
        for draft in links:
            link = FakeLink(
                id=uuid.uuid4(),
                sender_id=draft.sender_id,
                batch_id=batch.id,
                expires_at=draft.expires_at,
            )
            self.links[draft.token_digest] = link
            stored.append(
                StoredGuestLink(
                    id=link.id,
                    envelope_id=uuid.uuid4(),
                    sender_id=draft.sender_id,
                )
            )
        return tuple(stored)

    def get_guest_envelope(self, token_digest, now):
        link = self.links.get(token_digest)
        if link is None:
            return None
        batch = self.batches[link.batch_id]
        obligations = [
            item for item in batch.obligations if item.sender_id == link.sender_id
        ]
        capability_scope(
            {"batch_version_id": batch.version_id, "sender_id": link.sender_id},
            [
                {
                    "obligation_id": item.id,
                    "batch_version_id": item.batch_version_id,
                    "sender_id": item.sender_id,
                }
                for item in obligations
            ],
        )
        # Counted per obligation, same as SqlAlchemyApiRepository: three
        # objections about one debt must not use up the right to say anything
        # about a different debt on the same link.
        objection_counts: dict[str, int] = {}
        for objection in self.objections:
            if objection["kind"] not in ("not_me", "wrong_amount"):
                continue
            key = str(objection["obligation_id"]) if objection["obligation_id"] else "*"
            objection_counts[key] = objection_counts.get(key, 0) + 1

        # Same derivation as SqlAlchemyApiRepository: a wrong-amount objection
        # naming an obligation makes that obligation disputed, and nothing else.
        disputed_ids = {
            str(objection["obligation_id"])
            for objection in self.objections
            if objection["kind"] == "wrong_amount" and objection["obligation_id"]
        }
        blocks = []
        for item in obligations:
            receipts = [
                receipt.amount_vnd
                for receipt in self.receipts.values()
                if receipt.obligation_id == item.id
            ]
            status = obligation_status(
                item.amount_vnd, [{"amount_vnd": amount} for amount in receipts]
            )
            blocks.append(
                {
                    "obligation_id": str(item.id),
                    "disputed": str(item.id) in disputed_ids,
                    "objections_used": objection_counts.get(str(item.id), 0),
                    "objections_allowed": OBJECTION_LIMIT,
                    "occasion_label": "bữa tối",
                    "amount_vnd": item.amount_vnd,
                    "recipient_display_name": "Nam",
                    "evidence_requested": any(
                        objection["kind"] == "evidence_request"
                        and objection["obligation_id"] == item.id
                        for objection in self.objections
                    ),
                    "already_reported": any(
                        report.link_id == link.id and report.obligation_id == item.id
                        for report in self.reports.values()
                    ),
                    "receiver_confirmed": status in {"confirmed", "over_confirmed"},
                }
            )
        state = "expired" if now >= link.expires_at else link.status
        envelope = {
            "recorded_by_display_name": "Nam",
            "claimed_person_display_name": "Hà",
            "link_state": state,
            "obligations": blocks,
            "reports_used": sum(
                report.link_id == link.id for report in self.reports.values()
            ),
            "reports_allowed": REPORT_LIMIT,
            # Counted, not assumed. This double said zero forever, so a test
            # could not have caught the quota never being reached.
            "objections_used": sum(
                objection["token_digest"] == token_digest
                and objection["kind"] in ("not_me", "wrong_amount")
                for objection in self.objections
            ),
            "objections_allowed": OBJECTION_LIMIT,
        }
        if self.leak_guest_input:
            envelope["group_balance"] = {"someone_else": 123}
        return GuestEnvelopeRecord(link_id=link.id, envelope=envelope)

    def get_payment_report_target(self, token_digest, obligation_id, now):
        link = self.links.get(token_digest)
        obligation = self.obligations.get(obligation_id)
        if link is None or obligation is None:
            return None
        batch = self.batches[link.batch_id]
        if (
            obligation.batch_version_id != batch.version_id
            or obligation.sender_id != link.sender_id
        ):
            return None
        return PaymentReportTarget(
            link_id=link.id,
            obligation_id=obligation.id,
            amount_vnd=obligation.amount_vnd,
            active_capability=link.status == "active" and now < link.expires_at,
            reports_used=sum(
                report.link_id == link.id for report in self.reports.values()
            ),
        )

    def save_guest_objection(self, *, token_digest, kind, obligation_id, reason, now):
        del now
        self.objections.append(
            {
                "token_digest": token_digest,
                "kind": kind,
                "obligation_id": obligation_id,
                "reason": reason,
            }
        )
        if kind == "not_me":
            link = self.links.get(token_digest)
            if link is not None:
                link.status = "revoked"

    def save_payment_report(self, *, target, idempotency_key, now):
        existing = next(
            (
                report
                for report in self.reports.values()
                if report.idempotency_key == idempotency_key
            ),
            None,
        )
        if existing is not None:
            if (
                existing.link_id != target.link_id
                or existing.obligation_id != target.obligation_id
            ):
                raise RepositoryConflict("IDEMPOTENCY_KEY_REUSED")
            report = existing
        else:
            report = FakeReport(
                id=uuid.uuid4(),
                link_id=target.link_id,
                obligation_id=target.obligation_id,
                amount_vnd=target.amount_vnd,
                idempotency_key=idempotency_key,
                reported_at=now,
            )
            self.reports[report.id] = report
        return PaymentReportRecord(
            id=report.id,
            obligation_id=report.obligation_id,
            amount_vnd=report.amount_vnd,
            receipt_amounts_vnd=tuple(
                receipt.amount_vnd
                for receipt in self.receipts.values()
                if receipt.obligation_id == report.obligation_id
            ),
        )

    def person_finance_summary(self, person_id, *, movement_limit):
        """Whatever a test put there, handed back.

        Deliberately not a reimplementation of the SQL. A fake that recomputed
        these totals would be a second answer to the money question, and the
        suite would then be checking the fake against itself. What the totals
        should be is settled in `tests/postgres/test_person_finance_postgres.py`
        against the real ledger; what this supports is the layer above -- who is
        allowed to ask, and what the route does with the answer.
        """
        summary = self.finances.get(person_id)
        if summary is None:
            return PersonFinanceSummary(
                person_id=person_id,
                display_name=None,
                spend_vnd=0,
                settled_vnd=0,
                outstanding_vnd=0,
                receivable_vnd=0,
                expense_count=0,
                group_count=0,
                movements=(),
            )
        return replace(summary, movements=summary.movements[:movement_limit])

    def list_batch_obligations(self, batch_id):
        obligations = list(self.obligations.values())
        if not obligations:
            return None
        disputes: dict[str, str | None] = {}
        for objection in self.objections:
            if objection["kind"] == "wrong_amount" and objection["obligation_id"]:
                disputes.setdefault(
                    str(objection["obligation_id"]), objection.get("reason")
                )
        rows = []
        for item in sorted(obligations, key=lambda o: str(o.sender_id)):
            receipts = [
                receipt.amount_vnd
                for receipt in self.receipts.values()
                if receipt.obligation_id == item.id
            ]
            # Earliest, matching the SQL: a repeat report is the same claim
            # said again, not a later one.
            claims = [
                report.reported_at
                for report in self.reports.values()
                if report.obligation_id == item.id
            ]
            key = str(item.id)
            rows.append(
                BatchObligationRow(
                    obligation_id=item.id,
                    sender_id=item.sender_id,
                    recipient_id=getattr(item, "recipient_id", item.sender_id),
                    amount_vnd=item.amount_vnd,
                    status=obligation_status(
                        item.amount_vnd, [{"amount_vnd": amount} for amount in receipts]
                    ),
                    disputed=key in disputes,
                    disputed_reason=disputes.get(key),
                    payment_reported_at=min(claims) if claims else None,
                )
            )
        return BatchBoard(context_id=CONTEXT_ID, obligations=tuple(rows))

    def list_context_batches(self, context_id):
        # The fake keeps no clock on a batch; a fixed instant stands in, and
        # `published_at` follows the status the way the SQL row's column does.
        rows = []
        for batch in self.batches.values():
            if batch.context_id != context_id:
                continue
            ids = {obligation.id for obligation in batch.obligations}
            board = self.list_batch_obligations(batch.id)
            obligations = tuple(
                row
                for row in (() if board is None else board.obligations)
                if row.obligation_id in ids
            )
            rows.append(
                ContextBatchRow(
                    batch_id=batch.id,
                    status=batch.status,
                    created_at=FAKE_BATCH_INSTANT,
                    published_at=(
                        FAKE_BATCH_INSTANT
                        if batch.status in ("published", "collecting")
                        else None
                    ),
                    obligation_count=len(obligations),
                    confirmed_count=sum(
                        1
                        for row in obligations
                        if row.status in ("confirmed", "over_confirmed")
                    ),
                    disputed_count=sum(1 for row in obligations if row.disputed),
                    total_vnd=sum(row.amount_vnd for row in obligations),
                )
            )
        return tuple(rows)

    def get_receipt_target(self, obligation_id):
        item = self.obligations.get(obligation_id)
        if item is None:
            return None
        return ReceiptTarget(
            obligation_id=item.id,
            recipient_id=item.recipient_id,
            amount_vnd=item.amount_vnd,
        )

    def save_receipt_confirmation(
        self,
        *,
        target,
        confirmed_by_id,
        amount_vnd,
        payment_report_id,
        idempotency_key,
        now,
    ):
        del now
        existing = next(
            (
                receipt
                for receipt in self.receipts.values()
                if receipt.idempotency_key == idempotency_key
            ),
            None,
        )
        if existing is not None:
            if (
                existing.obligation_id != target.obligation_id
                or existing.confirmed_by_id != confirmed_by_id
                or existing.amount_vnd != amount_vnd
                or existing.payment_report_id != payment_report_id
            ):
                raise RepositoryConflict("IDEMPOTENCY_KEY_REUSED")
            receipt = existing
        else:
            if payment_report_id is not None:
                report = self.reports.get(payment_report_id)
                if report is None or report.obligation_id != target.obligation_id:
                    raise RepositoryConflict("PAYMENT_REPORT_NOT_FOR_OBLIGATION")
            receipt = FakeReceipt(
                id=uuid.uuid4(),
                obligation_id=target.obligation_id,
                confirmed_by_id=confirmed_by_id,
                amount_vnd=amount_vnd,
                payment_report_id=payment_report_id,
                idempotency_key=idempotency_key,
            )
            self.receipts[receipt.id] = receipt
        return ReceiptRecord(
            id=receipt.id,
            obligation_id=receipt.obligation_id,
            amount_vnd=receipt.amount_vnd,
            receipt_amounts_vnd=tuple(
                value.amount_vnd
                for value in self.receipts.values()
                if value.obligation_id == receipt.obligation_id
            ),
        )

    # --- Sổ hai người và tờ giấy (ADR-0027) -------------------------------

    def create_outing(
        self,
        *,
        context_id,
        created_by_id,
        title,
        starts_on,
        ends_on,
        headcount,
        budget_per_person_vnd,
        now,
    ):
        """Added for the sheet that becomes an outing when both agree.

        The fake had none, so `ApiService._chot` had nothing to write into and
        the one write chain in this feature that crosses into another feature
        could not be driven here at all.
        """
        record = OutingRecord(
            id=uuid.uuid4(),
            context_id=context_id,
            created_by_id=created_by_id,
            title=title,
            starts_on=starts_on,
            ends_on=ends_on,
            headcount=headcount,
            budget_per_person_vnd=budget_per_person_vnd,
            created_at=now,
            stops=(),
        )
        self.outings[record.id] = record
        self.outings_by_context[record.id] = context_id
        return record

    def get_outing(self, outing_id):
        return self.outings.get(outing_id)

    def replace_outing_stops(self, *, outing_id, stops, expected_revision=None):
        """The agreed sheet's stops written onto its outing (2026-09-23).

        Only what `_chot` needs: the stops in order, one revision up. The
        timeline's own edit routes are proved against PostgreSQL, not here.
        """
        record = self.outings[outing_id]
        written = tuple(
            OutingStopRecord(
                id=uuid.uuid4(),
                position=i,
                minute_of_day=stop["minute_of_day"],
                label=stop["label"],
                place_name=stop.get("place_name"),
                place_id=stop.get("place_id"),
            )
            for i, stop in enumerate(stops)
        )
        record = dataclasses.replace(record, stops=written, timeline_revision=record.timeline_revision + 1)
        self.outings[outing_id] = record
        return record

    def list_outings(self, context_id):
        return tuple(
            outing
            for outing in self.outings.values()
            if outing.context_id == context_id
        )

    def _live_pair_cycle(self, notebook_id):
        return next(
            (
                cycle_id
                for cycle_id, cycle in self.pair_cycles.items()
                if cycle["notebook_id"] == notebook_id and cycle["state"] != "closed"
            ),
            None,
        )

    def _pair_notebook_record(self, context_id):
        notebook = self.pair_notebooks[context_id]
        cycle_id = self._live_pair_cycle(notebook["id"])
        cycle = None if cycle_id is None else self.pair_cycles[cycle_id]
        proposals = [
            row
            for row in self.pair_proposals.values()
            if cycle_id is not None and row["cycle_id"] == cycle_id
        ]
        consents = []
        for row in proposals:
            for (proposal_id, person_id), grant in self.pair_consents.items():
                if proposal_id != row["id"]:
                    continue
                consents.append(
                    PairConsentRecord(
                        proposal_id=proposal_id,
                        person_id=person_id,
                        purpose=row["purpose"],
                        granted_at=grant["granted_at"],
                        revoked_at=grant["revoked_at"],
                        proposal_expires_at=row["expires_at"],
                        terms_version=row["terms_version"],
                    )
                )
        return PairNotebookRecord(
            id=notebook["id"],
            context_id=context_id,
            cycle_id=cycle_id,
            cycle_state=None if cycle is None else cycle["state"],
            terms_version=1 if cycle is None else cycle["terms_version"],
            participants=()
            if cycle_id is None
            else tuple(self.pair_cycle_participants.get(cycle_id, ())),
            consents=tuple(consents),
            proposals=tuple(
                PairProposalRecord(
                    id=row["id"],
                    cycle_id=row["cycle_id"],
                    purpose=row["purpose"],
                    proposed_by_id=row["proposed_by_id"],
                    terms_version=row["terms_version"],
                    completed_at=row["completed_at"],
                    created_at=row["created_at"],
                    expires_at=row["expires_at"],
                )
                for row in proposals
            ),
            constraints=tuple(
                PairConstraintRecord(
                    owner_id=key[1],
                    kind=key[2],
                    content=row["content"],
                    version=row["version"],
                    updated_at=row["updated_at"],
                )
                for key, row in sorted(
                    self.pair_constraints.items(),
                    key=lambda kv: (kv[0][1].bytes, kv[0][2]),
                )
                if cycle_id is not None and key[0] == cycle_id
            ),
        )

    def get_pair_notebook(self, context_id):
        if context_id not in self.pair_notebooks:
            return None
        return self._pair_notebook_record(context_id)

    def create_pair_notebook(self, context_id, *, now):
        self.pair_notebooks[context_id] = {"id": uuid.uuid4(), "created_at": now}
        return self._pair_notebook_record(context_id)

    def lock_pair_notebook(self, context_id):
        # No lock to take. The two-connection races are in
        # tests/postgres/test_pair_papers_races_postgres.py.
        return self.get_pair_notebook(context_id)

    def open_pair_cycle(self, notebook_id, *, participants, terms_version, now):
        cycle_id = uuid.uuid4()
        self.pair_cycles[cycle_id] = {
            "notebook_id": notebook_id,
            "state": "pending",
            "terms_version": terms_version,
            "opened_at": None,
            "closed_at": None,
            "created_at": now,
        }
        self.pair_cycle_participants[cycle_id] = list(participants)
        return cycle_id

    def activate_pair_cycle(self, cycle_id, *, now):
        cycle = self.pair_cycles.get(cycle_id)
        if cycle is None or cycle["state"] == "closed":
            return
        cycle["state"] = "active"
        cycle["opened_at"] = cycle["opened_at"] or now

    def close_pair_cycle(self, cycle_id, *, now):
        cycle = self.pair_cycles.get(cycle_id)
        if cycle is None or cycle["state"] == "closed":
            return
        cycle["state"] = "closed"
        cycle["closed_at"] = now

    def create_consent_proposal(
        self, *, cycle_id, purpose, proposed_by_id, terms_version, expires_at, now
    ):
        row = {
            "id": uuid.uuid4(),
            "cycle_id": cycle_id,
            "purpose": purpose,
            "proposed_by_id": proposed_by_id,
            "terms_version": terms_version,
            "completed_at": None,
            "created_at": now,
            "expires_at": expires_at,
        }
        self.pair_proposals[row["id"]] = row
        return PairProposalRecord(**row)

    def get_consent_proposal(self, proposal_id):
        row = self.pair_proposals.get(proposal_id)
        return None if row is None else PairProposalRecord(**row)

    def grant_consent(self, proposal_id, person_id, *, now):
        key = (proposal_id, person_id)
        existing = self.pair_consents.get(key)
        if existing is not None:
            if existing["granted_at"] is None:
                existing["granted_at"] = now
                existing["revoked_at"] = None
            return
        self.pair_consents[key] = {"granted_at": now, "revoked_at": None}

    def complete_consent_proposal(self, proposal_id, *, now):
        row = self.pair_proposals.get(proposal_id)
        if row is not None and row["completed_at"] is None:
            row["completed_at"] = now

    def revoke_consents(self, cycle_id, purpose, person_id, *, now):
        touched = 0
        for (proposal_id, who), grant in self.pair_consents.items():
            proposal = self.pair_proposals.get(proposal_id)
            if proposal is None or proposal["cycle_id"] != cycle_id:
                continue
            if proposal["purpose"] != purpose or who != person_id:
                continue
            if grant["granted_at"] is None or grant["revoked_at"] is not None:
                continue
            grant["revoked_at"] = now
            touched += 1
        return touched

    def set_couple_member(self, person_id, cycle_id, *, now):
        existing = self.active_couple_members.get(person_id)
        if existing is not None:
            if existing == cycle_id:
                return
            raise RepositoryConflict("couple_slot_taken")
        self.active_couple_members[person_id] = cycle_id

    def clear_couple_member(self, person_id):
        self.active_couple_members.pop(person_id, None)

    def couple_cycle_for(self, person_id):
        return self.active_couple_members.get(person_id)

    def set_pair_constraint(self, *, cycle_id, owner_id, kind, content, now):
        key = (cycle_id, owner_id, kind)
        row = self.pair_constraints.get(key)
        if row is None:
            row = {"content": content, "version": 1, "updated_at": now}
            self.pair_constraints[key] = row
        else:
            row["content"] = content
            row["version"] += 1
            row["updated_at"] = now
        return PairConstraintRecord(
            owner_id=owner_id,
            kind=kind,
            content=row["content"],
            version=row["version"],
            updated_at=row["updated_at"],
        )

    def delete_pair_constraint(self, cycle_id, owner_id, kind):
        return self.pair_constraints.pop((cycle_id, owner_id, kind), None) is not None

    def get_pair_rhythm(self, cycle_id, tuan):
        return self.pair_rhythms.get((cycle_id, tuan))

    def set_pair_rhythm(self, *, cycle_id, tuan, nguoi_lo_id, chon_boi_id, now):
        row = PairRhythmRecord(cycle_id=cycle_id, tuan=tuan, nguoi_lo_id=nguoi_lo_id, chon_boi_id=chon_boi_id, updated_at=now)
        self.pair_rhythms[(cycle_id, tuan)] = row
        return row

    def _pair_paper_record(self, paper_id):
        paper = self.pair_papers[paper_id]
        versions = tuple(
            PairVersionRecord(
                version=key[1],
                content=dict(row["content"]),
                ly_do=row["ly_do"],
                nguon=dict(row["nguon"]),
                author_type=row["author_type"],
                sent_at=row["sent_at"],
                sent_by=row["sent_by"],
            )
            for key, row in sorted(
                self.pair_paper_versions.items(), key=lambda kv: kv[0][1]
            )
            if key[0] == paper_id
        )
        link = self.pair_paper_outings.get(paper_id)
        return PairPaperRecord(
            id=paper_id,
            context_id=paper["context_id"],
            cycle_id=paper["cycle_id"],
            is_temporary=paper["cycle_id"] is None,
            draft_owner_id=paper["draft_owner_id"],
            state=paper["state"],
            current_version=paper["current_version"],
            tuan=paper["tuan"],
            expires_at=paper["expires_at"],
            created_at=paper["created_at"],
            done_recorded_by_id=paper["done_recorded_by_id"],
            done_recorded_at=paper["done_recorded_at"],
            outing_id=None if link is None else link[1],
            versions=versions,
            views=tuple(
                PairViewRecord(version=key[1], person_id=key[2], seen_at=seen_at)
                for key, seen_at in sorted(
                    self.pair_paper_views.items(), key=lambda kv: kv[0][1]
                )
                if key[0] == paper_id
            ),
            responses=tuple(
                PairResponseRecord(
                    version=row["version"],
                    person_id=row["person_id"],
                    kind=row["kind"],
                    created_at=row["created_at"],
                )
                for row in self.pair_paper_responses
                if row["paper_id"] == paper_id
            ),
            keeps=tuple(self.pair_paper_keeps.get(paper_id, ())),
        )

    def create_pair_paper(
        self,
        *,
        context_id,
        cycle_id,
        draft_owner_id,
        tuan,
        expires_at,
        content,
        ly_do,
        nguon,
        author_type,
        now,
    ):
        paper_id = uuid.uuid4()
        self.pair_papers[paper_id] = {
            "context_id": context_id,
            "cycle_id": cycle_id,
            "draft_owner_id": draft_owner_id,
            "state": "nhap",
            "current_version": 1,
            "tuan": tuan,
            "expires_at": expires_at,
            "created_at": now,
            "done_recorded_by_id": None,
            "done_recorded_at": None,
        }
        self.pair_paper_versions[(paper_id, 1)] = {
            "content": dict(content),
            "ly_do": ly_do,
            "nguon": dict(nguon),
            "author_type": author_type,
            "sent_at": None,
            "sent_by": None,
            "created_at": now,
        }
        return self._pair_paper_record(paper_id)

    def get_pair_paper(self, paper_id):
        if paper_id not in self.pair_papers:
            return None
        return self._pair_paper_record(paper_id)

    def lock_pair_paper(self, paper_id):
        return self.get_pair_paper(paper_id)

    def list_pair_papers(self, context_id):
        return tuple(
            self._pair_paper_record(paper_id)
            for paper_id, paper in sorted(
                self.pair_papers.items(),
                key=lambda kv: (kv[1]["created_at"], kv[0].bytes),
                reverse=True,
            )
            if paper["context_id"] == context_id
        )

    def update_pair_draft(self, paper_id, *, content, ly_do):
        row = self.pair_paper_versions.get((paper_id, 1))
        if row is None:
            return
        row["content"] = dict(content)
        row["ly_do"] = ly_do

    def add_paper_version(
        self,
        *,
        paper_id,
        version,
        content,
        ly_do,
        nguon,
        author_type,
        sent_at,
        sent_by,
        now,
    ):
        self.pair_paper_versions[(paper_id, version)] = {
            "content": dict(content),
            "ly_do": ly_do,
            "nguon": dict(nguon),
            "author_type": author_type,
            "sent_at": sent_at,
            "sent_by": sent_by,
            "created_at": now,
        }

    def mark_version_sent(self, paper_id, version, *, sent_by, now):
        row = self.pair_paper_versions.get((paper_id, version))
        if row is None or row["sent_at"] is not None:
            return
        row["sent_at"] = now
        row["sent_by"] = sent_by

    def set_paper_state(
        self, paper_id, state, *, now, current_version=None, recorded_by_id=None
    ):
        paper = self.pair_papers.get(paper_id)
        if paper is None:
            return
        paper["state"] = state
        if current_version is not None:
            paper["current_version"] = current_version
        if state == "da_di":
            paper["done_recorded_by_id"] = recorded_by_id
            paper["done_recorded_at"] = now

    def mark_paper_viewed(self, paper_id, version, person_id, *, now):
        key = (paper_id, version, person_id)
        if key in self.pair_paper_views:
            return self.pair_paper_views[key]
        self.pair_paper_views[key] = now
        return now

    def add_paper_response(self, *, paper_id, version, person_id, kind, now):
        if kind == "dong_y" and any(
            row["paper_id"] == paper_id
            and row["version"] == version
            and row["person_id"] == person_id
            and row["kind"] == "dong_y"
            for row in self.pair_paper_responses
        ):
            raise RepositoryConflict("paper_already_agreed")
        self.pair_paper_responses.append(
            {
                "paper_id": paper_id,
                "version": version,
                "person_id": person_id,
                "kind": kind,
                "created_at": now,
            }
        )

    def link_paper_outing(self, *, paper_id, version, outing_id, now):
        if paper_id in self.pair_paper_outings:
            raise RepositoryConflict("paper_outing_exists")
        self.pair_paper_outings[paper_id] = (version, outing_id)

    def get_paper_outing(self, paper_id):
        link = self.pair_paper_outings.get(paper_id)
        return None if link is None else link[1]

    def add_paper_keep(self, *, paper_id, person_id, line, now):
        keep = PairKeepRecord(
            id=uuid.uuid4(), person_id=person_id, line=line, created_at=now
        )
        self.pair_paper_keeps.setdefault(paper_id, []).append(keep)
        return keep

    def close_open_pair_papers(self, context_id, *, now):
        counts = {"bo": 0, "huy": 0}
        for paper in self.pair_papers.values():
            if paper["context_id"] != context_id:
                continue
            if paper["state"] not in (
                "nhap",
                "da_gui",
                "da_xem",
                "de_nghi_sua",
                "dong_y",
            ):
                continue
            paper["state"] = "bo" if paper["state"] == "nhap" else "huy"
            counts[paper["state"]] += 1
        return counts


class ASGITestClient:
    """Small sync facade over HTTPX's ASGI transport.

    Starlette's synchronous TestClient deadlocks with the AnyIO/Python build in
    this execution environment. The app has no lifespan hooks, so invoking the
    ASGI transport directly exercises the same request stack without hiding the
    environment issue behind a sleep or timeout.
    """

    def __init__(self, app):
        self.app = app

    def request(self, method, path, **kwargs):
        async def send():
            transport = httpx.ASGITransport(app=self.app)
            async with httpx.AsyncClient(
                transport=transport, base_url="http://testserver"
            ) as client:
                return await client.request(method, path, **kwargs)

        return anyio.run(send)

    def post(self, path, **kwargs):
        return self.request("POST", path, **kwargs)

    def get(self, path, **kwargs):
        return self.request("GET", path, **kwargs)

    def put(self, path, **kwargs):
        return self.request("PUT", path, **kwargs)

    def patch(self, path, **kwargs):
        return self.request("PATCH", path, **kwargs)

    def delete(self, path, **kwargs):
        return self.request("DELETE", path, **kwargs)


@pytest.fixture
def repository():
    """The standard cast starts out inside the standard group.

    Every test here that spends money uses `helpers.expense_payload`, whose
    default people are `SENDER_ID` and `ADVANCER_ID` in `CONTEXT_ID`. That they
    belong to that group was always the intent -- it just was not written down
    anywhere, because nothing had ever asked. Now the ledger asks, so the fake
    has to state it.

    `OTHER_ID` is deliberately **not** here. Across this suite it is the person
    standing outside: `test_context_read` and `test_context_balances` both use
    it to prove that holding a group id is not membership. Seeding it turns
    those two green for the wrong reason -- measured, not guessed: adding it
    here flips both from 403 to 200. The three tests that do want it spending
    money say so themselves via `helpers.join_group`.

    That is the rule for anything added here later: seed the id only if every
    test in the suite agrees it is an insider. A fixture that seeds every id a
    test mentions would silence the membership gate entirely -- see
    `test_expense_participants_must_be_members.py`.
    """

    repository = FakeRepository()
    for person_id in (SENDER_ID, ADVANCER_ID):
        repository.active_memberships.add((CONTEXT_ID, person_id))
    return repository


@pytest.fixture
def client(repository, monkeypatch):
    async def run_sync_inline(function, *args, **kwargs):
        del kwargs
        return function(*args)

    # This runner's Python 3.13 thread executor deadlocks even for
    # ``asyncio.to_thread(lambda: 1)``. Execute Starlette's sync adapters inline
    # for the fake-only tests; production routes remain conventional sync routes.
    monkeypatch.setattr(anyio.to_thread, "run_sync", run_sync_inline)
    # Dev, explicitly. This suite asserts the header adapter, which is the
    # mode that trusts `X-Actor-*`; `create_app()` with no argument is prod
    # now, and a suite that quietly kept the old default would have been
    # testing an adapter the product no longer ships.
    app = create_app(auth_mode="dev")
    app.dependency_overrides[get_repository] = lambda: repository
    return ASGITestClient(app)
