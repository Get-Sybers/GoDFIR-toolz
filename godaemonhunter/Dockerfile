# syntax=docker/dockerfile:1
# get-sybers/godaemonhunter — the Linux daemon-parser matrix as one static
# binary (every parser a sub-tool; hunt is the layered all-streams run).
# Imported from the published module at build time (go install); the shared
# gopinfo module is pulled transitively. No Go source lives in this repo.

ARG GO_VERSION=1.27
ARG TOOL_VERSION=0.2.1
ARG GOMOUNT_VERSION=0.2.1
ARG GODFIR_REVISION=unknown
# the get-sybers image release this Dockerfile ships as: the ONE version the
# workflow tags the pushed container with (synchronised across images, §31)
ARG GODFIR_RELEASE=v0.2.1

# godaemonhunter is imported from its published module at build time
# (github.com/Get-Sybers/godaemonhunter), which pulls the shared gopinfo module
# transitively — no Go source lives in this repo.
FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN go install -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" \
      github.com/Get-Sybers/godaemonhunter@v${TOOL_VERSION}
# build-stage sanity gate
RUN /go/bin/godaemonhunter --version

# gomount: the CGO-free static disk-image reader the lane runs, imported from its
# published module (github.com/Get-Sybers/gomount)
FROM golang:${GO_VERSION}-alpine AS gomount-build
ARG GOMOUNT_VERSION
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN go install -trimpath -ldflags="-s -w -X main.version=${GOMOUNT_VERSION}" \
      github.com/Get-Sybers/gomount@v${GOMOUNT_VERSION}

FROM scratch
COPY --from=build /go/bin/godaemonhunter /godaemonhunter
COPY --from=gomount-build /go/bin/gomount /usr/local/bin/gomount
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
