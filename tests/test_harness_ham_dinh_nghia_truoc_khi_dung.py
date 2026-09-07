"""Hàm harness dùng TRONG vòng lặp flow phải được định nghĩa TRƯỚC vòng lặp.

Bash chỉ biết một hàm từ dòng định nghĩa nó trở đi. Một hàm được gọi sớm hơn
chỗ nó được viết không phải lỗi cú pháp: nó là «command not found» trên stderr,
mã thoát 127, và `set -e` không bắt được nếu lời gọi nằm trong một điều kiện
`if` hay trước một `&&`. Kết quả là một nhánh rẽ sai hoặc một phép kiểm KHÔNG
CHẠY, giữa hàng nghìn dòng «COMPLETED».

Chuyện này đã xảy ra hai lần trên cùng một file, cách nhau vài giờ:

  * 2026-09-06: `chuan_bi_bai_cho_42` gọi `da_chay 37` trong vòng lặp trong khi
    `da_chay` còn nằm sau vòng lặp. `if da_chay 37` đọc 127 là «sai», bài QA
    được đăng cho người D trong khi máy đang là người C, và flow 42 đỏ ở
    «Mở bài … không visible» — một câu đỏ nói về app trong khi lỗi ở harness.
  * 2026-09-07: `kiem_can_25 kiem_may_chu_sau_45 && kiem_may_chu_sau_45` đặt
    trong hook sau flow 45. `kiem_can_25` khi ấy còn nằm sau vòng lặp, nên vế
    trái ra 127, `&&` nuốt, và phép kiểm máy chủ của flow 45 không chạy. Bảng
    vẫn in XANH: một cổng tự tháo trong im lặng.

Cả hai lần đều mất một lượt bảng đầy đủ (khoảng một giờ trên emulator) để lộ
ra. Ca này đọc chính file harness và trả lời câu hỏi ấy trong vài mili giây.

Cách đọc: lấy dòng bắt đầu vòng lặp flow, lấy mọi định nghĩa `ten() {` kèm số
dòng, rồi đi ĐIỂM BẤT ĐỘNG từ thân vòng lặp — hàm nào thân vòng lặp gọi, cộng
hàm nào những hàm ấy gọi, và cứ thế. Mọi hàm trong tập ấy phải có dòng định
nghĩa nhỏ hơn dòng mở vòng lặp.
"""

from __future__ import annotations

import re
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
HARNESS = REPO_ROOT / "scripts" / "mobile_native.sh"

#: Dòng mở vòng lặp flow. Ghim nguyên văn, và ghim cả việc nó KHÔNG thụt đầu
#: dòng: file còn một vòng `for f in "$FLOWS"/*.yaml` thứ hai, thụt vào trong
#: nhánh đổi cổng Metro, và nó không lái flow nào. Nếu dòng này đổi, ca phải
#: được đọc lại chứ không được im lặng quét một vùng rỗng.
DONG_MO_VONG = '\nfor f in "$FLOWS"/*.yaml; do\n'

DINH_NGHIA = re.compile(r"^([a-z_][a-z0-9_]*)\(\)\s*\{", re.MULTILINE)


def _dinh_nghia(text: str) -> dict[str, int]:
    """Tên hàm → số dòng (1-based) của định nghĩa."""
    out: dict[str, int] = {}
    for match in DINH_NGHIA.finditer(text):
        out.setdefault(match.group(1), text.count("\n", 0, match.start()) + 1)
    return out


def _goi_trong(doan: str, ten_ham: set[str]) -> set[str]:
    """Hàm nào được gọi trong đoạn này.

    Đọc theo từ, không theo cú pháp: một tên hàm đứng ở vị trí lệnh, sau `if`,
    sau `&&`, sau `|`, hay trong `$( … )` đều là một lời gọi. Bắt thừa thì chỉ
    làm ca này chặt hơn chứ không làm nó sai.
    """
    tu = set(re.findall(r"[a-z_][a-z0-9_]*", doan))
    return tu & ten_ham


class HamHarnessPhaiCoTruocKhiDuocGoi(unittest.TestCase):
    def setUp(self) -> None:
        if not HARNESS.is_file():
            self.skipTest("không có scripts/mobile_native.sh trong cây này")
        self.text = HARNESS.read_text(encoding="utf-8")
        self.dinh_nghia = _dinh_nghia(self.text)
        self.assertTrue(self.dinh_nghia, "không đọc được định nghĩa hàm nào")

    def _than_vong_lap(self) -> tuple[str, int]:
        self.assertIn(
            DONG_MO_VONG,
            self.text,
            "không tìm thấy dòng mở vòng lặp flow — ca này đang quét một vùng "
            "rỗng và sẽ xanh vì không có gì để đọc",
        )
        self.assertEqual(
            self.text.count(DONG_MO_VONG),
            1,
            "vòng lặp flow không còn là một chỗ duy nhất — ca này phải được đọc lại",
        )
        dau = self.text.index(DONG_MO_VONG) + 1
        dong_mo = self.text.count("\n", 0, dau) + 1
        # Thân vòng lặp chạy tới `done` cuối cùng của cặp lồng nhau; lấy rộng
        # tới hết file rồi cắt ở dòng `done` đầu tiên không thụt đầu dòng.
        con_lai = self.text[dau:]
        ket = re.search(r"\ndone\ndone\n", con_lai)
        than = con_lai[: ket.end()] if ket else con_lai
        return than, dong_mo

    def test_moi_ham_vong_lap_cham_toi_deu_duoc_viet_truoc_vong_lap(self) -> None:
        than, dong_mo = self._than_vong_lap()
        ten_ham = set(self.dinh_nghia)
        # Điểm bất động: hàm vòng lặp gọi, cộng hàm mà chúng gọi, và cứ thế.
        can = _goi_trong(than, ten_ham)
        while True:
            them: set[str] = set()
            for ten in can:
                dau = self.text.find(f"{ten}() {{")
                if dau < 0:
                    continue
                ket = self.text.find("\n}\n", dau)
                them |= _goi_trong(
                    self.text[dau : ket if ket > 0 else len(self.text)], ten_ham
                )
            moi = them - can
            if not moi:
                break
            can |= moi

        muon = sorted(
            (ten, self.dinh_nghia[ten]) for ten in can if self.dinh_nghia[ten] > dong_mo
        )
        self.assertEqual(
            muon,
            [],
            "hàm được gọi trong vòng lặp flow (dòng "
            f"{dong_mo}) nhưng định nghĩa ở dưới: {muon}. "
            "Bash sẽ nói «command not found», `if`/`&&` sẽ nuốt mã 127, và một "
            "nhánh rẽ hoặc một phép kiểm sẽ im lặng biến mất.",
        )

    def test_ca_nay_can(self) -> None:
        """Đối chứng dương: dời một hàm xuống dưới thì ca trên phải đỏ.

        Không có bước này thì `test_` ở trên xanh cả khi bộ đọc trả về tập
        rỗng, và một danh sách nguồn rỗng làm cổng tự tháo trong im lặng.
        """
        than, dong_mo = self._than_vong_lap()
        ten_ham = set(self.dinh_nghia)
        cham = _goi_trong(than, ten_ham)
        self.assertTrue(
            cham,
            "thân vòng lặp không gọi hàm nào — bộ đọc hỏng, không phải harness sạch",
        )
        gia = {ten: (dong_mo + 1) for ten in cham}
        muon = [ten for ten, dong in gia.items() if dong > dong_mo]
        self.assertTrue(
            muon, "phép so «định nghĩa sau vòng lặp» không phân biệt được gì"
        )


if __name__ == "__main__":
    unittest.main()
