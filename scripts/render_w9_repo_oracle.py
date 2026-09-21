#!/usr/bin/env python3
"""Oracle for the Go port of the W9 repository methods (the root of trust).

ADR-0029 section 2.4. The seven routes of `routes/sessions.py` and
`routes/auth.py` reach thirteen SqlAlchemyApiRepository methods the earlier
waves had not ported, plus get_outing_invite_by_digest, get_outing,
ensure_invited_membership, get_person, create_person, get_account_identity,
list_person_context_summaries and get_friend_edge, ported before. This driver
is `render_w7_repo_oracle.py` -- itself every earlier wave's driver stacked --
with those methods and the seven routes added to its call table; the input, the
output, the per-step identity map and the statement log are the base script's,
so services/core/internal/repo/auth_repo_oracle_postgres_test.go compares the
same way the earlier oracles do.

    docker run --rm -i --network host -e ORACLE_DATABASE_URL=... \\
      -e MOBILE_PERSON_ID_KEY=... -v "$PWD/scripts":/oracle:ro \\
      --entrypoint python <api image> /oracle/render_w9_repo_oracle.py < cases.json

Four things this driver does that the W7 one does not, each so the two sides can
be compared at all where a secret is involved:

* an OtpChallengeRecord carries two `bytes`, which the base tagger refuses.
  They are tagged as hexadecimal; the frozensets ActorGrants answers with were
  already tagged by render_social_repo_oracle.py, sorted, and need nothing.
* `secrets.token_urlsafe` answers the case's `session_token` for the length of
  a route call that mints one, so the digest the two sides write into
  `account_sessions` is the digest of one token and the table dump compares it
  byte for byte. Nothing anywhere records the token itself.
* `uuid.uuid4`, as `app.api.service` sees it, answers the case's `ids` in order.
  The OTP door salts a code's digest with the challenge id it just minted, so a
  free-running uuid4 would put two different digests in the two dumps and the
  comparison would be masked exactly where the secret is. The ids SQLAlchemy
  mints from a column default (a session row, an identity row) are untouched and
  stay bound as `<generated-N>`.
* the SMS gateway and the Google verifier are stubs the case configures: a
  gateway that refuses (`sms_fails`) reaches the branch that burns the
  challenge it just wrote, and a token the verifier does not vouch for
  (`google_invalid`) is the 401 that must issue no statement at all.

No telephone number and no bearer token is written down here. A case carries the
number the Go side built at run time, and the server stores only digests of
both.
"""

from __future__ import annotations

import sys
import uuid as _real_uuid

sys.path.insert(0, "/oracle")
sys.path.insert(0, "/srv")

import render_repo_oracle as base  # noqa: E402
import render_w7_repo_oracle  # noqa: E402,F401  (W7 to pilot calls, per-step identity map)

from app.api import service as api_service  # noqa: E402
from app.api.google_identity import GoogleClaims, GoogleTokenInvalid  # noqa: E402
from app.api.sms import SmsDeliveryError  # noqa: E402

_uuid = base._uuid
_instant = base._instant
_chained_tag = base.tag


def tag(value):
    """The chain's tagger, plus the one shape only this wave's records carry.

    `bytes` -- a challenge's two digests -- is tagged as hex, which is what the
    value is. The only reason it can be compared at all is that both sides
    derive it from the same run key and the same pinned challenge id.

    ActorGrants' two frozensets need nothing here: render_social_repo_oracle.py
    already tags a set as its members sorted by their tagged JSON text.
    """
    if isinstance(value, bytes | bytearray | memoryview):
        return {"bytes": bytes(value).hex()}
    return _chained_tag(value)


base.tag = tag


def _optional_uuid(value):
    return None if value is None else _uuid(value)


PLACEHOLDER_ID_TOKEN = "id-token-mau"


def _digest(token: str) -> bytes:
    return api_service.token_digest(token)


def _hex(value: str) -> bytes:
    return bytes.fromhex(value)


