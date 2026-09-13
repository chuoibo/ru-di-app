"""Bản đồ xoá tài khoản và đoạn mã xoá là HAI cách đánh vần một chính sách.

`account_lifecycle.ERASURE` nói bảng nào đi và bảng nào ở; `erase_person` là
thứ thật sự xoá. Không một câu nào trong cây bắt hai thứ đó phải khớp — và một
phép đo độc lập đã chứng minh cái giá của nó: dời `pair_notebooks` từ nhánh
`keep` sang `delete` thì **cả 682 ca Postgres lẫn toàn bộ tầng ngoại tuyến vẫn
xanh**, vì `erase_person` không đọc bản đồ, nó có danh sách `wipe()` viết tay
riêng.

Hai hướng trôi dạt, hai hậu quả khác nhau:

- một bảng ở nhánh `delete` mà mã không xoá là một **lời hứa bị vỡ** — người ta
  bấm xoá tài khoản và dữ liệu ở lại;
- một bảng ở nhánh `keep` mà mã có xoá là **ký ức của người khác bị lấy mất** —
  cuốn sổ hai người là của cả hai.

Ca này đọc mã bằng AST chứ không chạy nó, nên nó không cần database và nó đỏ ở
lần sửa đầu tiên chứ không đợi ai đó xoá một tài khoản thật.
"""

from __future__ import annotations

import ast
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[2]))

from app.db import models  # noqa: E402
from app.domain.account_lifecycle import ERASURE  # noqa: E402

_NGUON = pathlib.Path(__file__).resolve().parents[2] / "app/api/repository.py"


#: Lớp hiện thực, không phải Protocol. Hai `erase_person` cùng tên trong file
#: này và cái đầu tiên là khai báo `...` — bản đầu của cổng đọc trúng nó, thấy
#: không có `wipe(` nào, và im lặng đồng ý với mọi bản đồ.
_LOP = "SqlAlchemyApiRepository"


def _than_erase_person() -> ast.FunctionDef:
    cay = ast.parse(_NGUON.read_text(encoding="utf-8"))
    for lop in ast.walk(cay):
        if not isinstance(lop, ast.ClassDef) or lop.name != _LOP:
            continue
        for node in lop.body:
            if isinstance(node, ast.FunctionDef) and node.name == "erase_person":
                return node
    raise AssertionError(f"không tìm thấy {_LOP}.erase_person trong repository.py")


def _bang_bi_xoa() -> set[str]:
    than = _than_erase_person()
    ten_bang: set[str] = set()
    for node in ast.walk(than):
        if not isinstance(node, ast.Call):
            continue
        if not isinstance(node.func, ast.Name) or node.func.id != "wipe":
            continue
        muc_tieu = node.args[0]
        assert isinstance(muc_tieu, ast.Name), (
            "tham số đầu của wipe() phải là tên model viết thẳng: một bí danh "
            "hay một biểu thức làm cổng này mù đúng chỗ nó sinh ra để nhìn"
        )
        model = getattr(models, muc_tieu.id, None)
        assert model is not None, f"wipe({muc_tieu.id}) không phải model nào"
        ten_bang.add(model.__tablename__)
    return ten_bang


def test_moi_bang_o_nhanh_delete_deu_that_su_bi_xoa():
    thieu = set(ERASURE["delete"]) - _bang_bi_xoa()
    assert thieu == set(), (
        f"bản đồ hứa xoá {sorted(thieu)} nhưng erase_person không chạm tới — "
        "người ta bấm «xoá tài khoản» và dữ liệu ở lại"
    )


def test_khong_bang_nao_bi_xoa_ma_ban_do_noi_o_lai():
    thua = _bang_bi_xoa() - set(ERASURE["delete"])
    assert thua == set(), (
        f"erase_person xoá {sorted(thua)} nhưng bản đồ không xếp chúng vào "
        "nhánh `delete` — nếu chúng nằm ở `keep` thì đây là ký ức của người "
        "khác đang bị lấy mất"
    )


def test_dem_loi_goi_khop_voi_so_dong_wipe_trong_nguon():
    """Đối chứng của chính cổng này: nếu AST bỏ sót một lời gọi `wipe(` thì hai
    ca trên xanh vì chúng không nhìn thấy nó, chứ không phải vì mã đúng."""
    than = _than_erase_person()
    goi = [
        node
        for node in ast.walk(than)
        if isinstance(node, ast.Call)
        and isinstance(node.func, ast.Name)
        and node.func.id == "wipe"
    ]
    dong = ast.get_source_segment(_NGUON.read_text(encoding="utf-8"), than) or ""
    assert dong.count("wipe(") - 1 == len(goi), (
        "số lời gọi wipe( trong nguồn khác số lời gọi AST đọc được "
        "(trừ một cho chính định nghĩa `def wipe(`)"
    )
