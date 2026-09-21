#!/usr/bin/env python3
"""Oracle for the Go port of the pilot wave's repository methods.

ADR-0029 section 2.4: the Go core (services/core/internal/repo) must read and
write exactly what SqlAlchemyApiRepository does. Rather than restate
app/api/repository.py from memory, this driver runs the real methods inside the
pinned API image, against the PostgreSQL schema the Go differential test
(services/core/internal/repo/oracle_postgres_test.go) prepared, and prints what
each call returned, which statements it issued, and what the probes read back.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_repo_oracle.py < cases.json

Input on stdin:

    {"clock": [iso instant, ...],
     "cases": [{"name": ..., "setup": [sql, ...],
                "steps": [{"call": method, "args": {...},
                           "before": [sql, ...], "probes": [sql, ...]}]}]}

Every case gets its own Session (the service's sessionmaker settings) and its
transaction is rolled back at the end, so cases never see each other. Setup,
before and probe SQL runs on the session's own connection, inside that
transaction: `before` just ahead of a call, the probes right after it, and every
probe row's first column is printed as text. A call that raises ends its case,
as a failed flush ends a request.

Values are tagged so JSON loses nothing: an int as its decimal string (a
Decimal is tagged apart), a float as its IEEE-754 bits, a datetime in UTC with
microseconds, a dataclass with its field names in declaration order, a dict as
ordered pairs, a list or tuple as a sequence.
"""

from __future__ import annotations

import dataclasses
import json
import os
import struct
import sys
import uuid
import warnings
from datetime import UTC, date, datetime
from decimal import Decimal
from zoneinfo import ZoneInfo

sys.path.insert(0, "/srv")

from sqlalchemy import create_engine, event  # noqa: E402
from sqlalchemy.orm import sessionmaker  # noqa: E402

from app.api.repository import WALL_CLOCK_ZONE, SqlAlchemyApiRepository  # noqa: E402


def tag(value):
    if value is None:
        return None
    if isinstance(value, bool):
        return {"bool": value}
    if type(value) is int:
        return {"int": str(value)}
    if isinstance(value, Decimal):
        return {"decimal": str(value)}
    if isinstance(value, float):
        return {"float": struct.pack(">d", value).hex()}
    if isinstance(value, uuid.UUID):
        return {"uuid": str(value)}
    if type(value) is str:
        return {"str": value}
    if isinstance(value, (int, str)):
        return {"subclass": [type(value).__name__, str(value)]}
    if isinstance(value, datetime):
        if value.tzinfo is None:
            return {"naive_datetime": value.isoformat(timespec="microseconds")}
        return {"datetime": value.astimezone(UTC).isoformat(timespec="microseconds")}
    if isinstance(value, date):
        return {"date": value.isoformat()}
    if dataclasses.is_dataclass(value):
        return {
            "record": type(value).__name__,
            "fields": [
                [field.name, tag(getattr(value, field.name))]
                for field in dataclasses.fields(value)
            ],
        }
    if isinstance(value, (list, tuple)):
        return {"seq": [tag(item) for item in value]}
    if isinstance(value, dict):
        return {"dict": [[tag(k), tag(v)] for k, v in value.items()]}
    raise TypeError(f"untagged {type(value).__name__}")


def _uuid(text: str) -> uuid.UUID:
    return uuid.UUID(text)


def _instant(text: str) -> datetime:
    return datetime.fromisoformat(text)


def _kwargs(args: dict, names: tuple[str, ...]) -> dict:
    return {name: args[name] for name in names if name in args}


def _list_memories(repository: SqlAlchemyApiRepository, args: dict):
    extra = _kwargs(args, ("kind", "place_id"))
    if "before" in args:
        created_at, memory_id = args["before"]
        extra["before"] = (_instant(created_at), _uuid(memory_id))
    if "viewer_id" in args:
        viewer = args["viewer_id"]
        extra["viewer_id"] = None if viewer is None else _uuid(viewer)
    return repository.list_memories(
        _uuid(args["context_id"]), limit=args["limit"], **extra
    )


