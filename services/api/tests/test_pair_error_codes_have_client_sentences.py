"""Mọi mã lỗi của sổ hai người phải có một câu tiếng Việt ở phía client.

Hai cách đánh vần một hợp đồng: máy chủ ném `ApiProblem(409, "paper_expired", …)`,
client tra `LOI_TO_GIAY["paper_expired"]`. Không có gì bắt hai danh sách khớp
nhau, và hậu quả của lệch không phải một lỗi — nó là **một câu sai** đặt trước
mặt người đang đứng giữa tuần của họ: `thongDiepNguoiDoc` trả câu 403 chung
«bạn không có quyền» cho `pair_chat_consent_required`, trong khi thứ còn thiếu
là một lời đồng ý chưa ai nói, và người đọc sẽ đi tìm một công tắc không tồn tại.

Ca này đọc **nguồn Python bằng AST** và **nguồn TypeScript bằng regex**, nên nó
chạy trong cổng backend mà không cần node. Nó nằm ở đây chứ không ở tầng mobile
vì đây là nơi danh sách mã lỗi được sinh ra; bên kia chỉ dịch.
"""

from __future__ import annotations

import ast
import pathlib
import re

_GOC = pathlib.Path(__file__).resolve().parents[3]
_SERVICE = _GOC / "services/api/app/api/service.py"
_CLIENT = _GOC / "apps/mobile/src/rudi/to-giay/to-giay-song.ts"

#: Những phương thức của `ApiService` thuộc sổ hai người. Viết tay, và có ca
#: đối chiếu ngay dưới để một phương thức thêm sau không lặng lẽ nằm ngoài.
_PHUONG_THUC = {
    "_pair_context_or_404",
    "_readable_paper_or_404",
    "_locked_paper",
    "_chuyen",
    "_chot",
    "_dong_y",
    "_de_nghi_sua",
    "pair_notebook",
    "propose_pair_consent",
    "grant_pair_consent",
    "revoke_pair_consent",
    "put_pair_constraint",
    "delete_pair_constraint",
    "preview_close_pair_notebook",
    "close_pair_notebook",
    "list_pair_papers",
    "draft_pair_paper",
    "pair_paper",
    "edit_pair_draft",
    "send_pair_paper",
    "mark_pair_paper_viewed",
    "respond_pair_paper",
    "withdraw_pair_paper",
    "skip_pair_week",
    "record_pair_outing_done",
    "keep_pair_paper_line",
    "take_companion_turn",
    "_require_pair_permission",
    "_pair_chat_consent",
}

#: Cùng mang chữ `pair` trong tên nhưng thuộc tính năng khác: `_require_pair_is_alive`
#: là phép gác của nhắn riêng (ADR-0023 §2.3.2), và câu tiếng Việt cho mã của nó
#: sống ở `LOI_NHAN_RIENG` bên client, không ở bảng này.
_KHONG_PHAI_SO_HAI_NGUOI = {"_require_pair_is_alive"}


def _cay() -> ast.Module:
    return ast.parse(_SERVICE.read_text(encoding="utf-8"))


def _ma_tu_apiproblem() -> set[str]:
    """Mã của mọi `ApiProblem(<status>, "<ma>", …)` viết thẳng trong các phương
    thức trên."""
    ma: set[str] = set()
    for node in ast.walk(_cay()):
        if not isinstance(node, ast.FunctionDef) or node.name not in _PHUONG_THUC:
            continue
        for con in ast.walk(node):
            if not isinstance(con, ast.Call):
                continue
            ten = con.func
            if not (isinstance(ten, ast.Name) and ten.id == "ApiProblem"):
                continue
            if len(con.args) >= 2 and isinstance(con.args[1], ast.Constant):
                gia_tri = con.args[1].value
                if isinstance(gia_tri, str):
                    ma.add(gia_tri)
    return ma


