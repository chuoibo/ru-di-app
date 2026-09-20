"""The brain token is fail-closed: absence, whitespace, and a mismatch."""

from __future__ import annotations

import hmac

import pytest

from app.api.internal_token import (
    INTERNAL_TOKEN_ENV_VAR,
    TEST_TOKEN,
    InternalTokenMissing,
    resolve_internal_token,
    tokens_match,
)


def test_resolve_strips_surrounding_whitespace(monkeypatch):
    monkeypatch.setenv(INTERNAL_TOKEN_ENV_VAR, f"  {TEST_TOKEN}  ")
    assert resolve_internal_token() == TEST_TOKEN


def test_resolve_refuses_absence(monkeypatch):
    monkeypatch.delenv(INTERNAL_TOKEN_ENV_VAR, raising=False)
    with pytest.raises(InternalTokenMissing):
        resolve_internal_token()


def test_resolve_refuses_whitespace_only(monkeypatch):
    monkeypatch.setenv(INTERNAL_TOKEN_ENV_VAR, "   ")
    with pytest.raises(InternalTokenMissing):
        resolve_internal_token()


def test_tokens_match_is_hmac_compare_digest():
    assert tokens_match(TEST_TOKEN, TEST_TOKEN) is True
    assert tokens_match(TEST_TOKEN, "other-brain-token") is False
    assert tokens_match(TEST_TOKEN, None) is False
    assert tokens_match(TEST_TOKEN, "") is False
    # Length mismatch must not raise: hmac.compare_digest returns False.
    assert hmac.compare_digest(TEST_TOKEN, TEST_TOKEN) is True
    assert tokens_match(TEST_TOKEN, TEST_TOKEN[:3]) is False
