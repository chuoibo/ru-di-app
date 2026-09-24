"""Internal brain HTTP seam (ADR-0029 §2.7).

Go owns auth, the database, and the limiter. Python owns the model step:
receipt and screenshot readers, the chat-expense reader, the companion,
place search and reasons, suggestions, the reel, and on-box face detection.
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
import os
from typing import Annotated, Any

from fastapi import APIRouter, Depends, FastAPI, Header, Request
from fastapi.responses import JSONResponse

from app.api.chat_expense_skill import ChatExpenseReader, run_chat_expense_skill
from app.api.deps import (
    Companion,
    ContextualSuggester,
    FaceDetector,
    Reeler,
    Suggester,
    get_chat_expense_reader,
    get_companion,
    get_contextual_suggester,
    get_face_detector,
    get_receipt_reader,
    get_reeler,
    get_screenshot_reader,
    get_suggester,
)
from app.api.internal_token import INTERNAL_TOKEN_HEADER, tokens_match
from app.api.receipt_skill import ReceiptReader, run_receipt_skill
from app.api.screenshot_skill import ScreenshotReader, run_screenshot_skill
from app.domain import money
from app.domain.chat_expense import ChatExpenseError
from app.domain.place_search import PlaceSearchError, ground_search
from app.domain.receipt import ReceiptError
from app.domain.screenshot import ScreenshotError
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


@router.post("/receipt-scan")
def receipt_scan(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    reader: Annotated[ReceiptReader, Depends(get_receipt_reader)],
) -> dict:
    image, content_type = _decode_image(body)
    try:
        return run_receipt_skill(image, content_type, reader=reader)
    except ReceiptError as exc:
        _LOGGER.info("brain receipt refused: %s", exc.code)
        raise _map_receipt(exc.code) from None
    except Exception:
        _LOGGER.warning("brain receipt reader failed")
        raise _code_error(502, "receipt_reader_unavailable") from None


def _map_receipt(code: str) -> BrainProblem:
    if code == "UNSUPPORTED_IMAGE_TYPE":
        return _code_error(415, "unsupported_image_type")
    if code == "IMAGE_TOO_LARGE":
        return _code_error(413, "image_too_large")
    if code == "RECEIPT_TOO_BLURRY":
        return _code_error(422, "receipt_too_blurry")
    if code == "RECEIPT_READER_NOT_CONFIGURED":
        return _code_error(503, "receipt_reader_not_configured")
    if code == "NOT_A_RECEIPT_PRICE_LIST":
        return _code_error(422, "not_a_receipt_price_list")
    if code == "NOT_A_RECEIPT":
        return _code_error(422, "not_a_receipt")
    return _code_error(422, "receipt_unreadable")


@router.post("/screenshot-scan")
def screenshot_scan(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    reader: Annotated[ScreenshotReader, Depends(get_screenshot_reader)],
) -> dict:
    image, content_type = _decode_image(body)
    try:
        return run_screenshot_skill(image, content_type, reader=reader)
    except ScreenshotError as exc:
        _LOGGER.info("brain screenshot refused: %s", exc.code)
        raise _map_screenshot(exc.code) from None
    except Exception:
        _LOGGER.warning("brain screenshot reader failed")
        raise _code_error(502, "screenshot_reader_unavailable") from None


def _map_screenshot(code: str) -> BrainProblem:
    if code == "UNSUPPORTED_IMAGE_TYPE":
        return _code_error(415, "unsupported_image_type")
    if code == "IMAGE_TOO_LARGE":
        return _code_error(413, "image_too_large")
    if code == "SCREENSHOT_READER_NOT_CONFIGURED":
        return _code_error(503, "screenshot_reader_not_configured")
    if code == "NOT_A_TRANSACTION":
        return _code_error(422, "not_a_transaction")
    if code == "MODEL_NAMED_A_PERSON":
        return _code_error(422, "screenshot_model_named_a_person")
    return _code_error(422, "screenshot_unreadable")


@router.post("/chat-expense")
def chat_expense(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    reader: Annotated[ChatExpenseReader, Depends(get_chat_expense_reader)],
) -> dict:
    text = body.get("text")
    if not isinstance(text, str):
        raise _code_error(422, "brain_request_invalid")
    try:
        return run_chat_expense_skill(text, reader=reader)
    except ChatExpenseError as exc:
        _LOGGER.info("brain chat expense refused: %s", exc.code)
        if exc.code == "CHAT_READER_NOT_CONFIGURED":
            raise _code_error(503, "chat_reader_not_configured") from None
        if exc.code == "MODEL_NAMED_A_PERSON":
            raise _code_error(422, "chat_expense_model_named_a_person") from None
        raise _code_error(422, "chat_expense_unreadable") from None
    except Exception:
        _LOGGER.warning("brain chat expense reader failed")
        raise _code_error(502, "chat_reader_unavailable") from None


@router.post("/companion-reply")
def companion_reply(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    companion: Annotated[Companion, Depends(get_companion)],
) -> dict:
    try:
        return companion.reply(
            conversation=_list_of_dict(body, "conversation"),
            members=_list_of_dict(body, "members"),
            places=_list_of_dict(body, "places"),
            budget_per_person_vnd=_optional_int(body.get("budget_per_person_vnd")),
        )
    except Exception:
        _LOGGER.warning("brain companion failed")
        raise _code_error(502, "companion_unavailable") from None


@router.post("/capabilities")
def inference_capabilities(
    _: Annotated[None, Depends(require_internal_token)],
) -> dict:
    """Report inference configuration only; never expose credential values."""

    configured = bool(os.environ.get("GEMINI_API_KEY", "").strip())
    return {
        "plan": {
            "available": configured,
            "reason": None if configured else "provider_not_configured",
        }
    }


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


@router.post("/suggestion")
def suggestion(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    suggester: Annotated[Suggester, Depends(get_suggester)],
) -> dict:
    history = body.get("history")
    places = body.get("places")
    if not isinstance(history, dict) or not isinstance(places, list):
        raise _code_error(422, "brain_request_invalid")
    try:
        card = suggester(history, places)
    except Exception:
        _LOGGER.warning("brain suggestion failed")
        raise _code_error(502, "suggestion_unavailable") from None
    return {"card": card}


@router.post("/contextual-suggestion")
def contextual_suggestion(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    suggester: Annotated[ContextualSuggester, Depends(get_contextual_suggester)],
) -> dict:
    digest = body.get("digest")
    places = body.get("places")
    if not isinstance(digest, dict) or not isinstance(places, list):
        raise _code_error(422, "brain_request_invalid")
    try:
        card = suggester(digest, places)
    except Exception:
        _LOGGER.warning("brain contextual suggestion failed")
        raise _code_error(502, "contextual_suggestion_unavailable") from None
    return {"card": card}


@router.post("/reel")
def reel(
    body: dict,
    _: Annotated[None, Depends(require_internal_token)],
    reeler: Annotated[Reeler, Depends(get_reeler)],
) -> dict:
    trip = body.get("trip")
    memories = body.get("memories")
    if not isinstance(trip, dict) or not isinstance(memories, list):
        raise _code_error(422, "brain_request_invalid")
    try:
        card = reeler(trip, memories)
    except Exception:
        _LOGGER.warning("brain reel failed")
        raise _code_error(502, "reel_unavailable") from None
    return {"card": card}


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


def _list_of_dict(body: dict, key: str) -> list[dict]:
    value = body.get(key)
    if not isinstance(value, list):
        raise _code_error(422, "brain_request_invalid")
    out: list[dict] = []
    for item in value:
        if isinstance(item, dict):
            out.append(item)
    return out


def _optional_int(value: Any) -> int | None:
    """An optional integer đồng, checked where every other đồng is checked.

    The predicate `isinstance(v, bool) or not isinstance(v, int)` used to be
    spelled out here, and `tests/test_one_money_check.py` caught it: money.py
    is the one file allowed to spell that shape, everything else calls it.
    That gate exists because the same three lines had already been pasted into
    seven places, each drifting a little.

    Only NOT_INTEGER is rejected, not every violation `vnd_violation` knows.
    A negative budget is nonsense and the caller already refused it, but
    tightening this door would change behaviour inside a branch whose whole
    claim is that behaviour did not change. It is written down instead.
    """
    if value is None:
        return None
    if money.vnd_violation(value) == money.NOT_INTEGER:
        raise _code_error(422, "brain_request_invalid")
    return value


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