def _ma_tu_bang(ten_bang: str, lay: str) -> set[str]:
    """Khoá (hoặc mã trong giá trị) của một bảng hằng ở tầng module."""
    for node in _cay().body:
        # `Assign` và `AnnAssign` là hai nút khác nhau, và một trong hai bảng
        # dưới kia có chú thích kiểu. Bản đầu chỉ đọc `Assign`, nên nó ném
        # «không tìm thấy bảng» — may là ném chứ không phải trả tập rỗng, vì
        # tập rỗng sẽ làm cả ca này xanh mà không đo gì.
        if isinstance(node, ast.AnnAssign):
            muc_tieu, gia_tri = node.target, node.value
        elif isinstance(node, ast.Assign):
            muc_tieu, gia_tri = node.targets[0], node.value
        else:
            continue
        ten = getattr(muc_tieu, "id", None) or getattr(muc_tieu, "attr", None)
        if ten != ten_bang or not isinstance(gia_tri, ast.Dict):
            continue
        node = ast.Assign(targets=[muc_tieu], value=gia_tri)
        if lay == "khoa":
            return {
                k.value
                for k in node.value.keys
                if isinstance(k, ast.Constant) and isinstance(k.value, str)
            }
        ra: set[str] = set()
        for v in node.value.values:
            if isinstance(v, ast.Tuple) and len(v.elts) >= 2:
                thu_hai = v.elts[1]
                if isinstance(thu_hai, ast.Constant) and isinstance(thu_hai.value, str):
                    ra.add(thu_hai.value)
        return ra
    raise AssertionError(f"không tìm thấy bảng {ten_bang} trong service.py")


#: Mã KHÔNG đọc được bằng AST: chúng đi qua `raise ApiProblem(409, exc.code.lower(), …)`,
#: nên chữ thật nằm ở `RepositoryConflict("…")` trong repository. Khai ở đây, và
#: ca dưới kiểm từng cái có mặt thật trong `repository.py` — một danh sách khai
#: mà không ai đối chiếu thì chỉ là một lối thoát.
_MA_QUA_XUNG_DOT = {"couple_slot_taken", "paper_already_agreed", "paper_outing_exists"}


def _ma_xung_dot_co_that() -> set[str]:
    nguon = (_GOC / "services/api/app/api/repository.py").read_text(encoding="utf-8")
    return set(re.findall(r'RepositoryConflict\("([a-z_]+)"\)', nguon))


def test_ma_qua_xung_dot_deu_co_that_trong_repository():
    thieu = _MA_QUA_XUNG_DOT - _ma_xung_dot_co_that()
    assert thieu == set(), (
        f"{sorted(thieu)} được khai là mã đi qua xung đột nhưng repository không "
        "ném chúng — bản khai đang che một mã đã chết"
    )


def _cau_cua_client() -> set[str]:
    """Khoá của `LOI_TO_GIAY` trong module client."""
    nguon = _CLIENT.read_text(encoding="utf-8")
    than = re.search(
        r"export const LOI_TO_GIAY: Record<string, string> = \{(.*?)\n\};",
        nguon,
        re.S,
    )
    assert than is not None, "không tìm thấy LOI_TO_GIAY trong module client"
    return set(re.findall(r"^\s*([a-z_]+):", than.group(1), re.M))


def test_moi_ma_loi_may_chu_nem_deu_co_cau_o_client():
    may_chu = (
        _ma_tu_apiproblem()
        | _ma_tu_bang("_LOI_TO_GIAY", "khoa")
        | _ma_tu_bang("_TU_CHOI_TO_GIAY", "ma")
        | _MA_QUA_XUNG_DOT
    )
    # Hai mã này do middleware idempotency ném, không phải feature này, và
    # client đã dịch chúng ở `IDEMPOTENCY_REFUSALS`.
    may_chu -= {"idempotency_request_in_flight", "idempotency_key_reuse"}
    thieu = may_chu - _cau_cua_client()
    assert thieu == set(), (
        f"máy chủ ném {sorted(thieu)} nhưng client không có câu nào cho chúng — "
        "người đọc sẽ nhận câu 403 chung, và với `pair_chat_consent_required` "
        "câu ấy nói sai nghĩa"
    )


