#!/usr/bin/env python3
"""Oracle for the Go port of the W3 repository methods (groups and the memory wall).

ADR-0029 section 2.4. The sixteen W3 routes (contexts, memberships, balances,
memories, check-ins, the widget, hearts and comments) reach seventeen
SqlAlchemyApiRepository methods the earlier waves had not ported. This driver
is `render_social_repo_oracle.py` -- itself `render_repo_oracle.py` plus the
W2 methods, the RepositoryConflict `code` and `cause`, and set tagging -- with
those methods added to its call table; the input, the output, the per-case
Session and the statement log are the base script's, so
services/core/internal/repo/groups_oracle_postgres_test.go compares the same
way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_groups_repo_oracle.py < cases.json

Two calls are sequences rather than one method, because the step that follows
needs an id the step before generated and a case's arguments are fixed in
advance:

* `flow.create_context` is the repository half of ApiService.create_context:
  create_context, add_member(role="admin") on the new id, accept_membership
  on the new membership. It answers the three records in that order.
* `flow.add_member_accept` is add_member followed by accept_membership on the
  membership it created, as an invitee's own accept would reach it.
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_social_repo_oracle  # noqa: E402,F401  (patches base.tag and base._error)

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _add_member(repository, args: dict):
    extra = {}
    if "role" in args:
        extra["role"] = args["role"]
    return repository.add_member(
        _uuid(args["context_id"]),
        _uuid(args["person_id"]),
        _uuid(args["invited_by_id"]),
        **extra,
    )


def _load_batch_inputs(repository, args: dict):
    ids = args["expense_version_ids"]
    return repository.load_batch_inputs(
        _uuid(args["context_id"]),
        None if ids is None else tuple(_uuid(i) for i in ids),
    )


def _create_context_flow(repository, args: dict):
    actor = _uuid(args["actor_id"])
    context = repository.create_context(args["display_name"], actor)
    membership = repository.add_member(context.id, actor, actor, role="admin")
    accepted = repository.accept_membership(membership.id, _instant(args["now"]))
    return (context, membership, accepted)


def _add_member_accept_flow(repository, args: dict):
    membership = _add_member(repository, args)
    accepted = repository.accept_membership(membership.id, _instant(args["now"]))
    return (membership, accepted)


base.CALLS.update(
    {
        # --- reused from earlier waves, called here in route order ----------
        "get_context": lambda repository, args: repository.get_context(
            _uuid(args["context_id"])
        ),
        "list_members": lambda repository, args: repository.list_members(
            _uuid(args["context_id"])
        ),
        # --- contexts and memberships ---------------------------------------
        "create_context": lambda repository, args: repository.create_context(
            args["display_name"], _uuid(args["created_by_id"])
        ),
        "update_context": lambda repository, args: repository.update_context(
            _uuid(args["context_id"]),
            changes={field: value for field, value in args["changes"]},
        ),
        "add_member": _add_member,
        "get_membership": lambda repository, args: repository.get_membership(
            _uuid(args["membership_id"])
        ),
        "accept_membership": lambda repository, args: repository.accept_membership(
            _uuid(args["membership_id"]), _instant(args["now"])
        ),
        "leave_context": lambda repository, args: repository.leave_context(
            _uuid(args["context_id"]), _uuid(args["person_id"]), _instant(args["now"])
        ),
        "membership_role": lambda repository, args: repository.membership_role(
            _uuid(args["context_id"]), _uuid(args["person_id"])
        ),
        "flow.create_context": _create_context_flow,
        "flow.add_member_accept": _add_member_accept_flow,
        # --- balances --------------------------------------------------------
        "load_batch_inputs": _load_batch_inputs,
        "load_confirmed_receipts": lambda repository, args: (
            repository.load_confirmed_receipts(_uuid(args["context_id"]))
        ),
        # --- memories ------------------------------------------------------------
        "get_place": lambda repository, args: repository.get_place(args["place_id"]),
        "create_memory": lambda repository, args: repository.create_memory(
            context_id=_uuid(args["context_id"]),
            author_id=_uuid(args["author_id"]),
            image_url=args["image_url"],
            caption=args["caption"],
            now=_instant(args["now"]),
            place_id=args["place_id"],
            place_name=args["place_name"],
        ),
        "create_checkin": lambda repository, args: repository.create_checkin(
            context_id=_uuid(args["context_id"]),
            author_id=_uuid(args["author_id"]),
            place_id=args["place_id"],
            place_name=args["place_name"],
            lat=float(args["lat"]),
            lng=float(args["lng"]),
            caption=args["caption"],
            now=_instant(args["now"]),
        ),
        "get_context_memory": lambda repository, args: (
            repository.get_context_memory(
                _uuid(args["context_id"]),
                _uuid(args["memory_id"]),
                viewer_id=_optional_uuid(args.get("viewer_id")),
            )
        ),
        "add_memory_reaction": lambda repository, args: (
            repository.add_memory_reaction(
                memory_id=_uuid(args["memory_id"]),
                person_id=_uuid(args["person_id"]),
                now=_instant(args["now"]),
            )
        ),
        "remove_memory_reaction": lambda repository, args: (
            repository.remove_memory_reaction(
                memory_id=_uuid(args["memory_id"]),
                person_id=_uuid(args["person_id"]),
            )
        ),
        "create_memory_comment": lambda repository, args: (
            repository.create_memory_comment(
                memory_id=_uuid(args["memory_id"]),
                author_id=_uuid(args["author_id"]),
                body=args["body"],
                now=_instant(args["now"]),
            )
        ),
        "list_memory_comments": lambda repository, args: (
            repository.list_memory_comments(_uuid(args["memory_id"]))
        ),
    }
)


if __name__ == "__main__":
    base.main()
