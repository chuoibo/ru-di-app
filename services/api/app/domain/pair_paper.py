"""The sheet of paper two people pass each other, as rules over plain dicts.

Spec «Nếp truyền giấy» section 3 and ADR-0027. Thirteen states, one transition
function, and three questions the service is not allowed to answer for itself:
is this sheet still in force, have both people agreed to THIS version, and may
the sender still take it back.

## Why the agreement is a row and not an inference

Section 3.4 says a person who presses send has agreed to what they sent. The
service could infer that at read time from `sent_by`, and the first draft of
this module did. It is written down instead: sending writes an explicit
`dong_y` response for the sender, and `da_du_dong_y` counts rows. Two reasons.
An inference has to be repeated identically in every reader, and a sheet Nếp
sent has no sender to infer from -- the rule «a sheet Nếp sent needs both
people» (ADR-0027) then falls out of counting rather than out of a second
branch somebody must remember.

## Why expiry is a read, not a state

Nothing in this product runs after a response (ADR-0024 describes an
`AfterResponse` hook that was never built; the only deferred work is a script).
So a week cannot «become» expired at midnight -- there is nobody to write the
row. `hieu_luc` is what every reader applies: past the deadline, an undecided
sheet reads `het_han`, and it never reads `chot`. Silence is not consent
(section 3.3 rule 2), and this is where that is enforced rather than promised.

Pure functions over dicts. No I/O, no ORM, no framework.
"""

from __future__ import annotations

import re
from datetime import UTC, date, datetime, timedelta
from zoneinfo import ZoneInfo

from app.domain import interests

__all__ = [
    "AUTHOR_TYPES",
    "MUI_GIO",
    "OPEN_STATES",
    "PAPER_STATES",
    "PLAN_STATES",
    "RESPONSE_KINDS",
    "TERMINAL",
    "PaperError",
    "chuyen",
    "co_the_rut",
    "da_du_dong_y",
    "han_tuan",
    "hieu_luc",
    "lam_giau_phac",
    "lam_giau_theo_gu",
    "gu_cho_nep",
    "loai_theo_gu",
    "ngay_de_xuat",
    "phac_to_giay",
    "tuan_cua",
]

#: The week is the one the two of them live in, not the one the server's
#: machine is in. Vietnam has kept a single offset since 1975, but the zone is
#: named rather than written as `+07:00` so that this and the repository's
#: `timezone('Asia/Ho_Chi_Minh', ...)` are provably the same clock.
MUI_GIO = ZoneInfo("Asia/Ho_Chi_Minh")

#: Section 5.1: a sheet belongs to a week, and Saturday is the day it proposes.
#: Kept as a number rather than a name because `date.weekday()` is what reads
#: it, and a name would need translating at every use.
_THU_BAY = 5

#: Section 3.1, in the order the week walks them. The CHECK constraint on
#: `pair_papers.state` is the second spelling of this tuple, and the client's
#: `TRANG_THAI_TO` is the third.
PAPER_STATES = (
    "nhap",
    "da_gui",
    "da_xem",
    "de_nghi_sua",
    "dong_y",
    "chot",
    "da_di",
    "da_giu",
    "nghi_tuan",
    "het_han",
    "rut",
    "bo",
    "huy",
)

#: Still being decided: these are the states a deadline can end (section 3.3).
OPEN_STATES = ("nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y")

#: A plan exists. Frozen for editing (ADR-0027: no v+1 after `chot`), but not
#: finished: `da_di` still becomes `da_giu` when somebody keeps a line.
PLAN_STATES = ("chot", "da_di")

#: The week is closed one way or another. `da_giu` is the good ending.
TERMINAL = ("nghi_tuan", "het_han", "rut", "bo", "huy", "da_giu")

AUTHOR_TYPES = ("human", "nep")
RESPONSE_KINDS = ("dong_y", "de_nghi_sua")


class PaperError(Exception):
    """A refusal with the code the wire uses, lowercase and stable."""

    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def tuan_cua(now: datetime) -> date:
    """The Monday of the week `now` falls in, in Vietnam.

    One sheet per week (section 5.1), so the week has to be a value a unique
    index can hold, and the same instant must land in the same week for both
    people no matter where their phones think they are.
    """
    here = now.astimezone(MUI_GIO)
    return (here - timedelta(days=here.weekday())).date()


