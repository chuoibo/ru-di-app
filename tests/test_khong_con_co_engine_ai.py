"""The AI engine switch is gone, and nothing may bring it back (ADR-0051).

Until 2026-09-30 two flags, MOBILE_AI_ENGINE_NEP and MOBILE_AI_ENGINE_GROUP,
chose between the Go engine and the Python brain for Nếp and the group
assistant. ADR-0051 deleted the brain path and the flags with it: the Go
engine is the only engine, and `core` no longer reads either name.

A flag left in a compose file or an env template would now be a knob wired to
nothing -- the kind of setting someone flips during an incident, sees no
effect, and concludes the wrong thing. So: no tracked configuration names
either flag, and no Go source reads them.

Parsed straight from the files: no Docker needed.
"""

from __future__ import annotations

import re
import subprocess
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
FLAG = re.compile(r"MOBILE_AI_ENGINE_(?:NEP|GROUP)\b")
CONFIG_SUFFIXES = {
    ".yml",
    ".yaml",
    ".sh",
    ".bash",
    ".env",
    ".json",
    ".toml",
    ".ini",
    ".cfg",
    ".conf",
    ".service",
    ".mk",
    ".example",
}
CONFIG_NAMES = {"Makefile", "Procfile", "fly.toml"}


def tracked() -> list[str]:
    listed = subprocess.run(
        ["git", "ls-files", "-z"], cwd=REPO_ROOT, capture_output=True, check=True
    )
    return [n for n in listed.stdout.decode("utf-8").split("\0") if n]


def is_config(name: str) -> bool:
    path = Path(name)
    return (
        path.suffix in CONFIG_SUFFIXES
        or path.name in CONFIG_NAMES
        or path.name.startswith(("Dockerfile", ".env", "docker-compose"))
    )


def hits(names, keep) -> list[str]:
    out = []
    for name in names:
        if not keep(name):
            continue
        path = REPO_ROOT / name
        if not path.is_file() or path.stat().st_size > 2 * 1024 * 1024:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for number, line in enumerate(text.splitlines(), 1):
            if FLAG.search(line):
                out.append(f"{name}:{number}: {line.strip()}")
    return out


class TheEngineFlagIsGoneTests(unittest.TestCase):
    def test_no_tracked_configuration_names_the_flags(self):
        names = tracked()
        configs = [n for n in names if is_config(n) and "/node_modules/" not in "/" + n]
        self.assertGreater(len(configs), 300, "the scan looked at too few files")
        self.assertEqual(hits(configs, lambda _: True), [])

    def test_no_go_source_reads_the_flags(self):
        names = [n for n in tracked() if n.startswith("services/core/") and n.endswith(".go")]
        self.assertGreater(len(names), 500, "the scan looked at too few Go files")
        self.assertEqual(hits(names, lambda _: True), [])

    def test_the_scan_sees_the_names(self):
        # Canary: every way the old flags were written.
        for line in [
            "      MOBILE_AI_ENGINE_NEP: go",
            "MOBILE_AI_ENGINE_GROUP=brain",
            "export MOBILE_AI_ENGINE_NEP=${MOBILE_AI_ENGINE_NEP:-go}",
            'const EnvAIEngineGroup = "MOBILE_AI_ENGINE_GROUP"',
        ]:
            self.assertIsNotNone(FLAG.search(line), line)
        self.assertIsNone(FLAG.search("MOBILE_AI_ENGINE_NEPX=1"))


if __name__ == "__main__":
    unittest.main()
