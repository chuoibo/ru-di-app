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


def test_native_gate_drives_the_whole_product_table_not_one_login_flow():
    # The PR that retired the phone door shrank this gate to one login flow
    # while the product flows moved to a fixture folder nothing executed. The
    # gate's chain must reach the full table again: gate -> e2e_slice --native
    # -> mobile_native.sh looping over every apps/mobile/.maestro flow.
    gate = (ROOT / "scripts/mobile_native_gate.sh").read_text(encoding="utf-8")
    slice_ = (ROOT / "scripts/e2e_slice.sh").read_text(encoding="utf-8")
    harness = (ROOT / "scripts/mobile_native.sh").read_text(encoding="utf-8")
    assert "exec scripts/e2e_slice.sh --native" in gate
    assert "scripts/mobile_native.sh --account" in slice_
    assert '--credentials "$WORK_DIR/credentials.json"' in slice_
    assert "world=native" in slice_
    assert 'FLOWS=".maestro"' in harness
    assert 'for f in "$FLOWS"/*.yaml; do' in harness
    assert "50-account-login.yaml" not in harness
    flows = sorted(p.name for p in (ROOT / "apps/mobile/.maestro").glob("[0-9]*.yaml"))
    for must in (
        "30-chat-that.yaml",
        "45-chan-bao-cao.yaml",
        "46-xoa-tai-khoan.yaml",
        "47-to-giay-hai-nguoi.yaml",
        "48-to-hen-chung.yaml",
        "50-account-login.yaml",
    ):
        assert must in flows, must
    assert not (ROOT / "apps/mobile/tests/fixtures/legacy_native").exists()


def test_native_harness_refuses_without_synthetic_acknowledgement():
    result = subprocess.run(
        ["bash", "scripts/mobile_native.sh", "--account", "--api-port", "9"],
        cwd=ROOT,
        env={"HOME": os.environ["HOME"], "PATH": "/usr/bin:/bin"},
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 2
    assert "synthetic-only" in result.stderr
    for retired in ("--otp", "--live", "--dang-nhap"):
        old = subprocess.run(
            ["bash", "scripts/mobile_native.sh", retired],
            cwd=ROOT,
            env={"HOME": os.environ["HOME"], "PATH": "/usr/bin:/bin"},
            capture_output=True,
            text=True,
            timeout=30,
        )
        assert old.returncode == 64, retired
        assert "ADR-0055" in old.stderr
