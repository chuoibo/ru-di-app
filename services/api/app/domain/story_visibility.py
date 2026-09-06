"""24-hour stories (L4, ADR-0022 §2.3): how long one lives and who sees it.

A story is a photograph of one's own with an optional caption, addressed to
one's friends, gone after 24 hours. Three rules live here and nowhere else:

- **Lifetime.** `expires_at` is computed from `created_at` by this module and
  written to the row; the database has no default for it and no clock of its
  own. Every query that asks «still live?» receives `now` from the service, so
  a test can stand on either side of the boundary without waiting a day.
- **Audience.** One word, `friends`. The tuple has one member on purpose: the
  CHECK on `stories.audience` is the second spelling, and widening it later is
  a decision (ADR), not a string.
- **Who may view.** The author always; anybody else exactly when they are the
  author's friend *now*, not blocked, and the story is still live. Facts are
  handed in as booleans somebody else proved, for the reason
  `post_audience.can_read` gives: friendship is read from `friend_requests`
  at the moment of the read, never from the caller.

Pure functions over plain dicts and aware datetimes. A naive datetime is
refused rather than assumed UTC: «still live?» at the wrong offset is the
difference between a story that is gone and one that lingers seven hours.
"""

from __future__ import annotations

from datetime import datetime, timedelta

__all__ = [
    "DEFAULT_STORY_AUDIENCE",
    "MAX_CAPTION_LENGTH",
    "STORY_AUDIENCES",
    "STORY_TTL",
    "StoryError",
    "can_view",
    "check_caption",
    "expires_at_for",
    "is_live",
    "order_authors",
]

#: 24 hours, and a constant rather than a parameter: the product promise is
#: the number, and a caller that could pass a longer one would be a caller
#: that could keep a story up.
STORY_TTL = timedelta(hours=24)

#: ADR-0022 §2.3. The one audience a story has. Mirrored by the CHECK on
#: `stories.audience`.
STORY_AUDIENCES = ("friends",)
DEFAULT_STORY_AUDIENCE = "friends"

#: Mirrored by the CHECK on `stories.caption` and the wire schema.
MAX_CAPTION_LENGTH = 200


class StoryError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def _aware(value: object) -> datetime:
    if (
        not isinstance(value, datetime)
        or value.tzinfo is None
        or value.utcoffset() is None
    ):
        raise StoryError("NAIVE_DATETIME")
    return value


def expires_at_for(created_at: datetime) -> datetime:
    """When a story written at `created_at` stops being shown."""
    return _aware(created_at) + STORY_TTL


def is_live(story: dict, now: datetime) -> bool:
    """Strictly before the deadline. At `now == expires_at` the story is gone;
    the SQL spelling is `expires_at > :now`, the same strictness."""
    return _aware(story.get("expires_at")) > _aware(now)


def check_caption(caption: object) -> None:
    """None or a string of at most `MAX_CAPTION_LENGTH` characters."""
    if caption is None:
        return
    if not isinstance(caption, str):
        raise StoryError("CAPTION_NOT_TEXT")
    if len(caption) > MAX_CAPTION_LENGTH:
        raise StoryError("CAPTION_TOO_LONG")


def can_view(
    story: dict,
    *,
    reader_id: str,
    is_friend: bool,
    is_blocked: bool,
    now: datetime,
) -> bool:
    """Whether one reader may see one story. The only place this is decided.

    The author first, unconditionally: their own expired story is still theirs
    to delete. Everybody else needs all three of friend, not blocked, live --
    and an audience word this module does not know fails closed, as
    `post_audience.can_read` does. `is_blocked` is carried from L5 onward;
    until then callers pass False, and the argument being required is what
    stops the L5 rule from being forgotten here.
    """
    if story.get("audience") not in STORY_AUDIENCES:
        return False
    if reader_id == story.get("author_id"):
        return True
    if is_blocked:
        return False
    if not is_live(story, now):
        return False
    return bool(is_friend)


def order_authors(groups: list[dict]) -> list[dict]:
    """The rail's order: one's own first, then authors with something unseen
    (newest first), then the rest (newest first). Stable for ties, so two
    authors whose newest stories share a timestamp keep the caller's order.

    Each group is `{"mine": bool, "all_seen": bool, "latest_at": datetime}`
    plus whatever the caller carries along; nothing else is read.
    """

    def key(group: dict) -> tuple[int, int, float]:
        mine = 0 if group.get("mine") else 1
        unseen = 0 if not group.get("all_seen") else 1
        latest = _aware(group.get("latest_at")).timestamp()
        return (mine, unseen, -latest)

    return sorted(groups, key=key)
