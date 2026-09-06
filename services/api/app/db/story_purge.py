"""Sweep stories that expired long ago (L4, ADR-0022 §2.3).

Expiry is a *read* rule -- `expires_at > :now` in every query -- so nothing has
to run for a story to disappear from the product. What this module does is
housekeeping: rows and image files that no reader can reach any more, kept for
a grace period in case somebody asks what was there, then removed.

Two rules keep it from removing something still in use:

- Only stories whose deadline is more than `older_than` in the past are
  candidates. A story that expired a minute ago is not.
- A personal photograph is deleted only when nothing points at it any more --
  no post and no story, live or not -- AND it is itself older than the grace
  period. A photo shared between a post and an expired story stays, because
  the post still shows it; a photo uploaded a minute ago stays too, because
  the story that will show it is still being written (review #577 S2).

Rows first, files second, and only after the caller has committed: the
function deletes rows and names the files, `remove_purged_files` unlinks them.
A file that is gone while its row survived a failed commit is a photograph
lost; a row that is gone while its file lingers is a few kilobytes on a disk
(review #577 S3).

Run from `scripts/purge_expired_stories.py`; the ORM work is here so the
Postgres tier can prove the rules without a subprocess. `dry_run` reports
what would go and touches nothing.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta

from sqlalchemy import delete, select
from sqlalchemy.orm import Session

from app.db.models import Post, Story, UploadedImage
from app.media.storage import PhotoStorage

DEFAULT_GRACE = timedelta(days=7)


@dataclass(frozen=True, slots=True)
class PurgeReport:
    stories: int
    images: int
    storage_keys: tuple[str, ...]
    dry_run: bool


def _personal_url(image: UploadedImage) -> str:
    return f"/people/{image.owner_person_id}/photos/{image.id}"


def purge_expired_stories(
    session: Session,
    *,
    now: datetime,
    older_than: timedelta = DEFAULT_GRACE,
    dry_run: bool = False,
) -> PurgeReport:
    """Delete stories expired for longer than `older_than`, then the personal
    photographs older than that which nothing references. Rows only: the
    storage keys of the deleted photographs come back in the report for
    `remove_purged_files`, to be called after the caller's commit."""
    if now.tzinfo is None or now.utcoffset() is None:
        raise ValueError("purge needs an aware `now`")
    cutoff = now - older_than
    stale_ids = list(
        session.scalars(select(Story.id).where(Story.expires_at <= cutoff))
    )
    if not dry_run and stale_ids:
        session.execute(delete(Story).where(Story.id.in_(stale_ids)))
        session.flush()

    referenced = set(
        session.scalars(select(Post.image_url).where(Post.image_url.is_not(None)))
    )
    live_story_urls = select(Story.image_url)
    if dry_run and stale_ids:
        live_story_urls = live_story_urls.where(Story.id.not_in(stale_ids))
    referenced |= set(session.scalars(live_story_urls))

    orphans = [
        image
        for image in session.scalars(
            select(UploadedImage).where(
                UploadedImage.purpose == "personal",
                UploadedImage.created_at <= cutoff,
            )
        )
        if _personal_url(image) not in referenced
    ]
    keys = tuple(image.storage_key for image in orphans)
    if not dry_run and orphans:
        session.execute(
            delete(UploadedImage).where(
                UploadedImage.id.in_([image.id for image in orphans])
            )
        )
        session.flush()
    return PurgeReport(
        stories=len(stale_ids), images=len(orphans), storage_keys=keys, dry_run=dry_run
    )


def remove_purged_files(storage: PhotoStorage, keys: tuple[str, ...]) -> int:
    """Unlink the files a committed purge named. Answers how many were there;
    a file already gone is housekeeping already done, not an error."""
    return sum(1 for key in keys if storage.delete(key))
