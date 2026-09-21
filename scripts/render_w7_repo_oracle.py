#!/usr/bin/env python3
"""Oracle for the Go port of the W7 repository methods (a trip's plan and its invitations).

ADR-0029 section 2.4. The eleven routes of routes/outings.py reach fourteen
SqlAlchemyApiRepository methods the earlier waves had not ported, plus
get_outing, create_outing, get_place, get_context, list_members, get_person and
is_member, ported before. This driver is `render_pair_repo_oracle.py` -- itself
every earlier wave's driver stacked -- with those methods and the eleven routes
added to its call table; the input, the output and the statement log are the
base script's, so services/core/internal/repo/outings_repo_oracle_postgres_test.py
compares the same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_w7_repo_oracle.py < cases.json

Three things this driver does that the base one does not, each so a step reads
the way one request would:

* Every step starts with `session.expunge_all()`. The case keeps its one
  transaction, but the identity map is a fresh request's: `session.get` of a
  row an earlier step loaded issues its SELECT again, as the next request
  would, instead of answering from memory. Without this the two sides could
  not agree on the statements of `get_outing` or `get_outing_stop`.
* `route.*` runs the real ApiService method behind one route on a fresh
  ApiService, with `app.api.service._now` answering the case's `now`. The
  actor is a `member` with the case's `actor_id`. The route's answer is not
  compared (the step answers None); its statements, probes and refusal are. An
  ApiProblem is recorded with `code` = "<status>:<code>".
* `secrets.token_urlsafe` answers the case's `token` for the length of a route
  call that mints one. A trip invitation stores only sha256 of its token, so
  pinning the token is what lets the two sides be compared byte for byte in the
  outing_invites dump -- and nothing anywhere records the token itself, which
  is the property the invite door is built on.

A case writes `itinerary_days` as a list of JSON strings rather than of
objects. Python's dicts keep the order their keys arrived in and the record
carries that order into the comparison; a Go map does not, so the text is what
travels, and the driver `json.loads` each element back into the dict the
repository would have been handed.
"""

from __future__ import annotations

import json
import sys
import warnings
from datetime import date

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_pair_repo_oracle  # noqa: E402,F401  (W8 to pilot calls, ApiProblem tagging)

from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.repository import SqlAlchemyApiRepository  # noqa: E402
from app.api.schemas import (  # noqa: E402
    ItineraryPreviewRequest,
    ItineraryRequest,
    OutingCreateRequest,
    OutingInviteCreateRequest,
    OutingTimelineRequest,
)

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _optional_day(value):
    return None if value is None else date.fromisoformat(value)


def run_case(factory, statements: list, case: dict) -> dict:
    """render_repo_oracle.run_case, with a fresh identity map per step."""
    session = factory()
    steps = []
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
            out["probes"] = [
                [row[0] for row in driver.execute(sql).fetchall()]
                for sql in step.get("probes", [])
            ]
    finally:
        session.rollback()
        session.close()
    return {"name": case["name"], "steps": steps}


base.run_case = run_case


# --- repository methods ------------------------------------------------------


def _timeline_stops(rows: list) -> list[dict]:
    return [
        {
            "minute_of_day": row["minute_of_day"],
            "label": row["label"],
            "place_name": row["place_name"],
            "place_id": row["place_id"],
        }
        for row in rows
    ]


def _itinerary_stops(rows: list) -> list[dict]:
    return [
        {
            "id": row["id"],
            "minute_of_day": row["minute_of_day"],
            "label": row["label"],
            "place_name": row["place_name"],
            "place_id": row["place_id"],
            "day": _optional_day(row["day"]),
            "duration_minutes": row["duration_minutes"],
            "time_locked": row["time_locked"],
            "meeting_point": row["meeting_point"],
        }
        for row in rows
    ]


def _replace_outing_stops(repository, args: dict):
    return repository.replace_outing_stops(
        outing_id=_uuid(args["outing_id"]),
        stops=_timeline_stops(args["stops"]),
        expected_revision=args["expected_revision"],
    )


def _replace_outing_itinerary(repository, args: dict):
    return repository.replace_outing_itinerary(
        outing_id=_uuid(args["outing_id"]),
        stops=_itinerary_stops(args["stops"]),
        itinerary_days=[json.loads(day) for day in args["itinerary_days"]],
        expected_revision=args["expected_revision"],
    )


def _create_outing_invite(repository, args: dict):
    return repository.create_outing_invite(
        outing_id=_uuid(args["outing_id"]),
        source=args["source"],
        invited_person_id=_optional_uuid(args["invited_person_id"]),
        invited_by_id=_uuid(args["invited_by_id"]),
        token_digest=api_service.token_digest(args["token"]),
        expires_at=_instant(args["expires_at"]),
        now=_instant(args["now"]),
    )


# --- routes ------------------------------------------------------------------


class _FixedSecrets:
    """`secrets` as the service sees it while one route step mints a token."""

    def __init__(self, token: str) -> None:
        self._token = token

    def token_urlsafe(self, _nbytes: int | None = None) -> str:
        return self._token


def _actor(args: dict) -> Actor:
    return Actor(
        id=_uuid(args["actor_id"]), roles=frozenset({"member"}), context_ids=frozenset()
    )


