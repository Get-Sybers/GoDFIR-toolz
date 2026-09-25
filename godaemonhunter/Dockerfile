# syntax=docker/dockerfile:1
# get-sybers/godaemonhunter — the Linux matrix as one structured binary: every daemon parser embedded as a sub-tool, plus hunt, the layered Layer-1 → knowledge store → enriched-daemon-parser run.
# Static Go on the shared pinfo module and all twelve parser modules.
# Self-orchestrating, env-driven, run-once-exit: `hunt` batches the whole
# matrix over GODAEMONHUNTER_INPUT_DIR into GODAEMONHUNTER_OUT_DIR and prints
# one aggregate JSON summary line; `<subtool>` runs one parser's ordinary
# batch mode under its canonical <SUBTOOL>_* block.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
# Build from the REPO ROOT (the image copies pinfo/ and every parser module):
#   docker build -t get-sybers/godaemonhunter:latest -f godaemonhunter/Dockerfile .

ARG GO_VERSION=1.25
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
# Module graph first: the dispatcher requires every sibling module by
# replace directive, so each go.mod must exist before the prefetch.
COPY pinfo/go.mod pinfo/
COPY gohost/go.mod gohost/
COPY gousers/go.mod gousers/
COPY gonetwork/go.mod gonetwork/
COPY gojournal/go.mod gojournal/go.sum gojournal/
COPY goauditd/go.mod goauditd/
COPY gowtmp/go.mod gowtmp/
COPY gosyslog/go.mod gosyslog/
COPY gounit/go.mod gounit/
COPY gocron/go.mod gocron/
COPY goshell/go.mod goshell/
COPY gotrash/go.mod gotrash/
COPY goctl/go.mod goctl/
COPY godaemonhunter/go.mod godaemonhunter/go.sum godaemonhunter/
WORKDIR /src/godaemonhunter
RUN go mod download
WORKDIR /src
COPY pinfo/ pinfo/
COPY gohost/ gohost/
COPY gousers/ gousers/
COPY gonetwork/ gonetwork/
COPY gojournal/ gojournal/
COPY goauditd/ goauditd/
COPY gowtmp/ gowtmp/
COPY gosyslog/ gosyslog/
COPY gounit/ gounit/
COPY gocron/ gocron/
COPY goshell/ goshell/
COPY gotrash/ gotrash/
COPY goctl/ goctl/
COPY godaemonhunter/ godaemonhunter/
WORKDIR /src/godaemonhunter
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /godaemonhunter .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /godaemonhunter --version

FROM scratch
COPY --from=build /godaemonhunter /godaemonhunter
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=godaemonhunter
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
ENTRYPOINT ["/godaemonhunter"]
LABEL org.opencontainers.image.title="get-sybers/godaemonhunter" \
      org.opencontainers.image.description="The Linux matrix as one structured binary: every daemon parser embedded as a sub-tool, plus hunt — the layered run that builds the Layer-1 knowledge store and runs every daemon parser enriched by it. Env-driven multi-tool entrypoint on the shared pinfo module; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="godaemonhunter" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