def han_tuan(now: datetime) -> datetime:
    """When a sheet drafted at `now` stops being answerable.

    Midnight ending Sunday, local. The bound is exclusive and `hieu_luc` reads
    it with `>=`, so «the week is over» and «the next week has begun» are one
    instant rather than two adjacent ones with a second of nothing between.

    The arithmetic is local and the answer is UTC. Handing back the local
    datetime put a `+07:00` deadline on a wire where every other timestamp ends
    in `Z`, which is two spellings of one instant for a client to get wrong.
    """
    here = now.astimezone(MUI_GIO)
    monday = (here - timedelta(days=here.weekday())).replace(
        hour=0, minute=0, second=0, microsecond=0
    )
    return (monday + timedelta(days=7)).astimezone(UTC)


def ngay_de_xuat(now: datetime) -> date:
    """The day a fresh sheet proposes: this week's Saturday, or today if
    Saturday has already gone.

    A draft that offers a date in the past is worse than a blank one: the
    person has to notice and fix it before they can use it, and the one thing
    a pre-filled sheet is for is not making them start from nothing.
    """
    here = now.astimezone(MUI_GIO)
    today = here.date()
    saturday = tuan_cua(now) + timedelta(days=_THU_BAY)
    return saturday if saturday >= today else today


def hieu_luc(paper: dict, *, now: datetime) -> str:
    """The state a reader sees, with the deadline applied.

    Only an undecided sheet can run out of time. A plan that stands, a memory,
    and a week already closed all read as themselves forever: a deadline that
    could rewrite `chot` would be a way for silence to cancel an agreement,
    which is the mirror of the rule it is supposed to serve.
    """
    state = paper["state"]
    if state not in OPEN_STATES:
        return state
    expires_at = paper.get("expires_at")
    if expires_at is not None and now >= expires_at:
        return "het_han"
    return state


def da_du_dong_y(responses: tuple[dict, ...] | list[dict], version: int) -> bool:
    """Two different people have agreed to the SAME version.

    Agreement never travels to a later version (section 3.3 rule 3), so the
    version is part of the question, not a detail of the answer.
    """
    agreed = {
        str(row["person_id"])
        for row in responses
        if row["kind"] == "dong_y" and int(row["version"]) == version
    }
    return len(agreed) >= 2


def co_the_rut(
    paper: dict,
    versions: tuple[dict, ...] | list[dict],
    views: tuple[dict, ...] | list[dict],
    responses: tuple[dict, ...] | list[dict],
    *,
    actor_id: str,
) -> bool:
    """May `actor_id` take this sheet back (ADR-0027)?

    Only the sender, only while the sheet is `da_gui`, and only while the other
    person has neither opened it nor answered. After a look there is something
    to talk about, and the way out is a new version rather than a disappearance:
    a sheet that can vanish from the other person's screen after they read it
    is a sheet they cannot trust they read.

    The sender's own rows do not count against them. Sending writes the
    sender's `dong_y`, so «no response» has to mean «none from anybody else».
    """
    if paper["state"] != "da_gui":
        return False
    version = int(paper["current_version"])
    current = next(
        (v for v in versions if int(v["version"]) == version),
        None,
    )
    if current is None or current.get("author_type") != "human":
        # A sheet Nếp sent has no human sender to withdraw it.
        return False
    if str(current.get("sent_by") or "") != actor_id:
        return False
    seen_by_other = any(
        int(row["version"]) == version and str(row["person_id"]) != actor_id
        for row in views
    )
    answered_by_other = any(
        int(row["version"]) == version and str(row["person_id"]) != actor_id
        for row in responses
    )
    return not seen_by_other and not answered_by_other


#: What each event may be applied to. Read with section 3.3's table: the third
#: column there is `hieu_luc`, not a transition, which is why no event names it.
_TU: dict[str, tuple[str, ...]] = {
    "gui": ("nhap",),
    "xem": ("da_gui", "da_xem"),
    "dong_y": ("da_gui", "da_xem", "dong_y"),
    "de_nghi_sua": ("da_gui", "da_xem", "dong_y"),
    "rut": ("da_gui",),
    "nghi_tuan": OPEN_STATES,
    "bo": ("nhap",),
    "da_di": ("chot",),
    "giu": ("da_di", "da_giu"),
    "huy": ("chot",),
}


