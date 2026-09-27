#!/usr/bin/env python3
"""Oracle for the Go port of the W10 repository methods (people, and ending an account).

ADR-0029 section 2.4. The thirteen routes of routes/people.py reach eighteen
SqlAlchemyApiRepository methods the earlier waves had not ported, erase_person
among them, plus get_person, update_person_profile, list_person_interests,
get_friend_edge and get_place, ported before. This driver is
`render_pair_repo_oracle.py` -- itself every earlier wave's driver stacked --
with those methods and the thirteen routes added to its call table; the input,
the output and the statement log are the base script's, so
services/core/internal/repo/people_repo_oracle_postgres_test.go compares the
same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -e PEOPLE_ORACLE_MEDIA=/abs/dir -v /abs/dir:/abs/dir \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_people_repo_oracle.py < cases.json

Three differences from the base script's case runner, each so a step can be
read the way one request would be:

* Every step starts with `session.expunge_all()`. The case keeps its one
  transaction, but the identity map is a fresh request's: `session.get` of a
  row an earlier step loaded issues its SELECT again, as it would in the next
  request, instead of answering from memory.
* `PhotoStorage.delete` and the service logger write into the statement log.
  A delete is `-- storage.delete <key> -> True|False`, or `raised <class>`
  with the errno of an OSError; a log record is `-- log <LEVEL> <message>`.
  So the log pins where the files go relative to the SQL, what a missing
  file or an unlinkable one does, and what the service reports about it.
* A probe spelled `SELECT 'media tree <n>' WHERE false` is not run as SQL: it
  answers the file tree under `$PEOPLE_ORACLE_MEDIA/<n>`, one line per entry
  (relative path, kind, size of a file, permission bits), sorted.

`route.*` steps run the real ApiService method behind one route on a fresh
ApiService, with `app.api.service._now` answering the case's `now`, as
`render_pair_repo_oracle.py` does. The actor is a `member` with the case's
`actor_id`. `route.delete_own_account` hands the service a real PhotoStorage
rooted at `media_root`, whose `{media}` stands for $PEOPLE_ORACLE_MEDIA.
"""

from __future__ import annotations

import logging
import os
import stat
import sys
import warnings

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_pair_repo_oracle  # noqa: E402,F401  (W8 to pilot calls, ApiProblem tagging)

from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.repository import SqlAlchemyApiRepository  # noqa: E402
from app.api.schemas import AccountDeleteRequest, ProfileUpdateRequest  # noqa: E402
from app.media.storage import PhotoStorage  # noqa: E402

_uuid = base._uuid
_instant = base._instant

MEDIA_BASE = os.environ.get("PEOPLE_ORACLE_MEDIA", "")
MEDIA_TREE_PREFIX = "SELECT 'media tree "

#: The running case's statement log, while a case runs.
STATEMENTS: list | None = None


def _mark(text: str) -> None:
    if STATEMENTS is not None:
        STATEMENTS.append([f"-- {text}", 1])


def _describe(exc: BaseException) -> str:
    if isinstance(exc, OSError):
        return f"{type(exc).__name__} errno {exc.errno}"
    return type(exc).__name__


_real_delete = PhotoStorage.delete


def _traced_delete(self, key):
    try:
        found = _real_delete(self, key)
    except Exception as exc:
        _mark(f"storage.delete {key} raised {_describe(exc)}")
        raise
    _mark(f"storage.delete {key} -> {found}")
    return found


PhotoStorage.delete = _traced_delete


class _LogMarks(logging.Handler):
    def emit(self, record: logging.LogRecord) -> None:
        _mark(f"log {record.levelname} {record.getMessage()}")


_service_logger = logging.getLogger(api_service.__name__)
_service_logger.addHandler(_LogMarks())
_service_logger.setLevel(logging.INFO)
_service_logger.propagate = False


def _tree(root: str) -> list[str]:
    rows = []
    for dirpath, dirnames, filenames in os.walk(root):
        for name in dirnames + filenames:
            full = os.path.join(dirpath, name)
            st = os.lstat(full)
            if stat.S_ISDIR(st.st_mode):
                kind, size = "d", "-"
            elif stat.S_ISLNK(st.st_mode):
                kind, size = "l", "-"
            elif stat.S_ISREG(st.st_mode):
                kind, size = "f", str(st.st_size)
            else:
                kind, size = "?", "-"
            rel = os.path.relpath(full, root)
            rows.append(f"{rel} {kind} {size} {oct(stat.S_IMODE(st.st_mode))}")
    rows.sort()
    return rows


def _probe(driver, sql: str) -> list:
    if sql.startswith(MEDIA_TREE_PREFIX):
        index = sql[len(MEDIA_TREE_PREFIX) :].split("'", 1)[0]
        return _tree(os.path.join(MEDIA_BASE, index))
    return [row[0] for row in driver.execute(sql).fetchall()]


def run_case(factory, statements: list, case: dict) -> dict:
    """render_repo_oracle.run_case, plus the three differences above."""
    global STATEMENTS
    session = factory()
    steps = []
    STATEMENTS = statements
    try:
        driver = session.connection().connection.driver_connection
        for sql in case.get("setup", []):
            driver.execute(sql)
        for step in case["steps"]:
            for sql in step.get("before", []):
                driver.execute(sql)
            session.expunge_all()
            out: dict = {"result": None, "error": None}
            statements.clear()
            with warnings.catch_warnings(record=True) as caught:
                warnings.simplefilter("always")
                try:
                    value = base.CALLS[step["call"]](
                        SqlAlchemyApiRepository(session), step["args"]
                    )
                    out["result"] = base.tag(value)
                except Exception as exc:  # the case ends like a failed request
                    out["error"] = base._error(exc)
            out["warnings"] = [type(w.message).__name__ for w in caught]
            out["statements"] = list(statements)
            steps.append(out)
            if out["error"] is not None:
                break
            out["probes"] = [_probe(driver, sql) for sql in step.get("probes", [])]
    finally:
        STATEMENTS = None
        session.rollback()
        session.close()
    return {"name": case["name"], "steps": steps}


