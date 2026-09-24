# syntax=docker/dockerfile:1
# get-sybers/gousers — Linux account-and-access surface parser: passwd, shadow, group, gshadow, sudoers (+sudoers.d), sshd_config (+drop-ins), authorized_keys and known_hosts become typed records — native values verbatim, shadow day-counts also rendered as dates.
# Static Go on the shared pinfo module. Self-orchestrating, env-driven,
# run-once-exit: with no arguments the binary batches over GOUSERS_INPUT_DIR
# into GOUSERS_OUT_DIR (one folder per item) and prints one JSON summary line;
# argv is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies the sibling pinfo/ module):
#   docker build -t get-sybers/gousers:latest -f gousers/Dockerfile .

ARG GO_VERSION=1.24
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY pinfo/ pinfo/
COPY gousers/go.mod gousers/
WORKDIR /src/gousers
RUN go mod download
COPY gousers/*.go gousers/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gousers .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /gousers --version

FROM scratch
COPY --from=build /gousers /gousers
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=gousers
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
ENTRYPOINT ["/gousers"]
LABEL org.opencontainers.image.title="get-sybers/gousers" \
      org.opencontainers.image.description="Linux account-and-access surface parser: passwd, shadow, group, gshadow, sudoers (+sudoers.d), sshd_config (+drop-ins), authorized_keys and known_hosts become typed records — native values verbatim, shadow day-counts also rendered as dates. Env-driven batch entrypoint on the shared pinfo module; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gousers" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
