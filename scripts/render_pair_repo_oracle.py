#!/usr/bin/env python3
"""Oracle for the Go port of the W8 repository methods (the two-person notebook).

ADR-0029 section 2.4. The nineteen routes of routes/pair_notebooks.py and
routes/pair_papers.py reach twenty-nine SqlAlchemyApiRepository methods the
earlier waves had not ported, plus create_outing (the second «ừ» on a sheet
makes an outing). This driver is `render_photo_repo_oracle.py` -- itself the
W6, W5, W4, W3, W2 and pilot drivers stacked -- and
`render_pair_consent_oracle.py` (W1's get_pair_notebook) with those methods
added to its call table; the input, the output, the per-case Session and the
statement log are the base script's, so
services/core/internal/repo/pair_repo_oracle_postgres_test.go compares the
same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_pair_repo_oracle.py < cases.json

Three kinds of call:

* A repository method by name, its arguments as JSON (ids and instants as
  strings, `tuan`, `starts_on` and `ends_on` as ISO dates). `set_paper_state`
  passes `current_version` and `recorded_by_id` only when the case gives them,
  so a case reaches the keyword defaults the way the service does.
* `flow.*`, a sequence whose later step needs an id the earlier one generated,
  in the order the service threads them.
* `route.*`, the real ApiService method behind one route, run on a fresh
  ApiService over the case's repository with `app.api.service._now` answering
  the case's `now` for the length of the call, as the Python postgres tests pin
  it. The actor is a `member` with the case's `actor_id`. The route's answer is
  not compared (the step answers None); its statements, probes and refusal
  are. An ApiProblem is recorded with `code` = "<status>:<code>".

`route.close_pair_notebook` accepts the revision "@current": the driver reads
the rows the preview digests through the raw psycopg connection -- which the
statement log does not see -- and hands the service the revision it will
compute, so a case can close a notebook whose revision depends on ids the case
did not choose. Every relation it reads is one the route reads itself, so the
relation locks the probe lists are the same with or without it.
"""

from __future__ import annotations

import sys
from datetime import date

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_photo_repo_oracle  # noqa: E402,F401  (W6 to pilot calls, conflict tagging)
import render_pair_consent_oracle  # noqa: E402,F401  (get_pair_notebook)

from pydantic import TypeAdapter  # noqa: E402

from app.api import service as api_service  # noqa: E402
from app.api.deps import Actor  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402
from app.api.schemas import (  # noqa: E402
    CloseNotebookRequest,
    PairConstraintPutRequest,
    PairProposalCreateRequest,
    PaperDraftEditRequest,
    PaperKeepRequest,
    PaperResponseRequest,
    PaperSendRequest,
    PaperWithdrawRequest,
)
from app.domain import pair_notebook  # noqa: E402

_uuid = base._uuid
_instant = base._instant
_chained_error = base._error


def _error(exc: BaseException) -> dict:
    out = _chained_error(exc)
    if isinstance(exc, ApiProblem):
        out["code"] = f"{exc.status_code}:{exc.code}"
    return out


base._error = _error


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _optional_instant(value):
    return None if value is None else _instant(value)


# --- repository methods ------------------------------------------------------


def _create_pair_paper(repository, args: dict):
    return repository.create_pair_paper(
        context_id=_uuid(args["context_id"]),
        cycle_id=_optional_uuid(args["cycle_id"]),
        draft_owner_id=_uuid(args["draft_owner_id"]),
        tuan=date.fromisoformat(args["tuan"]),
        expires_at=_instant(args["expires_at"]),
        content=args["content"],
        ly_do=args["ly_do"],
        nguon=args["nguon"],
        author_type=args["author_type"],
        now=_instant(args["now"]),
    )


def _add_paper_version(repository, args: dict):
    return repository.add_paper_version(
        paper_id=_uuid(args["paper_id"]),
        version=args["version"],
        content=args["content"],
        ly_do=args["ly_do"],
        nguon=args["nguon"],
        author_type=args["author_type"],
        sent_at=_optional_instant(args["sent_at"]),
        sent_by=_optional_uuid(args["sent_by"]),
        now=_instant(args["now"]),
    )