CALLS = {
    "is_member": lambda repository, args: repository.is_member(
        _uuid(args["context_id"]), _uuid(args["person_id"])
    ),
    "get_person": lambda repository, args: repository.get_person(
        _uuid(args["person_id"])
    ),
    "update_person_profile": lambda repository, args: (
        repository.update_person_profile(
            _uuid(args["person_id"]),
            changes={field: value for field, value in args["changes"]},
        )
    ),
    "list_person_interests": lambda repository, args: (
        repository.list_person_interests(_uuid(args["person_id"]))
    ),
    "set_person_interests": lambda repository, args: (
        repository.set_person_interests(
            _uuid(args["person_id"]), list(args["tags"]), _instant(args["now"])
        )
    ),
    "interests_by_person": lambda repository, args: (
        repository.interests_by_person([_uuid(p) for p in args["person_ids"]])
    ),
    "list_places": lambda repository, args: repository.list_places(
        **_kwargs(args, ("destination_id", "category"))
    ),
    "list_memories": _list_memories,
    "group_recap": lambda repository, args: repository.group_recap(
        _uuid(args["context_id"]), today=date.fromisoformat(args["today"])
    ),
    "create_report": lambda repository, args: repository.create_report(
        reporter_id=_uuid(args["reporter_id"]),
        target_type=args["target_type"],
        target_id=_uuid(args["target_id"]),
        reason=args["reason"],
        note=args["note"],
        now=_instant(args["now"]),
    ),
}


def _error(exc: BaseException) -> dict:
    orig = getattr(exc, "orig", None)
    diag = getattr(orig, "diag", None)
    return {
        "type": type(exc).__name__,
        "sqlstate": getattr(orig, "sqlstate", None),
        "constraint": getattr(diag, "constraint_name", None),
    }


def run_case(factory, statements: list, case: dict) -> dict:
    session = factory()
    steps = []
    try:
        driver = session.connection().connection.driver_connection
        for sql in case.get("setup", []):
            driver.execute(sql)
        for step in case["steps"]:
            for sql in step.get("before", []):
                driver.execute(sql)
            out: dict = {"result": None, "error": None}
            statements.clear()
            with warnings.catch_warnings(record=True) as caught:
                warnings.simplefilter("always")
                try:
                    value = CALLS[step["call"]](
                        SqlAlchemyApiRepository(session), step["args"]
                    )
                    out["result"] = tag(value)
                except Exception as exc:  # the case ends like a failed request
                    out["error"] = _error(exc)
            out["warnings"] = [type(w.message).__name__ for w in caught]
            out["statements"] = list(statements)
            steps.append(out)
            if out["error"] is not None:
                break
            out["probes"] = [
                [row[0] for row in driver.execute(sql).fetchall()]
                for sql in step.get("probes", [])
            ]
    finally:
        session.rollback()
        session.close()
    return {"name": case["name"], "steps": steps}


def main() -> None:
    spec = json.load(sys.stdin)
    engine = create_engine(os.environ["ORACLE_DATABASE_URL"], pool_pre_ping=True)
    statements: list = []

    @event.listens_for(engine, "before_cursor_execute")
    def _record(conn, cursor, statement, parameters, context, executemany):
        # A true executemany hands the driver a list of parameter sets, one
        # statement each; insertmanyvalues hands it one dict for one statement.
        many = executemany and isinstance(parameters, (list, tuple))
        statements.append([statement, len(parameters) if many else 1])

    factory = sessionmaker(bind=engine, expire_on_commit=False)
    zone = ZoneInfo(WALL_CLOCK_ZONE)
    out = {
        "clock": [
            _instant(instant).astimezone(zone).date().isoformat()
            for instant in spec.get("clock", [])
        ],
        "cases": [run_case(factory, statements, case) for case in spec["cases"]],
    }
    engine.dispose()
    json.dump(out, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
