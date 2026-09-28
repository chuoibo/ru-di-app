"""What the extraction model is told, the shape it must answer in, and the
structural check applied to that answer.

Nothing here reads the meaning of a sentence. Whether a candidate fact is the
person's own statement, whether it is about money or a sensitive trait, and
which closed kind it has, is the MODEL's classification, returned as enum
fields of the response schema. The code only keeps the items the model itself
classified as keepable, and counts the rest (trinho package doc: "refuses them
by the model's own structured classification ... not by keyword").
"""

from __future__ import annotations

import contextvars
import json
import logging
from typing import Any

log = logging.getLogger("ai_infer.mem")

# services/core aiharness/trinho LoaiSuThat, the closed kinds of a fact.
LOAI = (
    "thich_danh_muc",
    "thich_dia_diem",
    "ne_dia_diem",
    "diem_den_quen",
    "khung_gio_hay_di",
    "phuong_tien",
    "thoi_luong_chang",
    "nhip_len_keo",
    "dieu_da_dan",
)

# Who the statement is about, as the model judges it.
VE_AI = ("ban_than", "nguoi_khac")
# What the statement is, as the model judges it. Only the first is stored.
NOI_DUNG = (
    "so_thich_rang_buoc_di_choi",
    "tien_bac",
    "dac_diem_nhay_cam",
    "khong_lien_quan",
)
KEEP_VE_AI = "ban_than"
KEEP_NOI_DUNG = "so_thich_rang_buoc_di_choi"

MAX_TEXT = 160  # trinho.MaxNoiDung, runes

SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "memory": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "id": {"type": "string"},
                    "text": {"type": "string", "maxLength": MAX_TEXT},
                    "ve_ai": {"type": "string", "enum": list(VE_AI)},
                    "noi_dung": {"type": "string", "enum": list(NOI_DUNG)},
                    "loai": {"type": "string", "enum": list(LOAI)},
                },
                "required": ["text", "ve_ai", "noi_dung", "loai"],
            },
        }
    },
    "required": ["memory"],
}

# Appended to mem0's own extraction prompt as its "Custom Instructions"
# section (mem0 2.x: custom_instructions). The rules are stated for the model
# to judge; the schema above makes it say what it judged.
INSTRUCTIONS = """\
Bạn trích ký ức dài hạn cho trợ lý đi chơi «Nếp» của MỘT người dùng.
Chỉ đọc lời của chính người dùng trong phần tin nhắn mới. Mỗi ký ức là một câu
tiếng Việt có dấu, ngôi thứ ba ("Người dùng ..."), tối đa 160 ký tự, về một sở
thích hay ràng buộc ổn định của CHÍNH người đó khi đi chơi.

Với MỖI ứng viên, tự phân loại và điền đủ các trường của schema:
- ve_ai: "ban_than" nếu câu nói về chính người dùng; "nguoi_khac" nếu nói về
  bất kỳ ai khác (bạn bè, người yêu, gia đình, đồng nghiệp, người trong nhóm),
  kể cả khi người dùng là người kể.
- noi_dung: "so_thich_rang_buoc_di_choi" nếu là sở thích/ràng buộc đi chơi;
  "tien_bac" nếu dính tới tiền, giá, ngân sách, nợ, thu nhập, tài khoản;
  "dac_diem_nhay_cam" nếu dính tới sức khoẻ (kể cả dị ứng, ăn kiêng, thai kỳ),
  tôn giáo, chính trị, dân tộc, xu hướng tính dục, số giấy tờ, địa chỉ nhà,
  toạ độ; "khong_lien_quan" cho mọi thứ khác.
- loai: kiểu gần nhất trong danh sách đóng.

Không bịa, không suy diễn điều người dùng không nói. Không chép lại tin nhắn
nguyên văn. Nếu không có gì đáng nhớ, trả về {"memory": []}.
"""

# Kinds of the facts the last extraction in this context kept, by text, so
# the vector store can record the kind with the memory it inserts.
_KEPT: contextvars.ContextVar[dict[str, str] | None] = contextvars.ContextVar(
    "ai_infer_kept", default=None
)
# How many candidates the model itself classified as not keepable.
_REFUSED: contextvars.ContextVar[int] = contextvars.ContextVar(
    "ai_infer_refused", default=0
)


def begin() -> None:
    _KEPT.set({})
    _REFUSED.set(0)


def kept_kind(text: str) -> str | None:
    kept = _KEPT.get()
    return kept.get(text) if kept else None


def refused() -> int:
    return _REFUSED.get()


def filter_answer(raw: str) -> str:
    """Keep the items the model classified as the person's own outing
    preference with a closed kind; drop the rest and count them. An answer
    that is not the schema's JSON keeps nothing (fail closed)."""
    try:
        data = json.loads(raw, strict=False)
        items = data["memory"]
        if not isinstance(items, list):
            raise TypeError
    except (ValueError, KeyError, TypeError):
        log.warning("extraction answer outside the schema; nothing kept")
        return json.dumps({"memory": []})
    keep, dropped = [], 0
    kept = _KEPT.get()
    if kept is None:
        kept = {}
        _KEPT.set(kept)
    for i, it in enumerate(items):
        ok = (
            isinstance(it, dict)
            and it.get("ve_ai") == KEEP_VE_AI
            and it.get("noi_dung") == KEEP_NOI_DUNG
            and it.get("loai") in LOAI
            and isinstance(it.get("text"), str)
            and 0 < len(it["text"].strip()) <= MAX_TEXT
        )
        if not ok:
            dropped += 1
            continue
        text = it["text"].strip()
        keep.append({"id": str(it.get("id", i)), "text": text})
        kept[text] = it["loai"]
    _REFUSED.set(_REFUSED.get() + dropped)
    return json.dumps({"memory": keep}, ensure_ascii=False)