def _set_paper_state(repository, args: dict):
    extra = {}
    if "current_version" in args:
        extra["current_version"] = args["current_version"]
    if "recorded_by_id" in args:
        extra["recorded_by_id"] = _optional_uuid(args["recorded_by_id"])
    return repository.set_paper_state(
        _uuid(args["paper_id"]), args["state"], now=_instant(args["now"]), **extra
    )


def _open_cycle_propose_flow(repository, args: dict):
    now = _instant(args["now"])
    cycle_id = repository.open_pair_cycle(
        _uuid(args["notebook_id"]),
        participants=tuple(_uuid(p) for p in args["participants"]),
        terms_version=args["terms_version"],
        now=now,
    )
    proposal = repository.create_consent_proposal(
        cycle_id=cycle_id,
        purpose=args["purpose"],
        proposed_by_id=_uuid(args["proposed_by_id"]),
        terms_version=args["terms_version"],
        expires_at=_instant(args["expires_at"]),
        now=now,
    )
    repository.grant_consent(proposal.id, proposal.proposed_by_id, now=now)
    return (cycle_id, proposal)


def _create_outing_link_flow(repository, args: dict):
    now = _instant(args["now"])
    outing = repository.create_outing(
        context_id=_uuid(args["context_id"]),
        created_by_id=_uuid(args["created_by_id"]),
        title=args["title"],
        starts_on=date.fromisoformat(args["starts_on"]),
        ends_on=date.fromisoformat(args["ends_on"]),
        headcount=args["headcount"],
        budget_per_person_vnd=args["budget_per_person_vnd"],
        now=now,
    )
    repository.link_paper_outing(
        paper_id=_uuid(args["paper_id"]),
        version=args["version"],
        outing_id=outing.id,
        now=now,
    )
    return (outing, repository.get_paper_outing(_uuid(args["paper_id"])))


# --- routes ------------------------------------------------------------------


def _current_revision(repository, context_id, now) -> str:
    """The revision `_xem_truoc_dong_so` will compute, read without logging."""
    driver = repository.session.connection().connection.driver_connection
    papers = [
        {"id": row[0], "state": row[1], "current_version": row[2], "expires_at": row[3]}
        for row in driver.execute(
            "SELECT id::text, state, current_version, expires_at FROM pair_papers"
            " WHERE context_id = %s",
            (str(context_id),),
        ).fetchall()
    ]
    live = driver.execute(
        "SELECT c.id FROM pair_notebook_cycles c JOIN pair_notebooks n ON n.id = c.notebook_id"
        " WHERE n.context_id = %s AND c.state <> 'closed' ORDER BY c.created_at DESC LIMIT 1",
        (str(context_id),),
    ).fetchone()
    # The proposals are read only when a cycle is live, as get_pair_notebook
    # reads them: reading them anyway would add a relation lock the route
    # does not take.
    proposals = (
        []
        if live is None
        else [
            {"id": row[0], "completed_at": row[1], "expires_at": row[2]}
            for row in driver.execute(
                "SELECT id::text, completed_at, expires_at FROM pair_consent_proposals"
                " WHERE cycle_id = %s",
                (live[0],),
            ).fetchall()
        ]
    )
    return pair_notebook.xem_truoc_dong_so(papers, proposals, now=now)["revision"]


def _route(name, positional):
    def call(repository, args: dict):
        now = _instant(args["now"])
        original = api_service._now
        api_service._now = lambda: now
        try:
            service = api_service.ApiService(repository)
            actor = Actor(
                id=_uuid(args["actor_id"]),
                roles=frozenset({"member"}),
                context_ids=frozenset(),
            )
            getattr(service, name)(*positional(repository, args, now), actor)
        finally:
            api_service._now = original
        return None

    return call


def _context(repository, args, now):
    return (_uuid(args["context_id"]),)


def _paper(repository, args, now):
    return (_uuid(args["paper_id"]),)


def _close_body(repository, args, now):
    revision = args["revision"]
    if revision == "@current":
        revision = _current_revision(repository, _uuid(args["context_id"]), now)
    return (_uuid(args["context_id"]), CloseNotebookRequest(revision=revision))


_RESPONSE_BODY = TypeAdapter(PaperResponseRequest)

