# syntax=docker/dockerfile:1
# get-sybers/gore — Batch-driven Windows registry extractor (static Go on Velociraptor's regparser).

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gore .
# build-stage sanity gate
RUN /gore --version
# The bundled batch definition, with modes fixed here rather than inherited from
# the checkout, so uid 2000 can read it whatever umask the build host used.
COPY batch/default.reb /batch/default.reb
RUN chmod 0755 /batch && chmod 0644 /batch/default.reb

FROM scratch
COPY --from=build /gore /gore
COPY --from=build /batch /batch
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gore
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
ENTRYPOINT ["/gore"]
LABEL org.opencontainers.image.title="get-sybers/gore" \
      org.opencontainers.image.description="Batch-driven Windows registry extractor (static Go, regparser): content-detects every registry hive under the input tree, replays sibling .LOG1/.LOG2, extracts the keys and values a .reb batch definition names, and writes one record per value in the recmd_batch shape byakugan consumes as JSONL or CSV, one folder per hive. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gore" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/regparser"