def _route(name, positional):
    def call(repository, args: dict):
        now = _instant(args["now"])
        original_now, original_secrets = api_service._now, api_service.secrets
        api_service._now = lambda: now
        if "token" in args:
            api_service.secrets = _FixedSecrets(args["token"])
        try:
            service = api_service.ApiService(repository)
            getattr(service, name)(*positional(args), _actor(args))
        finally:
            api_service._now, api_service.secrets = original_now, original_secrets
        return None

    return call


def _outing(args: dict) -> tuple:
    return (_uuid(args["outing_id"]),)


ROUTES = {
    "route.create_outing": _route(
        "create_outing",
        lambda a: (
            _uuid(a["context_id"]),
            OutingCreateRequest.model_validate(a["body"]),
        ),
    ),
    "route.list_context_outings": _route(
        "list_context_outings", lambda a: (_uuid(a["context_id"]),)
    ),
    "route.replace_outing_timeline": _route(
        "replace_outing_timeline",
        lambda a: (
            _uuid(a["outing_id"]),
            OutingTimelineRequest.model_validate(a["body"]),
        ),
    ),
    "route.preview_outing_itinerary": _route(
        "preview_outing_itinerary",
        lambda a: (
            _uuid(a["outing_id"]),
            ItineraryPreviewRequest.model_validate(a["body"]),
        ),
    ),
    "route.replace_outing_itinerary": _route(
        "replace_outing_itinerary",
        lambda a: (_uuid(a["outing_id"]), ItineraryRequest.model_validate(a["body"])),
    ),
    "route.authorize_outing_itinerary": _route("authorize_outing_itinerary", _outing),
    "route.check_in_to_stop": _route(
        "check_in_to_stop", lambda a: (_uuid(a["stop_id"]),)
    ),
    "route.list_outing_checkins": _route("list_outing_checkins", _outing),
    "route.create_outing_invite": _route(
        "create_outing_invite",
        lambda a: (
            _uuid(a["outing_id"]),
            OutingInviteCreateRequest.model_validate(a["body"]),
        ),
    ),
    "route.revoke_outing_invite": _route(
        "revoke_outing_invite",
        lambda a: (_uuid(a["outing_id"]), _uuid(a["invite_id"])),
    ),
    "route.rotate_outing_invite": _route(
        "rotate_outing_invite_secret",
        lambda a: (_uuid(a["outing_id"]), _uuid(a["invite_id"])),
    ),
    "route.accept_outing_invite": _route(
        "accept_outing_invite", lambda a: (a["token"],)
    ),
}


base.CALLS.update(
    {
        "get_outing": lambda repository, args: repository.get_outing(
            _uuid(args["outing_id"])
        ),
        "list_outings": lambda repository, args: repository.list_outings(
            _uuid(args["context_id"])
        ),
        "get_outing_stop": lambda repository, args: repository.get_outing_stop(
            _uuid(args["stop_id"])
        ),
        "list_outing_checkins": lambda repository, args: (
            repository.list_outing_checkins(_uuid(args["outing_id"]))
        ),
        "create_stop_checkin": lambda repository, args: (
            repository.create_stop_checkin(
                stop_id=_uuid(args["stop_id"]),
                person_id=_uuid(args["person_id"]),
                now=_instant(args["now"]),
            )
        ),
        "replace_outing_stops": _replace_outing_stops,
        "replace_outing_itinerary": _replace_outing_itinerary,
        "create_outing_invite": _create_outing_invite,
        "find_outing_invite_for_person": lambda repository, args: (
            repository.find_outing_invite_for_person(
                _uuid(args["outing_id"]), _uuid(args["person_id"])
            )
        ),
        "get_outing_invite": lambda repository, args: repository.get_outing_invite(
            _uuid(args["invite_id"])
        ),
        "get_outing_invite_by_digest": lambda repository, args: (
            repository.get_outing_invite_by_digest(
                api_service.token_digest(args["token"])
            )
        ),
        "accept_outing_invite": lambda repository, args: (
            repository.accept_outing_invite(
                invite_id=_uuid(args["invite_id"]),
                accepted_by_id=_uuid(args["accepted_by_id"]),
                now=_instant(args["now"]),
            )
        ),
        "revoke_outing_invite": lambda repository, args: (
            repository.revoke_outing_invite(
                invite_id=_uuid(args["invite_id"]), now=_instant(args["now"])
            )
        ),
        "rotate_outing_invite_digest": lambda repository, args: (
            repository.rotate_outing_invite_digest(
                invite_id=_uuid(args["invite_id"]),
                token_digest=api_service.token_digest(args["token"]),
                expires_at=_instant(args["expires_at"]),
                now=_instant(args["now"]),
            )
        ),
        "ensure_invited_membership": lambda repository, args: (
            repository.ensure_invited_membership(
                context_id=_uuid(args["context_id"]),
                person_id=_uuid(args["person_id"]),
                invited_by_id=_uuid(args["invited_by_id"]),
                origin=args["origin"],
                now=_instant(args["now"]),
            )
        ),
        **ROUTES,
    }
)


if __name__ == "__main__":
    base.main()
