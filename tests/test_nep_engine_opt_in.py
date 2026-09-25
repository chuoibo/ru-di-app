"""Nếp's Go engine is opt-in, and so is the Gemini key on the public door.

ADR-0037 (proposed) moves Nếp's model call from the Python brain into Go, in
whichever process runs the AI worker -- by default `core serve`, the public
front door. Slice 6 shipped that engine behind MOBILE_AI_ENGINE_NEP, and its
review found two ways the switch could be thrown without anyone choosing to:

  1. docker-compose.yml read the flag from the host's `.env` and handed
     `core` the Gemini key on every stack, even with the flag at `brain`;
  2. nothing pinned that no committed configuration sets the flag to `go`.

The flag must stay `brain` until eval T1 is green in CI (slice 6b), ADR-0037
is signed and the key in core has had its security review (design 01 §9 q4).
These cases hold that: the base stack pins `brain` and gives core no key; the
one override file that opts in says so and needs the key explicitly; and no
other tracked configuration sets the flag to `go`.

Parsed straight from the files: no Docker needed.
"""

from __future__ import annotations

import re
import subprocess
import unittest
from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[1]
BASE = REPO_ROOT / "docker-compose.yml"
OPT_IN = REPO_ROOT / "docker-compose.nep-go.yml"
ENV_EXAMPLE = REPO_ROOT / ".env.example"
FLAG = "MOBILE_AI_ENGINE_NEP"
KEY = "GEMINI_API_KEY"

# A line that sets the flag to go, in YAML, shell, env, Makefile or
# Dockerfile syntax -- including a default that only looks like a reference:
# `${MOBILE_AI_ENGINE_NEP:-go}` is `go` on every host whose `.env` is silent.
#
# The value is `go`, bare or quoted, or reached through defaults nested to
# any depth: `${A:-${B:-"go"}}` (review round 3 of slice 6, N-a).
_VALUE_GO = r"""["']?(?:\$\{\w+:?[-=+]["']?)*go\b"""
SETS_GO = re.compile(
    FLAG
    + r"(?:"
    # FLAG: go / FLAG=go / "FLAG": "go" / Makefile FLAG := go, ?= go, += go,
    # ::= go -- and any of them with a default for a value.
    + r"""["']?\s*(?::{1,2}=|\?=|\+=|:|=)\s*"""
    + _VALUE_GO
    # ${FLAG:-go}, ${FLAG-go}, ${FLAG:=go}, ${FLAG=go}, ${FLAG:+go},
    # ${FLAG:-"go"}, ${FLAG:-${OTHER:-go}}
    + r"|:?[-=+]"
    + _VALUE_GO
    # Dockerfile: ENV FLAG go
    + r"""|\s+["']?go\b"""
    + r")"
)
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
}
CONFIG_NAMES = {"Makefile", "Procfile", "fly.toml"}


def core_environment(path: Path) -> dict:
    document = yaml.safe_load(path.read_text(encoding="utf-8"))
    return dict(document["services"]["core"].get("environment") or {})


class BaseStackTests(unittest.TestCase):
    def test_core_runs_the_brain_whatever_the_env_file_says(self):
        # A literal, not ${MOBILE_AI_ENGINE_NEP:-brain}: a `.env` line must
        # not be enough to move the public door onto the Go engine.
        self.assertEqual(core_environment(BASE).get(FLAG), "brain")

    def test_core_gets_no_gemini_key_by_default(self):
        self.assertNotIn(KEY, core_environment(BASE))

    def test_the_api_still_gets_its_key(self):
        # The bill reader in `api` is untouched by this: it keeps its key.
        document = yaml.safe_load(BASE.read_text(encoding="utf-8"))
        self.assertIn(KEY, document["services"]["api"]["environment"])


class OptInOverrideTests(unittest.TestCase):
    def test_the_override_turns_the_go_engine_on_and_hands_over_the_key(self):
        env = core_environment(OPT_IN)
        self.assertEqual(env.get(FLAG), "go")
        # Interpolated, never a literal, and loud when missing.
        self.assertRegex(str(env.get(KEY)), r"^\$\{" + KEY + r":\?")

    def test_the_override_touches_core_only(self):
        document = yaml.safe_load(OPT_IN.read_text(encoding="utf-8"))
        self.assertEqual(sorted(document["services"]), ["core"])

    def test_env_example_says_compose_needs_the_override(self):
        text = ENV_EXAMPLE.read_text(encoding="utf-8")
        self.assertIn("docker-compose.nep-go.yml", text)
        declared = {
            line.split("=", 1)[0].strip(): line.split("=", 1)[1].strip()
            for line in text.splitlines()
            if "=" in line and not line.lstrip().startswith("#")
        }
        self.assertEqual(declared.get(FLAG), "")