# --- repository methods ------------------------------------------------------


def _create_account_session(repository, args: dict):
    extra = {}
    if "issued_via" in args:
        extra["issued_via"] = args["issued_via"]
    return repository.create_account_session(
        person_id=_uuid(args["person_id"]),
        token_digest=_digest(args["token"]),
        issued_from_invite_id=_optional_uuid(args["issued_from_invite_id"]),
        expires_at=_instant(args["expires_at"]),
        now=_instant(args["now"]),
        **extra,
    )


def _consume_named_invite_secret(repository, args: dict):
    return repository.consume_named_invite_secret(
        invite_id=_uuid(args["invite_id"]),
        token_digest=_digest(args["token"]),
        accepted_by_id=_uuid(args["accepted_by_id"]),
        now=_instant(args["now"]),
    )


def _create_otp_challenge(repository, args: dict):
    return repository.create_otp_challenge(
        challenge_id=_uuid(args["challenge_id"]),
        phone_digest=_hex(args["phone_digest"]),
        code_digest=_hex(args["code_digest"]),
        expires_at=_instant(args["expires_at"]),
        now=_instant(args["now"]),
    )


def _record_otp_attempt(repository, args: dict):
    return repository.record_otp_attempt(
        challenge_id=_uuid(args["challenge_id"]),
        attempts=args["attempts"],
        consumed=args["consumed"],
        now=_instant(args["now"]),
    )


def _create_person_with_identity(repository, args: dict):
    return repository.create_person_with_identity(
        person_id=_uuid(args["person_id"]),
        display_name=args["display_name"],
        provider=args["provider"],
        subject=args["subject"],
        now=_instant(args["now"]),
    )


# --- routes ------------------------------------------------------------------


class _FixedSecrets:
    """`secrets` as the service sees it while one route step mints a token."""

    def __init__(self, token: str) -> None:
        self._token = token

    def token_urlsafe(self, _nbytes: int | None = None) -> str:
        return self._token


class _PinnedUuid:
    """`uuid` as `app.api.service` sees it, with uuid4 answering the case.

    Everything else is the real module, so the type annotations and the
    `uuid.UUID` constructions in that module keep working. A case that runs out
    of ids raises rather than falling back to a random one: a silent fallback
    would put an unpinned id into a digest and hide the mismatch it caused.
    """

    def __init__(self, ids: list) -> None:
        self._ids = [_uuid(text) for text in ids]

    def uuid4(self):
        if not self._ids:
            raise RuntimeError("the case ran out of pinned uuid4 values")
        return self._ids.pop(0)

    def __getattr__(self, name):
        return getattr(_real_uuid, name)


class _StubSms:
    """The gateway. Records nothing; refuses when the case says so."""

    def __init__(self, fails: bool) -> None:
        self._fails = fails

    def send_otp(self, *, canonical_phone: str, code: str, challenge_id) -> None:
        del canonical_phone, code, challenge_id
        if self._fails:
            raise SmsDeliveryError("gateway refused (dữ liệu mẫu)")


class _StubGoogle:
    """The verifier. `None` for a host with no client ids configured."""

    def __init__(self, subject, display_name, invalid: bool) -> None:
        self._subject = subject
        self._display_name = display_name
        self._invalid = invalid

    def verify(self, id_token: str) -> GoogleClaims:
        del id_token
        if self._invalid:
            raise GoogleTokenInvalid("not vouched for (dữ liệu mẫu)")
        return GoogleClaims(subject=self._subject, display_name=self._display_name)


def _route(body):
    """One request: the case's clock, its token, its ids, then the service."""

    def call(repository, args: dict):
        now = _instant(args["now"])
        original = (api_service._now, api_service.secrets, api_service.uuid)
        api_service._now = lambda: now
        if "session_token" in args:
            api_service.secrets = _FixedSecrets(args["session_token"])
        if "ids" in args:
            api_service.uuid = _PinnedUuid(args["ids"])
        try:
            body(api_service.ApiService(repository), args)
        finally:
            api_service._now, api_service.secrets, api_service.uuid = original
        return None

    return call