def test_client_khong_dich_ma_nao_may_chu_khong_con_nem():
    """Chiều ngược lại: một câu cho mã đã chết là một câu không ai đọc được, và
    nó làm bảng trông đầy đủ hơn thực tế."""
    may_chu = (
        _ma_tu_apiproblem()
        | _ma_tu_bang("_LOI_TO_GIAY", "khoa")
        | _ma_tu_bang("_TU_CHOI_TO_GIAY", "ma")
        | _MA_QUA_XUNG_DOT
    )
    # `cycle_closed` là mã của hợp đồng ADR-0027 mà lát 1 chưa có đường ném:
    # một chu kỳ đóng thì `cycle_id` về None và đường đi qua `cycle_not_active`.
    # Giữ câu cho nó là có chủ ý, ghi ở đây để lát sau không đọc nhầm là rác.
    thua = _cau_cua_client() - may_chu - {"cycle_closed"}
    assert thua == set(), f"client dịch {sorted(thua)} mà máy chủ không còn ném"


def test_danh_sach_phuong_thuc_khong_bo_sot_cua_nao():
    """Danh sách viết tay không tự biết mình thiếu: mọi phương thức của
    `ApiService` có tên mang `pair` phải nằm trong nó."""
    trong_lop = {
        node.name
        for node in ast.walk(_cay())
        if isinstance(node, ast.FunctionDef) and "pair" in node.name
    }
    thieu = trong_lop - _PHUONG_THUC - _KHONG_PHAI_SO_HAI_NGUOI
    assert thieu == set(), f"phương thức {sorted(thieu)} chưa có trong danh sách"


def _duong_cua_client() -> set[tuple[str, str]]:
    """Mỗi lời gọi trong module client, thành (METHOD, đường dạng FastAPI).

    Đọc `translatedAsActor<T>(LOI_TO_GIAY, \\`/…\\`, { … method: "X" … })` và đổi
    `${bien}` thành `{bien}`. Mẫu bám vào literal, đúng thứ mà
    `test_api_contract_unresolved_pin` bắt phải viết thẳng, nên nếu ai gom
    chúng lại sau một helper thì ca này mất dấu và ca kia đỏ — hai cổng nhìn
    cùng một chỗ từ hai phía.
    """
    nguon = _CLIENT.read_text(encoding="utf-8")
    ra: set[tuple[str, str]] = set()
    for duong, phan_con in re.findall(
        r"translatedAsActor<[^>]+>\(\s*LOI_TO_GIAY,\s*`([^`]+)`,\s*(\{[^;]*?\})\s*\)",
        nguon,
        re.S,
    ):
        method = re.search(r'method:\s*"([A-Z]+)"', phan_con)
        assert method is not None, f"lời gọi tới {duong} không nói method"
        ra.add((method.group(1), re.sub(r"\$\{([A-Za-z0-9_]+)\}", r"{\1}", duong)))
    return ra


def test_moi_duong_client_goi_deu_la_mot_route_that():
    """Cầu nối cuối cùng giữa hai cây: một lỗi chính tả trong đường dẫn client.

    Không tầng test nào khác bắt được nó. `tests/api` lái app bằng đường của
    CHÍNH nó, `npm test` không có máy chủ, và `check_server_routes_called` chỉ
    hỏi «có ai nhắc tới route này không» chứ không hỏi «cái client gõ có phải
    một route không». Một chữ sai ở đây là một màn 404 trên máy thật.
    """
    from app.api.main import create_app

    app = create_app(auth_mode="dev")
    that = {
        (method, route.path)
        for route in app.routes
        if getattr(route, "methods", None)
        for method in route.methods - {"HEAD", "OPTIONS"}
    }

    # Tên tham số hai bên không buộc phải trùng (`{paperId}` của client với
    # `{paper_id}` của route), nên so theo HÌNH: phương thức và các đoạn tĩnh.
    def hinh(cap: tuple[str, str]) -> tuple[str, tuple[str, ...]]:
        method, duong = cap
        return (
            method,
            tuple(
                "{}" if doan.startswith("{") else doan
                for doan in duong.strip("/").split("/")
            ),
        )

    hinh_that = {hinh(cap) for cap in that}
    la = sorted(cap for cap in _duong_cua_client() if hinh(cap) not in hinh_that)
    assert la == [], f"client gọi những đường máy chủ không có: {la}"


def test_client_goi_du_muoi_chin_cua():
    """Và đủ cả mười chín, để «gọi đúng» không đọc thành «gọi hết»."""
    assert len(_duong_cua_client()) == 19, sorted(_duong_cua_client())
