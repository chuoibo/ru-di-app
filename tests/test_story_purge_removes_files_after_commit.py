"""Dọn story: file chỉ được gỡ SAU khi hàng đã commit (review #577 S3).

`app/db/story_purge.py::purge_expired_stories` xoá hàng và trả về khoá file;
`scripts/purge_expired_stories.py` commit rồi mới gọi `remove_purged_files`.
Thứ tự ấy là toàn bộ luật: một commit hỏng sau khi file đã bị gỡ là một tấm
ảnh mất hẳn trong khi hàng vẫn hứa nó còn.

Đọc bằng AST chứ không bằng mắt: một lần dời hai dòng cho nhau vẫn chạy được,
vẫn xanh ở mọi ca hiện có, và chỉ hỏng vào đúng ngày cơ sở dữ liệu từ chối
commit. Cổng này đỏ ngay khi thứ tự đảo, và đỏ nếu ai đó trả tham số `storage`
về cho hàm dọn để nó tự xoá file lần nữa.
"""

from __future__ import annotations

import ast
import pathlib
import unittest

REPO_ROOT = pathlib.Path(__file__).resolve().parents[1]
SCRIPT = REPO_ROOT / "scripts/purge_expired_stories.py"
MODULE = REPO_ROOT / "services/api/app/db/story_purge.py"


def _main_body(tree: ast.Module) -> ast.FunctionDef:
    for node in tree.body:
        if isinstance(node, ast.FunctionDef) and node.name == "main":
            return node
    raise AssertionError("scripts/purge_expired_stories.py không còn hàm main()")


def _calls(node: ast.AST, name: str, *, methods_only: bool = False) -> list[ast.Call]:
    """Calls to `name(...)` under `node`. `methods_only` narrows to `x.name(...)`,
    which is how `storage.delete` is told apart from SQLAlchemy's `delete()`."""
    found = []
    for child in ast.walk(node):
        if not isinstance(child, ast.Call):
            continue
        func = child.func
        if isinstance(func, ast.Attribute) and func.attr == name:
            found.append(child)
        elif not methods_only and isinstance(func, ast.Name) and func.id == name:
            found.append(child)
    return found


class StoryPurgeOrderTests(unittest.TestCase):
    def test_the_script_commits_before_it_unlinks(self) -> None:
        main = _main_body(ast.parse(SCRIPT.read_text(encoding="utf-8")))
        commits = _calls(main, "commit")
        unlinks = _calls(main, "remove_purged_files")
        self.assertEqual(len(commits), 1, "một lần commit, không hơn")
        self.assertEqual(len(unlinks), 1, "một lần gỡ file, không hơn")
        self.assertLess(
            commits[0].lineno,
            unlinks[0].lineno,
            "gỡ file trước khi commit: một commit hỏng làm mất ảnh mà hàng vẫn hứa",
        )

    def test_the_purge_function_does_not_touch_storage_itself(self) -> None:
        tree = ast.parse(MODULE.read_text(encoding="utf-8"))
        purge = next(
            node
            for node in tree.body
            if isinstance(node, ast.FunctionDef)
            and node.name == "purge_expired_stories"
        )
        argument_names = {arg.arg for arg in purge.args.args + purge.args.kwonlyargs}
        self.assertNotIn(
            "storage",
            argument_names,
            "hàm dọn nhận storage là hàm dọn tự xoá file trong cùng transaction",
        )
        # `x.delete(...)` only: `delete(UploadedImage)` is SQLAlchemy's row
        # delete and is exactly what this function is supposed to do.
        self.assertEqual(
            _calls(purge, "delete", methods_only=True),
            [],
            "một lời gọi storage.delete() trong hàm dọn là file bị gỡ trước commit",
        )
        # And the one function that may unlink still exists to be called.
        self.assertTrue(
            any(
                isinstance(node, ast.FunctionDef) and node.name == "remove_purged_files"
                for node in tree.body
            )
        )

    def test_the_script_calls_the_purge_without_a_storage_argument(self) -> None:
        main = _main_body(ast.parse(SCRIPT.read_text(encoding="utf-8")))
        purges = _calls(main, "purge_expired_stories")
        self.assertEqual(len(purges), 1)
        self.assertNotIn("storage", {kw.arg for kw in purges[0].keywords if kw.arg})


if __name__ == "__main__":
    unittest.main()
