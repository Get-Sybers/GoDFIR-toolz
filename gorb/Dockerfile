# syntax=docker/dockerfile:1
# get-sybers/gorb — Windows Recycle Bin $I record parser (static Go).

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gorb .
# build-stage sanity gate
RUN /gorb --version

FROM scratch
COPY --from=build /gorb /gorb
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gorb
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
ENTRYPOINT ["/gorb"]
LABEL org.opencontainers.image.title="get-sybers/gorb" \
      org.opencontainers.image.description="Windows Recycle Bin \$I parser (static Go): picks every v1 (Vista-8.0) and v2 (Win8.1/10/11) \$I record out of the input tree by header and writes SourceName, FileType, FileName, FileSize and DeletedOn as JSONL or CSV, one folder per record. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gorb" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Get-Sybers/GoDFIR-toolz/tree/main/gorb"
