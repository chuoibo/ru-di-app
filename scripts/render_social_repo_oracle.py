#!/usr/bin/env python3
"""Oracle for the Go port of the W2 repository methods (friends, stories, posts, votes).

ADR-0029 section 2.4. The W2 routes reach thirty SqlAlchemyApiRepository
methods the pilot wave had not ported. This driver is `render_repo_oracle.py`
with those methods added to its call table; the input, the output, the
per-case Session and the statement log are that script's, so
services/core/internal/repo/social_oracle_postgres_test.go compares the same
way the pilot oracle does.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_social_repo_oracle.py < cases.json

Two additions to the base script, both about losing nothing:

* An error also records `code` when it is a RepositoryConflict, and the
  SQLSTATE and constraint of the exception it was raised from (`cause`), so a
  conflict mapped from the wrong violation is a mismatch.
* A set or frozenset is tagged as its members sorted by their tagged JSON
  text: iteration order of a set of strings depends on PYTHONHASHSEED and
  carries no meaning, while the members do.
"""

from __future__ import annotations

import json
import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402

from app.api.errors import RepositoryConflict  # noqa: E402

_base_tag = base.tag
_base_error = base._error


def tag(value):
    if isinstance(value, (set, frozenset)):
        members = [_base_tag(item) for item in value]
        members.sort(key=lambda item: json.dumps(item, sort_keys=True))
        return {"set": members}
    return _base_tag(value)


def _error(exc: BaseException) -> dict:
    out = _base_error(exc)
    out["code"] = None
    out["cause"] = None
    if not isinstance(exc, RepositoryConflict):
        # Any other exception's __cause__ is a driver detail (a psycopg class
        # per SQLSTATE) that the SQLSTATE above already carries.
        return out
    out["code"] = exc.code
    cause = exc.__cause__
    if cause is not None:
        orig = getattr(cause, "orig", None)
        out["cause"] = {
            "type": type(cause).__name__,
            "sqlstate": getattr(orig, "sqlstate", None),
            "constraint": getattr(getattr(orig, "diag", None), "constraint_name", None),
        }
    return out


base.tag = tag
base._error = _error

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _list_post_comments(repository, args: dict):
    extra = {}
    if "after" in args:
        after = args["after"]
        extra["after"] = (
            None if after is None else (_instant(after[0]), _uuid(after[1]))
        )
    return repository.list_post_comments(
        _uuid(args["post_id"]), limit=args["limit"], **extra
    )


