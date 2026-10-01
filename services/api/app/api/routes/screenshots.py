"""`POST /screenshots/scan`: served by Go since ADR-0052.

The screenshot reader moved to `services/core` (internal/routes/scans_ai.go,
internal/aiharness/docanh, internal/domain/screenshot). The declaration stays:
the Go front door takes its route order, request contract and limiter from
this app's table (services/core/ownership/routes.json, `python: frozen`).
"""

from __future__ import annotations

from typing import Annotated

from fastapi import APIRouter, Depends, Request, UploadFile

from app.api.deps import Actor, get_actor
from app.api.errors import ApiProblem
from app.api.schemas import ErrorResponse, ScreenshotScanResponse
from app.api.search_rate_limit import FixedWindowLimiter

router = APIRouter(tags=["screenshots"])


def get_screenshot_scan_limiter(request: Request) -> FixedWindowLimiter:
    """The one limiter `create_app` built; Go keeps the same window."""

    return request.app.state.screenshot_scan_limiter


@router.post(
    "/screenshots/scan",
    response_model=ScreenshotScanResponse,
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
def scan_screenshot(
    image: UploadFile,
    actor: Annotated[Actor, Depends(get_actor)],
    limiter: Annotated[FixedWindowLimiter, Depends(get_screenshot_scan_limiter)],
) -> ScreenshotScanResponse:
    """Declaration only: the Go core serves this route (ADR-0052)."""

    del image, actor, limiter
    raise ApiProblem(410, "served_by_go", "POST /screenshots/scan do core Go phục vụ.")
