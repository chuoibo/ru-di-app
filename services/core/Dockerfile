# Go front door (ADR-0029). Both stages are pinned by digest, like
# services/api/Dockerfile: the parity harness records which binary answered,
# and a floating tag lets two builds of one commit differ.
ARG GO_IMAGE=golang:1.25.14-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437
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
 && go build -trimpath -ldflags="-s -w" -o /out/core ./cmd/core

# distroless/static: no shell, no package manager, nothing to exec into.
FROM ${RUNTIME_IMAGE}
COPY --from=build /out/core /core
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