def repo_files() -> list[str]:
    """Tracked files; in an exported tree with no .git, every file on disk
    outside version control, dependency and build directories."""
    listed = subprocess.run(
        ["git", "ls-files", "-z"], cwd=REPO_ROOT, capture_output=True
    )
    if listed.returncode == 0:
        return listed.stdout.decode("utf-8").split("\0")
    skip = {
        ".git",
        "node_modules",
        ".expo",
        ".expo-build-check",
        "__pycache__",
        ".venv",
        "venv",
    }
    names = []
    for path in REPO_ROOT.rglob("*"):
        parts = path.relative_to(REPO_ROOT).parts
        if skip.intersection(parts[:-1]):
            continue
        if path.is_file():
            names.append("/".join(parts))
    return names


class NothingElseFlipsTheFlagTests(unittest.TestCase):
    def test_no_tracked_configuration_sets_the_flag_to_go(self):
        tracked = repo_files()
        hits = []
        scanned = 0
        for name in tracked:
            if not name or name == OPT_IN.name:
                continue
            path = REPO_ROOT / name
            # Configuration, wherever it lives: what a host, a stack, a script
            # or CI starts core with. Documentation and code comments may name
            # the value; the code's own default is pinned by
            # services/core/cmd/core/nep_engine_test.go (TestNepEngineFlag).
            base = path.name
            if not (
                path.suffix in CONFIG_SUFFIXES
                or base in CONFIG_NAMES
                or base.startswith(("Dockerfile", ".env", "docker-compose"))
            ):
                continue
            if name.startswith("phase0/") or "/node_modules/" in "/" + name:
                continue
            if not path.is_file() or path.stat().st_size > 2 * 1024 * 1024:
                continue
            try:
                text = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue
            scanned += 1
            for number, line in enumerate(text.splitlines(), 1):
                if line.lstrip().startswith(("#", "//")):
                    continue
                if SETS_GO.search(line):
                    hits.append(f"{name}:{number}: {line.strip()}")
        self.assertGreater(scanned, 300, "the scan looked at too few files")
        self.assertEqual(
            hits, [], "MOBILE_AI_ENGINE_NEP=go outside docker-compose.nep-go.yml"
        )

    def test_the_scan_sees_a_line_that_would_flip_it(self):
        # Canary: each syntax the scan must catch.
        for line in [
            "      MOBILE_AI_ENGINE_NEP: go",
            "MOBILE_AI_ENGINE_NEP=go core serve",
            'export MOBILE_AI_ENGINE_NEP="go"',
            "  MOBILE_AI_ENGINE_NEP: 'go'",
            # Defaults (review round 2 of slice 6): each reads as `go` on a
            # host that sets nothing.
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-go}",
            'MOBILE_AI_ENGINE_NEP="${MOBILE_AI_ENGINE_NEP-go}"',
            ": ${MOBILE_AI_ENGINE_NEP:=go}",
            "export MOBILE_AI_ENGINE_NEP=${MOBILE_AI_ENGINE_NEP:+go}",
            "      MOBILE_AI_ENGINE_NEP: ${NEP_ENGINE:-go}",
            "ENV MOBILE_AI_ENGINE_NEP go",
            # Review round 3 of slice 6 (N-a): Makefile assignments, quoted
            # defaults and nested defaults.
            "MOBILE_AI_ENGINE_NEP := go",
            "MOBILE_AI_ENGINE_NEP ?= go",
            "MOBILE_AI_ENGINE_NEP += go",
            "MOBILE_AI_ENGINE_NEP ::= go",
            "export MOBILE_AI_ENGINE_NEP := go",
            '      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-"go"}',
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-'go'}",
            'MOBILE_AI_ENGINE_NEP="${MOBILE_AI_ENGINE_NEP:-"go"}"',
            "      MOBILE_AI_ENGINE_NEP: ${A:-${B:-go}}",
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-${NEP_ENGINE:-go}}",
            "MOBILE_AI_ENGINE_NEP=${A:-${B:-${C:-'go'}}}",
            "core: MOBILE_AI_ENGINE_NEP ?= ${NEP:-go}",
        ]:
            self.assertIsNotNone(SETS_GO.search(line), line)
        for line in [
            "MOBILE_AI_ENGINE_NEP: brain",
            "MOBILE_AI_ENGINE_NEP=",
            "MOBILE_AI_ENGINE_NEP: google",
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-brain}",
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:?set it}",
            "ENV MOBILE_AI_ENGINE_NEP brain",
            "MOBILE_AI_ENGINE_NEP := brain",
            "MOBILE_AI_ENGINE_NEP ?= $(NEP_ENGINE)",
            "      MOBILE_AI_ENGINE_NEP: ${A:-${B:-brain}}",
            "      MOBILE_AI_ENGINE_NEP: ${MOBILE_AI_ENGINE_NEP:-\"google\"}",
            # Comparisons read the flag; they do not set it.
            'if [ "$MOBILE_AI_ENGINE_NEP" != go ]; then exit 1; fi',
            'if [ "$MOBILE_AI_ENGINE_NEP" == go ]; then echo on; fi',
            "ifeq ($(MOBILE_AI_ENGINE_NEP),go)",
        ]:
            self.assertIsNone(SETS_GO.search(line), line)


if __name__ == "__main__":
    unittest.main()
