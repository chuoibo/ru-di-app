"""HTTP surface of the sidecar. Only Go calls it, on loopback / the private
network, with a shared internal token. Bodies are never logged: the access
line is method, route, status and milliseconds; a validation error names the
field, never the value that failed.

Run: uvicorn --factory ai_infer.app:from_env --host 127.0.0.1 --port 8090
"""

from __future__ import annotations

import hmac
import logging
import time
from typing import Annotated, Literal, Optional

from fastapi import Depends, FastAPI, Header, HTTPException, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, StringConstraints

from ai_infer import config
from ai_infer.sparse import common
from ai_infer.sparse.encoders import Encoder

log = logging.getLogger("ai_infer")

OwnerId = Annotated[
    str,
    StringConstraints(min_length=1, max_length=128, pattern=r"^[^\s\x00-\x1f\x7f]+$"),
]
Text = Annotated[str, StringConstraints(min_length=1, max_length=common.MAX_TEXT_CHARS)]
MemText = Annotated[str, StringConstraints(min_length=1, max_length=2000)]


class SparseReq(BaseModel):
    kind: Literal["query", "doc"]
    texts: list[Text] = Field(min_length=1, max_length=common.MAX_TEXTS)


class SparseVec(BaseModel):
    indices: list[int]
    values: list[float]


class SparseResp(BaseModel):
    model: str
    model_revision: str
    kind: str
    vectors: list[SparseVec]


class Message(BaseModel):
    # Only the person's own words are ever sent for extraction (never the
    # assistant's answers, never group chat): the role is fixed.
    role: Literal["user"]
    content: MemText


class AddReq(BaseModel):
    user_id: OwnerId
    messages: list[Message] = Field(min_length=1, max_length=20)


class SearchReq(BaseModel):
    user_id: OwnerId
    query: MemText
    top_k: int = Field(default=5, ge=1, le=10)
    threshold: float = Field(default=0.3, ge=0.0, le=1.0)


class ListReq(BaseModel):
    user_id: OwnerId
    limit: int = Field(default=100, ge=1, le=1000)


class DeleteReq(BaseModel):
    user_id: OwnerId
    memory_id: Annotated[
        str, StringConstraints(min_length=1, max_length=64, pattern=r"^[A-Za-z0-9-]+$")
    ]


class OwnerReq(BaseModel):
    user_id: OwnerId


RerankText = Annotated[str, StringConstraints(min_length=1, max_length=4000)]


class RerankReq(BaseModel):
    # The vLLM/Cohere /rerank contract internal/rerank sends (Go).
    model: Annotated[str, StringConstraints(min_length=1, max_length=128)]
    query: Annotated[str, StringConstraints(min_length=1, max_length=2000)]
    documents: list[RerankText] = Field(min_length=1, max_length=64)
    top_n: int = Field(ge=1, le=64)
    return_documents: bool = False


def _item(r: dict) -> dict:
    meta = r.get("metadata") or {}
    return {
        "id": r["id"],
        "text": r.get("memory", ""),
        "score": r.get("score"),
        "loai": meta.get("loai"),
    }