ROUTES = {
    "route.pair_notebook": _route("pair_notebook", _context),
    "route.propose_pair_consent": _route(
        "propose_pair_consent",
        lambda r, a, n: (
            _uuid(a["context_id"]),
            PairProposalCreateRequest.model_validate(a["body"]),
        ),
    ),
    "route.grant_pair_consent": _route(
        "grant_pair_consent",
        lambda r, a, n: (_uuid(a["context_id"]), _uuid(a["proposal_id"])),
    ),
    "route.revoke_pair_consent": _route(
        "revoke_pair_consent", lambda r, a, n: (_uuid(a["context_id"]), a["purpose"])
    ),
    "route.put_pair_constraint": _route(
        "put_pair_constraint",
        lambda r, a, n: (
            _uuid(a["context_id"]),
            a["kind"],
            PairConstraintPutRequest.model_validate(a["body"]),
        ),
    ),
    "route.delete_pair_constraint": _route(
        "delete_pair_constraint", lambda r, a, n: (_uuid(a["context_id"]), a["kind"])
    ),
    "route.preview_close_pair_notebook": _route("preview_close_pair_notebook", _context),
    "route.close_pair_notebook": _route("close_pair_notebook", _close_body),
    "route.list_pair_papers": _route("list_pair_papers", _context),
    "route.draft_pair_paper": _route("draft_pair_paper", _context),
    "route.pair_paper": _route("pair_paper", _paper),
    "route.edit_pair_draft": _route(
        "edit_pair_draft",
        lambda r, a, n: (
            _uuid(a["paper_id"]),
            PaperDraftEditRequest.model_validate(a["body"]),
        ),
    ),
    "route.send_pair_paper": _route(
        "send_pair_paper",
        lambda r, a, n: (_uuid(a["paper_id"]), PaperSendRequest.model_validate(a["body"])),
    ),
    "route.mark_pair_paper_viewed": _route(
        "mark_pair_paper_viewed", lambda r, a, n: (_uuid(a["paper_id"]), a["version"])
    ),
    "route.respond_pair_paper": _route(
        "respond_pair_paper",
        lambda r, a, n: (
            _uuid(a["paper_id"]),
            a["version"],
            _RESPONSE_BODY.validate_python(a["body"]),
        ),
    ),
    "route.withdraw_pair_paper": _route(
        "withdraw_pair_paper",
        lambda r, a, n: (
            _uuid(a["paper_id"]),
            PaperWithdrawRequest.model_validate(a["body"]),
        ),
    ),
    "route.skip_pair_week": _route("skip_pair_week", _paper),
    "route.record_pair_outing_done": _route("record_pair_outing_done", _paper),
    "route.keep_pair_paper_line": _route(
        "keep_pair_paper_line",
        lambda r, a, n: (_uuid(a["paper_id"]), PaperKeepRequest.model_validate(a["body"])),
    ),
}


