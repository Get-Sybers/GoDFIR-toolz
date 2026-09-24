# syntax=docker/dockerfile:1
# get-sybers/gosyslog — Syslog-family text log parser: syslog, messages, auth.log, secure, kern.log, cron and friends — rotations and gzip included — one record per line, RFC3164/ISO/RFC5424 timestamps normalised, sshd/sudo/pam/cron families typed by the parser.
# Static Go on the shared pinfo module. Self-orchestrating, env-driven,
# run-once-exit: with no arguments the binary batches over GOSYSLOG_INPUT_DIR
# into GOSYSLOG_OUT_DIR (one folder per item) and prints one JSON summary line;
# argv is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies the sibling pinfo/ module):
#   docker build -t get-sybers/gosyslog:latest -f gosyslog/Dockerfile .

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY pinfo/ pinfo/
COPY gosyslog/go.mod gosyslog/
WORKDIR /src/gosyslog
RUN go mod download
COPY gosyslog/*.go gosyslog/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gosyslog .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /gosyslog --version

FROM scratch
COPY --from=build /gosyslog /gosyslog
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=gosyslog
user=dfir uid=2000 gid=2000
static_binary=true
shell=false python=false pkg_mgr=false
HARDENED
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
ENV HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/gosyslog"]
LABEL org.opencontainers.image.title="get-sybers/gosyslog" \
      org.opencontainers.image.description="Syslog-family text log parser: syslog, messages, auth.log, secure, kern.log, cron and friends — rotations and gzip included — one record per line, RFC3164/ISO/RFC5424 timestamps normalised, sshd/sudo/pam/cron families typed by the parser. Env-driven batch entrypoint on the shared pinfo module; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gosyslog" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
