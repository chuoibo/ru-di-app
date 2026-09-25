#!/usr/bin/env python3
"""Oracle for the Go port of `app.domain.pair_notebook` (ADR-0029 section 2.4).

`services/core/internal/domain/pairnotebook` must answer exactly as the Python
module does. Instead of restating the rules, this script calls the real
functions inside the pinned API image and records what each returned. The Go
test (`oracle_test.go` in that package) replays every case.

Each invocation renders one file, chosen by MODE:

    docker run --rm -i --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d - MODE < scripts/render_pair_notebook_goldens.py \\
      > services/core/internal/domain/pairnotebook/testdata/python_MODE.json

`--list` prints every MODE and its target path, tab separated:

    IMAGE=mobile-parity-api:7bf58e3d
    docker run --rm -i --network none --entrypoint python "$IMAGE" - --list \\
      < scripts/render_pair_notebook_goldens.py |
    while IFS=$'\\t' read -r mode path; do
      docker run --rm -i --network none --entrypoint python "$IMAGE" - "$mode" \\
        < scripts/render_pair_notebook_goldens.py > "$path"
    done

Modes: `edges` holds the module constants and the named edge cases (every
input of tests/domain/test_pair_notebook.py plus the boundaries the service
reaches: expiry exactly at `now` and one microsecond either side, revocation,
one side only, extra and repeated people, the cycle's participant list against
the members' list). `fuzz-<k>` is shard k of a seeded fuzz; shards are
contiguous slices of one fixed case list, so one shard rendered alone gives the
same bytes.

Three case kinds, all over inputs of the shapes the service builds (`str` ids,
aware datetimes or None), because the Go port is typed:

* `consents`: `granted_by` for every person in `ask`, `granted_purposes`,
  `can_bat_doi`, and `chat_consent_active` when `now` is not None;
* `preview`: `xem_truoc_dong_so`, plus `pair_paper.hieu_luc` of every paper and
  `dang_cho` of every proposal;
* `han_de_nghi`.

Encoding: an aware datetime is `"$dt:"` plus its isoformat at microseconds
with `|` before the UTC offset (a negative offset right after the microseconds
would otherwise read as one long digit run to the guard); a frozenset
is its sorted list; the preview's hex revision is `"$hex:"` plus the digits in
groups of four joined by `_`, so no digest can form a digit run the repository
guard refuses. No other str starts with `$`.
"""

from __future__ import annotations

import json
import random
import re
import sys
from datetime import UTC, datetime, timedelta, timezone

sys.path.insert(0, "/srv")

from app.domain import pair_notebook, pair_paper  # noqa: E402

SEED = 27
FUZZ_CASES = 3000
FUZZ_SHARDS = 6
TARGET = "services/core/internal/domain/pairnotebook/testdata/python_{mode}.json"

#: Copies of the digit rules in scripts/repo_guard.py plus the hex, e-mail and
#: long-token rules. A rendered line that matches one could not be committed,
#: so rendering stops instead.
GUARD_RULES = (
    re.compile(r"(?<![A-Za-z0-9])\d(?:[ .-]?\d){8,63}(?![A-Za-z0-9])"),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*(?:3|5|7|8|9)"
        r"(?:[ ().-]*\d){8}(?!\d)"
    ),
    re.compile(
        r"(?<!\d)(?:(?:\+|00)?84(?:[ ().-]*0)?|0)[ ().-]*2"
        r"(?:[ ().-]*\d){8,9}(?!\d)"
    ),
    re.compile(r"(?<![0-9A-Fa-f])[0-9A-Fa-f]{64}(?![0-9A-Fa-f])"),
    re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}"),
    re.compile(r"[A-Za-z0-9+/_-]{2049,}"),
)

A = "a1a1a1a1-b1b1-4c1c-8d1d-e1e1e1e1e1e1"
B = "a2a2a2a2-b2b2-4c2c-8d2d-e2e2e2e2e2e2"
C = "a3a3a3a3-b3b3-4c3c-8d3d-e3e3e3e3e3e3"
#: Tastes gu_hai_nguoi reads: out of vocabulary order, one retired tag.
GU = {A: ["cafe", "an-uong", "game"], B: ["game", "outdoor", "cafe", "tag-da-bo"], C: ["karaoke"]}
NOW = datetime(2026, 9, 13, 12, 0, tzinfo=UTC)
MICRO = timedelta(microseconds=1)
VIETNAM = timezone(timedelta(hours=7))
HONOLULU = timezone(timedelta(hours=-10))
E_DOT_CIRCUMFLEX = chr(0x1EC7)
CJK_MIDDLE = chr(0x4E2D)
GRINNING_FACE = chr(0x1F600)