base.CALLS.update(
    {
        # --- the notebook and its cycles ----------------------------------------
        "create_pair_notebook": lambda repository, args: (
            repository.create_pair_notebook(
                _uuid(args["context_id"]), now=_instant(args["now"])
            )
        ),
        "lock_pair_notebook": lambda repository, args: repository.lock_pair_notebook(
            _uuid(args["context_id"])
        ),
        "open_pair_cycle": lambda repository, args: repository.open_pair_cycle(
            _uuid(args["notebook_id"]),
            participants=tuple(_uuid(p) for p in args["participants"]),
            terms_version=args["terms_version"],
            now=_instant(args["now"]),
        ),
        "activate_pair_cycle": lambda repository, args: (
            repository.activate_pair_cycle(
                _uuid(args["cycle_id"]), now=_instant(args["now"])
            )
        ),
        "close_pair_cycle": lambda repository, args: repository.close_pair_cycle(
            _uuid(args["cycle_id"]), now=_instant(args["now"])
        ),
        # --- proposals and grants ------------------------------------------------
        "create_consent_proposal": lambda repository, args: (
            repository.create_consent_proposal(
                cycle_id=_uuid(args["cycle_id"]),
                purpose=args["purpose"],
                proposed_by_id=_uuid(args["proposed_by_id"]),
                terms_version=args["terms_version"],
                expires_at=_instant(args["expires_at"]),
                now=_instant(args["now"]),
            )
        ),
        "get_consent_proposal": lambda repository, args: (
            repository.get_consent_proposal(_uuid(args["proposal_id"]))
        ),
        "grant_consent": lambda repository, args: repository.grant_consent(
            _uuid(args["proposal_id"]),
            _uuid(args["person_id"]),
            now=_instant(args["now"]),
        ),
        "complete_consent_proposal": lambda repository, args: (
            repository.complete_consent_proposal(
                _uuid(args["proposal_id"]), now=_instant(args["now"])
            )
        ),
        "revoke_consents": lambda repository, args: repository.revoke_consents(
            _uuid(args["cycle_id"]),
            args["purpose"],
            _uuid(args["person_id"]),
            now=_instant(args["now"]),
        ),
        "set_couple_member": lambda repository, args: repository.set_couple_member(
            _uuid(args["person_id"]), _uuid(args["cycle_id"]), now=_instant(args["now"])
        ),
        "clear_couple_member": lambda repository, args: (
            repository.clear_couple_member(_uuid(args["person_id"]))
        ),
        # --- shared constraints ---------------------------------------------------
        "set_pair_constraint": lambda repository, args: (
            repository.set_pair_constraint(
                cycle_id=_uuid(args["cycle_id"]),
                owner_id=_uuid(args["owner_id"]),
                kind=args["kind"],
                content=args["content"],
                now=_instant(args["now"]),
            )
        ),
        "delete_pair_constraint": lambda repository, args: (
            repository.delete_pair_constraint(
                _uuid(args["cycle_id"]), _uuid(args["owner_id"]), args["kind"]
            )
        ),
        # --- papers ----------------------------------------------------------------
        "create_pair_paper": _create_pair_paper,
        "get_pair_paper": lambda repository, args: repository.get_pair_paper(
            _uuid(args["paper_id"])
        ),
        "lock_pair_paper": lambda repository, args: repository.lock_pair_paper(
            _uuid(args["paper_id"])
        ),
        "list_pair_papers": lambda repository, args: repository.list_pair_papers(
            _uuid(args["context_id"])
        ),
        "update_pair_draft": lambda repository, args: repository.update_pair_draft(
            _uuid(args["paper_id"]), content=args["content"], ly_do=args["ly_do"]
        ),
        "add_paper_version": _add_paper_version,
        "mark_version_sent": lambda repository, args: repository.mark_version_sent(
            _uuid(args["paper_id"]),
            args["version"],
            sent_by=_optional_uuid(args["sent_by"]),
            now=_instant(args["now"]),
        ),
        "set_paper_state": _set_paper_state,
        "mark_paper_viewed": lambda repository, args: repository.mark_paper_viewed(
            _uuid(args["paper_id"]),
            args["version"],
            _uuid(args["person_id"]),
            now=_instant(args["now"]),
        ),
        "add_paper_response": lambda repository, args: repository.add_paper_response(
            paper_id=_uuid(args["paper_id"]),
            version=args["version"],
            person_id=_uuid(args["person_id"]),
            kind=args["kind"],
            now=_instant(args["now"]),
        ),
        "link_paper_outing": lambda repository, args: repository.link_paper_outing(
            paper_id=_uuid(args["paper_id"]),
            version=args["version"],
            outing_id=_uuid(args["outing_id"]),
            now=_instant(args["now"]),
        ),
        "get_paper_outing": lambda repository, args: repository.get_paper_outing(
            _uuid(args["paper_id"])
        ),
        "add_paper_keep": lambda repository, args: repository.add_paper_keep(
            paper_id=_uuid(args["paper_id"]),
            person_id=_uuid(args["person_id"]),
            line=args["line"],
            now=_instant(args["now"]),
        ),
        "close_open_pair_papers": lambda repository, args: (
            repository.close_open_pair_papers(
                _uuid(args["context_id"]), now=_instant(args["now"])
            )
        ),
        "create_outing": lambda repository, args: repository.create_outing(
            context_id=_uuid(args["context_id"]),
            created_by_id=_uuid(args["created_by_id"]),
            title=args["title"],
            starts_on=date.fromisoformat(args["starts_on"]),
            ends_on=date.fromisoformat(args["ends_on"]),
            headcount=args["headcount"],
            budget_per_person_vnd=args["budget_per_person_vnd"],
            now=_instant(args["now"]),
        ),
        # --- sequences -------------------------------------------------------------
        "flow.open_cycle_propose": _open_cycle_propose_flow,
        "flow.create_outing_link": _create_outing_link_flow,
        **ROUTES,
    }
)


if __name__ == "__main__":
    base.main()
