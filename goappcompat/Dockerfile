# syntax=docker/dockerfile:1
# get-sybers/goappcompat — AppCompatCache (ShimCache) parser (static Go on Velociraptor's regparser).
# Self-orchestrating, env-driven, run-once-exit: with no arguments the binary
# batches over GOAPPCOMPAT_INPUT_DIR into GOAPPCOMPAT_OUT_DIR (one folder per
# SYSTEM hive, .LOG1/.LOG2 replayed into GOAPPCOMPAT_WORK_DIR)
# and prints one JSON summary line; argv (-f/-d …) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
#   docker build -t get-sybers/goappcompat:latest -f goappcompat/Dockerfile goappcompat

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goappcompat .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /goappcompat --version

FROM scratch
COPY --from=build /goappcompat /goappcompat
COPY <<EOF /etc/dfir-hardened
schema=1
tool=goappcompat
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
ENTRYPOINT ["/goappcompat"]
LABEL org.opencontainers.image.title="get-sybers/goappcompat" \
      org.opencontainers.image.description="AppCompatCache (ShimCache) parser (static Go, regparser): content-detects every SYSTEM hive with an AppCompatCache under the input tree, replays sibling .LOG1/.LOG2, and writes one record per cache entry (ControlSet, CacheEntryPosition, Path, LastModifiedTimeUTC, SourceFile) as JSONL or CSV, one folder per hive. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goappcompat" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/regparser"
