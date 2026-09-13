"""The static schema reader must visit both branches and reject broken graphs."""

from graphlib import CycleError

import pytest

from tests.db.test_migration_matches_models import _upgrade_bodies


def write_revisions(tmp_path, graph):
    for revision, parent in graph.items():
        (tmp_path / f"{revision}.py").write_text(
            f"revision = {revision!r}\ndown_revision = {parent!r}\n"
            "def upgrade():\n    pass\n",
            encoding="utf-8",
        )


def test_merge_visits_both_arms_before_merge_once(tmp_path):
    write_revisions(
        tmp_path, {"root": None, "a": "root", "b": "root", "merge": ("a", "b")}
    )
    sources = [source for source, _ in _upgrade_bodies(tmp_path)]
    assert len(sources) == 4
    assert sources[0].startswith("revision = 'root'")
    assert sources[-1].startswith("revision = 'merge'")
    assert {source.splitlines()[0] for source in sources[1:3]} == {
        "revision = 'a'",
        "revision = 'b'",
    }


@pytest.mark.parametrize(
    "graph,error",
    [
        ({"root": None, "a": "missing"}, AssertionError),
        ({"a": "b", "b": "a"}, CycleError),
        ({"root": None, "a": "root", "b": "root"}, AssertionError),
        ({"a": None, "b": None, "merge": ("a", "b")}, AssertionError),
    ],
)
def test_invalid_graph_is_refused(tmp_path, graph, error):
    write_revisions(tmp_path, graph)
    with pytest.raises(error):
        _upgrade_bodies(tmp_path)
