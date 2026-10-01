"""`POST /receipts/scan`: served by Go since ADR-0052.

The bill reader moved to `services/core` (internal/routes/scans_ai.go,
internal/aiharness/docbill, internal/domain/receipt), and with it every line
that read a photograph. What stays here is the route's declaration: the Go
front door still takes its route order, its request contract and the limiter
it shares from this app's table (services/core/ownership/routes.json,
`python: frozen`, state PY-DELETED). Reached directly, it says where it went.
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, Request, UploadFile

from app.api.deps import Actor, get_actor
from app.api.errors import ApiProblem
from app.api.schemas import ErrorResponse, ReceiptScanResponse
from app.api.search_rate_limit import FixedWindowLimiter

router = APIRouter(tags=["receipts"])


def get_receipt_scan_limiter(request: Request) -> FixedWindowLimiter:
    """The one limiter `create_app` built; Go keeps the same window."""

    return request.app.state.receipt_scan_limiter


@router.post(
    "/receipts/scan",
    response_model=ReceiptScanResponse,
    responses={
        401: {"model": ErrorResponse},
        413: {"model": ErrorResponse},
        415: {"model": ErrorResponse},
        422: {"model": ErrorResponse},
        429: {"model": ErrorResponse},
        502: {"model": ErrorResponse},
        503: {"model": ErrorResponse},
    },
)
def scan_receipt(
    image: UploadFile,
    actor: Annotated[Actor, Depends(get_actor)],
    limiter: Annotated[FixedWindowLimiter, Depends(get_receipt_scan_limiter)],
) -> ReceiptScanResponse:
    """Declaration only: the Go core serves this route (ADR-0052)."""

    del image, actor, limiter
    raise ApiProblem(410, "served_by_go", "POST /receipts/scan do core Go phục vụ.")
