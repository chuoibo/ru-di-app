"""The sticker vocabulary (ADR-0021 §2.1): a closed list the server owns.

A sticker message carries only an id in `body`; the picture lives in the
client as a vector component keyed by that id. Three copies of this list
exist on purpose -- here, `packages/shared/stickers.json`, and the client's
`src/rudi/chat/sticker.ts` -- and a repo-root test reads all three as text
and refuses the day they disagree. Nothing here is decoration: an id outside
this tuple is a 422 on the write and a «?» tile on the read.

Ids are ASCII slugs. No digits-only ids and nothing ten characters of digits
long, so the repo guard never mistakes one for an account number.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

__all__ = ["STICKER_ID_PATTERN", "STICKERS", "STICKER_IDS", "Sticker", "is_sticker"]

#: Mirrored by the CHECK on `messages.body` for `kind = 'sticker'`.
STICKER_ID_PATTERN = re.compile(r"^[a-z0-9-]{1,32}$")


@dataclass(frozen=True, slots=True)
class Sticker:
    id: str
    label: str


STICKERS: tuple[Sticker, ...] = (
    Sticker("di-thoi", "Đi thôi!"),
    Sticker("an-gi", "Ăn gì?"),
    Sticker("cafe-khong", "Cà phê không?"),
    Sticker("ok-chot", "OK, chốt!"),
    Sticker("cho-ti", "Chờ tí"),
    Sticker("ket-xe", "Kẹt xe"),
    Sticker("tra-tien-ne", "Trả tiền nè"),
    Sticker("tuyet-voi", "Tuyệt vời"),
)

STICKER_IDS: frozenset[str] = frozenset(sticker.id for sticker in STICKERS)


def is_sticker(value: object) -> bool:
    """True only for an id in the vocabulary. Not a pattern check: the pattern
    is the database's floor, the tuple is the product's ceiling."""
    return isinstance(value, str) and value in STICKER_IDS