# ---------------------------------------------------------------------------
# Encoding
# ---------------------------------------------------------------------------


class Hex(str):
    """A hex digest, rendered by `enc` in groups of four."""


def enc(value: object) -> object:
    if value is None or isinstance(value, bool):
        return value
    if isinstance(value, Hex):
        return "$hex:" + "_".join(value[i : i + 4] for i in range(0, len(value), 4))
    if isinstance(value, datetime):
        if value.tzinfo is None:
            raise TypeError("naive datetimes never reach the service")
        text = value.isoformat(timespec="microseconds")
        if text[-6] not in "+-":
            raise ValueError(f"offset with seconds: {text}")
        return "$dt:" + text[:-6] + "|" + text[-6:]
    if isinstance(value, str):
        if value.startswith("$"):
            raise ValueError("a plain str must not start with $")
        return value
    if isinstance(value, int):
        if not -(10**8) < value < 10**8:
            raise ValueError("large ints are not part of this corpus")
        return value
    if isinstance(value, (list, tuple)):
        return [enc(item) for item in value]
    if isinstance(value, (set, frozenset)):
        return sorted(enc(item) for item in value)
    if isinstance(value, dict):
        return {key: enc(item) for key, item in value.items()}
    raise TypeError(f"unencodable {type(value).__name__}")


# ---------------------------------------------------------------------------
# Cases
# ---------------------------------------------------------------------------


def consent(person: str, purpose: str, **over: object) -> dict:
    return {
        "person_id": person,
        "purpose": purpose,
        "granted_at": NOW - timedelta(hours=1),
        "revoked_at": None,
        "proposal_expires_at": NOW + timedelta(days=3),
        **over,
    }


def consents_case(
    name: str,
    consents: list[dict],
    participants: list[str],
    now: datetime | None,
    ask: list[str] | None = None,
) -> dict:
    people = list(dict.fromkeys([A, B, C, A.upper(), *participants]))
    ask = people if ask is None else ask
    result = {
        "granted_by": [
            [person, pair_notebook.granted_by(consents, person, now=now)]
            for person in ask
        ],
        "granted_purposes": pair_notebook.granted_purposes(
            consents, participants, now=now
        ),
        "can_bat_doi": pair_notebook.can_bat_doi(consents, participants, now=now),
    }
    if now is not None:
        result["chat_consent_active"] = pair_notebook.chat_consent_active(
            consents, participants, now=now
        )
        # ADR-0034: what each person asked about may see of the two tastes.
        result["gu"] = [
            [person, pair_notebook.gu_hai_nguoi(consents, participants, person, GU, now=now)]
            for person in ask
        ]
    return {
        "fn": "consents",
        "name": name,
        "consents": consents,
        "participants": participants,
        "now": now,
        "ask": ask,
        "result": result,
    }


def paper(state: str, paper_id: str, **over: object) -> dict:
    return {
        "id": paper_id,
        "state": state,
        "current_version": 1,
        "expires_at": None,
        **over,
    }


def proposal(proposal_id: str, completed_at=None, expires_at=None) -> dict:
    return {"id": proposal_id, "completed_at": completed_at, "expires_at": expires_at}


def preview_case(
    name: str, papers: list[dict], proposals: list[dict], now: datetime
) -> dict:
    preview = pair_notebook.xem_truoc_dong_so(papers, proposals, now=now)
    return {
        "fn": "preview",
        "name": name,
        "papers": papers,
        "proposals": proposals,
        "now": now,
        "result": {
            "preview": {**preview, "revision": Hex(preview["revision"])},
            "hieu_luc": [pair_paper.hieu_luc(p, now=now) for p in papers],
            "dang_cho": [pair_notebook.dang_cho(p, now=now) for p in proposals],
        },
    }


def han_case(name: str, now: datetime) -> dict:
    return {
        "fn": "han_de_nghi",
        "name": name,
        "now": now,
        "result": pair_notebook.han_de_nghi(now),
    }