def chuyen(paper: dict, su_kien: str, *, now: datetime, **facts) -> dict:
    """The sheet after `su_kien`, or a `PaperError` naming the refusal.

    The service does the writing; this says what the write is allowed to be.
    Keeping it here rather than in the service is what lets every one of these
    rules be proven without a database, and it is the only place the thirteen
    states are joined up.

    `facts` are things only the caller can know, already proved from stored
    rows: `du_dong_y` (both agreed to the current version), `co_the_rut`, and
    `nguoi_ghi` (somebody recorded the outing happened).
    """
    if su_kien not in _TU:
        raise PaperError("paper_event_unknown")
    state = hieu_luc(paper, now=now)
    if state == "het_han" and paper["state"] in OPEN_STATES:
        # The deadline has passed and nobody noticed until this write. Say so
        # with the code the client already knows, rather than «wrong state»:
        # the person did nothing wrong, the week did.
        raise PaperError("paper_expired")
    if su_kien == "de_nghi_sua" and state in PLAN_STATES:
        # ADR-0027: after `chot` the versions are frozen. A counter-proposal
        # would silently rewrite a plan both people already agreed to.
        raise PaperError("paper_frozen")
    if state not in _TU[su_kien]:
        raise PaperError("paper_wrong_state")

    if su_kien == "gui":
        return {**paper, "state": "da_gui"}
    if su_kien == "xem":
        return {**paper, "state": "da_xem"}
    if su_kien == "dong_y":
        if facts.get("du_dong_y"):
            return {**paper, "state": "chot"}
        return {**paper, "state": "dong_y"}
    if su_kien == "de_nghi_sua":
        # A counter-proposal is a NEW version, sent by whoever proposed it, so
        # the sheet goes back to «sent, waiting» with the number moved on.
        return {
            **paper,
            "state": "da_gui",
            "current_version": int(paper["current_version"]) + 1,
        }
    if su_kien == "rut":
        if not facts.get("co_the_rut"):
            raise PaperError("paper_not_withdrawable")
        return {**paper, "state": "rut"}
    if su_kien == "nghi_tuan":
        # Section 3.3's table: a draft nobody received is simply dropped for
        # the week; a sheet already sent is cancelled, because the other person
        # has seen something and is owed an ending rather than a silence.
        return {**paper, "state": "nghi_tuan" if state == "nhap" else "huy"}
    if su_kien == "bo":
        return {**paper, "state": "bo"}
    if su_kien == "da_di":
        if not facts.get("nguoi_ghi"):
            # Section 3.1: `da_di` carries who recorded it. Never inferred from
            # the date or from a location -- a week that passed is not a
            # promise that anybody went.
            raise PaperError("paper_needs_recorder")
        return {**paper, "state": "da_di"}
    if su_kien == "giu":
        return {**paper, "state": "da_giu"}
    return {**paper, "state": "huy"}


def phac_to_giay(
    routine: dict, rang_buoc: tuple[dict, ...] | list[dict], *, now: datetime
) -> dict:
    """What Nếp puts on the paper when it is the notebook's turn to open.

    A fixed template over facts the notebook already holds, and no model call:
    section 8 says a draft says «chưa biết», never «chắc hợp». So every stop
    comes back with `place_id: null` and `can_kiem: true`, and the reason line
    states the observation that prompted it rather than a recommendation.

    Shape, one main stop and an optional next one (section 5.2). Three stops
    were the first design, because the paper folds in three; that let the
    stationery decide how an evening goes.

    `nguon` is the provenance ADR-0019 asks for: which sources this draft was
    built from. Only shared ones appear here, and the caller is what keeps a
    private source out -- this function cannot reach one.
    """
    ngay = routine["ngay"]
    if not isinstance(ngay, date):
        raise PaperError("paper_draft_needs_date")
    chang: list[dict] = [
        {
            "gio": str(routine["gio"]),
            "viec": str(routine["viec"]),
            "place_id": None,
            "can_kiem": True,
        }
    ]
    tiep = routine.get("di_tiep")
    if tiep:
        chang.append(
            {
                "gio": str(tiep["gio"]),
                "viec": str(tiep["viec"]),
                "place_id": None,
                "can_kiem": True,
            }
        )
    return {
        "content": {"ngay": ngay.isoformat(), "chang": chang},
        "ly_do": str(routine.get("ly_do") or ""),
        "nguon": {
            "scope": "chung",
            "dung": ["routine", *(["rang_buoc"] if rang_buoc else [])],
            "luc": now.isoformat(),
        },
    }


