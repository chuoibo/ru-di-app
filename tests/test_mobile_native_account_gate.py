"""The device gate must refuse missing hardware before building or provisioning."""

from __future__ import annotations

import os
import pathlib
import subprocess


ROOT = pathlib.Path(__file__).resolve().parents[1]


def test_native_gate_refuses_an_unavailable_device_before_mutating():
    result = subprocess.run(
        ["bash", "scripts/mobile_native_gate.sh"],
        cwd=ROOT,
        env={
            "HOME": os.environ["HOME"],
            "PATH": "/usr/bin:/bin",
            "ANDROID_SERIAL": "rudi-qa-device-does-not-exist",
        },
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 2
    assert "KHÔNG ĐO ĐƯỢC" in result.stderr
    assert "BUILD SUCCESSFUL" not in result.stdout
    assert "database dùng một lần" not in result.stdout
