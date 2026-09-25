# syntax=docker/dockerfile:1
# get-sybers/goshell — Shell and REPL history parser: bash (HISTTIMEFORMAT stamps), zsh extended (metafied bytes decoded), fish, sh, python, mysql and psql histories become one record per command, sequence order preserved, timestamps only when the artefact carries them.
# Static Go on the shared pinfo module. Self-orchestrating, env-driven,
# run-once-exit: with no arguments the binary batches over GOSHELL_INPUT_DIR
# into GOSHELL_OUT_DIR (one folder per item) and prints one JSON summary line;
# argv is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies the sibling pinfo/ module):
#   docker build -t get-sybers/goshell:latest -f goshell/Dockerfile .

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY pinfo/ pinfo/
COPY goshell/go.mod goshell/
WORKDIR /src/goshell
RUN go mod download
COPY goshell/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goshell .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /goshell --version

FROM scratch
COPY --from=build /goshell /goshell
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=goshell
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
ENTRYPOINT ["/goshell"]
LABEL org.opencontainers.image.title="get-sybers/goshell" \
      org.opencontainers.image.description="Shell and REPL history parser: bash (HISTTIMEFORMAT stamps), zsh extended (metafied bytes decoded), fish, sh, python, mysql and psql histories become one record per command, sequence order preserved, timestamps only when the artefact carries them. Env-driven batch entrypoint on the shared pinfo module; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goshell" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