#: The longest reason line a person may write (`PaperDraftEditRequest.ly_do`);
#: Nếp's draft keeps to it too, so editing it never starts over the limit.
_DAI_LY_DO = 200

#: What a draft calls the stop at a place of this catalogue kind. Anything not
#: listed keeps the routine's own line («Ăn tối»).
_VIEC_THEO_LOAI = {"cafe": "Cà phê", "vui-choi": "Đi chơi", "di-choi-dem": "Đi chơi tối"}

#: The catalogue kind a taste points at (ADR-0034 §2.2), for the tastes the
#: catalogue has a kind for: `mon-local` and `outdoor` are read off words and
#: traits, not a kind, and shopping/karaoke have nothing to read at all.
_LOAI_THEO_GU = {"an-uong": "quan-an-local", "cafe": "cafe", "nightlife": "di-choi-dem", "game": "vui-choi"}

#: Which of a constraint's characters are folded to lower case: ASCII and the
#: Latin blocks Vietnamese is written in. Folding stops there on purpose, so
#: the Go port can fold the same code points the same way; U+0130 is the one
#: letter in these blocks whose lower case is two code points, and it is kept.
_KHOI_GAP = ((0x41, 0x5A), (0xC0, 0x24F), (0x1E00, 0x1EFF))


def _gap(chu: str) -> str:
    return "".join(
        c.lower()
        if any(dau <= ord(c) <= cuoi for dau, cuoi in _KHOI_GAP) and len(c.lower()) == 1
        else c
        for c in chu
    )


def _cum_tu_cam(rang_buoc) -> list[str]:
    """The phrases of the two boxes, one per comma, semicolon, slash or line."""
    out = []
    for rb in rang_buoc:
        for manh in re.split(r"[,;/\n]", str(rb.get("content") or "")):
            cum = _gap(manh).strip(" \t\r")
            if len(cum) >= 2:
                out.append(cum)
    return out


def _pham(row: dict, cam: list[str]) -> bool:
    chu = [str(row.get("name") or ""), str(row.get("category") or "")]
    chu += [k for k in row.get("kinds") or [] if isinstance(k, str)]
    chu += [t for t in row.get("traits") or [] if isinstance(t, str)]
    gap = [_gap(c) for c in chu]
    return any(c in g for c in cam for g in gap)


def _cau_vua(lua_chon: list[list[str]]) -> str | None:
    """The first of these sentence lists that fits the reason line, joined."""
    for cau in lua_chon:
        noi = " ".join(c for c in cau if c)
        if noi and len(noi) <= _DAI_LY_DO:
            return noi
    return None


def gu_cho_nep(
    nguoi_chia: list[str],
    gu_theo_nguoi: dict[str, list[str]],
    ten_theo_nguoi: dict[str, str],
    *,
    ca_hai: bool,
) -> list[dict]:
    """The tastes Nếp may use, most shared first (ADR-0034 §2.2).

    `nguoi_chia` are the people who turned `chia_gu` on, in participant order;
    nobody else's taste is in `gu_theo_nguoi` for this to read. What both have
    and both shared (`ca_hai`) comes first, as one taste of both; then each
    sharer's own, in turn. Vocabulary order inside each group, and a tag the
    vocabulary no longer has is dropped.
    """
    theo = {p: [t for t in interests.INTEREST_IDS if t in set(gu_theo_nguoi.get(p, []))] for p in nguoi_chia}
    out: list[dict] = []
    chung: list[str] = []
    if ca_hai and len(nguoi_chia) == 2:
        a, b = nguoi_chia
        chung = [t for t in theo[a] if t in theo[b]]
        out += [{"tag": t, "chung": True, "ten": None, "nguoi": list(nguoi_chia)} for t in chung]
    for p in nguoi_chia:
        out += [
            {"tag": t, "chung": False, "ten": ten_theo_nguoi.get(p) or "Người ấy", "nguoi": [p]}
            for t in theo[p]
            if t not in chung
        ]
    return out