def edge_cases() -> list[dict]:
    two = [A, B]
    both_chat = [consent(A, "doc_chat"), consent(B, "doc_chat")]
    doi = [consent(A, "bat_doi", proposal_id="PR-D"), consent(B, "bat_doi", proposal_id="PR-D")]
    cases = [
        # ADR-0034: taste, per person, inside «Một đôi» only.
        consents_case("taste: friends only", [consent(B, "chia_gu", proposal_id="PG-B")], two, NOW),
        consents_case("taste: couple, nobody shares", doi, two, NOW),
        consents_case("taste: couple, one shares", doi + [consent(B, "chia_gu", proposal_id="PG-B")], two, NOW),
        consents_case(
            "taste: couple, both share",
            doi + [consent(A, "chia_gu", proposal_id="PG-A"), consent(B, "chia_gu", proposal_id="PG-B")],
            two,
            NOW,
        ),
        consents_case(
            "taste: taken back",
            doi + [consent(A, "chia_gu", proposal_id="PG-A"), consent(B, "chia_gu", proposal_id="PG-B", revoked_at=NOW - MICRO)],
            two,
            NOW,
        ),
        consents_case(
            "taste: both share on one proposal is still two people",
            doi + [consent(A, "chia_gu", proposal_id="PG"), consent(B, "chia_gu", proposal_id="PG")],
            two,
            NOW,
        ),
        # QA 23/09: each person filed their own proposal for the same rung.
        # Per purpose that read as «both agreed»; per proposal it is nothing.
        consents_case(
            "two proposals, each granted only by its proposer",
            [
                consent(A, "bat_doi", proposal_id="PR-A"),
                consent(B, "bat_doi", proposal_id="PR-B"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "both on one proposal, a stray third proposal beside it",
            [
                consent(A, "doc_chat", proposal_id="PR-1"),
                consent(B, "doc_chat", proposal_id="PR-1"),
                consent(A, "doc_chat", proposal_id="PR-2"),
            ],
            two,
            NOW,
        ),
        # An agreed rung stands until revoked: its offer window no longer
        # applies once the proposal was completed.
        consents_case(
            "completed proposal past its offer window still stands",
            [
                consent(A, "bat_doi", proposal_id="PR-1", proposal_expires_at=NOW - timedelta(days=1), proposal_completed_at=NOW - timedelta(days=8)),
                consent(B, "bat_doi", proposal_id="PR-1", proposal_expires_at=NOW - timedelta(days=1), proposal_completed_at=NOW - timedelta(days=8)),
            ],
            two,
            NOW,
        ),
        consents_case(
            "uncompleted proposal past its window lapses",
            [
                consent(A, "bat_doi", proposal_id="PR-1", proposal_expires_at=NOW - timedelta(days=1)),
                consent(B, "bat_doi", proposal_id="PR-1", proposal_expires_at=NOW - timedelta(days=1)),
            ],
            two,
            NOW,
        ),
        consents_case("no consents", [], two, NOW),
        consents_case("no consents, nobody", [], [], NOW),
        consents_case("one side only", [consent(A, "doc_chat")], two, NOW),
        consents_case("the other side only", [consent(B, "doc_chat")], two, NOW),
        consents_case("both sides", both_chat, two, NOW),
        consents_case("both sides, participants reversed", both_chat, [B, A], NOW),
        consents_case(
            "revoked at now",
            [consent(A, "doc_chat", revoked_at=NOW), consent(B, "doc_chat")],
            two,
            NOW,
        ),
        consents_case(
            "revoked in the future still revokes",
            [
                consent(A, "doc_chat", revoked_at=NOW + timedelta(days=1)),
                consent(B, "doc_chat"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "not granted",
            [consent(A, "doc_chat", granted_at=None), consent(B, "doc_chat")],
            two,
            NOW,
        ),
        consents_case(
            "granted after now still counts",
            [
                consent(A, "doc_chat", granted_at=NOW + timedelta(days=2)),
                consent(B, "doc_chat"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "no proposal deadline",
            [
                consent(A, "doc_chat", proposal_expires_at=None),
                consent(B, "doc_chat"),
            ],
            two,
            NOW,
        ),
    ]
    for label, delta in (
        ("one microsecond before", -MICRO),
        ("exactly at", timedelta(0)),
        ("one microsecond after", MICRO),
        ("one second before", -timedelta(seconds=1)),
    ):
        expires = NOW + delta
        rows = [
            consent(A, "doc_chat", proposal_expires_at=expires),
            consent(B, "doc_chat", proposal_expires_at=expires),
        ]
        cases.append(consents_case(f"proposal deadline {label} now", rows, two, NOW))
        cases.append(
            consents_case(
                f"proposal deadline {label} now, now at +07:00",
                rows,
                two,
                NOW.astimezone(VIETNAM),
            )
        )
        cases.append(
            consents_case(
                f"proposal deadline {label} now, deadline at -10:00",
                [
                    {**row, "proposal_expires_at": expires.astimezone(HONOLULU)}
                    for row in rows
                ],
                two,
                NOW,
            )
        )
    cases += [
        consents_case(
            "a lapsed proposal is ignored when now is None",
            [
                consent(A, "doc_chat", proposal_expires_at=NOW - timedelta(days=9)),
                consent(B, "doc_chat"),
            ],
            two,
            None,
        ),
        consents_case("now is None, nothing lapsed", both_chat, two, None),
        consents_case(
            "extra person who has not granted",
            both_chat,
            [A, B, C],
            NOW,
        ),
        consents_case(
            "extra person who has granted",
            [*both_chat, consent(C, "doc_chat")],
            [A, B, C],
            NOW,
        ),
        consents_case("one participant", both_chat, [A], NOW),
        consents_case("one participant twice", both_chat, [A, A], NOW),
        consents_case("a repeated participant", both_chat, [A, B, A], NOW),
        consents_case(
            "a stranger's grant does not stand in",
            [consent(A, "doc_chat"), consent(C, "doc_chat")],
            two,
            NOW,
        ),
        consents_case(
            "unknown purposes",
            [
                consent(A, "doc_het"),
                consent(B, "doc_het"),
                consent(A, ""),
                consent(B, ""),
                consent(A, "DOC_CHAT"),
                consent(B, "DOC_CHAT"),
                consent(A, "doc_chat "),
                consent(B, "doc_chat "),
            ],
            two,
            NOW,
        ),
        consents_case(
            "a lower rung never implies a higher one",
            [consent(A, "lap_so"), consent(B, "lap_so")],
            two,
            NOW,
        ),
        consents_case(
            "bat_doi from both, doc_chat from one",
            [
                consent(A, "lap_so"),
                consent(A, "bat_doi"),
                consent(A, "doc_chat"),
                consent(B, "lap_so"),
                consent(B, "bat_doi"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "a lapsed row and a live row of one purpose",
            [
                consent(A, "doc_chat", proposal_expires_at=NOW - timedelta(days=8)),
                consent(A, "doc_chat"),
                consent(B, "doc_chat"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "a revoked row and a live row of one purpose",
            [
                consent(A, "doc_chat", revoked_at=NOW - timedelta(days=1)),
                consent(A, "doc_chat"),
                consent(B, "doc_chat"),
            ],
            two,
            NOW,
        ),
        consents_case(
            "participants from the cycle",
            [consent(A, "doc_chat"), consent(B, "doc_chat")],
            [B, A],
            NOW,
        ),
        consents_case(
            "participants from the members after a third joined",
            [consent(A, "doc_chat"), consent(B, "doc_chat")],
            [A, B, C],
            NOW,
        ),
        consents_case(
            "participants from the members after one left",
            [consent(A, "doc_chat"), consent(B, "doc_chat")],
            [A, C],
            NOW,
        ),
        consents_case(
            "ids compare as written",
            [consent(A.upper(), "doc_chat"), consent(B, "doc_chat")],
            two,
            NOW,
        ),
        consents_case(
            "unicode and empty ids",
            [consent("", "bat_doi"), consent("ngu" + E_DOT_CIRCUMFLEX, "bat_doi")],
            ["", "ngu" + E_DOT_CIRCUMFLEX],
            NOW,
        ),
    ]

    papers = [
        paper("nhap", "p1"),
        paper("da_gui", "p2"),
        paper("da_xem", "p3"),
        paper("chot", "p4"),
        paper("da_di", "p5"),
        paper("da_giu", "p6"),
        paper("het_han", "p7"),
    ]
    cases += [
        preview_case("empty", [], [], NOW),
        preview_case("three kinds of cost", papers, [], NOW),
        preview_case("reading order is not a change", list(reversed(papers)), [], NOW),
        preview_case(
            "every state and an unknown one",
            [paper(state, f"s{i}") for i, state in enumerate(pair_paper.PAPER_STATES)]
            + [paper("la", "s-la")],
            [],
            NOW,
        ),
        preview_case(
            "proposals waiting, done and lapsed",
            [],
            [
                proposal("d1", None, NOW + timedelta(days=1)),
                proposal("d2", NOW, NOW + timedelta(days=1)),
                proposal("d3", None, NOW - timedelta(seconds=1)),
                proposal("d4", None, None),
                proposal("d5", NOW + timedelta(days=5), None),
            ],
            NOW,
        ),
        preview_case(
            "ids that need care",
            [
                paper("da_gui", "p:1"),
                paper("chot", "p\n1"),
                paper("nhap", CJK_MIDDLE),
                paper("nhap", GRINNING_FACE),
                paper("da_xem", ""),
                paper("da_xem", "p1"),
                paper("da_xem", "p1"),
            ],
            [proposal("p1:da_xem"), proposal(""), proposal(E_DOT_CIRCUMFLEX)],
            NOW,
        ),
    ]
    for label, delta in (
        ("one microsecond before", -MICRO),
        ("exactly at", timedelta(0)),
        ("one microsecond after", MICRO),
    ):
        expires = NOW + delta
        cases.append(
            preview_case(
                f"deadline {label} now",
                [
                    paper(state, f"x-{state}", expires_at=expires)
                    for state in ("nhap", "da_gui", "dong_y", "chot", "da_di", "huy")
                ],
                [
                    proposal("dn-x", None, expires),
                    proposal("dn-y", None, expires.astimezone(VIETNAM)),
                ],
                NOW,
            )
        )
    cases += [
        han_case("now", NOW),
        han_case("now at +07:00", NOW.astimezone(VIETNAM)),
        han_case("microseconds", NOW + timedelta(microseconds=123457)),
        han_case(
            "across a year end", datetime(2030, 12, 28, 23, 59, 59, 999999, tzinfo=UTC)
        ),
        han_case(
            "across February in a leap year",
            datetime(2028, 2, 25, 1, 0, tzinfo=HONOLULU),
        ),
    ]
    return cases


# ---------------------------------------------------------------------------
# Fuzz
# ---------------------------------------------------------------------------

PEOPLE = (A, B, C, A.upper(), "", "ngu" + E_DOT_CIRCUMFLEX)
PURPOSES = ("lap_so", "bat_doi", "doc_chat", "doc_chat", "doc_het", "", "DOC_CHAT", "chia_gu", "bat_doi")
DELTAS = (
    -timedelta(days=3),
    -timedelta(seconds=1),
    -MICRO,
    timedelta(0),
    MICRO,
    timedelta(seconds=1),
    timedelta(days=3),
    timedelta(days=7),
)
ZONES = (UTC, UTC, VIETNAM, HONOLULU)
PAPER_IDS = ("p1", "p2", "p3", "p:1", "p\n2", CJK_MIDDLE, "", "dn:p1")
PAPER_STATES = (*pair_paper.PAPER_STATES, "la")


def fuzz_cases() -> list[dict]:
    rng = random.Random(SEED)

    def instant(optional: float = 0.0) -> datetime | None:
        if rng.random() < optional:
            return None
        base = NOW + rng.choice(DELTAS)
        return base.astimezone(rng.choice(ZONES))

    cases = []
    for index in range(FUZZ_CASES):
        roll = rng.random()
        if roll < 0.7:
            rows = [
                {
                    "person_id": rng.choice(PEOPLE),
                    "purpose": rng.choice(PURPOSES),
                    "granted_at": instant(0.2),
                    "revoked_at": instant(0.75),
                    "proposal_expires_at": instant(0.1),
                }
                for _ in range(rng.randint(0, 7))
            ]
            size = rng.choice((0, 1, 2, 2, 2, 2, 3))
            participants = [rng.choice(PEOPLE[:4]) for _ in range(size)]
            if rng.random() < 0.6:
                # Mostly agreed: without this half, two live grants of one
                # purpose from two distinct participants almost never meet.
                if rng.random() < 0.7:
                    participants = rng.sample([A, B], 2)
                rows = rows[: rng.randint(0, 3)]
                for person in participants:
                    rows.append(
                        {
                            "person_id": person,
                            "purpose": rng.choice(("doc_chat", "doc_chat", "bat_doi", "bat_doi", "chia_gu")),
                            "granted_at": instant(0.05),
                            "revoked_at": instant(0.9),
                            "proposal_expires_at": rng.choice(
                                (None, NOW + timedelta(days=3), instant())
                            ),
                        }
                    )
                    if rng.random() < 0.5:
                        # ADR-0034: each person's own taste switch, own proposal.
                        rows.append(
                            {
                                "person_id": person,
                                "purpose": "chia_gu",
                                "granted_at": instant(0.05),
                                "revoked_at": instant(0.9),
                                "proposal_expires_at": NOW + timedelta(days=3),
                                "proposal_id": "PG-" + person[:4],
                            }
                        )
                rng.shuffle(rows)
            cases.append(
                consents_case(
                    f"fuzz {index}", rows, participants, instant(0.1), ask=None
                )
            )
        elif roll < 0.95:
            papers = [
                {
                    "id": rng.choice(PAPER_IDS),
                    "state": rng.choice(PAPER_STATES),
                    "current_version": rng.randint(1, 3),
                    "expires_at": instant(0.3),
                }
                for _ in range(rng.randint(0, 5))
            ]
            proposals = [
                {
                    "id": rng.choice(PAPER_IDS),
                    "completed_at": instant(0.7),
                    "expires_at": instant(0.2),
                }
                for _ in range(rng.randint(0, 3))
            ]
            cases.append(preview_case(f"fuzz {index}", papers, proposals, instant()))
        else:
            cases.append(han_case(f"fuzz {index}", instant()))
    return cases


# ---------------------------------------------------------------------------
# Rendering
# ---------------------------------------------------------------------------


def modes() -> dict[str, str]:
    out = {"edges": TARGET.format(mode="pair_notebook")}
    for shard in range(FUZZ_SHARDS):
        out[f"fuzz-{shard}"] = TARGET.format(mode=f"pair_notebook_fuzz_{shard}")
    return out


def render(mode: str) -> dict:
    if mode == "edges":
        return {
            "mode": mode,
            "constants": {
                "cycle_states": pair_notebook.CYCLE_STATES,
                "consent_purposes": pair_notebook.CONSENT_PURPOSES,
                "constraint_kinds": pair_notebook.CONSTRAINT_KINDS,
                "han_de_nghi_seconds": int(pair_notebook.HAN_DE_NGHI.total_seconds()),
                "notebook_error_code": pair_notebook.NotebookError(
                    "notebook_revision_stale"
                ).code,
                "all": list(pair_notebook.__all__),
            },
            "cases": [
                {key: enc(value) for key, value in case.items()}
                for case in edge_cases()
            ],
        }
    shard = int(mode.removeprefix("fuzz-"))
    if not 0 <= shard < FUZZ_SHARDS:
        raise SystemExit(f"no shard {shard}")
    every = fuzz_cases()
    size = -(-len(every) // FUZZ_SHARDS)
    return {
        "mode": mode,
        "fuzz": {"shard": shard, "shards": FUZZ_SHARDS, "total": len(every)},
        "cases": [
            {key: enc(value) for key, value in case.items()}
            for case in every[shard * size : (shard + 1) * size]
        ],
    }


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: render_pair_notebook_goldens.py MODE|--list")
    if sys.argv[1] == "--list":
        for mode, path in modes().items():
            print(f"{mode}\t{path}")
        return
    if sys.argv[1] not in modes():
        raise SystemExit(f"unknown mode {sys.argv[1]!r}")
    text = json.dumps(render(sys.argv[1]), indent=1, ensure_ascii=False) + "\n"
    for number, line in enumerate(text.splitlines(), 1):
        for rule in GUARD_RULES:
            if rule.search(line):
                raise SystemExit(
                    f"line {number} would trip the repository guard: {line!r}"
                )
    if len(text.encode("utf-8")) >= 1 << 20:
        raise SystemExit("rendered file reaches one MiB; add shards")
    sys.stdout.write(text)


if __name__ == "__main__":
    main()
