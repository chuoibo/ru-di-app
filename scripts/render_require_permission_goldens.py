"""Goldens for RequirePermission in services/core/internal/service.

Calls the real `app.api.service._require_permission` inside the pinned API image
and prints every outcome as JSON:

    docker run --rm -i --network none --entrypoint python <api image> - \\
      < scripts/render_require_permission_goldens.py \\
      > services/core/internal/service/testdata/python_require_permission.json

For every action in the permissions table: allowed through the actor's role,
through an extra role only, refused for no role and for an unrelated role, and,
when the rule needs predicates, the last one false, a truthy non-True value, the
string "True", or absent. Plus an unknown action, unknown roles on either side
and a missing resource id. A context value proves a predicate only when it `is
True`; each value is written with that verdict so the Go side receives exactly
the boolean Python acted on.
"""

from __future__ import annotations

import json
import sys
import uuid

from app.api.deps import Actor
from app.api.errors import ApiProblem
from app.api.service import _require_permission
from app.domain import permissions

ACTOR_ID = uuid.UUID("6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d")
RESOURCE_ID = "resource-under-test"


def outcome(action, actor_roles, extra_roles, resource_id, values):
    actor = Actor(id=ACTOR_ID, roles=frozenset(actor_roles), context_ids=frozenset())
    context = {"resource_id": resource_id, **values}
    try:
        _require_permission(action, actor, context, extra_roles=frozenset(extra_roles))
    except ApiProblem as exc:
        return {
            "kind": "problem",
            "status": exc.status_code,
            "code": exc.code,
            "detail": exc.detail,
        }
    except permissions.PermissionError_ as exc:
        return {"kind": "raises", "code": str(exc)}
    return {"kind": "allowed"}


def case(name, action, actor_roles, extra_roles, values, resource_id=RESOURCE_ID):
    return {
        "name": name,
        "action": action,
        "actor_roles": list(actor_roles),
        "extra_roles": list(extra_roles),
        "resource_id": resource_id,
        "context": [
            {"name": key, "proves": value is True, "python": repr(value)}
            for key, value in values.items()
        ],
        "outcome": outcome(action, actor_roles, extra_roles, resource_id, values),
    }


def cases():
    out = []
    for action in permissions.ACTIONS:
        rule = permissions._TABLE[action]
        roles = sorted(rule["roles"])
        requires = list(rule["requires"])
        full = {name: True for name in requires}
        first = roles[:1]
        out.append(case("allowed_by_actor_role", action, first, [], full))
        out.append(case("no_roles", action, [], [], full))
        out.append(case("role_only_via_extra", action, [], first, full))
        outsider = [role for role in permissions.ROLES if role not in rule["roles"]]
        if outsider:
            out.append(case("unrelated_role", action, outsider[:1], [], full))
        if requires:
            last = requires[-1]
            out.append(case("last_false", action, first, [], {**full, last: False}))
            out.append(case("last_truthy_int", action, first, [], {**full, last: 1}))
            out.append(
                case("last_string_true", action, first, [], {**full, last: "True"})
            )
            out.append(
                case(
                    "last_missing",
                    action,
                    first,
                    [],
                    {key: True for key in requires[:-1]},
                )
            )
    probe = permissions.ACTIONS[0]
    out.append(case("unknown_action", "no_such_action", ["member"], [], {}))
    out.append(case("unknown_actor_role", probe, ["member", "superuser"], [], {}))
    out.append(case("unknown_extra_role", probe, ["member"], ["superuser"], {}))
    out.append(case("no_resource_id", probe, ["member"], [], {}, resource_id=None))
    return out


def main() -> None:
    document = {
        "actor_id": str(ACTOR_ID),
        "provenance": "api_service",
        "cases": cases(),
    }
    json.dump(document, sys.stdout, ensure_ascii=False, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
