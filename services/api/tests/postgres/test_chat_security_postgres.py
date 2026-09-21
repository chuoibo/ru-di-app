"""Real PostgreSQL evidence for legacy chat replay and read watermark fixes."""

from __future__ import annotations

import uuid
from concurrent.futures import ThreadPoolExecutor
from datetime import UTC, datetime, timedelta
from threading import Event
from time import monotonic, sleep

import pytest
from sqlalchemy import text
from sqlalchemy.orm import Session

from app.api.repository import SqlAlchemyApiRepository
from app.api.service import token_digest
from app.db.models import Context, Message, MessageKind, Person

from .test_idempotency_postgres import ADVANCER_ID, SENDER_ID
from .test_idempotency_postgres import live_client as live_client


@pytest.mark.parametrize("revocation", ["membership", "session", "blocked_pair"])
def test_cached_chat_response_rechecks_live_access(live_client, revocation):  # noqa: F811
    group = uuid.uuid4()
    live_client.seed_group(group)
    path = f"/contexts/{group}/messages"
    headers = {
        "X-Actor-ID": str(ADVANCER_ID),
        "X-Actor-Roles": "member",
        "Idempotency-Key": str(uuid.uuid4()),
    }
    if revocation == "session":
        token = "synthetic-session-" + str(group)
        live_client.app.state.auth_mode = "prod"
        live_client.connection.execute(
            text(
                "INSERT INTO account_sessions(id,person_id,token_digest,issued_via,expires_at) "
                "VALUES (:id,:person,:digest,'genesis',now()+interval '1 hour')"
            ),
            {"id": uuid.uuid4(), "person": ADVANCER_ID, "digest": token_digest(token)},
        )
        headers["Authorization"] = "Bearer " + token
    elif revocation == "blocked_pair":
        live_client.connection.execute(
            text("UPDATE contexts SET kind='pair',pair_key=:key WHERE id=:id"),
            {"id": group, "key": ":".join(sorted([str(ADVANCER_ID), str(SENDER_ID)]))},
        )
    payload = {"kind": "text", "body": "Tin thử riêng tư (dữ liệu mẫu)"}
    first = live_client.post(path, json=payload, headers=headers)
    assert first.status_code == 201, first.text
    replay = live_client.post(path, json=payload, headers=headers)
    assert replay.status_code == 201 and replay.content == first.content
    if revocation == "membership":
        sql = "UPDATE memberships SET state='left',left_at=now() WHERE context_id=:group AND person_id=:person"
        expected = 403
    elif revocation == "session":
        sql = "UPDATE account_sessions SET revoked_at=now() WHERE person_id=:person"
        expected = 401
    else:
        sql = "INSERT INTO friend_requests(id,requester_id,addressee_id,state,decided_by_id,decided_at) VALUES(:id,:person,:other,'blocked',:other,now())"
        expected = 409
    live_client.connection.execute(
        text(sql),
        {"group": group, "person": ADVANCER_ID, "other": SENDER_ID, "id": uuid.uuid4()},
    )
    denied = live_client.post(path, json=payload, headers=headers)
    assert denied.status_code == expected, denied.text
    assert "Idempotency-Replayed" not in denied.headers
    assert payload["body"] not in denied.text
    assert (
        live_client.connection.scalar(
            text("SELECT count(*) FROM messages WHERE context_id=:group"),
            {"group": group},
        )
        == 1
    )


@pytest.mark.parametrize("first_read", [False, True])
def test_concurrent_read_marks_are_atomic_and_monotonic(postgres_engine, first_read):
    at = datetime.now(UTC)
    person_id, context_id = uuid.uuid4(), uuid.uuid4()
    with Session(postgres_engine) as seed, seed.begin():
        seed.add(Person(id=person_id, display_name="Read race (dữ liệu mẫu)"))
        seed.flush()
        seed.add(
            Context(id=context_id, display_name="Chat test", created_by_id=person_id)
        )
        seed.flush()
        for i in range(3):
            seed.add(
                Message(
                    id=uuid.uuid4(),
                    context_id=context_id,
                    author_id=person_id,
                    kind=MessageKind.TEXT,
                    body="Tin thử",
                    created_at=at + timedelta(seconds=i),
                )
            )
    try:
        with Session(postgres_engine) as reader:
            repository = SqlAlchemyApiRepository(reader)
            records = repository.list_messages(context_id, limit=10).messages
            oldest, middle, newest = sorted(records, key=lambda m: (m.created_at, m.id))
        if not first_read:
            with Session(postgres_engine) as seed, seed.begin():
                SqlAlchemyApiRepository(seed).set_read_mark(
                    context_id=context_id, person_id=person_id, message=oldest, now=at
                )
        with Session(postgres_engine) as winner, winner.begin():
            SqlAlchemyApiRepository(winner).set_read_mark(
                context_id=context_id, person_id=person_id, message=newest, now=at
            )
            ready = Event()
            worker_pid = []

            def older_read():
                with Session(postgres_engine) as loser, loser.begin():
                    loser.execute(text("SET LOCAL statement_timeout = '5s'"))
                    worker_pid.append(loser.scalar(text("SELECT pg_backend_pid()")))
                    ready.set()
                    return SqlAlchemyApiRepository(loser).set_read_mark(
                        context_id=context_id,
                        person_id=person_id,
                        message=middle,
                        now=at + timedelta(seconds=1),
                    )

            with ThreadPoolExecutor(max_workers=1) as executor:
                pending = executor.submit(older_read)
                assert ready.wait(2)
                deadline = monotonic() + 2
                blocked = False
                with postgres_engine.connect() as observer:
                    while monotonic() < deadline:
                        blocked = observer.scalar(
                            text(
                                "SELECT wait_event_type='Lock' FROM pg_stat_activity "
                                "WHERE pid=:pid"
                            ),
                            {"pid": worker_pid[0]},
                        )
                        observer.commit()
                        if blocked:
                            break
                        sleep(0.01)
                winner.commit()
                result = pending.result(timeout=5)
                assert blocked, "The two database connections did not contend"
                assert result.last_read_message_id == newest.id
        with Session(postgres_engine) as reader:
            mark = SqlAlchemyApiRepository(reader).get_read_mark(context_id, person_id)
            assert mark.last_read_message_id == newest.id
    finally:
        with postgres_engine.begin() as connection:
            for table in ("context_read_marks", "messages"):
                connection.execute(
                    text(f"DELETE FROM {table} WHERE context_id=:id"),
                    {"id": context_id},
                )
            connection.execute(
                text("DELETE FROM contexts WHERE id=:id"), {"id": context_id}
            )
            connection.execute(
                text("DELETE FROM people WHERE id=:id"), {"id": person_id}
            )
