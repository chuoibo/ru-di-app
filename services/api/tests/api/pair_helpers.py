"""Two people, one pair conversation, and a clock the test moves.

Shared by `test_pair_notebook.py` and `test_pair_papers.py` because both need
the same three sentences of setup and the same two-step consent, and a second
spelling of «open the notebook» is how two files start proving different rules
by accident.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

from app.api.repository import ContextRecord, PersonRecord

from .helpers import actor_headers

# Letters interleaved: the repo guard reads nine consecutive digits as an
# account number and blocks the commit.
TOI = uuid.UUID("aa11aa11-0a0a-4a0a-8a0a-0a0a0a0a0a11")
NGUOI_KIA = uuid.UUID("bb22bb22-0b0b-4b0b-8b0b-0b0b0b0b0b22")
NGUOI_LA = uuid.UUID("cc33cc33-0c0c-4c0c-8c0c-0c0c0c0c0c33")
CAP = uuid.UUID("dd44dd44-0d0d-4d0d-8d0d-0d0d0d0d0d44")
HOI = uuid.UUID("ee55ee55-0e0e-4e0e-8e0e-0e0e0e0e0e55")

#: A Wednesday, noon in Vietnam. Chosen so the week's Saturday is still ahead:
#: the sheet a fresh draft proposes has to be a day somebody could still go on.
T0 = datetime(2030, 9, 18, 5, tzinfo=UTC)
TUAN = "2030-09-16"
THU_BAY = "2030-09-21"
#: Midnight ending Sunday, local, as UTC. `hieu_luc` reads it with `>=`.
HAN_TUAN = "2030-09-22T17:00:00Z"
TUAN_SAU = timedelta(days=8)


def seed_pair(repository):
    """Three people, one pair with two of them, and one group with a stranger."""
    for pid, name in (
        (TOI, "Tôi"),
        (NGUOI_KIA, "Người Ấy"),
        (NGUOI_LA, "Người Lạ"),
    ):
        repository.people[pid] = PersonRecord(id=pid, display_name=name, created_at=T0)
    repository.contexts[CAP] = ContextRecord(
        id=CAP,
        display_name="Người Ấy",
        created_by_id=TOI,
        created_at=T0,
        kind="pair",
        pair_key=f"{min(TOI.hex, NGUOI_KIA.hex)}:{max(TOI.hex, NGUOI_KIA.hex)}",
    )
    repository.active_memberships |= {(CAP, TOI), (CAP, NGUOI_KIA)}
    # A group, so «that is not a pair» has something to be asked about.
    repository.contexts[HOI] = ContextRecord(
        id=HOI, display_name="Hội bạn", created_by_id=TOI, created_at=T0
    )
    repository.active_memberships |= {(HOI, TOI), (HOI, NGUOI_LA)}


def head(actor):
    return actor_headers(actor_id=actor, roles="member")


def de_nghi(client, purpose, *, actor=TOI, context=CAP):
    return client.post(
        f"/contexts/{context}/notebook/proposals",
        json={"purpose": purpose},
        headers=head(actor),
    )


def dong_thuan(client, purpose, *, nguoi_de_nghi=TOI, nguoi_dong_y=NGUOI_KIA):
    """Both rungs of one purpose, which is what «granted» means here."""
    offered = de_nghi(client, purpose, actor=nguoi_de_nghi)
    assert offered.status_code == 201, offered.text
    proposal_id = offered.json()["id"]
    granted = client.post(
        f"/contexts/{CAP}/notebook/proposals/{proposal_id}/grant",
        headers=head(nguoi_dong_y),
    )
    assert granted.status_code == 200, granted.text
    return proposal_id


def lap_so(client):
    """The notebook, open, which almost every paper case needs first."""
    return dong_thuan(client, "lap_so")


def noi_dung(ngay=THU_BAY, gio="19:00", viec="Ăn tối, quán mới"):
    return {"ngay": ngay, "chang": [{"gio": gio, "viec": viec}]}
