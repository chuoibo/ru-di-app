"""Báo cáo nội dung hoặc người (ADR-0023 §2.4).

Một route, một bảng, không có màn quản trị trong v1. Điều đáng nói nhất về
module này là những gì nó KHÔNG làm: không khoá ai, không ẩn gì, không trả lời
người bị báo cáo, và không echo lại ghi chú người gửi vừa viết. Một sản phẩm
hứa nhiều hơn thế mà không có ai đọc bảng là một sản phẩm nói dối về việc nó
bảo vệ ai.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, status

from app.api.deps import Actor, get_actor, get_repository
from app.api.repository import ApiRepository
from app.api.schemas import ErrorResponse, ReportCreateRequest, ReportResponse
from app.api.service import ApiService

router = APIRouter(tags=["reports"])


@router.post(
    "/reports",
    response_model=ReportResponse,
    status_code=status.HTTP_201_CREATED,
    responses={401: {"model": ErrorResponse}, 422: {"model": ErrorResponse}},
)
def create_report(
    request: ReportCreateRequest,
    actor: Annotated[Actor, Depends(get_actor)],
    repository: Annotated[ApiRepository, Depends(get_repository)],
) -> ReportResponse:
    """Gửi một báo cáo. Trả về id và giờ, không trả lại ghi chú.

    Không kiểm mục tiêu có tồn tại không: một báo cáo về thứ vừa bị xoá vẫn là
    báo cáo người vận hành cần đọc, và một 404 ở đây sẽ nói cho người gửi biết
    thứ họ báo cáo còn hay mất.
    """
    return ApiService(repository).create_report(request, actor)
