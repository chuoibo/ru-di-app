"""Một flow Maestro thêm vào `.maestro/` mà bảng không bao giờ chạy.

`scripts/mobile_native.sh` chọn flow bằng một `case` viết tay. Nhánh cuối của
nó là `*)` với `continue`, nên một file mới **rơi ra ngoài trong im lặng**: bảng
in «đã chạy 23 flow», không có dòng đỏ nào, và flow mới chưa từng chạy. Đó là
chuyện vừa xảy ra với `47-to-giay-hai-nguoi.yaml` — hai lượt bảng, tám mươi
phút máy, và bằng chứng cho tính năng thì không tồn tại.

Cái làm nó tệ hơn một lỗi thường: bảng vẫn XANH. Một cổng im lặng bỏ qua thứ nó
được thêm vào để đo là cổng nói dối theo hướng dễ tin nhất.
"""

from __future__ import annotations

import pathlib
import re

_GOC = pathlib.Path(__file__).resolve().parents[1]
_SCRIPT = _GOC / "scripts/mobile_native.sh"
_FLOWS = _GOC / "apps/mobile/.maestro"


def _tien_to_duoc_khai() -> set[str]:
    """Mọi tiền tố `NN-` xuất hiện trong `case` chọn flow của script."""
    nguon = _SCRIPT.read_text(encoding="utf-8")
    dau = nguon.index('for f in "$FLOWS"/*.yaml; do')
    than = nguon[dau : nguon.index("esac", dau)]
    return set(re.findall(r"(\d{2})-\*", than))


#: Quy ước đánh số của bảng, đọc ra từ chính `case`: 00–19 là bảng MẶC ĐỊNH
#: (chạy trên fixture, không cần máy chủ) và đi qua nhánh `*)`; 20–89 là bảng
#: SỐNG (cần API thật) và mỗi số phải tự khai; 90+ là flow lẻ chạy bằng tay.
#: Chỉ khoảng giữa mới có lỗ: một flow sống không khai sẽ rơi vào `*)`, bị bỏ
#: qua ở mọi chế độ sống, và ở bảng mặc định thì chạy mà không có máy chủ.
_SONG_TU, _SONG_DEN = 20, 89


def _tien_to_co_that() -> set[str]:
    """Tiền tố `NN-` của mọi flow SỐNG, bỏ subflow `_*` ra."""
    return {
        m.group(1)
        for f in _FLOWS.glob("*.yaml")
        if not f.name.startswith("_")
        for m in [re.match(r"(\d{2})-", f.name)]
        if m and _SONG_TU <= int(m.group(1)) <= _SONG_DEN
    }


def test_moi_flow_co_so_deu_duoc_bang_nhac_ten():
    thieu = sorted(_tien_to_co_that() - _tien_to_duoc_khai())
    assert thieu == [], (
        f"flow {thieu} có trong .maestro nhưng không tiền tố nào trong `case` của "
        "mobile_native.sh nhắc tới — bảng sẽ bỏ qua chúng trong im lặng và vẫn in xanh"
    )


def test_bang_khong_nhac_ten_flow_khong_ton_tai():
    thua = sorted(_tien_to_duoc_khai() - _tien_to_co_that())
    assert thua == [], (
        f"`case` nhắc tới flow {thua} mà .maestro không có — một nhánh chết đọc "
        "như một flow đang được chạy"
    )
