# Go front door (ADR-0029). Both stages are pinned by digest, like
# services/api/Dockerfile: the parity harness records which binary answered,
# and a floating tag lets two builds of one commit differ.
ARG GO_IMAGE=golang:1.26.8-bookworm@sha256:a688600ca24f8a4d3ca77f95b0dd40704a9fc787c826660eb7ba0b641b8b175d
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

FROM ${GO_IMAGE} AS build
WORKDIR /src
# The image already carries the toolchain go.mod names; never fetch another.
ENV GOTOOLCHAIN=local CGO_ENABLED=0
# go.sum comes in with go.mod so the download is checked against the hashes
# the repository recorded, not only against the public checksum database.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go vet ./... \
 && go build -trimpath -ldflags="-s -w" -o /out/core ./cmd/core \
 && go build -trimpath -ldflags="-s -w" -o /out/rudi-ingest ./cmd/rudi-ingest

# distroless/static: no shell, no package manager, nothing to exec into.
FROM ${RUNTIME_IMAGE}
COPY --from=build /out/core /core
# The catalogue loader ships in the same image, so the sync service runs the
# binary that was vetted with the server it feeds (entrypoint /rudi-ingest).
COPY --from=build /out/rudi-ingest /rudi-ingest
# The same uid as services/api, so both processes can share the media volume
# once uploads move to Go.
USER 10001:10001
EXPOSE 8000
# Liveness of this process only, on its loopback port. It deliberately does not
# go through Python: restarting core never fixed a dead API.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/core", "healthcheck"]
ENTRYPOINT ["/core"]
CMD ["serve"]
