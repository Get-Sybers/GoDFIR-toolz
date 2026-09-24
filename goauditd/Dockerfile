# syntax=docker/dockerfile:1
# get-sybers/goauditd — Linux audit log parser: audit.log records coalesced into events by their audit(sec.usec:serial) id, hex-encoded fields decoded, execve argv reassembled, one record per event.
# Static Go on the shared pinfo module. Self-orchestrating, env-driven,
# run-once-exit: with no arguments the binary batches over GOAUDITD_INPUT_DIR
# into GOAUDITD_OUT_DIR (one folder per item) and prints one JSON summary line;
# argv is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies the sibling pinfo/ module):
#   docker build -t get-sybers/goauditd:latest -f goauditd/Dockerfile .

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY pinfo/ pinfo/
COPY goauditd/go.mod goauditd/
WORKDIR /src/goauditd
RUN go mod download
COPY goauditd/*.go goauditd/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goauditd .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /goauditd --version

FROM scratch
COPY --from=build /goauditd /goauditd
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=goauditd
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
ENTRYPOINT ["/goauditd"]
LABEL org.opencontainers.image.title="get-sybers/goauditd" \
      org.opencontainers.image.description="Linux audit log parser: audit.log records coalesced into events by their audit(sec.usec:serial) id, hex-encoded fields decoded, execve argv reassembled, one record per event. Env-driven batch entrypoint on the shared pinfo module; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goauditd" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
