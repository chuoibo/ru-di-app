"""After Maestro flow 40: count the group AI's answers and check they are grounded.

`scripts/mobile_native.sh` (kiem_may_chu_sau_40) runs this against the live API
after flow 40 has asked `/plan` in the «Plan QA» group. It prints one line,
``answers|unknown_places|blind|kinds``, which the shell reads.

Two shapes of answer reach a room. Since slice 7 (ADR-0039, proposed) an app
that names a trigger gets a `tra_loi` reply: ``{"kind": "tra_loi", "payload":
{"tac_gia": "rudi-ai", "phan": [card, ...]}}``, where every part is one of the
cards below. An app from before that change gets the card itself, unwrapped.
Both are counted, and grounding is checked inside every part: a check that only
reads top-level ``text|places|itinerary`` cards reports "no AI card at all" for
a `/plan` answered in the thread, and never looks at a single place id in it.

Grounded means every place id a card names is in the public catalogue
(GET /places). A places or itinerary part from which no id could be read is
counted as blind rather than as clean: that is how the first version of this
check passed for a week while reading keys nobody sends.

Pure where it can be (`dem_the_ai`), so tests/test_kiem_the_ai_sau_40.py holds
it to synthetic messages without a device, a server or a model.
"""

from __future__ import annotations

import json
import sys
import urllib.request
from collections.abc import Iterable

#: The card kinds an answer part may be; the same list GroundCard accepts.
LOAI_THE = ("text", "places", "itinerary")


def _la_dict(value: object) -> bool:
    return isinstance(value, dict)


def phan_cua_tin(card: object) -> tuple[str, list[dict]] | None:
    """The cards to check inside one AI message, and a label for its shape.

    None when the message is not an answer this check knows: a poll, an
    expense draft, anything else the AI engine never publishes for /plan.
    """
    if not _la_dict(card):
        return None
    kind = card.get("kind")
    if kind in LOAI_THE:
        return kind, [card]
    if kind != "tra_loi":
        return None
    payload = card.get("payload")
    phan = payload.get("phan") if _la_dict(payload) else None
    parts = (
        [p for p in phan if _la_dict(p) and p.get("kind") in LOAI_THE]
        if isinstance(phan, list)
        else []
    )
    return "tra_loi[" + "+".join(sorted({p["kind"] for p in parts})) + "]", parts


def _place_ids(card: dict) -> list[object]:
    payload = card.get("payload")
    if not _la_dict(payload):
        return []
    ids = [pl.get("id") for pl in payload.get("places") or [] if _la_dict(pl)]
    ids += [
        st["place"].get("id") if _la_dict(st.get("place")) else None
        for st in payload.get("stops") or []
        if _la_dict(st)
    ]
    return ids


def dem_the_ai(
    messages: Iterable[dict], catalogue: set[object]
) -> tuple[int, int, int, str]:
    """Count answers, place ids outside the catalogue, and blind parts.

    An answer is an ai_card with no author whose card is one of LOAI_THE or a
    `tra_loi` wrapping them. A `tra_loi` with no readable part is one answer
    and one blind part: the check cannot see into it, which is not "clean".
    """
    answers = 0
    unknown = 0
    blind = 0
    kinds: set[str] = set()
    for message in messages:
        if message.get("kind") != "ai_card" or message.get("author_id") is not None:
            continue
        found = phan_cua_tin(message.get("card"))
        if found is None:
            continue
        label, parts = found
        answers += 1
        kinds.add(label)
        if not parts:
            blind += 1
            continue
        for part in parts:
            ids = _place_ids(part)
            # An entry whose id cannot be read is a shape the check cannot see
            # into: GroundCard always gives a catalogue place with an id, so a
            # missing one is drift, and it counts as blind, never as clean.
            if part["kind"] in ("places", "itinerary") and (
                not ids or any(not i for i in ids)
            ):
                blind += 1
            unknown += sum(1 for i in ids if i and i not in catalogue)
    return answers, unknown, blind, ",".join(sorted(kinds))


def main(argv: list[str]) -> int:
    goc, tok, ctx = argv[1:4]

    def get(path: str) -> dict:
        req = urllib.request.Request(
            goc + path, headers={"Authorization": "Bearer " + tok}
        )
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.load(r)

    messages = get(f"/contexts/{ctx}/messages?limit=50").get("messages", [])
    catalogue = {p.get("id") for p in get("/places").get("places", [])}
    print("%d|%d|%d|%s" % dem_the_ai(messages, catalogue))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
