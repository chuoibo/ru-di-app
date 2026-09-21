#!/usr/bin/env python3
"""Oracle for the Go port of the WAI repository methods.

create_message, list_messages, list_destinations, list_place_photos,
list_outing_memories, group_photos_at_place, set_membership_role.

This driver is render_w7_repo_oracle.py (expunge_all per step, stacked CALLS)
with those seven methods added. services/core/internal/repo/wai_repo_oracle_postgres_test.go
compares the tagged result, every statement, and the probes the same way the
earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_wai_repo_oracle.py < cases.json
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_w7_repo_oracle  # noqa: E402,F401  (expunge_all + stacked CALLS)

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _cursor(value):
    if value is None:
        return None
    created_at, row_id = value
    return (_instant(created_at), _uuid(row_id))


def _create_message(repository, args):
    return repository.create_message(
        context_id=_uuid(args["context_id"]),
        author_id=_optional_uuid(args.get("author_id")),
        kind=args["kind"],
        body=args.get("body"),
        image_url=args.get("image_url"),
        card=args.get("card"),
        now=_instant(args["now"]),
        reply_to_id=_optional_uuid(args.get("reply_to_id")),
    )


def _list_messages(repository, args):
    return repository.list_messages(
        _uuid(args["context_id"]),
        limit=args["limit"],
        before=_cursor(args.get("before")),
        after=_cursor(args.get("after")),
    )


def _list_outing_memories(repository, args):
    return repository.list_outing_memories(
        _uuid(args["outing_id"]),
        limit=args["limit"],
        viewer_id=_optional_uuid(args.get("viewer_id")),
    )


base.CALLS.update(
    {
        "create_message": _create_message,
        "list_messages": _list_messages,
        "list_destinations": lambda repository, args: repository.list_destinations(),
        "list_place_photos": lambda repository, args: repository.list_place_photos(
            args["place_id"]
        ),
        "list_outing_memories": _list_outing_memories,
        "group_photos_at_place": lambda repository, args: (
            repository.group_photos_at_place(
                args["place_id"],
                viewer_id=_uuid(args["viewer_id"]),
                limit=args["limit"],
            )
        ),
        "set_membership_role": lambda repository, args: (
            repository.set_membership_role(
                _uuid(args["context_id"]), _uuid(args["person_id"]), args["role"]
            )
        ),
    }
)


if __name__ == "__main__":
    base.main()
