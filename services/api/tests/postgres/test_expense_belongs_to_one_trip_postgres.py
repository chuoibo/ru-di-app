"""Each expense belongs to at most one trip (ADR-0054, QA UI-149).

A trip used to claim every expense dated on its days, so two trips sharing a
day both counted one dinner and the group's settlement hero printed
27.411.356đ for a 13.705.678đ ledger. Parity stayed green because both
servers double-counted alike; only a real ledger with overlapping trips shows
the difference, so it is built here, through the real service and repository.

The rule (ADR-0054 §2.2), the same for new and old rows:

1. the trip the bill was written from, which must be a trip of its group;
2. else the one trip of the group covering its Vietnam day;
3. else no trip -- two trips sharing the day are never guessed between.

Set once: a later confirmation may repeat it or leave it out, never move it.
"""

from __future__ import annotations

import importlib.util
import uuid
from datetime import UTC, date, datetime
from pathlib import Path

import pytest
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.deps import Actor
from app.api.errors import ApiProblem
from app.api.repository import SqlAlchemyApiRepository
from app.api.schemas import ExpenseConfirmationRequest, ExpenseInput
from app.api.service import ApiService
from app.db.models import (
    Context,
    Expense,
    Membership,
    MembershipRole,
    MembershipState,
    Outing,
    Person,
)

TOTAL_VND = 90_000
ROLES = frozenset({"member", "advancer"})
# The 29th in Vietnam: trips 1 and 2 both cover it.
SHARED_DAY = datetime(2030, 8, 29, 5, 0, tzinfo=UTC)
# The 10th of September: trip 3 alone covers it.
ONE_TRIP_DAY = datetime(2030, 9, 10, 5, 0, tzinfo=UTC)
TODAY = date(2030, 9, 30)


