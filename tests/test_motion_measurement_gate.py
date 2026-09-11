"""Run the motion runner's negative controls without an emulator or Maestro."""

from pathlib import Path
import subprocess


def test_motion_gate_rejects_corrupt_measurements_and_restores_settings():
    root = Path(__file__).resolve().parents[1]
    result = subprocess.run(
        ["bash", str(root / "docs/claude/2026-09-10/motion/do-motion-canary.sh")],
        cwd=root,
        capture_output=True,
        text=True,
        timeout=180,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "31 nhánh đúng kỳ vọng" in result.stdout
