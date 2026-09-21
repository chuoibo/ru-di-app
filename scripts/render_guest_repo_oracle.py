#!/usr/bin/env python3
"""Oracle for the Go port of the W5 repository methods (the guest routes).

ADR-0029 section 2.4. The seven guest routes under /g/{token} reach four
SqlAlchemyApiRepository methods the earlier waves had not ported:
get_guest_envelope, get_payment_report_target, save_payment_report and
save_guest_objection. This driver is `render_money_repo_oracle.py` -- itself
the W4, W3, W2 and pilot drivers stacked -- with those methods added to its
call table; the input, the output, the per-case Session and the statement log
are the base script's, so
services/core/internal/repo/guests_oracle_postgres_test.go compares the same
way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_guest_repo_oracle.py < cases.json

Token digests arrive as hex. A case's steps share one Session, so a case whose
steps are the calls one route makes, in its order, runs them against the same
identity map the request would.

Two calls differ from a bare method call:

* `save_guest_objection` is followed by `session.flush()`. The method only
  adds an audit event and assigns to the link; the request's commit is what
  flushes them, and nothing in the service reads the session in between.
  Without the flush the step would record no write at all.
* `flow.report_payment` is get_payment_report_target then save_payment_report
  on the target it returned, as ApiService.report_payment threads them (the
  permission check between the two is the service's, not the repository's).
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_money_repo_oracle  # noqa: E402,F401  (W4, W3 and W2 calls, conflict tagging)

from app.api.repository import PaymentReportTarget  # noqa: E402

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _digest(args: dict) -> bytes:
    return bytes.fromhex(args["token_digest"])


def _save_payment_report(repository, args: dict, target=None):
    if target is None:
        target = PaymentReportTarget(
            link_id=_uuid(args["link_id"]),
            obligation_id=_uuid(args["obligation_id"]),
            amount_vnd=args["amount_vnd"],
            active_capability=args["active_capability"],
            reports_used=args["reports_used"],
        )
    return repository.save_payment_report(
        target=target,
        idempotency_key=_uuid(args["idempotency_key"]),
        now=_instant(args["now"]),
    )


def _report_payment_flow(repository, args: dict):
    target = repository.get_payment_report_target(
        _digest(args), _uuid(args["obligation_id"]), _instant(args["now"])
    )
    if target is None:
        return (None, None)
    return (target, _save_payment_report(repository, args, target=target))


def _save_guest_objection(repository, args: dict):
    repository.save_guest_objection(
        token_digest=_digest(args),
        kind=args["kind"],
        obligation_id=_optional_uuid(args["obligation_id"]),
        reason=args["reason"],
        now=_instant(args["now"]),
    )
    # The commit at the end of the request flushes what the method left
    # pending; see the module docstring.
    repository.session.flush()


base.CALLS.update(
    {
        "get_guest_envelope": lambda repository, args: (
            repository.get_guest_envelope(_digest(args), _instant(args["now"]))
        ),
        "get_payment_report_target": lambda repository, args: (
            repository.get_payment_report_target(
                _digest(args), _uuid(args["obligation_id"]), _instant(args["now"])
            )
        ),
        "save_payment_report": _save_payment_report,
        "flow.report_payment": _report_payment_flow,
        "save_guest_objection": _save_guest_objection,
    }
)


if __name__ == "__main__":
    base.main()
