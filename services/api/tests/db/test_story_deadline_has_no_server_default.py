"""ADR-0022 §2.3: `stories.expires_at` is computed by the domain, never by the
database. A `server_default` on that column would be a second clock for the
product's one number, out of reach of every test that stands on the boundary.
Read from the migration source and from the model, so neither can grow one."""

from __future__ import annotations

import ast
import pathlib
import unittest

API = pathlib.Path(__file__).resolve().parents[2]
MIGRATION = API / "app/db/migrations/versions/8f4d0b6a2e75_them_story_24_gio.py"


def _keywords(call: ast.Call) -> set[str]:
    return {keyword.arg for keyword in call.keywords if keyword.arg}


class StoryDeadlineTests(unittest.TestCase):
    def test_the_migration_gives_expires_at_no_server_default(self):
        tree = ast.parse(MIGRATION.read_text(encoding="utf-8"))
        columns = [
            node
            for node in ast.walk(tree)
            if isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "Column"
            and node.args
            and isinstance(node.args[0], ast.Constant)
        ]
        by_name = {node.args[0].value: node for node in columns}
        self.assertIn("expires_at", by_name, "migration phải tạo cột expires_at")
        self.assertNotIn("server_default", _keywords(by_name["expires_at"]))
        # The sibling that SHOULD have one, so the scan is proven to see them.
        self.assertIn("server_default", _keywords(by_name["created_at"]))

    def test_the_model_agrees(self):
        from app.db.models import Story

        column = Story.__table__.c.expires_at
        self.assertIsNone(column.server_default)
        self.assertIsNone(column.default)
        self.assertFalse(column.nullable)
        self.assertIsNotNone(Story.__table__.c.created_at.server_default)


if __name__ == "__main__":
    unittest.main()