base.run_case = run_case


# --- routes ------------------------------------------------------------------


def _actor(args: dict) -> Actor:
    return Actor(
        id=_uuid(args["actor_id"]), roles=frozenset({"member"}), context_ids=frozenset()
    )


def _route(name, positional, *, storage: bool = False):
    def call(repository, args: dict):
        now = _instant(args["now"])
        original = api_service._now
        api_service._now = lambda: now
        try:
            extra = {}
            if storage:
                root = args["media_root"].replace("{media}", MEDIA_BASE)
                extra["photo_storage"] = PhotoStorage(root)
            service = api_service.ApiService(repository, **extra)
            getattr(service, name)(*positional(args), _actor(args))
        finally:
            api_service._now = original
        return None

    return call


def _none(args: dict) -> tuple:
    return ()


def _person(args: dict) -> tuple:
    return (_uuid(args["person_id"]),)


def _place(args: dict) -> tuple:
    return (args["place_id"],)


ROUTES = {
    "route.list_my_contexts": _route("list_my_contexts", _none),
    "route.get_my_profile": _route("get_my_profile", _none),
    "route.update_my_profile": _route(
        "update_my_profile",
        lambda a: (ProfileUpdateRequest.model_validate(a["body"]),),
    ),
    "route.list_saved_places": _route("list_saved_places", _none),
    "route.save_place": _route("save_place", _place),
    "route.unsave_place": _route("unsave_place", _place),
    "route.list_blocked_people": _route("list_blocked_people", _none),
    "route.delete_own_account": _route(
        "delete_own_account",
        lambda a: (AccountDeleteRequest.model_validate(a["body"]),),
        storage=True,
    ),
    "route.block_person": _route("block_person", _person),
    "route.unblock_person": _route("unblock_person", _person),
    "route.open_direct_message": _route("open_direct_message", _person),
    "route.get_person_profile": _route("get_person_profile", _person),
    "route.register_person": _route(
        "register_person", lambda a: (_uuid(a["person_id"]), a["display_name"])
    ),
}


base.CALLS.update(
    {
        # --- people -----------------------------------------------------------
        "create_person": lambda repository, args: repository.create_person(
            _uuid(args["person_id"]), args["display_name"]
        ),
        "rename_person": lambda repository, args: repository.rename_person(
            _uuid(args["person_id"]), args["display_name"]
        ),
        "are_friends": lambda repository, args: repository.are_friends(
            _uuid(args["a"]), _uuid(args["b"])
        ),
        "share_active_context": lambda repository, args: (
            repository.share_active_context(_uuid(args["a"]), _uuid(args["b"]))
        ),
        "same_couple": lambda repository, args: (
            repository.same_couple(_uuid(args["a"]), _uuid(args["b"]))
        ),
        "profile_counts": lambda repository, args: repository.profile_counts(
            _uuid(args["person_id"])
        ),
        "list_login_providers": lambda repository, args: (
            repository.list_login_providers(_uuid(args["person_id"]))
        ),
        # --- conversations -----------------------------------------------------
        "get_pair_context": lambda repository, args: repository.get_pair_context(
            args["pair_key"]
        ),
        "create_pair_context": lambda repository, args: (
            repository.create_pair_context(
                pair_key=args["pair_key"],
                member_ids=tuple(_uuid(m) for m in args["member_ids"]),
                created_by_id=_uuid(args["created_by_id"]),
                now=_instant(args["now"]),
            )
        ),
        "list_person_context_summaries": lambda repository, args: (
            repository.list_person_context_summaries(_uuid(args["person_id"]))
        ),
        "count_unread_messages": lambda repository, args: (
            repository.count_unread_messages(
                _uuid(args["context_id"]), _uuid(args["person_id"])
            )
        ),
        # --- saved places ------------------------------------------------------
        "list_saved_places": lambda repository, args: repository.list_saved_places(
            _uuid(args["person_id"])
        ),
        "save_place": lambda repository, args: repository.save_place(
            _uuid(args["person_id"]), args["place_id"], _instant(args["now"])
        ),
        "unsave_place": lambda repository, args: repository.unsave_place(
            _uuid(args["person_id"]), args["place_id"]
        ),
        # --- blocks ------------------------------------------------------------
        "open_block_edge": lambda repository, args: repository.open_block_edge(
            blocker_id=_uuid(args["blocker_id"]),
            addressee_id=_uuid(args["addressee_id"]),
            now=_instant(args["now"]),
        ),
        "lift_block_edge": lambda repository, args: repository.lift_block_edge(
            blocker_id=_uuid(args["blocker_id"]),
            addressee_id=_uuid(args["addressee_id"]),
            now=_instant(args["now"]),
        ),
        "list_blocked": lambda repository, args: repository.list_blocked(
            _uuid(args["person_id"])
        ),
        # --- ending an account --------------------------------------------------
        "revoke_all_account_sessions": lambda repository, args: (
            repository.revoke_all_account_sessions(
                _uuid(args["person_id"]), now=_instant(args["now"])
            )
        ),
        "erase_person": lambda repository, args: repository.erase_person(
            _uuid(args["person_id"]), now=_instant(args["now"])
        ),
        **ROUTES,
    }
)


if __name__ == "__main__":
    base.main()