base.CALLS.update(
    {
        # --- friends ---------------------------------------------------------
        "get_friend_edge": lambda repository, args: repository.get_friend_edge(
            _uuid(args["person_a"]), _uuid(args["person_b"])
        ),
        "get_friend_request": lambda repository, args: (
            repository.get_friend_request(
                _uuid(args["request_id"]), _uuid(args["reader_id"])
            )
        ),
        "open_friend_request": lambda repository, args: (
            repository.open_friend_request(
                requester_id=_uuid(args["requester_id"]),
                addressee_id=_uuid(args["addressee_id"]),
                now=_instant(args["now"]),
            )
        ),
        "decide_friend_request": lambda repository, args: (
            repository.decide_friend_request(
                request_id=_uuid(args["request_id"]),
                state=args["state"],
                decided_by_id=_uuid(args["decided_by_id"]),
                now=_instant(args["now"]),
            )
        ),
        "list_friend_requests": lambda repository, args: (
            repository.list_friend_requests(
                _uuid(args["person_id"]), direction=args["direction"]
            )
        ),
        "list_friends": lambda repository, args: repository.list_friends(
            _uuid(args["person_id"])
        ),
        "get_account_identity": lambda repository, args: (
            repository.get_account_identity(args["provider"], args["subject"])
        ),
        # --- stories ---------------------------------------------------------
        "get_person_image": lambda repository, args: repository.get_person_image(
            _uuid(args["person_id"]), _uuid(args["image_id"])
        ),
        "create_story": lambda repository, args: repository.create_story(
            author_id=_uuid(args["author_id"]),
            image_url=args["image_url"],
            caption=args["caption"],
            audience=args["audience"],
            now=_instant(args["now"]),
            expires_at=_instant(args["expires_at"]),
        ),
        "get_story": lambda repository, args: repository.get_story(
            _uuid(args["story_id"])
        ),
        "list_live_stories_for": lambda repository, args: (
            repository.list_live_stories_for(
                _uuid(args["reader_id"]), now=_instant(args["now"])
            )
        ),
        "mark_story_seen": lambda repository, args: repository.mark_story_seen(
            _uuid(args["story_id"]),
            _uuid(args["viewer_id"]),
            now=_instant(args["now"]),
        ),
        "delete_story": lambda repository, args: repository.delete_story(
            _uuid(args["story_id"])
        ),
        # --- posts -----------------------------------------------------------
        "create_post": lambda repository, args: repository.create_post(
            author_id=_uuid(args["author_id"]),
            audience=args["audience"],
            context_id=_optional_uuid(args["context_id"]),
            body=args["body"],
            image_url=args["image_url"],
            now=_instant(args["now"]),
        ),
        "get_post": lambda repository, args: repository.get_post(
            _uuid(args["post_id"])
        ),
        "list_posts_visible_to": lambda repository, args: (
            repository.list_posts_visible_to(
                _uuid(args["reader_id"]), limit=args["limit"]
            )
        ),
        "list_person_posts_visible_to": lambda repository, args: (
            repository.list_person_posts_visible_to(
                _uuid(args["person_id"]), _uuid(args["reader_id"]), limit=args["limit"]
            )
        ),
        "post_social_counts": lambda repository, args: (
            repository.post_social_counts(
                tuple(_uuid(p) for p in args["post_ids"]),
                viewer_id=_uuid(args["viewer_id"]),
            )
        ),
        "add_post_reaction": lambda repository, args: repository.add_post_reaction(
            post_id=_uuid(args["post_id"]),
            person_id=_uuid(args["person_id"]),
            kind=args["kind"],
            now=_instant(args["now"]),
        ),
        "remove_post_reaction": lambda repository, args: (
            repository.remove_post_reaction(
                post_id=_uuid(args["post_id"]),
                person_id=_uuid(args["person_id"]),
                kind=args["kind"],
            )
        ),
        "create_post_comment": lambda repository, args: (
            repository.create_post_comment(
                post_id=_uuid(args["post_id"]),
                author_id=_uuid(args["author_id"]),
                body=args["body"],
                now=_instant(args["now"]),
            )
        ),
        "get_post_comment": lambda repository, args: repository.get_post_comment(
            _uuid(args["comment_id"])
        ),
        "delete_post_comment": lambda repository, args: (
            repository.delete_post_comment(_uuid(args["comment_id"]))
        ),
        "list_post_comments": _list_post_comments,
        # --- votes -----------------------------------------------------------
        "get_outing": lambda repository, args: repository.get_outing(
            _uuid(args["outing_id"])
        ),
        "create_vote": lambda repository, args: repository.create_vote(
            context_id=_uuid(args["context_id"]),
            outing_id=_optional_uuid(args["outing_id"]),
            created_by_id=_uuid(args["created_by_id"]),
            question=args["question"],
            options=[
                {"label": option["label"], "place_name": option["place_name"]}
                for option in args["options"]
            ],
            now=_instant(args["now"]),
        ),
        "get_vote": lambda repository, args: repository.get_vote(
            _uuid(args["vote_id"])
        ),
        "list_votes": lambda repository, args: repository.list_votes(
            _uuid(args["context_id"])
        ),
        "upsert_ballot": lambda repository, args: repository.upsert_ballot(
            vote_id=_uuid(args["vote_id"]),
            option_id=_uuid(args["option_id"]),
            voter_id=_uuid(args["voter_id"]),
            now=_instant(args["now"]),
        ),
        "close_vote": lambda repository, args: repository.close_vote(
            vote_id=_uuid(args["vote_id"]),
            closed_by_id=_uuid(args["closed_by_id"]),
            now=_instant(args["now"]),
        ),
    }
)


if __name__ == "__main__":
    base.main()