def loai_theo_gu(gu: list[dict]) -> str | None:
    """The catalogue kind of the first usable taste in `gu`, or None."""
    for muc in gu:
        loai = _LOAI_THEO_GU.get(str(muc["tag"]))
        if loai is not None:
            return loai
    return None


def lam_giau_theo_gu(
    phac: dict,
    *,
    gu: list[dict],
    ung_vien: list[dict],
    da_di: list[str],
    rang_buoc,
) -> dict:
    """Nếp's draft, told what the two like -- only what they chose to share.

    `gu` is the service's list, most shared first: each `{"tag", "chung",
    "ten", "nguoi"}` is one taste, `chung` when both have it and shared it,
    otherwise one sharer's (`ten` names them); `nguoi` are the ids whose taste
    it is. A taste nobody shared is never in it (ADR-0034 §2.1), so this reads
    nothing it was not given.

    The history comes first: a draft that already proposes a place (from the
    kind the two chose last time) is returned unchanged. Otherwise the first
    taste the catalogue has a kind for picks the kind: a place of it in
    `ung_vien` they have not been to, the best rated, not meeting a phrase of
    the two boxes, becomes the main stop; with no such place the stop is only
    named after the kind. The reason line says whose taste it was.
    """
    dau = phac["content"]["chang"][0]
    if dau.get("place_id"):
        return phac
    muc = next((m for m in gu if str(m["tag"]) in _LOAI_THEO_GU), None)
    if muc is None:
        return phac
    loai = _LOAI_THEO_GU[str(muc["tag"])]
    nhan = next(t.label for t in interests.INTEREST_TAGS if t.id == muc["tag"])
    ai = "Hai bạn cùng thích" if muc["chung"] else f"{muc['ten']} thích"
    cam = _cum_tu_cam(rang_buoc)
    chon, hang = None, None
    for row in ung_vien:
        if row.get("category") != loai or row["id"] in da_di or _pham(row, cam):
            continue
        khoa = (
            -1.0 if row.get("rating") is None else float(row["rating"]),
            -1 if row.get("rating_count") is None else int(row["rating_count"]),
        )
        if chon is None or khoa > hang:
            chon, hang = row, khoa
    truoc = str(phac["ly_do"])
    moi = {**dau, "viec": _VIEC_THEO_LOAI.get(loai, dau["viec"])}
    ly_do = None
    if chon is not None:
        kiem = (
            "Đã tránh chỗ trùng chữ trong hai ô ràng buộc; món thì hai bạn kiểm lại."
            if cam
            else "Chỗ này chưa ai kiểm, hai bạn xem lại."
        )
        thu = f"{ai} {nhan}: thử {chon['name']}, hai bạn chưa đi."
        thu_ngan = f"Thử {chon['name']}, hai bạn chưa đi."
        ly_do = _cau_vua([[truoc, thu, kiem], [thu, kiem], [thu_ngan, kiem]])
        if ly_do is not None:
            moi["place_id"] = str(chon["id"])
    if ly_do is None:
        moi = {**dau, "viec": _VIEC_THEO_LOAI.get(loai, dau["viec"])}
        cau = f"{ai} {nhan}, nên Nếp phác theo đó."
        ly_do = _cau_vua([[truoc, cau], [cau]]) or truoc
    dung = list(phac["nguon"]["dung"])
    them = [f"gu:{nguoi}" for nguoi in muc["nguoi"]]
    if dung and dung[-1] == "rang_buoc":
        dung = [*dung[:-1], *them, "rang_buoc"]
    else:
        dung = [*dung, *them]
    return {
        "content": {**phac["content"], "chang": [moi, *phac["content"]["chang"][1:]]},
        "ly_do": ly_do,
        "nguon": {**phac["nguon"], "dung": dung},
    }


