# syntax=docker/dockerfile:1
# get-sybers/gosbe — Windows ShellBags (BagMRU) parser (static Go on Velociraptor's regparser).
# Self-orchestrating, env-driven, run-once-exit: with no arguments the binary
# batches over GOSBE_INPUT_DIR into GOSBE_OUT_DIR (one folder per NTUSER.DAT /
# UsrClass.dat hive, .LOG1/.LOG2 replayed into GOSBE_WORK_DIR)
# and prints one JSON summary line; argv (-f/-d …) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
#   docker build -t get-sybers/gosbe:latest -f gosbe/Dockerfile gosbe

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gosbe .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /gosbe --version

FROM scratch
COPY --from=build /gosbe /gosbe
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gosbe
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
ENTRYPOINT ["/gosbe"]
LABEL org.opencontainers.image.title="get-sybers/gosbe" \
      org.opencontainers.image.description="Windows ShellBags (BagMRU) parser (static Go, regparser): finds every NTUSER.DAT / UsrClass.dat under the input tree, replays sibling .LOG1/.LOG2, walks every BagMRU root, decodes the shell items and writes one record per shellbag with its reconstructed AbsolutePath as JSONL, one folder per hive. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gosbe" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/regparser"
