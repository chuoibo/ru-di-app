"""An expense already in a collection is not re-confirmed (ADR-0056 §2.1).

Audit 2026-10-05, RS-03: confirming v2 of an expense whose v1 was batched and
published wrote new allocations the next batch collected again -- the same
dinner owed twice. Corrections now go through an amendment every affected
party accepts.
"""

from __future__ import annotations

from .helpers import actor_headers, create_batch, expense_payload, publish_batch


def test_reconfirming_a_batched_expense_is_refused(client, repository):
    proposal = client.post("/expenses", headers=actor_headers(), json=expense_payload())
    assert proposal.status_code == 201, proposal.text
    body = proposal.json()
    confirm = {
        "proposal": body["proposal"],
        "expected_allocations": body["allocation"]["allocations"],
        "acknowledge_as_advancer": True,
    }
    first = client.post(
        f"/expenses/{body['expense_id']}/confirm", headers=actor_headers(), json=confirm
    )
    assert first.status_code == 201, first.text
    # Before any batch, a second version is still an ordinary correction.
    second = client.post(
        f"/expenses/{body['expense_id']}/confirm", headers=actor_headers(), json=confirm
    )
    assert second.status_code == 201, second.text
    batch = create_batch(client, repository)
    publish_batch(client, batch["batch_id"] if "batch_id" in batch else batch["id"])
    versions_before = len(repository.confirmed)

    again = client.post(
        f"/expenses/{body['expense_id']}/confirm", headers=actor_headers(), json=confirm
    )

    assert again.status_code == 409, again.text
    assert again.json()["code"] == "expense_in_batch"
    assert len(repository.confirmed) == versions_before
