"""The two-person notebook: consent, its cycle, and what closing it costs.

Spec «Nếp truyền giấy» sections 6 and 7, ADR-0027 K1, K2, K6.

## Why consent is a proposal plus grants, and not a level

The first design had `consent_level: int`, and a number is exactly the shape
that lets a later feature quietly qualify: 2 means «notebook», 3 means «couple»,
and the code that reads `>= 2` has already granted the thing 3 means. Here a
consent is (purpose, person, terms version, cycle), each purpose is asked for
separately, and a lower rung never implies a higher one. Reading a grant is
reading a row that names what was agreed to.

## Why a cycle

Closing the notebook does not delete it (section 7.6: «đóng sổ là đóng» --
sheets that were open are cancelled, plans that stood stay readable). Opening
another one later is a new agreement, not a revival of the old, so consent is
scoped to a cycle and a closed cycle's grants never apply to the next one.

Pure functions over dicts. No I/O, no ORM, no framework.
"""

from __future__ import annotations

import hashlib
from datetime import datetime, timedelta

from app.domain import pair_paper

__all__ = [
    "CONSENT_PURPOSES",
    "CONSTRAINT_KINDS",
    "CYCLE_STATES",
    "NotebookError",
    "can_bat_doi",
    "chat_consent_active",
    "dang_cho",
    "granted_by",
    "granted_purposes",
    "han_de_nghi",
    "xem_truoc_dong_so",
]

#: How long an unanswered offer stands. A week, so that «I will think about it»
#: survives one of them being away, and so that an offer nobody answered stops
#: being an offer rather than sitting in the notebook forever waiting to be
#: accepted by somebody who has forgotten it was asked.
HAN_DE_NGHI = timedelta(days=7)

#: `pending` until both have granted `lap_so`; `closed` is final for that
#: cycle. The CHECK on `pair_notebook_cycles.state` spells this again.
CYCLE_STATES = ("pending", "active", "closed")

#: The consent ladder of section 6.1, tier 2 upward. Tier 1 («nhận lời đi
#: chơi») is the invitation itself and has no row. `doc_chat` is tier 4 and is
#: off until somebody turns it on: there is no default that reads a chat.
CONSENT_PURPOSES = ("lap_so", "bat_doi", "doc_chat")

#: The two shared constraints of section 6.4. Two, not a free list: a list
#: grows into a profile, and this is meant to stay the smallest thing that
#: keeps a draft from being wrong about somebody.
CONSTRAINT_KINDS = ("khong_an_duoc", "dung")


class NotebookError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def han_de_nghi(now: datetime) -> datetime:
    """When an offer made at `now` lapses."""
    return now + HAN_DE_NGHI


def _live(consent: dict, *, now: datetime | None) -> bool:
    """A grant that is still in force: granted, not revoked, not expired.

    The expiry is the PROPOSAL's, not the grant's. An offer nobody answered
    within its window stops being an offer; a grant that was answered stands
    until it is revoked.
    """
    if not consent.get("granted_at"):
        return False
    if consent.get("revoked_at"):
        return False
    expires_at = consent.get("proposal_expires_at")
    if expires_at is not None and now is not None and now >= expires_at:
        return False
    return True


def granted_by(
    consents: tuple[dict, ...] | list[dict],
    person_id: str,
    *,
    now: datetime | None = None,
) -> frozenset[str]:
    """What ONE person has granted and not taken back.

    Never the answer to «may this happen» -- that is `granted_purposes`, and
    both people are part of it. This is only for showing a person their own
    switches, and for showing each of them which of the two has not answered
    yet: a screen that said «đang chờ» without saying whose turn it is leaves
    both of them waiting for the other.
    """
    who = str(person_id)
    return frozenset(
        str(row["purpose"])
        for row in consents
        if str(row["person_id"]) == who
        and str(row["purpose"]) in CONSENT_PURPOSES
        and _live(row, now=now)
    )


