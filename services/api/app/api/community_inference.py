"""Inference-only adapter for an operator-managed, private community model.

The endpoint implements /moderate and /nep. It has no database credentials,
does not fetch user URLs, and must inspect all supplied media before asserting
media_checked. Missing configuration is a closed failure, never an approval.
"""

from __future__ import annotations

import ipaddress
import json
import os
import urllib.request
from urllib.parse import urlsplit


def infer_community(action: str, body: dict) -> dict:
    """Forward a bounded source bundle only to an explicitly private endpoint."""
    base = os.environ.get("COMMUNITY_INFERENCE_URL", "")
    parsed = urlsplit(base)
    if (
        parsed.scheme not in {"http", "https"}
        or parsed.username is not None
        or parsed.password is not None
        or parsed.query
        or parsed.fragment
    ):
        raise RuntimeError("community_inference_not_configured")
    try:
        address = ipaddress.ip_address(parsed.hostname or "")
    except ValueError:
        # Numeric addresses prevent DNS rebinding or an accidental public host.
        raise RuntimeError("community_inference_private_address_required") from None
    if not (address.is_loopback or address.is_private) or address.is_unspecified:
        raise RuntimeError("community_inference_private_address_required")
    if action not in {"moderate", "nep"}:
        raise ValueError("invalid_action")
    raw = json.dumps(body, ensure_ascii=False).encode()
    if len(raw) > 90 * 1024 * 1024:
        raise ValueError("source_too_large")

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise RuntimeError("inference_redirect_refused")

    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    request = urllib.request.Request(
        base.rstrip("/") + "/" + action,
        data=raw,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with opener.open(request, timeout=45) as response:
        result = response.read(128 * 1024 + 1)
    if len(result) > 128 * 1024:
        raise ValueError("inference_response_too_large")
    answer = json.loads(result)
    if not isinstance(answer, dict):
        raise ValueError("invalid_inference_response")
    return answer
