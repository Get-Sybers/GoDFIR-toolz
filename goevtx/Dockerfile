# syntax=docker/dockerfile:1
# get-sybers/goevtx — Windows Event Log (.evtx) parser (static Go on Velociraptor's go-evtx).
# Self-orchestrating, env-driven, run-once-exit: with no arguments the binary
# batches over GOEVTX_INPUT_DIR into GOEVTX_OUT_DIR (one folder per log)
# and prints one JSON summary line; argv (-f/-d …) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
#   docker build -t get-sybers/goevtx:latest -f goevtx/Dockerfile goevtx

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goevtx .
# build-stage sanity gate
RUN /goevtx --version

FROM scratch
COPY --from=build /goevtx /goevtx
COPY <<EOF /etc/dfir-hardened
schema=1
tool=goevtx
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
ENTRYPOINT ["/goevtx"]
LABEL org.opencontainers.image.title="get-sybers/goevtx" \
      org.opencontainers.image.description="Windows Event Log (.evtx) parser (static Go, go-evtx): finds every event log under the input tree and writes one record per event in the shape byakugan's winevt/evtx maps consume (EventId/Level/Provider/Channel/Computer/EventRecordId/TimeCreated/UserId + Payload EventData.Data[@Name/#text]), one folder per log. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goevtx" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/evtx"
