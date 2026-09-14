#!/usr/bin/env python3
"""Render every decision of `app.domain.permissions`, for the Go port.

ADR-0029 section 2.4: the Go core must allow and refuse exactly what the
Python API does, and a refusal reason is the detail of a 403, so the strings
are wire bytes too. Instead of restating the table, this imports the real
module, calls `AuthorizationFacts`, `can` and `denial_reason`, and records
every answer, including the `PermissionError_` codes.

Run inside the pinned API image (the script is read from stdin):

    docker run --rm -i --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d - < scripts/render_permissions_goldens.py \\
      > services/core/internal/domain/permissions/testdata/python_permissions.json

## What the table can read, and how that is enumerated

`denial_reason` reads three things from the facts: whether `roles` meets the
rule's role set, and which of the rule's `requires` are in `proven`, in order.
`actor_id` and `provenance` matter only as empty or not, and `roles` only as
known or not, all inside `__post_init__`. `resource_id` is never read. Every
subset of 35 predicates for 116 actions is far too many to write down, so:

* `table` holds, per action, the rule and the reason for EVERY subset of the
  rule's roles times EVERY subset of its `requires` -- the fields that action
  reads. Today that is at most 16 answers per action.
* This script then proves that projection sound before writing anything: for
  every action, every one of the 1024 subsets of ROLES, every subset of the
  rule's `requires`, and four sets of foreign predicates added on top (none,
  every other known predicate, outsider strings, both), the real answer must
  equal the projected one. The Go test walks the same space and compares
  against the projection; `exhaustive_cases` is the count both must reach.
* `sample` is a seeded random draw from the whole product, outsiders
  included (unknown actions and roles, empty ids), recorded case by case.
* `malformed` is a small full product over the values that raise.
* `untyped` records what Python does when facts is not the dataclass, which
  the Go type system rules out.

Roles and predicates in `malformed` and `sample` are indices into
`role_vocab` and `predicate_vocab`, which keeps every line short.
"""

from __future__ import annotations

import itertools
import json
import random
import sys

sys.path.insert(0, "/srv")

from app.domain import permissions  # noqa: E402
from app.domain.permissions import (  # noqa: E402
    ACTIONS,
    ROLES,
    AuthorizationFacts,
    PermissionError_,
    can,
    denial_reason,
)

TABLE = permissions._TABLE
KNOWN_PREDICATES = sorted(
    {name for rule in TABLE.values() for name in rule["requires"]}
)
OUTSIDER_ROLES = ["", "superuser", "Member", "member ", "owner"]
OUTSIDER_PREDICATES = [
    "",
    "IS_SELF",
    "is_self ",
    "all_recipients_eligible",
    "role_not_permitted",
    "action_permitted_to_nobody",
    "resource_id",
]
OUTSIDER_ACTIONS = [
    "",
    "publsh_batch",
    "PUBLISH_BATCH",
    "publish_batch ",
    " publish_batch",
    "role_not_permitted",
]
ROLE_VOCAB = list(ROLES) + OUTSIDER_ROLES
PREDICATE_VOCAB = KNOWN_PREDICATES + OUTSIDER_PREDICATES
EXTRAS = ["none", "other_known", "outsiders", "other_known+outsiders"]
ACTOR = "u1"
PROVENANCE = "api_service"
RESOURCE = "r1"
SAMPLE_SIZE = 2000
SEED = 29

assert not set(OUTSIDER_ROLES) & set(ROLES)
assert not set(OUTSIDER_PREDICATES) & set(KNOWN_PREDICATES)
assert not set(OUTSIDER_ACTIONS) & set(TABLE)
assert list(ACTIONS) == sorted(TABLE)


def outcome(action, actor_id, roles, resource_id, proven, provenance):
    """Everything Python answers for one call, exceptions included."""
    try:
        facts = AuthorizationFacts(
            actor_id=actor_id,
            roles=frozenset(roles),
            resource_id=resource_id,
            proven=frozenset(proven),
            provenance=provenance,
        )
    except PermissionError_ as exc:
        return {"facts_error": exc.code, "error": None, "reason": None, "can": None}
    try:
        reason, error = denial_reason(action, facts), None
    except PermissionError_ as exc:
        reason, error = None, exc.code
    try:
        allowed, can_error = can(action, facts), None
    except PermissionError_ as exc:
        allowed, can_error = None, exc.code
    if can_error != error:
        raise SystemExit(f"can and denial_reason raise differently for {action!r}")
    return {"facts_error": None, "error": error, "reason": reason, "can": allowed}


def subset(items, mask):
    return [item for bit, item in enumerate(items) if mask >> bit & 1]


def extra_predicates(kind, requires):
    other = [name for name in KNOWN_PREDICATES if name not in requires]
    return {
        "none": [],
        "other_known": other,
        "outsiders": OUTSIDER_PREDICATES,
        "other_known+outsiders": other + OUTSIDER_PREDICATES,
    }[kind]


