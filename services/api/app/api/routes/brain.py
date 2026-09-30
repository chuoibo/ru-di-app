"""Internal brain HTTP seam (ADR-0029 §2.7).

Go owns auth, the database, the limiter and, since ADR-0051, the model calls
that have moved so far. Still here until their Go ports land: community
moderation and Nếp's community draft, place search and reasons. On-box face
detection is OpenCV rather than a model (TODO: redo in Go by another
mechanism).
Nothing in this module opens a repository session. Errors return a closed
`code` and never interpolate a prompt, a model string, or image bytes.

These routes are not in OpenAPI. The public FastAPI table never lists them:
`create_app` hangs this sub-application behind a door that only the backend
network should reach, gated by `X-Internal-Token`.
"""

from __future__ import annotations

import base64
import binascii
import logging
from typing import Annotated, Any

from fastapi import APIRouter, Depends, FastAPI, Header, Request
from fastapi.responses import JSONResponse

from app.api.deps import FaceDetector, get_face_detector
from app.api.internal_token import INTERNAL_TOKEN_HEADER, tokens_match
from app.domain.place_search import PlaceSearchError, ground_search
from app.media.face_detection import FaceDetectorUnavailable
from app.places.catalog import CATEGORIES
from app.places.reasons import ReasonRow, gemini_reasons
from app.places.search import gemini_search
from app.places.taste import UNKNOWN, TasteProfile

router = APIRouter(
    prefix="/internal/brain/v1",
    include_in_schema=False,
    tags=["brain"],
)
_LOGGER = logging.getLogger(__name__)


class BrainProblem(Exception):
    """An internal refusal: HTTP status plus a closed code, nothing else."""

    def __init__(self, status_code: int, code: str) -> None:
        super().__init__(code)
        self.status_code = status_code
        self.code = code


async def brain_problem_response(request: Request, exc: BrainProblem) -> JSONResponse:
    del request
    return JSONResponse(status_code=exc.status_code, content={"code": exc.code})


def require_internal_token(
    request: Request,
    x_internal_token: Annotated[str | None, Header(alias=INTERNAL_TOKEN_HEADER)] = None,
) -> None:
    """Refuse anything that is not the process's own token."""

    expected = getattr(request.app.state, "internal_token", "")
    if not tokens_match(expected, x_internal_token):
        raise BrainProblem(401, "internal_token_invalid")


def _code_error(status: int, code: str) -> BrainProblem:
    return BrainProblem(status, code)


def _decode_image(body: dict) -> tuple[bytes, str]:
    raw = body.get("image")
    content_type = body.get("content_type")
    if not isinstance(raw, str) or not isinstance(content_type, str):
        raise _code_error(422, "brain_request_invalid")
    try:
        return base64.b64decode(raw, validate=True), content_type
    except (binascii.Error, ValueError):
        raise _code_error(422, "brain_request_invalid") from None


@router.get("/ready")
def ready(_: Annotated[None, Depends(require_internal_token)]) -> dict[str, str]:
    """The brain process is up. Deliberately does not touch a model or a DB."""

    return {"status": "ready"}


@router.post("/community-moderate")
def community_moderate(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
) -> dict:
    """Infer relevance and safety; Go retains all publication authority."""
    from app.api.community_inference import infer_community

    try:
        return infer_community("moderate", body)
    except Exception:
        raise _code_error(502, "community_ai_unavailable") from None


@router.post("/community-nep")
def community_nep(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
) -> dict:
    """Return a draft from the explicitly confirmed excerpt only."""
    from app.api.community_inference import infer_community

    try:
        return infer_community("nep", body)
    except Exception:
        raise _code_error(502, "community_ai_unavailable") from None


@router.post("/place-search")
def place_search(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
) -> dict:
    query = body.get("query")
    catalogue = body.get("catalogue")
    if not isinstance(query, str) or not isinstance(catalogue, list):
        raise _code_error(422, "brain_request_invalid")
    try:
        raw = gemini_search(query, catalogue, _taste(body.get("group")))
        if raw is None:
            return {"source": "none", "results": []}
        return ground_search(raw, catalogue, CATEGORIES)
    except PlaceSearchError:
        return {"source": "none", "results": []}
    except Exception:
        _LOGGER.warning("brain place search failed")
        return {"source": "none", "results": []}


@router.post("/place-reasons")
def place_reasons(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
) -> dict:
    rows_in = body.get("rows")
    if not isinstance(rows_in, list):
        raise _code_error(422, "brain_request_invalid")
    rows: list[ReasonRow] = []
    for item in rows_in:
        if not isinstance(item, dict) or not isinstance(item.get("place"), dict):
            continue
        rows.append(ReasonRow(place=item["place"]))
    written = gemini_reasons(rows, _taste(body.get("group")))
    return {
        place_id: {"reason": value.reason, "verdict": value.verdict}
        for place_id, value in written.items()
    }


@router.post("/face-boxes")
def face_boxes(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    detector: Annotated[FaceDetector, Depends(get_face_detector)],
) -> dict:
    image, _content_type = _decode_image(body)
    try:
        found = detector.detect(image)
    except FaceDetectorUnavailable:
        raise _code_error(503, "face_detector_not_configured") from None
    except Exception:
        _LOGGER.warning("brain face detector failed")
        raise _code_error(502, "face_detection_failed") from None
    return {
        "image_width": found.image_width,
        "image_height": found.image_height,
        "faces": [
            {
                "x": box.x,
                "y": box.y,
                "width": box.width,
                "height": box.height,
            }
            for box in found.boxes
        ],
    }


def _taste(raw: Any) -> TasteProfile:
    if not isinstance(raw, dict):
        return UNKNOWN
    try:
        interests = raw.get("interests") or []
        return TasteProfile(
            basis=raw.get("basis") or "chua-biet",
            interests=tuple(interests) if isinstance(interests, list) else (),
            budget_per_person_vnd=raw.get("budget_per_person_vnd"),
            size=raw.get("size"),
            people=int(raw.get("people") or 0),
            people_answered=int(raw.get("people_answered") or 0),
        )
    except (TypeError, ValueError):
        return UNKNOWN


class BrainDoor:
    """ASGI door: `/internal/` never enters the public route table.

    Registration order of the public app stays 0..155. A request whose path
    starts with `/internal/` is handed to the brain sub-app; everything else
    continues into CORS, guest headers, and idempotency as before.
    """

    def __init__(self, app, brain) -> None:
        self.app = app
        self.brain = brain

    async def __call__(self, scope, receive, send) -> None:
        if scope["type"] == "http" and str(scope.get("path") or "").startswith(
            "/internal/"
        ):
            await self.brain(scope, receive, send)
            return
        await self.app(scope, receive, send)


def build_brain_app(token: str) -> FastAPI:
    """A FastAPI app that serves only the brain, with no docs and no OpenAPI."""

    inner = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)
    inner.state.internal_token = token
    inner.include_router(router)
    inner.add_exception_handler(BrainProblem, brain_problem_response)
    return inner
