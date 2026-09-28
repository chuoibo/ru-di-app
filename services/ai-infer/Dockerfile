# syntax=docker/dockerfile:1

# Inference-only sidecar (ADR-0031). Base pinned by digest, the same
# python:3.12-slim index digest services/api uses. NOT built on the machine
# that wrote it (no Docker there): the first `docker build` is on CI.
ARG PYTHON_IMAGE=python:3.12-slim@sha256:09f7da3bc104798d0afb40bc08d23ab2da20a76130cec1f2ef170848f5d85217

FROM ${PYTHON_IMAGE} AS build
WORKDIR /build
RUN python -m venv /venv
ENV PATH="/venv/bin:$PATH"
COPY requirements.txt ./
# The runtime tree only: no pytest, no torch/transformers (requirements-milco.txt
# is for an image built after the MILCO licence review), no spaCy.
RUN pip install --no-cache-dir --upgrade pip \
 && pip install --no-cache-dir -r requirements.txt \
 && for mod in pytest spacy torch transformers; do \
      if python -c "import importlib.util,sys; sys.exit(0 if importlib.util.find_spec('$mod') else 1)"; then \
        echo "FATAL: $mod reached the runtime venv" >&2; exit 1; fi; \
    done

FROM ${PYTHON_IMAGE} AS runtime
RUN useradd --create-home --uid 10002 infer
WORKDIR /srv
COPY --from=build /venv /venv
# MEM0_TELEMETRY is forced off in code too (ai_infer/mem/__init__.py); MEM0_DIR
# is where mem0 writes its config.json -- mount a tmpfs there, never a volume.
ENV PATH="/venv/bin:$PATH" \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    MEM0_TELEMETRY=false \
    MEM0_DIR=/tmp/mem0 \
    HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1
COPY --chown=infer:infer ai_infer ./ai_infer
USER infer
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8090/healthz', timeout=2).status == 200 else 1)"
CMD ["uvicorn", "--factory", "ai_infer.app:from_env", "--host", "0.0.0.0", "--port", "8090", "--no-access-log"]
