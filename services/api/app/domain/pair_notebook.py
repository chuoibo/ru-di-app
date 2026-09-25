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

from app.domain import interests, pair_paper

__all__ = [
    "CONSENT_PURPOSES",
    "CONSTRAINT_KINDS",
    "CYCLE_STATES",
    "NotebookError",
    "PER_PERSON_PURPOSES",
    "can_bat_doi",
    "chat_consent_active",
    "dang_cho",
    "granted_by",
    "granted_purposes",
    "gu_hai_nguoi",
    "nguoi_lo_suy",
    "vai_tuan",
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
#: `chia_gu` (ADR-0034) is not a rung both climb: each person turns it on for
#: THEMSELVES -- «let the other see my taste, and let Nếp use it here» -- and a
#: proposal for it has one answer, its proposer's. It sits in this tuple so the
#: per-person switches (`my_consents`, `their_consents_granted`) carry it.
CONSENT_PURPOSES = ("lap_so", "bat_doi", "doc_chat", "chia_gu")

#: Purposes one person decides alone (ADR-0034 §2.1).
PER_PERSON_PURPOSES = ("chia_gu",)

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
    until it is revoked. «Answered» is the proposal's `completed_at`: before
    2026-09-23 the window was applied to completed proposals too, so an agreed
    «Một đôi» would have switched itself off a week later.
    """
    if not consent.get("granted_at"):
        return False
    if consent.get("revoked_at"):
        return False
    if consent.get("proposal_completed_at") is not None:
        return True
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

    Both ON THE SAME PROPOSAL (ADR-0027: «cả hai chấp nhận cùng đề nghị»). Two
    people who each filed their own proposal for one purpose have each agreed
    with themselves; counted per purpose, that read as «both agreed» while no
    proposal had been completed, and the same count gates whether Nếp may read
    the chat (QA 23/09). A row without `proposal_id` belongs to one shared
    group, which is the old per-purpose reading for a caller that has no ids.
    """
    people = {str(p) for p in participants}
    if len(people) < 2:
        return frozenset()
    agreed: dict[tuple[str, object], set[str]] = {}
    for row in consents:
        purpose = str(row["purpose"])
        if purpose not in CONSENT_PURPOSES or not _live(row, now=now):
            continue
        who = str(row["person_id"])
        if who not in people:
            continue
        proposal = row.get("proposal_id")
        key = (purpose, None if proposal is None else str(proposal))
        agreed.setdefault(key, set()).add(who)
    return frozenset(purpose for (purpose, _), who in agreed.items() if who >= people)


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


def gu_hai_nguoi(
    consents: tuple[dict, ...] | list[dict],
    participants: tuple[str, ...] | list[str],
    toi: str,
    gu_theo_nguoi: dict[str, list[str]],
    *,
    now: datetime,
) -> dict | None:
    """What of the two tastes one person may see (ADR-0034 §2.1–2.2).

    Only in a notebook both have made «Một đôi»; `None` otherwise, because a
    taste is not something two friends' notebook shares. Within it: the other
    person's tags only if THEY turned `chia_gu` on, and the tags the two have
    in common only if BOTH did -- «common» names the other person's taste too,
    so it needs their yes as much as «theirs» does. My own switch is reported
    so the screen can offer it; my own tags are on my profile already.

    Tags come back in vocabulary order, and a stored tag the vocabulary no
    longer has is left out rather than shown as a raw id.
    """
    people = [str(p) for p in participants]
    me = str(toi)
    if not can_bat_doi(consents, people, now=now):
        return None
    other = next((p for p in people if p != me), None)
    mine_shared = "chia_gu" in granted_by(consents, me, now=now)
    theirs_shared = other is not None and "chia_gu" in granted_by(consents, other, now=now)
    their_tags = set(gu_theo_nguoi.get(other, [])) if theirs_shared and other is not None else set()
    my_tags = set(gu_theo_nguoi.get(me, []))
    theirs = [tag for tag in interests.INTEREST_IDS if tag in their_tags]
    common = [tag for tag in theirs if tag in my_tags] if mine_shared else []
    return {
        "mine_shared": mine_shared,
        "theirs_shared": theirs_shared,
        "theirs": theirs,
        "common": common,
    }


def nguoi_lo_suy(
    participants: list[str],
    to_giay: list[dict],
    *,
    cycle_id: str,
    nguoi_lap_so: str | None,
) -> dict:
    """Who tends to take the lead in this notebook (ADR-0034 §2.3–2.4).

    Read only from what the two did in THIS cycle's notebook, which both
    already hold (ADR-0027 §4): a sheet a person sent first (version 1, by a
    human) counts two, a «đề nghị sửa» they answered with counts one. Nothing
    about who they are -- no gender, no profile, no chat -- goes in.

    The highest score leads. A tie, or nothing yet, goes to whoever opened
    the notebook (`nguoi_lap_so`), and failing that to the first participant.
    `diem` is every participant's score, in participant order, so a screen can
    say why without the rule being restated there.
    """
    people = list(dict.fromkeys(str(p) for p in participants))
    diem = {p: 0 for p in people}
    for to in to_giay:
        if str(to.get("cycle_id")) != str(cycle_id):
            continue
        dau = next((v for v in to.get("versions", ()) if v.get("version") == 1), None)
        if dau is not None and dau.get("author_type") == "human" and dau.get("sent_by") is not None:
            ai = str(dau["sent_by"])
            if ai in diem:
                diem[ai] += 2
        for tl in to.get("responses", ()):
            ai = str(tl.get("person_id"))
            if tl.get("kind") == "de_nghi_sua" and ai in diem:
                diem[ai] += 1
    if not people:
        return {"nguoi_lo": [], "diem": []}
    cao = max(diem.values())
    dau_bang = [p for p in people if diem[p] == cao]
    if len(dau_bang) == 1:
        lo = dau_bang[0]
    elif nguoi_lap_so is not None and str(nguoi_lap_so) in dau_bang:
        lo = str(nguoi_lap_so)
    else:
        lo = dau_bang[0]
    return {"nguoi_lo": [lo], "diem": [[p, diem[p]] for p in people]}


def vai_tuan(suy: dict, chon: dict | None, participants: list[str]) -> dict:
    """This week's «Người lo»: what the two chose for it, or the inference.

    `chon` is the week's stored choice, `{"nguoi_lo_id": id | None}`; None
    there means «Hôm nay mình share», both lead. Choosing is not a permission:
    it decides whose turn the week reads as, nothing else (ADR-0034 §2.4).
    """
    people = list(dict.fromkeys(str(p) for p in participants))
    if chon is None:
        return {"nguoi_lo": list(suy["nguoi_lo"]), "cach": "suy", "diem": suy["diem"]}
    ai = chon.get("nguoi_lo_id")
    return {
        "nguoi_lo": people if ai is None else [str(ai)],
        "cach": "chon",
        "diem": suy["diem"],
    }


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
