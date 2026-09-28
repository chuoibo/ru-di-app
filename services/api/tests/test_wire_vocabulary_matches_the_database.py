"""The wire schema and the CHECK constraints must name the same values.

They drifted once and it cost an afternoon. `places.source` learned `vnlocal`
in the database and in `models.py`; the Pydantic model in `routes/places.py`
did not. Every read of a fed row answered 500, and 3,699 tests stayed green --
because no test had a fed row travelling the read path.

That is a fixture gap nobody can be relied upon to notice. This file closes it
a different way: it reads both vocabularies out of the source and compares
them, so a value added to one and not the other is red before anybody runs a
server.

Parsed from source rather than imported and introspected, because a CHECK
constraint's text is the thing PostgreSQL enforces, and a helper that
reconstructs it could agree with the model while both disagree with the
database.
"""

from __future__ import annotations

import ast
import pathlib
import re
import unittest

API_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODELS = API_ROOT / "app/db/models.py"
ROUTES = API_ROOT / "app/api/routes/places.py"


def _check_constraint_values(source: str, name: str) -> set[str]:
    """The quoted values inside the named CHECK constraint."""
    match = re.search(
        r'CheckConstraint\(\s*(?P<body>(?:"[^"]*"\s*)+),\s*name="' + name + r'"',
        source,
    )
    if match is None:
        raise AssertionError(f"không thấy CheckConstraint tên {name!r}")
    body = " ".join(re.findall(r'"([^"]*)"', match.group("body")))
    return set(re.findall(r"'([^']+)'", body))


def _literal_values(source: str, model: str, field: str) -> set[str]:
    """The values of a `Literal[...]` annotation on one field of one model.

    Scoped to the class, because a field name is not unique in this module and
    the first match is the wrong one: `source` is declared three times, and two
    of those are `Literal["ai", "none"]` on a different concept entirely --
    where a sentence of explanation came from, not where a place came from.
    Reading the first one compares the catalogue's sources against the model's,
    which is a comparison between two unrelated vocabularies.
    """
    tree = ast.parse(source)
    for node in ast.walk(tree):
        if not isinstance(node, ast.ClassDef) or node.name != model:
            continue
        for item in node.body:
            if not isinstance(item, ast.AnnAssign):
                continue
            target = item.target
            if not isinstance(target, ast.Name) or target.id != field:
                continue
            found: set[str] = set()
            for inner in ast.walk(item.annotation):
                if isinstance(inner, ast.Constant) and isinstance(inner.value, str):
                    found.add(inner.value)
            if found:
                return found
        raise AssertionError(f"{model} không khai Literal cho {field!r}")
    raise AssertionError(f"không thấy lược đồ {model!r}")


class WireVocabularyMatchesTheDatabase(unittest.TestCase):
    def setUp(self) -> None:
        self.models = MODELS.read_text(encoding="utf-8")
        self.routes = ROUTES.read_text(encoding="utf-8")

    def test_source_vocabulary(self) -> None:
        """Every source the database accepts, the wire can describe.

        One-directional on purpose: the wire may not name a source the database
        would refuse, and the database may not hold one the wire cannot carry.
        """
        database = _check_constraint_values(self.models, "place_source_known")
        wire = _literal_values(self.routes, "Place", "source")
        self.assertEqual(
            database,
            wire,
            "`places.source` chấp nhận "
            f"{sorted(database)} còn lược đồ wire khai {sorted(wire)}. "
            "Một dòng mang giá trị chỉ có ở một bên sẽ ghi được vào database "
            "rồi trả 500 lúc đọc.",
        )

    def test_geo_precision_vocabulary(self) -> None:
        """The same, for how a coordinate says it was arrived at.

        This one matters beyond a 500: a level the wire cannot carry arrives as
        null, and a screen reads null as «no precision stated» -- which is the
        one thing a point with coordinates must never be able to say.
        """
        database = _check_constraint_values(self.models, "place_geo_precision_known")
        wire = _literal_values(self.routes, "Place", "geo_precision")
        self.assertEqual(
            database,
            wire,
            f"`places.geo_precision` chấp nhận {sorted(database)} "
            f"còn lược đồ wire khai {sorted(wire)}.",
        )

    def test_coordinates_are_optional_on_both_sides(self) -> None:
        """A quarter of the catalogue has no coordinates and never will.

        The column was widened and the wire model was not, so every fed row
        without a point answered 500. Checked as text because that is what the
        next person will change.
        """
        self.assertIn(
            "lat: Mapped[float | None]",
            self.models,
            "`places.lat` phải cho phép null: một phần tư danh mục không có "
            "toạ độ và sẽ không bao giờ có.",
        )
        self.assertIn(
            "lat: float | None",
            self.routes,
            "Lược đồ wire phải cho phép `lat` null, nếu không mọi dòng không "
            "có toạ độ sẽ trả 500.",
        )


if __name__ == "__main__":
    unittest.main()
