"""Nếp xin một tấm ảnh hoặc một đoạn video, và đọc lại kết quả của chính mình.

Ba tầng, ba việc rõ ràng:

  - tầng này biết NGƯỜI. Nó xác thực, đúc một job id mang bằng chứng sở hữu
    (`nep_media.py`), rồi chuyển tiếp;
  - `vnlocal-proxy` biết CHỖ NGỒI và QUOTA. Nó xếp hàng, mượn seat, chạy agy,
    đếm hạn mức theo `tenant` — một chuỗi mờ, nên nó không bao giờ biết người
    dùng là ai;
  - worker Celery biết cách CHẠY. 99 giây một tấm ảnh, đo thật, nên không lời
    gọi nào ở đây chờ kết quả.

Không bảng mới. Quyền sở hữu nằm trong chính job id, hạn mức nằm ở nơi biết
quota thật. Một bảng chỉ để trả lời «job này của ai» là một bảng phải migrate,
phải backfill và phải dọn, cho một câu hỏi mà HMAC trả lời xong trong một dòng.

Token của proxy KHÔNG BAO GIỜ rời máy chủ. Thiết bị nói chuyện với tầng này
bằng bearer của chính nó, còn tầng này giữ token proxy trong biến môi trường.
"""

from __future__ import annotations

import os
from typing import Annotated, Any, Literal

import httpx
from fastapi import APIRouter, Body, Depends, HTTPException
from fastapi.responses import Response

from app.api.deps import Actor, get_actor
from app.api.nep_media import job_id_moi, la_chu_job
from app.api.schemas import ApiModel

router = APIRouter(tags=["nep"])


class NepMediaDaNhan(ApiModel):
    """Biên nhận. Không phải kết quả: một tấm ảnh mất 99 giây đo thật."""

    job_id: str
    loai: Literal["anh", "video"]
    trang_thai: Literal["dang-cho"]


class NepMediaTinhHinh(ApiModel):
    """Tình hình một job, DỰNG LẠI tường minh chứ không chuyển tiếp nguyên văn.

    Proxy phục vụ nhiều thứ khác trong nhà và hình dạng câu trả lời của nó là
    chuyện nội bộ của nó. Chuyển tiếp nguyên JSON thì hợp đồng của API này trở
    thành «proxy nói gì thì trả thế», và một trường mới bên kia sẽ lặng lẽ thành
    một trường công khai bên này.
    """

    job_id: str
    trang_thai: Literal["dang-cho", "dang-chay", "xong", "hong"]
    so_byte: int | None = None
    loi: str | None = None


URL_ENV = "NEP_PROXY_URL"
TOKEN_ENV = "NEP_PROXY_TOKEN"
KHOA_ENV = "MOBILE_PERSON_ID_KEY"
TIMEOUT = httpx.Timeout(10.0, connect=5.0)


def _cau_hinh() -> tuple[str, str, str]:
    url = (os.environ.get(URL_ENV) or "").strip().rstrip("/")
    token = (os.environ.get(TOKEN_ENV) or "").strip()
    khoa = (os.environ.get(KHOA_ENV) or "").strip()
    if not url or not token:
        # 503, không 500: máy chủ này chạy tốt, chỉ là tính năng media chưa
        # được cấu hình ở đây. Câu chữ trên app phân biệt hai thứ đó.
        raise HTTPException(status_code=503, detail="nep_media_chua_cau_hinh")
    if len(khoa) < 32:
        raise HTTPException(status_code=503, detail="nep_media_thieu_khoa")
    return url, token, khoa


def _goi_proxy(method: str, duong: str, **kw) -> httpx.Response:
    url, token, _ = _cau_hinh()
    try:
        with httpx.Client(timeout=TIMEOUT) as client:
            return client.request(
                method,
                f"{url}{duong}",
                headers={"Authorization": f"Bearer {token}"},
                **kw,
            )
    except httpx.HTTPError as exc:
        raise HTTPException(
            status_code=502, detail=f"nep_media_khong_goi_duoc: {exc}"
        ) from exc