def render_table():
    rows = []
    exhaustive = 0
    for action in ACTIONS:
        rule_roles = sorted(TABLE[action]["roles"])
        requires = list(TABLE[action]["requires"])
        reasons = []
        for role_mask in range(1 << len(rule_roles)):
            row = []
            for proven_mask in range(1 << len(requires)):
                got = outcome(
                    action,
                    ACTOR,
                    subset(rule_roles, role_mask),
                    RESOURCE,
                    subset(requires, proven_mask),
                    PROVENANCE,
                )
                if got["error"] or got["facts_error"]:
                    raise SystemExit(f"{action}: projection raised {got}")
                row.append(got["reason"])
            reasons.append(row)
        # Soundness of the projection over the whole role space.
        for role_mask in range(1 << len(ROLES)):
            roles = subset(ROLES, role_mask)
            projected_role_mask = sum(
                1 << bit for bit, name in enumerate(rule_roles) if name in roles
            )
            for proven_mask in range(1 << len(requires)):
                expected = reasons[projected_role_mask][proven_mask]
                for kind in EXTRAS:
                    proven = subset(requires, proven_mask) + extra_predicates(
                        kind, requires
                    )
                    got = outcome(action, ACTOR, roles, RESOURCE, proven, PROVENANCE)
                    if got["reason"] != expected or got["can"] != (expected is None):
                        raise SystemExit(
                            f"{action}: projection unsound for roles={roles} "
                            f"proven={proven}: {got} != {expected!r}"
                        )
                    exhaustive += 1
        rows.append(
            {
                "action": action,
                "roles": rule_roles,
                "requires": requires,
                "reasons": reasons,
            }
        )
    return rows, exhaustive


def case(action, actor_id, role_idx, resource_id, proven_idx, provenance):
    got = outcome(
        action,
        actor_id,
        [ROLE_VOCAB[i] for i in role_idx],
        resource_id,
        [PREDICATE_VOCAB[i] for i in proven_idx],
        provenance,
    )
    return {
        "action": action,
        "actor_id": actor_id,
        "roles": sorted(role_idx),
        "resource_id": resource_id,
        "proven": sorted(proven_idx),
        "provenance": provenance,
        **got,
    }


def role_index(*names):
    return [ROLE_VOCAB.index(name) for name in names]


def predicate_index(*names):
    return [PREDICATE_VOCAB.index(name) for name in names]


def render_malformed():
    actions = [
        "publish_batch",
        "delete_audit_history",
        "create_post",
        "publsh_batch",
        "",
        "PUBLISH_BATCH",
        "publish_batch ",
    ]
    role_sets = [
        role_index(),
        role_index("batch_owner"),
        role_index("superuser"),
        role_index("batch_owner", "superuser"),
        role_index("Member"),
    ]
    proven_sets = [predicate_index(), predicate_index("owns_batch")]
    resources = [None, "", RESOURCE]
    cases = []
    product = itertools.product(
        actions, ["u1", ""], ["api_service", ""], role_sets, proven_sets
    )
    for action, actor_id, provenance, role_idx, proven_idx in product:
        resource_id = resources[len(cases) % len(resources)]
        cases.append(
            case(action, actor_id, role_idx, resource_id, proven_idx, provenance)
        )
    # Truthy but blank: Python only asks `not value`.
    product = itertools.product(
        ["publish_batch", "publsh_batch"],
        [" ", "0"],
        [" ", "x"],
        [role_index("batch_owner"), role_index("")],
    )
    for action, actor_id, provenance, role_idx in product:
        cases.append(
            case(
                action,
                actor_id,
                role_idx,
                None,
                predicate_index("owns_batch"),
                provenance,
            )
        )
    return cases


def render_sample():
    rng = random.Random(SEED)
    cases = []
    for _ in range(SAMPLE_SIZE):
        if rng.random() < 0.05:
            action = rng.choice(OUTSIDER_ACTIONS)
            requires = []
        else:
            action = rng.choice(ACTIONS)
            requires = list(TABLE[action]["requires"])
        actor_id = rng.choices(["u1", "", " ", "0"], weights=[90, 4, 3, 3])[0]
        provenance = rng.choices(["api_service", "", " ", "x"], weights=[90, 4, 3, 3])[
            0
        ]
        resource_id = rng.choice([None, "", RESOURCE])
        role_idx = [i for i, name in enumerate(ROLES) if rng.random() < 0.3]
        role_idx += [
            len(ROLES) + i for i in range(len(OUTSIDER_ROLES)) if rng.random() < 0.01
        ]
        proven_idx = []
        for i, name in enumerate(PREDICATE_VOCAB):
            if name in requires:
                chance = 0.75
            elif name in KNOWN_PREDICATES:
                chance = 0.25
            else:
                chance = 0.05
            if rng.random() < chance:
                proven_idx.append(i)
        cases.append(
            case(action, actor_id, role_idx, resource_id, proven_idx, provenance)
        )
    return cases


def render_untyped():
    rows = []
    for action in ("publish_batch", "publsh_batch"):
        for label, value in (("dict", {"owns_batch": True}), ("none", None)):
            try:
                denial_reason(action, value)
                error = None
            except PermissionError_ as exc:
                error = exc.code
            rows.append({"action": action, "facts": label, "error": error})
    return rows


def dump(value):
    return json.dumps(value, ensure_ascii=True, separators=(",", ":"))


def main():
    table, exhaustive = render_table()
    header = {
        "roles": list(ROLES),
        "actions": list(ACTIONS),
        "known_predicates": KNOWN_PREDICATES,
        "role_vocab": ROLE_VOCAB,
        "predicate_vocab": PREDICATE_VOCAB,
        "exhaustive_extras": EXTRAS,
        "exhaustive_cases": exhaustive,
    }
    sections = {
        "table": table,
        "malformed": render_malformed(),
        "sample": render_sample(),
        "untyped": render_untyped(),
    }
    out = ["{"]
    for key, value in header.items():
        out.append(f"{dump(key)}:{dump(value)},")
    for number, (key, rows) in enumerate(sections.items()):
        out.append(f"{dump(key)}:[")
        out.append(",\n".join(dump(row) for row in rows))
        out.append("]" if number == len(sections) - 1 else "],")
    out.append("}")
    sys.stdout.write("\n".join(out) + "\n")


main()
