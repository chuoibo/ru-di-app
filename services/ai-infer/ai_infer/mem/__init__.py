"""mem0 as a library behind our own endpoints (research mem0.md §7).

Importing this package fixes mem0's process environment BEFORE mem0 itself
is imported by any submodule: mem0 reads MEM0_TELEMETRY once, at import time,
and defaults it to on (PostHog, us.i.posthog.com, md5 of user ids). The
sidecar never allows it, whatever the environment says.
"""

from __future__ import annotations

import os
import sys
import tempfile


class TelemetryAlreadyOn(RuntimeError):
    """mem0 was imported before this package with its telemetry on."""


def _prepare() -> None:
    if "mem0" in sys.modules:
        tel = sys.modules.get("mem0.memory.telemetry")
        if tel is not None and getattr(tel, "MEM0_TELEMETRY", True):
            raise TelemetryAlreadyOn(
                "mem0 was imported with telemetry on before ai_infer.mem"
            )
    os.environ["MEM0_TELEMETRY"] = "false"
    # mem0 writes config.json (an anonymous id) under MEM0_DIR at import; keep
    # it out of $HOME. The container points MEM0_DIR at a tmpfs.
    if not os.environ.get("MEM0_DIR"):
        os.environ["MEM0_DIR"] = tempfile.mkdtemp(prefix="ai-infer-mem0-")


_prepare()
