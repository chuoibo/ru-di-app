"""Ai sở hữu một job media của Nếp, và làm sao chứng minh, không cần bảng mới.

Proxy (`vnlocal-proxy`) là nơi giữ job: nó biết seat, quota và tiến độ. Nó KHÔNG
biết người dùng, và không nên biết — nó phục vụ nhiều thứ khác trong nhà. Vậy
quyền sở hữu phải do tầng này chứng minh.

Cách làm: job id mang một tiền tố dẫn xuất từ `person_id` bằng HMAC dưới một
khoá của máy chủ, cộng một phần ngẫu nhiên không đoán được:

    <the-nguoi>-<ngau-nhien>

Đọc một job thì tính lại `the-nguoi` từ người đang gọi rồi so tiền tố. Không ai
dựng được id của người khác vì không biết khoá, và không ai đoán được id của
người khác vì phần đuôi là 128 bit ngẫu nhiên. Bằng chứng nằm trong chính cái
id, nên không cần một bảng chỉ để trả lời «job này của ai».

`the_nguoi` cũng là `tenant` gửi sang proxy để proxy đếm hạn mức từng người mà
vẫn không biết người đó là ai.

Thuần: không FastAPI, không cơ sở dữ liệu. Chạy được dưới pytest trần.
"""

from __future__ import annotations

import hashlib
import hmac
import re
import secrets

__all__ = [
    "DAI_THE",
    "LOAI_MEDIA",
    "job_id_moi",
    "la_chu_job",
    "the_nguoi",
]

#: Đủ dài để không đụng nhau giữa vài triệu người, đủ ngắn để id còn đọc được.
DAI_THE = 16
LOAI_MEDIA = ("anh", "video")

# Nối chuỗi chứ không f-string: regex này có `{8,}`, và trong f-string thì
# mọi ngoặc nhọn phải nhân đôi, khiến một biểu thức vốn đã khó đọc thành
# khó đọc gấp đôi.
_DANG_JOB = re.compile(r"^[a-f0-9]{" + str(DAI_THE) + r"}-[A-Za-z0-9_-]{8,}$")


def the_nguoi(person_id: str, khoa: str) -> str:
    """Thẻ mờ của một người. Cùng người cùng khoá thì cùng thẻ, và chỉ vậy."""
    if not person_id or not str(person_id).strip():
        raise ValueError("person_id rỗng")
    if not khoa or len(khoa) < 32:
        # Cùng luật với `MOBILE_PERSON_ID_KEY`: một khoá ngắn là một khoá dò
        # được, và thẻ dò được thì quyền sở hữu không còn là bằng chứng.
        raise ValueError("khoá phải dài ít nhất 32 ký tự")
    dau = hmac.new(
        khoa.encode("utf-8"), str(person_id).strip().encode("utf-8"), hashlib.sha256
    )
    return dau.hexdigest()[:DAI_THE]


def job_id_moi(person_id: str, khoa: str) -> str:
    return f"{the_nguoi(person_id, khoa)}-{secrets.token_urlsafe(16)}"


def la_chu_job(job_id: str, person_id: str, khoa: str) -> bool:
    """Người này có phải chủ job không. Sai dạng cũng là không, không phải lỗi."""
    if not isinstance(job_id, str) or not _DANG_JOB.match(job_id):
        return False
    return hmac.compare_digest(job_id[:DAI_THE], the_nguoi(person_id, khoa))
