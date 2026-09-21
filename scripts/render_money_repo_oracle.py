#!/usr/bin/env python3
"""Oracle for the Go port of the W4 repository methods (the money routes).

ADR-0029 section 2.4. The fourteen W4 routes (expenses, bills, the group
budget, collection batches, receipt confirmation and the personal finance
screen) reach fifteen SqlAlchemyApiRepository methods the earlier waves had
not ported. This driver is `render_groups_repo_oracle.py` -- itself the W3,
W2 and pilot drivers stacked -- with those methods added to its call table;
the input, the output, the per-case Session and the statement log are the
base script's, so services/core/internal/repo/money_oracle_postgres_test.go
compares the same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_money_repo_oracle.py < cases.json

Arguments that are records in Python arrive as plain JSON and are rebuilt
here into the very types the service hands the repository: `ExpenseInput`
through pydantic (as the route parses it), `ObligationDraft` with its
`AllocationRow` sources, `GuestLinkDraft` with a hex token digest, and
`ReceiptTarget`.

Three calls are sequences, because the step that follows needs a record or an
id the step before produced, as the service threads them:

* `flow.publish` is load_batch_for_publish then save_published_batch on the
  record it returned.
* `flow.confirm_receipt` is get_receipt_target then save_receipt_confirmation
  on that target.
* `flow.create_expense_confirm` is create_expense then get_expense and
  save_expense_confirmation on the new id.
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_groups_repo_oracle  # noqa: E402,F401  (W3 calls, W2 tagging)

from app.api.repository import (  # noqa: E402
    AllocationRow,
    BatchForPublish,
    GuestLinkDraft,
    ObligationDraft,
    ReceiptTarget,
)
from app.api.schemas import ExpenseInput  # noqa: E402

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _bill_items(args: dict) -> list[dict]:
    return [
        {
            "item_key": item["item_key"],
            "name": item["name"],
            "quantity": item["quantity"],
            "unit_price_vnd": item["unit_price_vnd"],
            "line_total_vnd": item["line_total_vnd"],
            "position": item["position"],
            "suggested_participant_ids": [
                _uuid(p) for p in item["suggested_participant_ids"]
            ],
        }
        for item in args["items"]
    ]


def _create_bill(repository, args: dict):
    return repository.create_bill(
        context_id=_uuid(args["context_id"]),
        created_by_id=_uuid(args["created_by_id"]),
        printed_total_vnd=args["printed_total_vnd"],
        items_total_vnd=args["items_total_vnd"],
        confidence=args["confidence"],
        needs_review=args["needs_review"],
        items=_bill_items(args),
        surcharges=[dict(s) for s in args["surcharges"]],
        discounts=[dict(d) for d in args["discounts"]],
        now=_instant(args["now"]),
    )


def _save_expense_confirmation(repository, args: dict, expense_id=None):
    return repository.save_expense_confirmation(
        expense_id=expense_id or _uuid(args["expense_id"]),
        proposal=ExpenseInput.model_validate(args["proposal"]),
        allocator_expense={"warnings": list(args["warnings"])},
        rollups={name: value for name, value in args["rollups"]},
        allocations={_uuid(p): amount for p, amount in args["allocations"]},
        confirmed_by_id=_uuid(args["confirmed_by_id"]),
        payer_acknowledgement=args["payer_acknowledgement"],
        now=_instant(args["now"]),
    )


def _create_expense_confirm_flow(repository, args: dict):
    identity = repository.create_expense(_uuid(args["context_id"]))
    loaded = repository.get_expense(identity.id)
    record = _save_expense_confirmation(repository, args, expense_id=identity.id)
    return (identity, loaded, record)


def _drafts(args: dict) -> tuple[ObligationDraft, ...]:
    return tuple(
        ObligationDraft(
            sender_id=_uuid(draft["sender_id"]),
            recipient_id=_uuid(draft["recipient_id"]),
            amount_vnd=draft["amount_vnd"],
            source_expense_version_ids=tuple(
                _uuid(v) for v in draft["source_expense_version_ids"]
            ),
            sources=tuple(
                AllocationRow(
                    id=_uuid(source["id"]),
                    participant_id=_uuid(source["participant_id"]),
                    amount_vnd=source["amount_vnd"],
                )
                for source in draft["sources"]
            ),
        )
        for draft in args["obligations"]
    )


def _links(args: dict) -> tuple[GuestLinkDraft, ...]:
    return tuple(
        GuestLinkDraft(
            sender_id=_uuid(link["sender_id"]),
            token_digest=bytes.fromhex(link["token_digest"]),
            expires_at=_instant(link["expires_at"]),
        )
        for link in args["links"]
    )


def _save_published_batch(repository, args: dict, batch=None):
    if batch is None:
        # Only `id` and `version_id` are read; the rest is the shape.
        batch = BatchForPublish(
            id=_uuid(args["batch_id"]),
            version_id=_uuid(args["version_id"]),
            owner_id=_uuid(args["actor_id"]),
            status="frozen",
            context_id=_uuid(args["actor_id"]),
            advancer_acknowledged=True,
            obligations=(),
        )
    return repository.save_published_batch(
        batch=batch,
        status=args["status"],
        links=_links(args),
        actor_id=_uuid(args["actor_id"]),
        now=_instant(args["now"]),
    )


def _publish_flow(repository, args: dict):
    batch = repository.load_batch_for_publish(_uuid(args["batch_id"]))
    if batch is None:
        return (None, None)
    return (batch, _save_published_batch(repository, args, batch=batch))


def _save_receipt_confirmation(repository, args: dict, target=None):
    if target is None:
        target = ReceiptTarget(
            obligation_id=_uuid(args["obligation_id"]),
            recipient_id=_uuid(args["recipient_id"]),
            amount_vnd=args["target_amount_vnd"],
        )
    return repository.save_receipt_confirmation(
        target=target,
        confirmed_by_id=_uuid(args["confirmed_by_id"]),
        amount_vnd=args["amount_vnd"],
        payment_report_id=_optional_uuid(args["payment_report_id"]),
        idempotency_key=_uuid(args["idempotency_key"]),
        now=_instant(args["now"]),
    )


def _confirm_receipt_flow(repository, args: dict):
    target = repository.get_receipt_target(_uuid(args["obligation_id"]))
    if target is None:
        return (None, None)
    return (target, _save_receipt_confirmation(repository, args, target=target))


base.CALLS.update(
    {
        # --- expenses ------------------------------------------------------------
        "create_expense": lambda repository, args: repository.create_expense(
            _uuid(args["context_id"])
        ),
        "get_expense": lambda repository, args: repository.get_expense(
            _uuid(args["expense_id"])
        ),
        "save_expense_confirmation": _save_expense_confirmation,
        "flow.create_expense_confirm": _create_expense_confirm_flow,
        # --- bills ---------------------------------------------------------------
        "create_bill": _create_bill,
        "get_bill": lambda repository, args: repository.get_bill(
            _uuid(args["bill_id"])
        ),
        "confirm_bill_assignments": lambda repository, args: (
            repository.confirm_bill_assignments(
                bill_id=_uuid(args["bill_id"]),
                assignments=[
                    {
                        "item_key": assignment["item_key"],
                        "participant_ids": [
                            _uuid(p) for p in assignment["participant_ids"]
                        ],
                    }
                    for assignment in args["assignments"]
                ],
                decided_by_id=_uuid(args["decided_by_id"]),
                now=_instant(args["now"]),
            )
        ),
        "claim_bill_items": lambda repository, args: repository.claim_bill_items(
            bill_id=_uuid(args["bill_id"]),
            participant_id=_uuid(args["participant_id"]),
            item_keys=list(args["item_keys"]),
            now=_instant(args["now"]),
        ),
        # --- batches -------------------------------------------------------------
        "save_frozen_batch": lambda repository, args: repository.save_frozen_batch(
            context_id=_uuid(args["context_id"]),
            owner_id=_uuid(args["owner_id"]),
            due_at=_instant(args["due_at"]),
            obligations=_drafts(args),
            now=_instant(args["now"]),
        ),
        "load_batch_for_publish": lambda repository, args: (
            repository.load_batch_for_publish(_uuid(args["batch_id"]))
        ),
        "save_published_batch": _save_published_batch,
        "flow.publish": _publish_flow,
        "list_batch_obligations": lambda repository, args: (
            repository.list_batch_obligations(_uuid(args["batch_id"]))
        ),
        "list_context_batches": lambda repository, args: (
            repository.list_context_batches(_uuid(args["context_id"]))
        ),
        # --- receipts ------------------------------------------------------------
        "get_receipt_target": lambda repository, args: (
            repository.get_receipt_target(_uuid(args["obligation_id"]))
        ),
        "save_receipt_confirmation": _save_receipt_confirmation,
        "flow.confirm_receipt": _confirm_receipt_flow,
        # --- finance -------------------------------------------------------------
        "person_finance_summary": lambda repository, args: (
            repository.person_finance_summary(
                _uuid(args["person_id"]), movement_limit=args["movement_limit"]
            )
        ),
    }
)


if __name__ == "__main__":
    base.main()