def create_app(
    settings: config.Settings,
    *,
    encoder: Optional[Encoder] = None,
    memory=None,
    openrouter=None,
) -> FastAPI:
    app = FastAPI(title="ai-infer", docs_url=None, redoc_url=None, openapi_url=None)
    token = settings.token.encode()

    def auth(authorization: Annotated[Optional[str], Header()] = None) -> None:
        got = (authorization or "").removeprefix("Bearer ").encode()
        if (
            not authorization
            or not authorization.startswith("Bearer ")
            or not hmac.compare_digest(got, token)
        ):
            raise HTTPException(status_code=401, detail="unauthorized")

    Auth = Depends(auth)

    @app.middleware("http")
    async def access_log(request: Request, call_next):
        t = time.perf_counter()
        resp = await call_next(request)
        log.info(
            "%s %s %d %.1fms",
            request.method,
            request.url.path,
            resp.status_code,
            (time.perf_counter() - t) * 1000,
        )
        return resp

    @app.exception_handler(RequestValidationError)
    async def invalid(_: Request, exc: RequestValidationError):
        # Field paths and error kinds only: FastAPI's default echoes the input.
        errs = [
            {"loc": list(e.get("loc", ())), "type": e.get("type")} for e in exc.errors()
        ]
        return JSONResponse(status_code=422, content={"detail": errs})

    @app.get("/healthz")
    def healthz():
        # Touches no model and no store: restarting this does not fix them.
        return {
            "ok": True,
            "sparse": settings.sparse_mode,
            "memory": settings.gemini_mode,
        }

    @app.post("/rerank", dependencies=[Auth])
    def rerank(req: RerankReq):
        # Only the configured model: the sidecar spends the owner's key.
        from ai_infer.openrouter import RERANK_MODEL, OpenRouterError

        if openrouter is None:
            raise HTTPException(status_code=503, detail="reranker not configured")
        if req.model != RERANK_MODEL:
            raise HTTPException(status_code=400, detail="model not allowed")
        try:
            results = openrouter.rerank(req.query, req.documents, min(req.top_n, len(req.documents)))
        except OpenRouterError as exc:
            raise HTTPException(status_code=502, detail=str(exc)) from None
        return {"model": RERANK_MODEL, "results": results}

    @app.post("/v1/sparse", response_model=SparseResp, dependencies=[Auth])
    def sparse(req: SparseReq):
        if encoder is None:
            raise HTTPException(status_code=503, detail="sparse encoder disabled")
        vecs = encoder.encode(list(req.texts), req.kind)
        if len(vecs) != len(req.texts):
            raise HTTPException(
                status_code=500, detail="encoder answered the wrong number of vectors"
            )
        for idx, val in vecs:
            common.check(idx, val, req.kind)
        return SparseResp(
            model=encoder.model,
            model_revision=encoder.model_revision,
            kind=req.kind,
            vectors=[SparseVec(indices=i, values=v) for i, v in vecs],
        )

    def mem():
        if memory is None:
            raise HTTPException(status_code=503, detail="memory disabled")
        return memory

    from ai_infer.mem.service import DeleteIncomplete, NotFound, OwnerLeak  # noqa: E402

    @app.exception_handler(NotFound)
    async def not_found(_: Request, __: NotFound):
        return JSONResponse(status_code=404, content={"detail": "not found"})

    @app.exception_handler(DeleteIncomplete)
    async def incomplete(_: Request, exc: DeleteIncomplete):
        return JSONResponse(
            status_code=500,
            content={"detail": "delete incomplete", "remaining": exc.remaining},
        )

    @app.exception_handler(OwnerLeak)
    async def leak(_: Request, __: OwnerLeak):
        log.error("owner leak refused")
        return JSONResponse(status_code=500, content={"detail": "owner check failed"})

    @app.post("/v1/memory/add", dependencies=[Auth])
    def add(req: AddReq):
        added, refused = mem().add(req.user_id, [m.content for m in req.messages])
        return {
            "added": [{"id": a.id, "text": a.text, "loai": a.loai} for a in added],
            "refused": refused,
        }

    @app.post("/v1/memory/search", dependencies=[Auth])
    def search(req: SearchReq):
        return {
            "items": [
                _item(r)
                for r in mem().search(req.user_id, req.query, req.top_k, req.threshold)
            ]
        }

    @app.post("/v1/memory/list", dependencies=[Auth])
    def list_(req: ListReq):
        items = [_item(r) for r in mem().list(req.user_id, req.limit)]
        return {"items": items, "count": len(items)}

    @app.post("/v1/memory/delete", dependencies=[Auth])
    def delete(req: DeleteReq):
        return {"deleted": mem().delete(req.user_id, req.memory_id), "remaining": 0}

    @app.post("/v1/memory/delete_all", dependencies=[Auth])
    def delete_all(req: OwnerReq):
        deleted, remaining = mem().delete_all(req.user_id)
        return {"deleted": deleted, "remaining": remaining}

    @app.post("/v1/memory/purge_user", dependencies=[Auth])
    def purge_user(req: OwnerReq):
        deleted, remaining, seconds = mem().purge_user(req.user_id)
        return {
            "deleted": deleted,
            "remaining": remaining,
            "compaction_s": round(seconds, 2),
        }

    return app


def from_env() -> FastAPI:
    """Production entry point: everything from the environment."""
    logging.basicConfig(
        level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s"
    )
    s = config.load()
    from ai_infer.sparse.encoders import build as build_encoder

    encoder = build_encoder(s)
    openrouter = None
    if s.openrouter_api_key:
        from ai_infer.openrouter import OpenRouter

        openrouter = OpenRouter(s.openrouter_api_key, s.openrouter_base_url, s.openrouter_timeout_s)
    memory = None
    if s.gemini_mode != config.GEMINI_OFF:
        from ai_infer.mem import bootstrap, gemini
        from ai_infer.mem.service import MemoryService

        if s.gemini_mode == config.GEMINI_REAL:
            llm = gemini.GeminiLlm(
                s.gemini_api_key, s.gemini_base_url, s.gemini_timeout_s
            )
            emb = gemini.GeminiEmbed(
                s.gemini_api_key, s.gemini_base_url, s.gemini_timeout_s
            )
        else:
            llm, emb = gemini.StubLlm(), gemini.StubEmbed()
        memory = MemoryService(
            bootstrap.build(
                llm=llm,
                embed=emb,
                milvus_uri=s.milvus_uri,
                milvus_token=s.milvus_token,
                db_name=s.milvus_db,
                collection=s.memory_collection,
            )
        )
    return create_app(s, encoder=encoder, memory=memory, openrouter=openrouter)
