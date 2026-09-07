"""What one person can report, and for what (ADR-0023 §2.4).

Two closed vocabularies and one length limit. They are here, in the domain,
for the reason every other vocabulary in this codebase is: the CHECK on the
table, the wire enum and the client's list are three spellings of the same
tuple, and a test compares them instead of a reviewer.

There is no admin screen in v1 and this module knows nothing about one. A
report is a row an operator reads; it grants nobody a power over anybody, so
nothing here decides anything about the target.

Pure: `dict` in, `dict` out, `ReportError` on refusal.
"""

from __future__ import annotations

__all__ = [
    "MAX_NOTE_LENGTH",
    "REASONS",
    "TARGET_TYPES",
    "ReportError",
    "validate_report",
]

#: What may be reported. `person` is a person, the other four are things a
#: person wrote; the row records which, so an operator can find it.
TARGET_TYPES = ("person", "post", "message", "comment", "story")

#: Why. Closed on purpose: free text as the only reason is a field nobody can
#: count, and «other» plus a note is what free text is for.
REASONS = ("spam", "harassment", "inappropriate", "impersonation", "other")

MAX_NOTE_LENGTH = 500


class ReportError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def validate_report(
    *, target_type: object, reason: object, note: object = None
) -> dict:
    """The three fields, checked, as the repository wants them.

    The target's id is not checked here: whether a post exists is a question
    for a table, and a report about a thing that was deleted a second ago is
    still a report worth keeping.
    """
    if target_type not in TARGET_TYPES:
        raise ReportError("UNKNOWN_TARGET_TYPE")
    if reason not in REASONS:
        raise ReportError("UNKNOWN_REASON")
    if note is None:
        cleaned = None
    elif isinstance(note, str):
        cleaned = note.strip() or None
        if cleaned is not None and len(cleaned) > MAX_NOTE_LENGTH:
            raise ReportError("NOTE_TOO_LONG")
    else:
        raise ReportError("NOTE_NOT_TEXT")
    return {"target_type": target_type, "reason": reason, "note": cleaned}
