# syntax=docker/dockerfile:1
# get-sybers/gowtmp — Linux login-record parser (utmp/wtmp/btmp/lastlog),
# static Go on the shared pinfo module. Self-orchestrating, env-driven,
# run-once-exit: with no arguments the binary batches over GOWTMP_INPUT_DIR
# into GOWTMP_OUT_DIR (one folder per file) and prints one JSON summary line;
# argv (-f/-d/--tar) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies the sibling pinfo/ module):
#   docker build -t get-sybers/gowtmp:latest -f gowtmp/Dockerfile .

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY pinfo/ pinfo/
COPY gowtmp/go.mod gowtmp/
WORKDIR /src/gowtmp
RUN go mod download
COPY gowtmp/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gowtmp .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /gowtmp --version

FROM scratch
COPY --from=build /gowtmp /gowtmp
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gowtmp
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
ENTRYPOINT ["/gowtmp"]
LABEL org.opencontainers.image.title="get-sybers/gowtmp" \
      org.opencontainers.image.description="Linux login-record parser (static Go, pinfo module): finds every utmp, wtmp, btmp and lastlog file under the input tree — rotations and gzip included — and writes one record per entry (native utmp vocabulary, login source host and IP, pid, terminal, exit status; lastlog per-uid last logins) as JSONL, one folder per file, with Origin/Snapshot/Residue provenance stamped from a stage manifest. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gowtmp" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
