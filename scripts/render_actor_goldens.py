#!/usr/bin/env python3
"""Render how the API resolves a dev-mode actor and a bearer header, for Go.

ADR-0029: the Go front door must refuse and accept exactly the identities the
Python API does. The rules look simple and are not -- `uuid.UUID()` removes
"urn:" and "uuid:" anywhere, strips braces, drops every hyphen, and then hands
the rest to `int(..., 16)`, which accepts surrounding whitespace, a sign, a
"0x" prefix and single underscores between digits. So instead of restating
them, this calls the real functions (`app.api.deps.get_actor`,
`app.api.deps.bearer_token`, `uuid.UUID`) and records every answer.

Run inside the pinned API image:

    docker run --rm -v "$PWD":/repo:ro -w /srv --entrypoint python \\
      <api image> /repo/scripts/render_actor_goldens.py \\
      > services/core/internal/auth/testdata/python_actor.json

Header values are latin-1 strings, which is how ASGI decodes header bytes.
"""

from __future__ import annotations

import json
import sys
import uuid
from types import SimpleNamespace

sys.path.insert(0, "/srv")

from app.api import deps  # noqa: E402
from app.api.errors import ApiProblem  # noqa: E402

HEX = "abcdefabcdefabcdefabcdefabcdefab"  # 32 hex characters, letters only
CANONICAL = f"{HEX[:8]}-{HEX[8:12]}-{HEX[12:16]}-{HEX[16:20]}-{HEX[20:]}"

UUID_INPUTS = [
    CANONICAL,
    CANONICAL.upper(),
    "{" + CANONICAL + "}",
    "{{" + CANONICAL + "}",
    "{" + CANONICAL,
    "urn:uuid:" + CANONICAL,
    "URN:UUID:" + CANONICAL,
    "uurn:uid:" + CANONICAL,
    "urn:" + CANONICAL,
    HEX,
    "-".join(HEX),
    " " + HEX[1:],
    HEX[1:] + " ",
    "\xa0" + HEX[1:],
    "\x85" + HEX[1:],
    "\x1c" + HEX[1:],
    HEX[:4] + "_" + HEX[5:],
    HEX[:4] + "__" + HEX[6:],
    "_" + HEX[1:],
    HEX[1:] + "_",
    "0x" + HEX[2:],
    "0X_" + HEX[3:],
    "+" + HEX[1:],
    # No minus-sign-before-zeros case: uuid.UUID deletes every hyphen before
    # int() runs, so a leading '-' only shortens the text (see '-' + HEX[1:]),
    # and a run of zeros would be a digit string the repo guard refuses.
    "-" + HEX[1:],
    "+-" + HEX[2:],
    "- " + HEX[2:],
    HEX[1:],
    HEX + "a",
    "",
    "g" + HEX[1:],
    CANONICAL.replace("-", "", 1),
    "uuid:" + CANONICAL[:10] + "uuid:" + CANONICAL[10:],
]

ACTOR_INPUTS = [
    {"id": None, "roles": None, "contexts": None},
    {"id": "", "roles": None, "contexts": None},
    {"id": CANONICAL, "roles": None, "contexts": None},
    {"id": CANONICAL.upper(), "roles": "member", "contexts": None},
    {"id": "{" + CANONICAL + "}", "roles": "member, advancer", "contexts": None},
    {"id": CANONICAL, "roles": "member,,advancer,", "contexts": None},
    {"id": CANONICAL, "roles": " member\xa0", "contexts": None},
    {"id": CANONICAL, "roles": "Member", "contexts": None},
    {"id": CANONICAL, "roles": "member,emperor", "contexts": None},
    {"id": CANONICAL, "roles": "", "contexts": ""},
    {"id": CANONICAL, "roles": "group_admin,platform_moderator", "contexts": CANONICAL},
    {"id": CANONICAL, "roles": None, "contexts": f"{CANONICAL}, {HEX}"},
    {"id": CANONICAL, "roles": None, "contexts": " , "},
    {"id": CANONICAL, "roles": None, "contexts": "khong-phai-uuid"},
    {"id": "khong-phai-uuid", "roles": "emperor", "contexts": "khong-phai-uuid"},
    {"id": CANONICAL, "roles": "emperor", "contexts": "khong-phai-uuid"},
]

BEARER_INPUTS = [
    None,
    "Bearer tok",
    "bearer  tok ",
    "BEARER tok",
    "Bearer",
    "Bearer ",
    "Bearer \xa0",
    "Basic tok",
    "Bearer\ttok",
    "Bearer a b",
    " Bearer tok",
    "",
]


def problem(exc: ApiProblem) -> dict:
    return {"status": exc.status_code, "code": exc.code, "detail": exc.detail}


def main() -> None:
    request = SimpleNamespace(
        app=SimpleNamespace(state=SimpleNamespace(auth_mode="dev"))
    )
    out: dict[str, list] = {"uuid": [], "actor": [], "bearer": []}
    for value in UUID_INPUTS:
        try:
            out["uuid"].append({"input": value, "uuid": str(uuid.UUID(value))})
        except ValueError:
            out["uuid"].append({"input": value, "error": True})
    for case in ACTOR_INPUTS:
        try:
            actor = deps.get_actor(
                request, None, None, case["id"], case["roles"], case["contexts"]
            )
            result = {
                "id": str(actor.id),
                "roles": sorted(actor.roles),
                "contexts": sorted(str(c) for c in actor.context_ids),
            }
            out["actor"].append({"input": case, "actor": result})
        except ApiProblem as exc:
            out["actor"].append({"input": case, "problem": problem(exc)})
    for value in BEARER_INPUTS:
        try:
            out["bearer"].append({"input": value, "token": deps.bearer_token(value)})
        except ApiProblem as exc:
            out["bearer"].append({"input": value, "problem": problem(exc)})
    json.dump(out, sys.stdout, ensure_ascii=True, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