def _create_session(service, args):
    service.bootstrap_session_from_invite(args["token"])


def _actor_for_session_token(service, args):
    service.actor_for_session_token(args["bearer"])


def _list_sessions(service, args):
    actor = service.actor_for_session_token(args["bearer"])
    service.list_account_sessions(actor, current_token=args["bearer"])


def _revoke_current_session(service, args):
    # No actor dependency on this route: the handler reads the header and hands
    # the token straight to the service.
    service.revoke_session_token(args["bearer"])


def _revoke_session(service, args):
    actor = service.actor_for_session_token(args["bearer"])
    service.revoke_account_session(_uuid(args["session_id"]), actor)


def _request_otp(service, args):
    service.request_otp(
        args["phone"],
        sender=_StubSms(bool(args.get("sms_fails"))),
        debug_code=args["code"],
    )


def _verify_otp(service, args):
    service.verify_otp(_uuid(args["challenge_id"]), args["phone"], args["code"])


def _login_google(service, args):
    verifier = None
    if args.get("google_configured", True):
        verifier = _StubGoogle(
            args.get("subject"),
            args.get("display_name"),
            bool(args.get("google_invalid")),
        )
    # A case that does not care about the token carries none; the verifier is a
    # stub, so the placeholder decides nothing. Go stands in the same string.
    service.login_with_google(
        args.get("id_token") or PLACEHOLDER_ID_TOKEN, verifier=verifier
    )


base.CALLS.update(
    {
        "create_account_session": _create_account_session,
        "get_account_session_by_digest": lambda repository, args: (
            repository.get_account_session_by_digest(_digest(args["token"]))
        ),
        "get_account_session": lambda repository, args: (
            repository.get_account_session(_uuid(args["session_id"]))
        ),
        "list_account_sessions": lambda repository, args: (
            repository.list_account_sessions(
                _uuid(args["person_id"]), now=_instant(args["now"])
            )
        ),
        "revoke_account_session": lambda repository, args: (
            repository.revoke_account_session(
                session_id=_uuid(args["session_id"]), now=_instant(args["now"])
            )
        ),
        "actor_grants": lambda repository, args: (
            repository.actor_grants(_uuid(args["person_id"]))
        ),
        "consume_named_invite_secret": _consume_named_invite_secret,
        "create_otp_challenge": _create_otp_challenge,
        "recent_otp_challenges": lambda repository, args: (
            repository.recent_otp_challenges(
                _hex(args["phone_digest"]), _instant(args["since"])
            )
        ),
        "get_otp_challenge": lambda repository, args: (
            repository.get_otp_challenge(_uuid(args["challenge_id"]))
        ),
        "record_otp_attempt": _record_otp_attempt,
        "get_account_identity": lambda repository, args: (
            repository.get_account_identity(args["provider"], args["subject"])
        ),
        "upsert_account_identity": lambda repository, args: (
            repository.upsert_account_identity(
                person_id=_uuid(args["person_id"]),
                provider=args["provider"],
                subject=args["subject"],
                now=_instant(args["now"]),
            )
        ),
        "create_person_with_identity": _create_person_with_identity,
        "create_person": lambda repository, args: (
            repository.create_person(_uuid(args["person_id"]), args["display_name"])
        ),
        "route.create_session": _route(_create_session),
        "route.actor_for_session_token": _route(_actor_for_session_token),
        "route.list_sessions": _route(_list_sessions),
        "route.revoke_current_session": _route(_revoke_current_session),
        "route.revoke_session": _route(_revoke_session),
        "route.request_otp": _route(_request_otp),
        "route.verify_otp": _route(_verify_otp),
        "route.login_google": _route(_login_google),
    }
)


if __name__ == "__main__":
    base.main()
