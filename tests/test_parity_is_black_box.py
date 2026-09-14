"""The parity harness must not import the code it judges (ADR-0029).

`parity/` compares the Python API with the Go front door. If it imported a
package from `services/core`, one change to that package would move both sides
of the comparison at once: a broken encoder could produce a broken expected
value and a broken actual value that agree with each other. Kept black-box, the
harness only ever sees bytes on the wire and rows in the database.

The check reads the harness's full dependency closure from `go list -deps`, not
its import lines, so a transitive import through a helper package is caught.

What this does not prove: that the harness is correct. That is the canary's job
(`parity canary`), which damages responses on purpose and must be caught.
"""

from __future__ import annotations

import pathlib
import shutil
import subprocess
import unittest

REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
PARITY = REPO_ROOT / "parity"
FORBIDDEN_PREFIX = "mobile/services/core"


def violations(dependencies: list[str]) -> list[str]:
    """Import paths from the Go core found in a dependency closure."""
    return sorted(
        dep
        for dep in dependencies
        if dep == FORBIDDEN_PREFIX or dep.startswith(FORBIDDEN_PREFIX + "/")
    )


class ParityIsBlackBox(unittest.TestCase):
    def test_the_check_goes_red_on_a_core_import(self) -> None:
        self.assertEqual(
            violations(
                [
                    "fmt",
                    "mobile/parity/internal/runner",
                    "mobile/services/core/internal/pyjson",
                ]
            ),
            ["mobile/services/core/internal/pyjson"],
        )
        self.assertEqual(violations(["fmt", "mobile/services/corelike"]), [])

    def test_go_mod_does_not_require_or_replace_the_core(self) -> None:
        go_mod = (PARITY / "go.mod").read_text(encoding="utf-8")
        self.assertNotIn(
            FORBIDDEN_PREFIX,
            go_mod,
            "parity/go.mod names the Go core; the harness must stay black-box",
        )

    def test_the_harness_dependency_closure_has_no_core_package(self) -> None:
        if shutil.which("go") is None:
            self.skipTest(
                "go is not on PATH; the `core` CI job and gate.sh run this with go"
            )
        result = subprocess.run(
            ["go", "list", "-deps", "-f", "{{.ImportPath}}", "./..."],
            cwd=PARITY,
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        dependencies = result.stdout.split()
        self.assertIn(
            "mobile/parity/internal/runner",
            dependencies,
            "go list returned no harness packages; the check measured nothing",
        )
        self.assertEqual(violations(dependencies), [])


if __name__ == "__main__":
    unittest.main()
