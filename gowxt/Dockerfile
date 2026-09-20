# syntax=docker/dockerfile:1
# get-sybers/gowxt — Windows Timeline (ActivitiesCache.db) parser (static Go on modernc.org/sqlite).
# Self-orchestrating, env-driven, run-once-exit: with no arguments the binary
# batches over GOWXT_INPUT_DIR into GOWXT_OUT_DIR (one folder per database,
# the SQLite working copy lives in GOWXT_WORK_DIR)
# and prints one JSON summary line; argv (-f/-d …) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
#   docker build -t get-sybers/gowxt:latest -f gowxt/Dockerfile gowxt

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gowxt .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /gowxt --version

FROM scratch
COPY --from=build /gowxt /gowxt
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gowxt
user=dfir uid=2000 gid=2000
static_binary=true
shell=false python=false pkg_mgr=false
EOF
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
ENV HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/gowxt"]
LABEL org.opencontainers.image.title="get-sybers/gowxt" \
      org.opencontainers.image.description="Windows Timeline (ActivitiesCache.db) parser (static Go, modernc.org/sqlite, no cgo): finds every SQLite database with an Activity table under the input tree and writes one record per Activity row (executable, display text, content info, start/end/modified/expiration times, duration, type) as JSONL or CSV, one folder per database. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gowxt" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://gitlab.com/cznic/sqlite"