def lam_giau_phac(
    phac: dict,
    *,
    lich_su: list[dict],
    cho_cu: dict | None,
    ung_vien: list[dict],
    rang_buoc,
) -> dict:
    """Nếp's template, told what this notebook already knows.

    The template alone said «18:30 · Ăn tối» every week (QA 23/09): it read
    none of what the two had shared. What it may read here is only what both
    of them already hold (ADR-0027 §4: agreeing to keep a notebook is agreeing
    to keep its cycle's history):

    - `lich_su`, the contents of this cycle's agreed sheets, newest first. The
      newest one's hour is the hour they keep; its place, if the catalogue
      still has it (`cho_cu`), is the kind of place they chose.
    - `ung_vien`, the catalogue's places of that same kind in that same city.
      One they have not been to is proposed -- the best rated, the earlier row
      on a tie -- unless its words meet a phrase of the two boxes.

    The boxes are matched on words only. The catalogue does not prove what a
    dish contains (ADR-0027 §7), so the stop stays `can_kiem` and the reason
    says what was and was not checked. Nothing here is a model call, and a
    notebook with no agreed sheet gets the template back unchanged.
    """
    lich_su = [nd for nd in lich_su if nd.get("chang")]
    if not lich_su:
        return phac
    chinh = lich_su[0]["chang"][0]
    gio = str(chinh["gio"])
    goc = str(phac["ly_do"])
    noi_cu = cho_cu is not None and cho_cu["id"] == chinh.get("place_id")
    if noi_cu:
        truoc = f"Lần trước hai bạn hẹn {gio} ở {cho_cu['name']}; Nếp giữ giờ đó."
    else:
        truoc = f"Lần trước hai bạn hẹn {gio}, «{chinh['viec']}»; Nếp giữ giờ đó."
    truoc_ngan = f"Lần trước hai bạn hẹn {gio}; Nếp giữ giờ đó."
    dau = {**phac["content"]["chang"][0], "gio": gio}
    them = ["lich_su"]
    chon = None
    if cho_cu is not None:
        da_di = {
            c["place_id"] for nd in lich_su for c in nd["chang"] if c.get("place_id")
        }
        cam = _cum_tu_cam(rang_buoc)
        hang = None
        for row in ung_vien:
            if row["id"] in da_di or _pham(row, cam):
                continue
            khoa = (
                -1.0 if row.get("rating") is None else float(row["rating"]),
                -1 if row.get("rating_count") is None else int(row["rating_count"]),
            )
            if chon is None or khoa > hang:
                chon, hang = row, khoa
    # A proposed place is never said without what was and was not checked:
    # the history sentence gives way first, and a place whose name leaves no
    # room for the check is not proposed at all.
    ly_do = None
    if chon is not None:
        kiem = (
            "Đã tránh chỗ trùng chữ trong hai ô ràng buộc; món thì hai bạn kiểm lại."
            if cam
            else "Chỗ này chưa ai kiểm, hai bạn xem lại."
        )
        # The kind is named after the place it came from; when the newest
        # sheet already named that place, «chỗ đó» points back at it.
        thu = (
            f"Thử {chon['name']}, cùng kiểu chỗ đó mà hai bạn chưa đi."
            if noi_cu
            else f"Thử {chon['name']}: cùng kiểu {cho_cu['name']}, hai bạn chưa đi."
        )
        thu_ngan = f"Thử {chon['name']}, chỗ hai bạn chưa đi."
        ly_do = _cau_vua(
            [
                [goc, truoc, thu, kiem],
                [truoc, thu, kiem],
                [truoc_ngan, thu_ngan, kiem],
                [thu_ngan, kiem],
            ]
        )
        if ly_do is not None:
            dau["place_id"] = str(chon["id"])
            dau["viec"] = _VIEC_THEO_LOAI.get(str(chon.get("category")), dau["viec"])
            them.append("danh_muc")
    if ly_do is None:
        ly_do = _cau_vua([[goc, truoc], [truoc], [truoc_ngan]]) or ""
    dung = list(phac["nguon"]["dung"])
    return {
        "content": {**phac["content"], "chang": [dau, *phac["content"]["chang"][1:]]},
        "ly_do": ly_do,
        "nguon": {**phac["nguon"], "dung": [dung[0], *them, *dung[1:]]},
    }
