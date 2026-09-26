# syntax=docker/dockerfile:1
# get-sybers/goese — ESE database dumper for SRUM / SUM (static Go on Velociraptor's go-ese).

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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goese .
# build-stage sanity gate
RUN /goese --version

FROM scratch
COPY --from=build /goese /goese
COPY <<EOF /etc/dfir-hardened
schema=1
tool=goese
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
ENTRYPOINT ["/goese"]
LABEL org.opencontainers.image.title="get-sybers/goese" \
      org.opencontainers.image.description="ESE database dumper (static Go, go-ese): finds every SRUM (SRUDB.dat) and SUM (SUM/*.mdb) database under the input tree and dumps every table, with SruDbIdMapTable AppIdName/UserIdName enrichment and friendly names for the SRUM provider tables, as JSONL or CSV, one folder per database. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goese" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/go-ese"