def _cua_toi(job_id: str, actor: Actor) -> None:
    _, _, khoa = _cau_hinh()
    if not la_chu_job(job_id, str(actor.id), khoa):
        # 404 chứ không 403: một người không được biết job của người khác có
        # tồn tại hay không. 403 là một câu trả lời, và ở đây nó là câu trả lời
        # sai người.
        raise HTTPException(status_code=404, detail="khong_thay_job")


@router.post("/me/nep/media", status_code=202, response_model=NepMediaDaNhan)
def tao_media(
    actor: Annotated[Actor, Depends(get_actor)],
    than: Annotated[dict[str, Any], Body()],
) -> NepMediaDaNhan:
    _, _, khoa = _cau_hinh()
    loai = str(than.get("loai") or "anh")
    if loai not in ("anh", "video"):
        raise HTTPException(status_code=422, detail="loai phải là anh hoặc video")

    the = job_id_moi(str(actor.id), khoa)
    goi: dict[str, Any] = {"job_id": the, "tenant": the.split("-", 1)[0]}
    if loai == "anh":
        goi["mo_ta"] = than.get("mo_ta")
        goi["ten_anh"] = than.get("ten_anh")
        goi["ty_le"] = than.get("ty_le")
        duong = "/v1/media"
    else:
        ids = than.get("anh_job_ids")
        if not isinstance(ids, list) or not ids:
            raise HTTPException(status_code=422, detail="cần anh_job_ids")
        # Chỉ được ghép từ ảnh của chính mình.
        for jid in ids:
            _cua_toi(str(jid), actor)
        goi["anh_job_ids"] = ids
        goi["giay_moi_anh"] = than.get("giay_moi_anh")
        duong = "/v1/media/video"

    tra = _goi_proxy("POST", duong, json=goi)
    if tra.status_code >= 400:
        # Chuyển nguyên trạng 422 và 429: chúng là câu trả lời cho người dùng,
        # không phải sự cố. Còn lại gộp thành 502, vì đó là chuyện giữa hai máy.
        if tra.status_code in (422, 429):
            raise HTTPException(
                status_code=tra.status_code, detail=tra.json().get("detail")
            )
        raise HTTPException(status_code=502, detail="nep_media_proxy_tu_choi")
    return NepMediaDaNhan(job_id=the, loai=loai, trang_thai="dang-cho")


@router.get("/me/nep/media/{job_id}", response_model=NepMediaTinhHinh)
def trang_thai_media(
    job_id: str, actor: Annotated[Actor, Depends(get_actor)]
) -> NepMediaTinhHinh:
    _cua_toi(job_id, actor)
    tra = _goi_proxy("GET", f"/v1/media/{job_id}")
    if tra.status_code >= 400:
        raise HTTPException(status_code=502, detail="nep_media_proxy_tu_choi")
    goc = tra.json()
    trang_thai = goc.get("trang_thai")
    if trang_thai not in ("dang-cho", "dang-chay", "xong", "hong"):
        # Một trạng thái lạ là một hợp đồng đã trượt, không phải một trạng thái
        # để đoán. Nói ra chứ đừng ánh xạ bừa về «đang chờ».
        raise HTTPException(status_code=502, detail="nep_media_trang_thai_la")
    return NepMediaTinhHinh(
        job_id=job_id,
        trang_thai=trang_thai,
        so_byte=goc.get("so_byte"),
        loi=(str(goc["loi"])[:400] if goc.get("loi") else None),
    )


@router.get("/me/nep/media/{job_id}/file")
def file_media(job_id: str, actor: Annotated[Actor, Depends(get_actor)]) -> Response:
    _cua_toi(job_id, actor)
    tra = _goi_proxy("GET", f"/v1/media/{job_id}/file")
    if tra.status_code == 404:
        raise HTTPException(status_code=404, detail="chua_co_media")
    if tra.status_code >= 400:
        raise HTTPException(status_code=502, detail="nep_media_proxy_tu_choi")
    return Response(
        content=tra.content,
        media_type=tra.headers.get("content-type", "application/octet-stream"),
        # Bytes đi qua chứ không chuyển hướng: một URL ký sẵn trỏ thẳng vào
        # proxy là một đường vòng qua chỗ kiểm quyền sở hữu ở trên.
        headers={"Cache-Control": "private, max-age=3600"},
    )
