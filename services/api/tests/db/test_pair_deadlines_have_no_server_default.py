"""ADR-0027: hai cái hạn của sổ hai người do domain tính, không do database.

`pair_papers.expires_at` là khung tuần và `pair_consent_proposals.expires_at`
là cửa sổ của một lời đề nghị. Một `server_default` kiểu `now() + interval` sẽ
là cách đánh vần thứ hai cho con số của sản phẩm, ở chỗ không ca nào đứng ở
biên chạm tới — cùng lập trường `stories.expires_at`, và ca này là bản sao có
chủ ý của ca ấy cho hai cột mới.

Đọc CẢ migration lẫn models, nên không bên nào mọc ra một cái mặc định."""

from __future__ import annotations

import ast
import pathlib
import unittest

API = pathlib.Path(__file__).resolve().parents[2]
MIGRATION = (
    API / "app/db/migrations/versions/c4f27a90d1e3_them_so_hai_nguoi_va_to_giay.py"
)

#: Bảng nào trong migration ấy phải có `expires_at` không mặc định.
BANG = ("pair_papers", "pair_consent_proposals")


def _keywords(call: ast.Call) -> set[str]:
    return {keyword.arg for keyword in call.keywords if keyword.arg}


def _columns_by_table(tree: ast.AST) -> dict[str, dict[str, ast.Call]]:
    """Mỗi `op.create_table("ten", sa.Column(...), ...)` thành {tên bảng: {cột: node}}."""
    ra: dict[str, dict[str, ast.Call]] = {}
    for node in ast.walk(tree):
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "create_table"
            and node.args
            and isinstance(node.args[0], ast.Constant)
        ):
            cot: dict[str, ast.Call] = {}
            for arg in node.args[1:]:
                if (
                    isinstance(arg, ast.Call)
                    and isinstance(arg.func, ast.Attribute)
                    and arg.func.attr == "Column"
                    and arg.args
                    and isinstance(arg.args[0], ast.Constant)
                ):
                    cot[arg.args[0].value] = arg
            ra[node.args[0].value] = cot
    return ra


class PairDeadlineTests(unittest.TestCase):
    def setUp(self) -> None:
        self.by_table = _columns_by_table(
            ast.parse(MIGRATION.read_text(encoding="utf-8"))
        )

    def test_migration_khong_cho_expires_at_mot_mac_dinh(self):
        for table in BANG:
            with self.subTest(table=table):
                columns = self.by_table.get(table, {})
                self.assertIn("expires_at", columns, f"{table} phải có cột expires_at")
                self.assertNotIn("server_default", _keywords(columns["expires_at"]))
                # Cột anh em CÓ mặc định, để chứng minh phép quét nhìn thấy chúng.
                self.assertIn("server_default", _keywords(columns["created_at"]))

    def test_models_noi_cung_mot_dieu(self):
        from app.db.models import PairConsentProposal, PairPaper

        for model in (PairPaper, PairConsentProposal):
            with self.subTest(model=model.__name__):
                deadline = model.__table__.c["expires_at"]
                self.assertIsNone(deadline.server_default)
                self.assertFalse(deadline.nullable)
                self.assertIsNotNone(model.__table__.c["created_at"].server_default)


if __name__ == "__main__":  # pragma: no cover
    unittest.main()
