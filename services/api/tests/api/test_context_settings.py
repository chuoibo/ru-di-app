"""`PATCH /contexts/{id}` -- rename a group or choose its chat theme (ADR-0021 §2.4).

Any active member may; a stranger holding the id is refused before the row is
read; the theme is one of five slugs and nothing else; the answer and every
later read of the group carry the theme.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import get_args

from app.api.repository import ContextRecord
from app.api.schemas import ChatTheme
from app.domain.chat_theme import THEMES

from .helpers import ADVANCER_ID, CONTEXT_ID, OTHER_ID, actor_headers

CREATED_AT = datetime(2030, 8, 27, 12, tzinfo=UTC)


def _seed(repository):
    repository.contexts[CONTEXT_ID] = ContextRecord(
        id=CONTEXT_ID,
        display_name="Hội đi Đà Lạt",
        created_by_id=ADVANCER_ID,
        created_at=CREATED_AT,
    )
    repository.active_memberships.add((CONTEXT_ID, ADVANCER_ID))


def _patch(client, body, *, actor=ADVANCER_ID):
    return client.patch(
        f"/contexts/{CONTEXT_ID}", json=body, headers=actor_headers(actor_id=actor)
    )


def test_a_new_group_starts_on_the_default_theme(client, repository):
    _seed(repository)
    response = client.get(f"/contexts/{CONTEXT_ID}", headers=actor_headers())
    assert response.status_code == 200
    assert response.json()["theme"] == "mac-dinh"


def test_a_member_can_choose_a_theme_and_the_group_remembers_it(client, repository):
    _seed(repository)
    response = _patch(client, {"theme": "hoang-hon"})
    assert response.status_code == 200, response.text
    assert response.json()["theme"] == "hoang-hon"
    assert response.json()["display_name"] == "Hội đi Đà Lạt", "tên không đổi"
    again = client.get(f"/contexts/{CONTEXT_ID}", headers=actor_headers())
    assert again.json()["theme"] == "hoang-hon"


def test_a_member_can_rename_the_group(client, repository):
    _seed(repository)
    response = _patch(client, {"display_name": "  Hội Đà Lạt 2030  "})
    assert response.status_code == 200, response.text
    assert response.json()["display_name"] == "Hội Đà Lạt 2030"
    assert response.json()["theme"] == "mac-dinh"


def test_both_at_once_is_one_update(client, repository):
    _seed(repository)
    response = _patch(client, {"display_name": "Mới", "theme": "ruc-ro"})
    assert response.status_code == 200, response.text
    assert (response.json()["display_name"], response.json()["theme"]) == (
        "Mới",
        "ruc-ro",
    )


def test_an_empty_update_is_refused(client, repository):
    _seed(repository)
    response = _patch(client, {})
    assert response.status_code == 422, response.text


def test_a_theme_outside_the_five_is_refused_at_the_boundary(client, repository):
    _seed(repository)
    for bad in ("hong", "#c93900", "MAC-DINH"):
        response = _patch(client, {"theme": bad})
        assert response.status_code == 422, (bad, response.text)
    assert repository.contexts[CONTEXT_ID].theme == "mac-dinh"


def test_a_blank_name_is_refused(client, repository):
    _seed(repository)
    response = _patch(client, {"display_name": "   "})
    assert response.status_code == 422, response.text


def test_a_field_this_model_does_not_name_is_refused(client, repository):
    _seed(repository)
    response = _patch(client, {"theme": "bien-dem", "created_by_id": str(OTHER_ID)})
    assert response.status_code == 422, response.text
    assert repository.contexts[CONTEXT_ID].created_by_id == ADVANCER_ID


def test_a_stranger_holding_the_id_is_refused_before_the_row_is_read(
    client, repository
):
    _seed(repository)
    response = _patch(client, {"theme": "bien-dem"}, actor=OTHER_ID)
    assert response.status_code == 403, response.text
    assert repository.contexts[CONTEXT_ID].theme == "mac-dinh"
    missing = client.patch(
        "/contexts/0a0a0a0a-0a0a-4a0a-8a0a-0a0a0a0a0a0a",
        json={"theme": "bien-dem"},
        headers=actor_headers(actor_id=OTHER_ID),
    )
    assert missing.status_code == 403, "không phải 404: id nhóm không được là oracle"


def test_the_wire_enum_and_the_domain_tuple_are_the_same_list():
    assert tuple(get_args(ChatTheme)) == THEMES
