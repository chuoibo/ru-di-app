"""scripts/eval_that.sh refuses before it spends anything, and never echoes the key.

These tests never build Go and never call the model: every case they run is
refused in the script's argument checks, which come before `go build`. What
they cannot speak for is a real run (the model, the replay, the trailer);
that is exercised offline by the Go tests the eval-kich-ban gate pins
(TestGhiRoiPhatLaiTrungDiem and its siblings) and, with a key, by the product
owner following docs/claude/2026-09-25/chay-model-that.md.
"""

from __future__ import annotations

import os
import subprocess
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts" / "eval_that.sh"
FAKE_KEY = "khoa-gia-KHONG-DUOC-IN-RA-abcxyz"


def run(args: list[str], key: str | None) -> subprocess.CompletedProcess[str]:
    env = {
        k: v
        for k, v in os.environ.items()
        if k
        not in (
            "GEMINI_API_KEY",
            "MOBILE_GEMINI_BASE_URL",
            "AGY_PROXY_URL",
            "AGY_PROXY_KEY",
        )
    }
    # A PATH without go: a case that got past the checks would fail loudly
    # at `command -v go` instead of building anything.
    env["PATH"] = "/usr/bin:/bin"
    if key is not None:
        env["GEMINI_API_KEY"] = key
    return subprocess.run(
        ["bash", str(SCRIPT), *args],
        cwd=REPO_ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
    )


class EvalThatRefuses(unittest.TestCase):
    def test_no_key_names_the_environment_settings(self) -> None:
        r = run(["--bo", "corpus/nep-kich-ban.json", "--tran-goi", "500"], key=None)
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn("cài đặt môi trường", r.stderr)
        self.assertIn("phiên MỚI", r.stderr)

    def test_no_ceiling_is_refused(self) -> None:
        r = run(["--bo", "corpus/nep-kich-ban.json"], key=FAKE_KEY)
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn("--tran-goi", r.stderr)
        self.assertNotIn(FAKE_KEY, r.stdout + r.stderr)

    def test_bad_ceiling_and_loopback_override_are_refused(self) -> None:
        for args in (
            ["--bo", "corpus/nep-kich-ban.json", "--tran-goi", "abc"],
            ["--tran-goi", "5"],
            ["--bo", "khong-co.json", "--tran-goi", "5"],
        ):
            r = run(args, key=FAKE_KEY)
            self.assertEqual(r.returncode, 2, (args, r.stderr))
            self.assertNotIn(FAKE_KEY, r.stdout + r.stderr)
        env_args = ["--bo", "corpus/nep-kich-ban.json", "--tran-goi", "5"]
        env = {k: v for k, v in os.environ.items()}
        env.update(
            {
                "GEMINI_API_KEY": FAKE_KEY,
                "MOBILE_GEMINI_BASE_URL": "http://127.0.0.1:9/",
                "PATH": "/usr/bin:/bin",
            }
        )
        r = subprocess.run(
            ["bash", str(SCRIPT), *env_args],
            cwd=REPO_ROOT,
            env=env,
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn("MOBILE_GEMINI_BASE_URL", r.stderr)
        self.assertNotIn(FAKE_KEY, r.stdout + r.stderr)

    def test_agy_without_its_key_is_refused_and_replay_drops_agy(self) -> None:
        """ADR-0052: the real run's model goes through agy when configured.

        Half a configuration is refused before anything is built, naming the
        missing variable and printing no value; and the replay that must make
        zero calls runs without either agy variable, like without the key.
        """
        env = {
            k: v
            for k, v in os.environ.items()
            if k not in ("AGY_PROXY_URL", "AGY_PROXY_KEY", "MOBILE_GEMINI_BASE_URL")
        }
        env.update(
            {
                "GEMINI_API_KEY": FAKE_KEY,
                "AGY_PROXY_URL": "http://127.0.0.1:9",
                "PATH": "/usr/bin:/bin",
            }
        )
        r = subprocess.run(
            [
                "bash",
                str(SCRIPT),
                "--bo",
                "corpus/nep-kich-ban.json",
                "--tran-goi",
                "5",
            ],
            cwd=REPO_ROOT,
            env=env,
            capture_output=True,
            text=True,
            timeout=30,
        )
        self.assertEqual(r.returncode, 2, r.stderr)
        self.assertIn("AGY_PROXY_KEY", r.stderr)
        self.assertNotIn(FAKE_KEY, r.stdout + r.stderr)
        # The replay command line, not the header comment that describes it.
        replay = next(
            line
            for line in SCRIPT.read_text(encoding="utf-8").splitlines()
            if "env -u GEMINI_API_KEY" in line and not line.lstrip().startswith("#")
        )
        self.assertIn("-u AGY_PROXY_URL", replay)
        self.assertIn("-u AGY_PROXY_KEY", replay)


if __name__ == "__main__":
    unittest.main()