def granted_purposes(
    consents: tuple[dict, ...] | list[dict],
    participants: tuple[str, ...] | list[str],
    *,
    now: datetime | None = None,
) -> frozenset[str]:
    """Purposes BOTH participants have granted and neither has taken back.

    «Both» is the whole rule and it is why this returns a set rather than a
    per-person map: nothing in a two-person notebook is unlocked by one person
    agreeing with themselves.
    """
    people = {str(p) for p in participants}
    if len(people) < 2:
        return frozenset()
    return frozenset.intersection(
        *(granted_by(consents, person, now=now) for person in people)
    )


def can_bat_doi(
    consents: tuple[dict, ...] | list[dict],
    participants: tuple[str, ...] | list[str],
    *,
    now: datetime | None = None,
) -> bool:
    """Both people have said yes to «Một đôi» (tier 3).

    Tier 2 does not imply this and never will: the database says the same
    thing a second way, with a deferred trigger over `active_couple_members`,
    because this is the rung that makes a person's notebook exclusive.
    """
    return "bat_doi" in granted_purposes(consents, participants, now=now)


def chat_consent_active(
    consents: tuple[dict, ...] | list[dict],
    participants: tuple[str, ...] | list[str],
    *,
    now: datetime,
) -> bool:
    """May Nếp read this pair's messages (tier 4)?

    Asked at the moment of every read, never cached: ADR-0027 says consent is
    checked at every boundary, and a revocation that only takes effect on the
    next restart is not a revocation. The companion asks this before it opens
    the conversation, not after.
    """
    return "doc_chat" in granted_purposes(consents, participants, now=now)


def _revision(rows: list[str]) -> str:
    """A short digest of exactly what the preview counted.

    The point is not secrecy, it is that «close» must refuse a preview the
    person read before somebody else changed something: the count they agreed
    to and the rows being closed have to be the same rows. A digest of the
    (id, state) pairs is the cheapest honest way to say «still these».
    """
    material = "\n".join(sorted(rows)).encode("utf-8")
    return hashlib.sha256(material).hexdigest()[:16]


def xem_truoc_dong_so(
    papers: tuple[dict, ...] | list[dict],
    proposals: tuple[dict, ...] | list[dict],
    *,
    now: datetime,
) -> dict:
    """What closing the notebook would do, counted, with a revision to pin it.

    Three numbers, not two. A draft only its owner has seen is DROPPED, a sheet
    waiting for an answer is CANCELLED, and a plan that stands is LOCKED and
    stays readable. The first cut of the client folded the first two together
    and said «1 tờ sẽ khoá» about a draft it then showed as «Đã bỏ» on the very
    next screen; a person reading a destructive confirmation caught it at once
    (blind read, Phase 2). The wire carries all three for that reason.
    """
    nhap_bo = 0
    to_huy = 0
    to_khoa = 0
    material: list[str] = []
    for paper in papers:
        state = pair_paper.hieu_luc(paper, now=now)
        material.append(f"{paper['id']}:{state}")
        if state == "nhap":
            nhap_bo += 1
        elif state in pair_paper.OPEN_STATES:
            to_huy += 1
        elif state in pair_paper.PLAN_STATES:
            to_khoa += 1
    cho = [p for p in proposals if dang_cho(p, now=now)]
    material.extend(f"dn:{p['id']}" for p in cho)
    return {
        "revision": _revision(material),
        "so_nhap_bo": nhap_bo,
        "so_to_huy": to_huy,
        "so_to_khoa": to_khoa,
        "so_de_nghi_huy": len(cho),
    }


def dang_cho(proposal: dict, *, now: datetime) -> bool:
    """An offer still waiting: nobody has completed it and it has not lapsed.

    Public because two readers ask it -- the preview counts offers that closing
    would cancel, and the notebook screen lists the ones a person can still
    answer. Two spellings of «still waiting» is how one screen shows a button
    for something the other has already given up on.
    """
    if proposal.get("completed_at"):
        return False
    expires_at = proposal.get("expires_at")
    if expires_at is not None and now >= expires_at:
        return False
    return True
