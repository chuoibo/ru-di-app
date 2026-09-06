"""Every personal photograph leaves the server through ONE gate (ADR-0022 §2.1).

`read_person_photo` is the only method that asks «may this reader see it»
(`person_image_visible_to`), and the only route that answers bytes for a
personal photo calls it. A second reader of the bytes -- a thumbnail route, a
share endpoint -- written past this gate would be exactly the leak the gate
exists for, so the shape is pinned by AST rather than by review.
"""

from __future__ import annotations

import ast
import pathlib
import unittest

API = pathlib.Path(__file__).resolve().parents[2] / "app" / "api"


def _calls(tree: ast.AST, attr: str) -> list[ast.Call]:
    return [
        node
        for node in ast.walk(tree)
        if isinstance(node, ast.Call)
        and isinstance(node.func, ast.Attribute)
        and node.func.attr == attr
    ]


class PersonalPhotoGateTests(unittest.TestCase):
    def test_the_visibility_question_is_asked_in_exactly_one_place(self):
        tree = ast.parse((API / "service.py").read_text(encoding="utf-8"))
        asked = _calls(tree, "person_image_visible_to")
        self.assertEqual(len(asked), 1, "một cửa, không phải hai")
        owner = next(
            node
            for node in ast.walk(tree)
            if isinstance(node, ast.FunctionDef) and node.name == "read_person_photo"
        )
        self.assertEqual(len(_calls(owner, "person_image_visible_to")), 1)

    def test_the_bytes_route_goes_through_that_gate(self):
        tree = ast.parse((API / "routes" / "photos.py").read_text(encoding="utf-8"))
        route = next(
            node
            for node in ast.walk(tree)
            if isinstance(node, ast.FunctionDef) and node.name == "read_person_photo"
        )
        self.assertEqual(len(_calls(route, "read_person_photo")), 1)
        # And no other route module hands out personal photo bytes.
        for path in (API / "routes").glob("*.py"):
            if path.name == "photos.py":
                continue
            self.assertNotIn(
                "get_person_image", path.read_text(encoding="utf-8"), path.name
            )


if __name__ == "__main__":
    unittest.main()