def _migration():
    path = next(
        Path(__file__).resolve().parents[2].glob(
            "app/db/migrations/versions/d5e1a7c3b902_*.py"
        )
    )
    spec = importlib.util.spec_from_file_location("khoan_chi_thuoc_mot_keo", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


class World:
    def __init__(self, session: Session) -> None:
        self.session = session
        self.payer = self._person("Người trả (dữ liệu mẫu)")
        self.friend = self._person("Bạn (dữ liệu mẫu)")
        self.group = self._group("Nhóm (dữ liệu mẫu)")
        self.other = self._group("Nhóm khác (dữ liệu mẫu)")
        self.trip1 = self._trip(self.group, "Đà Lạt (dữ liệu mẫu)", date(2030, 8, 28), date(2030, 8, 30))
        self.trip2 = self._trip(self.group, "Cà phê (dữ liệu mẫu)", date(2030, 8, 29), date(2030, 8, 29))
        self.trip3 = self._trip(self.group, "Hồ Tây (dữ liệu mẫu)", date(2030, 9, 10), date(2030, 9, 10))
        self.foreign = self._trip(self.other, "Chuyến nhóm khác (dữ liệu mẫu)", date(2030, 8, 29), date(2030, 8, 29))
        self.service = ApiService(SqlAlchemyApiRepository(session))
        self.actor = Actor(id=self.payer, roles=ROLES, context_ids=frozenset({self.group}))

    def _person(self, name: str) -> uuid.UUID:
        person = Person(id=uuid.uuid4(), display_name=name)
        self.session.add(person)
        self.session.flush()
        return person.id

    def _group(self, name: str) -> uuid.UUID:
        group = Context(id=uuid.uuid4(), display_name=name, created_by_id=self.payer)
        self.session.add(group)
        self.session.flush()
        for person in (self.payer, self.friend):
            self.session.add(
                Membership(
                    id=uuid.uuid4(),
                    context_id=group.id,
                    person_id=person,
                    state=MembershipState.ACTIVE,
                    joined_at=SHARED_DAY,
                    role=MembershipRole.MEMBER,
                )
            )
        self.session.flush()
        return group.id

    def _trip(self, group: uuid.UUID, title: str, starts: date, ends: date) -> uuid.UUID:
        trip = Outing(
            id=uuid.uuid4(),
            context_id=group,
            created_by_id=self.payer,
            title=title,
            starts_on=starts,
            ends_on=ends,
            headcount=2,
            budget_per_person_vnd=0,
        )
        self.session.add(trip)
        self.session.flush()
        return trip.id

    def proposal(self, occurred_at: datetime, outing_id: uuid.UUID | None = None) -> ExpenseInput:
        return ExpenseInput(
            context_id=self.group,
            description="Bữa tối (dữ liệu mẫu)",
            recorded_by_id=self.payer,
            paid_by_id=self.payer,
            verification_scope="totals_only",
            occurred_at=occurred_at,
            participants=sorted([self.payer, self.friend], key=lambda value: value.bytes),
            total_amount_vnd=TOTAL_VND,
            outing_id=outing_id,
        )

    def confirm(self, expense_id: uuid.UUID, proposal: ExpenseInput):
        half = TOTAL_VND // 2
        return self.service.confirm_expense(
            expense_id,
            ExpenseConfirmationRequest(
                proposal=proposal,
                expected_allocations={self.payer: half, self.friend: TOTAL_VND - half},
                acknowledge_as_advancer=True,
            ),
            self.actor,
        )

    def record(self, occurred_at: datetime, outing_id: uuid.UUID | None = None) -> uuid.UUID:
        proposal = self.proposal(occurred_at, outing_id)
        expense_id = self.service.propose_expense(proposal, self.actor).expense_id
        self.confirm(expense_id, proposal)
        return expense_id

    def trip_of(self, expense_id: uuid.UUID) -> uuid.UUID | None:
        return self.session.get(Expense, expense_id).outing_id

    def recap(self) -> dict[uuid.UUID, int]:
        records = SqlAlchemyApiRepository(self.session).group_recap(self.group, today=TODAY)
        return {r.outing.id: r.split_total_vnd for r in records}

    def ledger_total(self) -> int:
        return int(
            self.session.execute(
                text(
                    """
                    SELECT coalesce(sum(a.amount_vnd), 0)
                      FROM confirmed_allocations a
                      JOIN expense_versions v ON v.id = a.expense_version_id
                      JOIN expenses e ON e.id = v.expense_id
                     WHERE e.context_id = :g
                       AND v.version_number = (
                           SELECT max(x.version_number) FROM expense_versions x
                            WHERE x.expense_id = v.expense_id)
                    """
                ),
                {"g": self.group},
            ).scalar_one()
        )


@pytest.fixture
def world(postgres_session: Session) -> World:
    return World(postgres_session)


def test_a_bill_written_from_a_trip_belongs_to_that_trip_alone(world: World) -> None:
    expense = world.record(SHARED_DAY, world.trip2)
    assert world.trip_of(expense) == world.trip2
    recap = world.recap()
    assert recap[world.trip2] == TOTAL_VND
    assert recap[world.trip1] == 0, "the other trip sharing the day does not count it again"


def test_two_trips_sharing_its_day_claim_neither_without_a_name(world: World) -> None:
    expense = world.record(SHARED_DAY)
    assert world.trip_of(expense) is None
    recap = world.recap()
    assert recap[world.trip1] == recap[world.trip2] == 0


def test_the_one_trip_covering_its_day_claims_it(world: World) -> None:
    expense = world.record(ONE_TRIP_DAY)
    assert world.trip_of(expense) == world.trip3
    assert world.recap()[world.trip3] == TOTAL_VND


def test_the_trips_never_add_up_to_more_than_the_ledger(world: World) -> None:
    world.record(SHARED_DAY, world.trip1)
    world.record(SHARED_DAY)
    world.record(ONE_TRIP_DAY)
    world.record(SHARED_DAY, world.trip2)
    claimed = sum(world.recap().values())
    assert claimed <= world.ledger_total()
    assert claimed == 3 * TOTAL_VND, "every expense counted at most once"


@pytest.mark.parametrize("which", ["another group's trip", "a trip that does not exist"])
def test_a_trip_outside_the_group_is_refused_in_words(world: World, which: str) -> None:
    outing = world.foreign if which == "another group's trip" else uuid.uuid4()
    with pytest.raises(ApiProblem) as refused:
        world.service.propose_expense(world.proposal(SHARED_DAY, outing), world.actor)
    assert (refused.value.status_code, refused.value.code) == (422, "outing_not_in_context")


def test_the_trip_is_set_once(world: World) -> None:
    expense = world.record(SHARED_DAY, world.trip2)
    with pytest.raises(ApiProblem) as refused:
        world.confirm(expense, world.proposal(SHARED_DAY, world.trip1))
    assert (refused.value.status_code, refused.value.code) == (409, "expense_outing_mismatch")
    # Repeating it, or leaving it out, keeps it.
    world.confirm(expense, world.proposal(SHARED_DAY, world.trip2))
    world.confirm(expense, world.proposal(ONE_TRIP_DAY))
    assert world.trip_of(expense) == world.trip2


def test_the_ledger_refuses_another_groups_trip_by_itself(world: World, postgres_session: Session) -> None:
    """The composite key, not only the service: a row naming another group's trip cannot exist."""
    postgres_session.add(Expense(id=uuid.uuid4(), context_id=world.group, outing_id=world.foreign))
    with pytest.raises(Exception, match="fk_expenses_outing_context"):
        postgres_session.flush()


def test_the_migration_backfill_follows_the_same_rule(world: World, postgres_session: Session) -> None:
    shared = world.record(SHARED_DAY)
    alone = world.record(ONE_TRIP_DAY)
    outside = world.record(datetime(2030, 12, 25, 5, 0, tzinfo=UTC))
    # As rows written before the column existed.
    postgres_session.execute(text("UPDATE expenses SET outing_id = NULL WHERE context_id = :g"), {"g": world.group})
    postgres_session.execute(text(_migration().BACKFILL))
    postgres_session.expire_all()
    assert world.trip_of(alone) == world.trip3
    assert world.trip_of(shared) is None, "two trips cover the 29th: not guessed"
    assert world.trip_of(outside) is None, "no trip covers the 25th of December"
