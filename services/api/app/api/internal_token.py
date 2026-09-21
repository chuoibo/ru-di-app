"""The shared secret that lets Go call the brain, and nobody else.

The brain is not a public API. It is an internal HTTP seam for the Go core
(ADR-0029 §2.7): no OpenAPI entry, no database credential, and a token that
must be set or the process refuses to start. An empty value is the dangerous
configuration — it looks like a host that locked the door and is a host that
left it off the hinges — so absence and whitespace are the same refusal.

The token is compared with `hmac.compare_digest`. A mismatch and a missing
header are the same 401: the caller learns that the door is shut, not which
half of the handshake they got wrong.
"""

from __future__ import annotations

from collections.abc import Mapping

__all__ = [
    "INTERNAL_TOKEN_ENV_VAR",
    "INTERNAL_TOKEN_HEADER",
    "InternalTokenMissing",
    "resolve_internal_token",
    "tokens_match",
]

INTERNAL_TOKEN_ENV_VAR = "MOBILE_INTERNAL_TOKEN"
INTERNAL_TOKEN_HEADER = "X-Internal-Token"
# Letter-only so repo-guard never mistakes a test secret for an account number.
TEST_TOKEN = "test-brain-token"


class InternalTokenMissing(RuntimeError):
    """The process was asked to start without a brain token."""


def resolve_internal_token(environ: Mapping[str, str] | None = None) -> str:
    """The token this process will accept.

    Absent, empty, or only whitespace refuses to start. Surrounding whitespace
    is stripped so a compose file with a trailing newline is still a token,
    not a second state. The value is otherwise taken as-is: this is a secret,
    not a mode name, so case is significant.
    """

    import os

    env = os.environ if environ is None else environ
    raw = env.get(INTERNAL_TOKEN_ENV_VAR, "").strip()
    if not raw:
        raise InternalTokenMissing(
            f"{INTERNAL_TOKEN_ENV_VAR} must be set to a non-empty token; "
            "refusing to start with the brain door unlocked"
        )
    return raw


def tokens_match(expected: str, offered: str | None) -> bool:
    """True only when `offered` is exactly the configured token.

    Missing, empty, and wrong are the same answer. Length is not leaked by
    raising: `hmac.compare_digest` on str returns False when the lengths
    differ.
    """

    import hmac

    if offered is None:
        return False
    return hmac.compare_digest(expected, offered)
