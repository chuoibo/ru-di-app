"""Dọn story đã hết hạn quá lâu (L4, ADR-0022 §2.3).

Story biến mất khỏi sản phẩm ngay khi `expires_at` qua -- mọi truy vấn đều lọc
theo `now` -- nên script này không làm story «hết hạn». Nó chỉ dọn hàng và
file ảnh mà không ai còn đọc được nữa, sau một thời gian ân hạn (mặc định 7
ngày), để đĩa không giữ mãi ảnh của những gì đã qua.

Cần `MOBILE_DATABASE_URL` (như `genesis_session.py`) và `MOBILE_MEDIA_ROOT`
nếu muốn xoá cả file. `--dry-run` chỉ in số sẽ xoá.

    python3 scripts/purge_expired_stories.py --older-than-days 7 --dry-run
"""

from __future__ import annotations

import argparse
import sys
from datetime import UTC, datetime, timedelta
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO_ROOT / "services" / "api"))

from app.db.session import get_session_factory  # noqa: E402
from app.db.story_purge import purge_expired_stories, remove_purged_files  # noqa: E402
from app.media.storage import PhotoStorage  # noqa: E402


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--older-than-days", type=int, default=7)
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--keep-files",
        action="store_true",
        help="xoá hàng nhưng để file ảnh lại trên đĩa",
    )
    args = parser.parse_args(argv)
    if args.older_than_days < 0:
        parser.error("--older-than-days phải >= 0")

    factory = get_session_factory()
    with factory() as session:
        report = purge_expired_stories(
            session,
            now=datetime.now(UTC),
            older_than=timedelta(days=args.older_than_days),
            dry_run=args.dry_run,
        )
        if args.dry_run:
            session.rollback()
        else:
            session.commit()
    # Files only after the rows are committed: a failed commit must not leave
    # a photograph missing while its row still promises it.
    removed = 0
    if not args.dry_run and not args.keep_files:
        removed = remove_purged_files(PhotoStorage(), report.storage_keys)
    verb = "sẽ xoá" if report.dry_run else "đã xoá"
    print(
        f"{verb} {report.stories} story hết hạn và {report.images} ảnh mồ côi"
        + ("" if report.dry_run else f" ({removed} file gỡ khỏi đĩa)")
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
