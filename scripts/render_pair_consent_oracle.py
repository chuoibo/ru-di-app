#!/usr/bin/env python3
"""Oracle for the Go port of the pair-consent reads behind GET /contexts/{id}/map.

ADR-0029 section 2.4. `group_taste` asks `ApiService._pair_chat_consent`
before it sums a roster, and that reads four repository methods the pilot
wave had not ported: `get_context`, `get_pair_notebook`, `list_members` and
`budget_bands_by_person`. This driver is `render_repo_oracle.py` with those
methods and the real service step added to its call table; the input, the
output, the per-case Session and the statement log are that script's, so the
Go differential tests (services/core/internal/repo/pair_oracle_postgres_test.go
and services/core/internal/service/pair_consent_postgres_test.go) compare the
same way the pilot oracle does.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_pair_consent_oracle.py < cases.json

`service._pair_chat_consent` takes `context_id` and `now`: the service's clock
`app.api.service._now` answers `now` for the length of the call, as the Python
postgres tests pin it with monkeypatch, and the method runs on a fresh
`ApiService` over the case's repository.
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402

from app.api import service as api_service  # noqa: E402


def _pair_chat_consent(repository, args: dict):
    now = base._instant(args["now"])
    original = api_service._now
    api_service._now = lambda: now
    try:
        service = api_service.ApiService(repository)
        return service._pair_chat_consent(base._uuid(args["context_id"]))
    finally:
        api_service._now = original


base.CALLS.update(
    {
        "get_context": lambda repository, args: repository.get_context(
            base._uuid(args["context_id"])
        ),
        "list_members": lambda repository, args: repository.list_members(
            base._uuid(args["context_id"])
        ),
        "budget_bands_by_person": lambda repository, args: (
            repository.budget_bands_by_person(
                [base._uuid(p) for p in args["person_ids"]]
            )
        ),
        "get_pair_notebook": lambda repository, args: repository.get_pair_notebook(
            base._uuid(args["context_id"])
        ),
        "service._pair_chat_consent": _pair_chat_consent,
    }
)


if __name__ == "__main__":
    base.main()
