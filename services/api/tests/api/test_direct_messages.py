"""`POST /people/{person_id}/dm` (L2, ADR-0021 §2.5) against the fake.

What this layer proves: two friends get one pair whichever of them opens it
(201, then 200 with the same id); every other request -- a pending request, a
stranger, an id that is nobody -- is the same 404 with the same body; writing
to oneself is 422; the pair shows up in both conversation lists named after the
other person; the roster doors answer 409 `not_a_group` on it while the theme
still changes. What it does not prove: `uq_contexts_pair_key` under a real
race and the two-membership savepoint -- see
tests/postgres/test_direct_message_postgres.py.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime

from app.api.repository import ContextRecord, PersonRecord
from app.domain.direct import ROSTER_ONLY_DOORS

from .helpers import actor_headers

# Letters interleaved on purpose: the repo guard reads nine consecutive digits
# (hyphens allowed) as an account number and blocks the commit.
ME = uuid.UUID("aa00aa00-0a0a-4a0a-8a0a-0a0a0a0a0aa1")
FRIEND = uuid.UUID("bb00bb00-0b0b-4b0b-8b0b-0b0b0b0b0bb1")
GROUPMATE = uuid.UUID("cc00cc00-0c0c-4c0c-8c0c-0c0c0c0c0cc1")
STRANGER = uuid.UUID("dd00dd00-0d0d-4d0d-8d0d-0d0d0d0d0dd1")
GROUP = uuid.UUID("a1a1a1a1-0a0a-4a0a-8a0a-0a0a0a0a0a11")
T0 = datetime(2030, 8, 27, 12, tzinfo=UTC)


def _seed(repository):
    for pid, name in (
        (ME, "Tôi"),
        (FRIEND, "Bạn Thân"),
        (GROUPMATE, "Cùng Nhóm"),
        (STRANGER, "Người Lạ"),
    ):
        repository.people[pid] = PersonRecord(id=pid, display_name=name, created_at=T0)
    repository.contexts[GROUP] = ContextRecord(
        id=GROUP, display_name="Hội đi Đà Lạt", created_by_id=ME, created_at=T0
    )
    repository.active_memberships |= {(GROUP, ME), (GROUP, GROUPMATE)}
    repository.admin_memberships.add((GROUP, ME))
    accepted = repository.open_friend_request(
        requester_id=ME, addressee_id=FRIEND, now=T0
    )
    repository.decide_friend_request(
        request_id=accepted.id, state="accepted", decided_by_id=FRIEND, now=T0
    )
    # A question is not a yes: this one stays unanswered.
    repository.open_friend_request(requester_id=ME, addressee_id=GROUPMATE, now=T0)


def _open(client, me, other):
    return client.post(
        f"/people/{other}/dm", headers=actor_headers(actor_id=me, roles="member")
    )


def test_two_friends_get_one_pair_whichever_of_them_opens_it(client, repository):
    _seed(repository)

    first = _open(client, ME, FRIEND)
    assert first.status_code == 201, first.text
    pair = first.json()
    assert pair["kind"] == "pair"
    assert pair["display_name"] == "Bạn Thân", "cặp gọi bằng tên người kia"
    assert pair["counterpart"] == {"id": str(FRIEND), "display_name": "Bạn Thân"}
    assert pair["member_count"] == 2
    assert pair["my_state"] == "active" and pair["my_role"] == "member"
    assert pair["theme"] == "mac-dinh"
    assert pair["unread_count"] == 0 and pair["last_message"] is None
    assert uuid.UUID(pair["membership_id"])

    again = _open(client, ME, FRIEND)
    assert again.status_code == 200, "lần hai là tìm lại, không tạo mới"
    assert again.json()["id"] == pair["id"]

    from_the_other_side = _open(client, FRIEND, ME)
    assert from_the_other_side.status_code == 200
    assert from_the_other_side.json()["id"] == pair["id"], "một cặp, hai người mở"
    assert from_the_other_side.json()["display_name"] == "Tôi"
    assert from_the_other_side.json()["counterpart"]["id"] == str(ME)


def test_every_refusal_is_the_same_404_with_the_same_body(client, repository):
    _seed(repository)

    pending = _open(client, ME, GROUPMATE)
    stranger = _open(client, ME, STRANGER)
    nobody = _open(client, ME, uuid.uuid4())
    other_way = _open(client, GROUPMATE, ME)

    bodies = {r.status_code: r.json() for r in (pending, stranger, nobody, other_way)}
    assert set(bodies) == {404}, [
        r.status_code for r in (pending, stranger, nobody, other_way)
    ]
    assert pending.json() == stranger.json() == nobody.json() == other_way.json(), (
        "cửa này không được nói vì sao: chờ trả lời, người lạ hay không tồn tại"
    )
    assert pending.json()["code"] == "person_not_found"
    assert pending.json()["detail"] == "Chưa thể nhắn riêng với người này."
    assert (
        "friend" not in pending.text.lower()
        and "bạn" not in pending.json()["detail"].lower()
    )


def test_writing_to_oneself_is_a_422_not_a_404(client, repository):
    _seed(repository)

    response = _open(client, ME, ME)

    assert response.status_code == 422, response.text
    assert response.json()["code"] == "self_direct_message"


def test_the_pair_appears_in_both_conversation_lists_named_after_the_other(
    client, repository
):
    _seed(repository)
    pair_id = _open(client, ME, FRIEND).json()["id"]

    mine = client.get("/people/me/contexts", headers=actor_headers(actor_id=ME))
    theirs = client.get("/people/me/contexts", headers=actor_headers(actor_id=FRIEND))

    rows = {row["id"]: row for row in mine.json()["contexts"]}
    assert set(rows) == {str(GROUP), pair_id}
    assert rows[pair_id]["kind"] == "pair"
    assert rows[pair_id]["display_name"] == "Bạn Thân"
    assert rows[pair_id]["counterpart"]["id"] == str(FRIEND)
    assert rows[str(GROUP)]["kind"] == "group"
    assert rows[str(GROUP)]["counterpart"] is None
    assert rows[str(GROUP)]["display_name"] == "Hội đi Đà Lạt"

    their_rows = {row["id"]: row for row in theirs.json()["contexts"]}
    assert set(their_rows) == {pair_id}
    assert their_rows[pair_id]["display_name"] == "Tôi"
    assert their_rows[pair_id]["counterpart"]["id"] == str(ME)


def test_reading_the_pair_as_a_context_names_it_after_the_other_person(
    client, repository
):
    _seed(repository)
    pair_id = _open(client, ME, FRIEND).json()["id"]

    as_me = client.get(f"/contexts/{pair_id}", headers=actor_headers(actor_id=ME))
    as_friend = client.get(
        f"/contexts/{pair_id}", headers=actor_headers(actor_id=FRIEND)
    )
    as_groupmate = client.get(
        f"/contexts/{pair_id}", headers=actor_headers(actor_id=GROUPMATE)
    )

    assert as_me.status_code == 200, as_me.text
    assert as_me.json()["kind"] == "pair"
    assert as_me.json()["display_name"] == "Bạn Thân"
    assert as_me.json()["counterpart"]["id"] == str(FRIEND)
    assert as_friend.json()["display_name"] == "Tôi"
    assert as_groupmate.status_code == 403, "người thứ ba không đọc được cặp"

    group = client.get(f"/contexts/{GROUP}", headers=actor_headers(actor_id=ME))
    assert group.json()["kind"] == "group" and group.json()["counterpart"] is None


def test_roster_doors_are_closed_on_a_pair_but_the_theme_still_changes(
    client, repository
):
    _seed(repository)
    pair_id = _open(client, ME, FRIEND).json()["id"]
    me = actor_headers(actor_id=ME, roles="member")
    # Inviting needs `group_admin`, and nobody is an admin of a pair, so a real
    # member is refused on role before kind. The dev-mode header may claim the
    # role; that is what lets this case reach -- and prove -- the kind check.
    me_as_admin = actor_headers(actor_id=ME, roles="member,group_admin")

    invited = client.post(
        f"/contexts/{pair_id}/members",
        headers=me_as_admin,
        json={"person_id": str(GROUPMATE)},
    )
    left = client.delete(f"/contexts/{pair_id}/members/{ME}", headers=me)
    renamed = client.patch(
        f"/contexts/{pair_id}", headers=me, json={"display_name": "Nhóm mới"}
    )
    for door, response in (
        ("invite_context_member", invited),
        ("leave_context", left),
        ("rename_context", renamed),
    ):
        assert door in ROSTER_ONLY_DOORS
        assert response.status_code == 409, (door, response.text)
        assert response.json()["code"] == "not_a_group"

    # Nobody is an admin of a pair, so the role door refuses on permission
    # before it can refuse on kind -- the stranger's 403, not a member's 409.
    role = client.put(
        f"/contexts/{pair_id}/members/{FRIEND}/role", headers=me, json={"role": "admin"}
    )
    assert role.status_code == 403, role.text

    themed = client.patch(
        f"/contexts/{pair_id}", headers=me, json={"theme": "bien-dem"}
    )
    assert themed.status_code == 200, themed.text
    assert themed.json()["theme"] == "bien-dem"
    assert themed.json()["kind"] == "pair"
    assert themed.json()["display_name"] == "Bạn Thân", "đổi màu không đặt tên cho cặp"

    # And the group next door still renames.
    group = client.patch(
        f"/contexts/{GROUP}", headers=me, json={"display_name": "Hội đi Đà Lạt 2030"}
    )
    assert group.status_code == 200, group.text
    assert group.json()["display_name"] == "Hội đi Đà Lạt 2030"


def test_a_pair_is_not_a_group_on_the_profile_counts(client, repository):
    _seed(repository)
    before = client.get("/people/me", headers=actor_headers(actor_id=ME)).json()[
        "counts"
    ]
    assert before["contexts"] == 1

    _open(client, ME, FRIEND)

    after = client.get("/people/me", headers=actor_headers(actor_id=ME)).json()[
        "counts"
    ]
    assert after["contexts"] == 1, "một cuộc trò chuyện riêng không phải một nhóm"
    assert after["friends"] == before["friends"]
