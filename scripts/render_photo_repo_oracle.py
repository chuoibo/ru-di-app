#!/usr/bin/env python3
"""Oracle for the Go port of the W6 repository methods (the photo routes).

ADR-0029 section 2.4. The six photo routes reach seven
SqlAlchemyApiRepository methods: is_member and get_person_image, ported by
earlier waves, and shares_active_context, create_uploaded_image,
get_context_image, get_latest_avatar and person_image_visible_to, ported now.
This driver is `render_guest_repo_oracle.py` -- itself the W5, W4, W3, W2 and
pilot drivers stacked -- with those methods added to its call table; the
input, the output, the per-case Session and the statement log are the base
script's, so services/core/internal/repo/photos_oracle_postgres_test.go
compares the same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -v "$PWD/scripts":/oracle:ro --entrypoint python <api image> \\
      /oracle/render_photo_repo_oracle.py < cases.json

`create_uploaded_image` passes `purpose` only when the case gives one, so a
case can reach the Python default ("group") the way upload_context_photo does.

Two calls are sequences, because the read needs the id the write generated:

* `flow.upload_and_read_context_photo` is create_uploaded_image then
  get_context_image(record.context_id, record.id).
* `flow.upload_and_read_person_photo` is create_uploaded_image then
  get_person_image(record.owner_person_id, record.id).
"""

from __future__ import annotations

import sys

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_guest_repo_oracle  # noqa: E402,F401  (W5 to pilot calls, conflict tagging)

_uuid = base._uuid
_instant = base._instant


def _optional_uuid(value):
    return None if value is None else _uuid(value)


def _create_uploaded_image(repository, args: dict):
    extra = {}
    if "purpose" in args:
        extra["purpose"] = args["purpose"]
    return repository.create_uploaded_image(
        storage_key=args["storage_key"],
        context_id=_optional_uuid(args["context_id"]),
        owner_person_id=_optional_uuid(args["owner_person_id"]),
        uploaded_by_id=_uuid(args["uploaded_by_id"]),
        content_type=args["content_type"],
        byte_size=args["byte_size"],
        width=args["width"],
        height=args["height"],
        now=_instant(args["now"]),
        **extra,
    )


def _upload_and_read_context_photo(repository, args: dict):
    record = _create_uploaded_image(repository, args)
    return (record, repository.get_context_image(record.context_id, record.id))


def _upload_and_read_person_photo(repository, args: dict):
    record = _create_uploaded_image(repository, args)
    return (record, repository.get_person_image(record.owner_person_id, record.id))


base.CALLS.update(
    {
        "shares_active_context": lambda repository, args: (
            repository.shares_active_context(
                _uuid(args["viewer_id"]), _uuid(args["subject_id"])
            )
        ),
        "create_uploaded_image": _create_uploaded_image,
        "get_context_image": lambda repository, args: repository.get_context_image(
            _uuid(args["context_id"]), _uuid(args["image_id"])
        ),
        "get_latest_avatar": lambda repository, args: repository.get_latest_avatar(
            _uuid(args["person_id"])
        ),
        "person_image_visible_to": lambda repository, args: (
            repository.person_image_visible_to(
                _uuid(args["person_id"]),
                _uuid(args["image_id"]),
                _uuid(args["reader_id"]),
                now=_instant(args["now"]),
            )
        ),
        "flow.upload_and_read_context_photo": _upload_and_read_context_photo,
        "flow.upload_and_read_person_photo": _upload_and_read_person_photo,
    }
)


if __name__ == "__main__":
    base.main()
