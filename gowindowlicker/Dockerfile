# syntax=docker/dockerfile:1
# get-sybers/gowindowlicker — the Windows artefact dozen as one static binary
# (every parser a sub-tool; lick is the all-parsers sweep, the default).
# Both binaries are go-installed from their published modules at build time —
# gowindowlicker and the gomount disk-image reader, which pulls a disk image's
# artefact sets in-container so the parsers run on the image
# (GOWINDOWLICKER_IMAGE) — no mount, no FUSE, no privilege.

ARG GO_VERSION=1.27
ARG TOOL_VERSION=0.2.0
ARG GOMOUNT_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
# the get-sybers image release this Dockerfile ships as: the ONE version the
# workflow tags the pushed container with (synchronised across images, §31)
ARG GODFIR_RELEASE=v0.2.1

# gowindowlicker is imported from its published module at build time
# (github.com/Get-Sybers/gowindowlicker) — no Go source lives in this repo.
FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN go install -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" \
      github.com/Get-Sybers/gowindowlicker@v${TOOL_VERSION}
# build-stage sanity gate
RUN /go/bin/gowindowlicker --version
# gore's bundled batch definition travels with the published module; install it
# world-readable so uid 2000 can read it (the module cache is mode 0444).
# Locate the installed module in the cache; the glob covers Go's case-escaping
# of the Get-Sybers namespace in the module path.
RUN MODDIR="$(echo "$(go env GOMODCACHE)"/github.com/!get-!sybers/gowindowlicker@v${TOOL_VERSION})" && \
    mkdir -m 0755 /batch && install -m 0644 "$MODDIR/re/batch/default.reb" /batch/default.reb

# gomount: the CGO-free static disk-image reader, imported from its published
# module (github.com/Get-Sybers/gomount)
FROM golang:${GO_VERSION}-alpine AS gomount-build
ARG GOMOUNT_VERSION
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN go install -trimpath -ldflags="-s -w -X main.version=${GOMOUNT_VERSION}" \
      github.com/Get-Sybers/gomount@v${GOMOUNT_VERSION}

FROM scratch
COPY --from=build /go/bin/gowindowlicker /gowindowlicker
COPY --from=build /batch /batch
COPY --from=gomount-build /go/bin/gomount /usr/local/bin/gomount
COPY <<HARDENED /etc/dfir-hardened
schema=1
tool=gowindowlicker
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
ENTRYPOINT ["/gowindowlicker"]
LABEL org.opencontainers.image.title="get-sybers/gowindowlicker" \
      org.opencontainers.image.description="The Windows artefact dozen as one structured binary: every Windows Go parser (prefetch, ESE/SRUM, Recycle Bin, \$MFT, Amcache, ShimCache, evtx, registry, ShellBags, .lnk, jump lists, Windows Timeline) embedded as a sub-tool, plus lick — the sweep that runs every parser over one evidence tree. Env-driven multi-tool entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gowindowlicker" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Get-Sybers/gowindowlicker"
