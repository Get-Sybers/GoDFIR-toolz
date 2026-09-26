# syntax=docker/dockerfile:1
# get-sybers/gomft — raw NTFS $MFT parser (static Go on Velociraptor's go-ntfs).

# go-ntfs requires go >= 1.25 and the golang images set GOTOOLCHAIN=local, so the builder must be >= 1.25
ARG GO_VERSION=1.25
ARG TOOL_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS build
ARG TOOL_VERSION
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /gomft .
# build-stage sanity gate
RUN /gomft --version

FROM scratch
COPY --from=build /gomft /gomft
COPY <<EOF /etc/dfir-hardened
schema=1
tool=gomft
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
ENTRYPOINT ["/gomft"]
LABEL org.opencontainers.image.title="get-sybers/gomft" \
      org.opencontainers.image.description="Raw NTFS \$MFT parser (static Go, go-ntfs): content-detects every \$MFT under the input tree and writes one record per entry (entry/parent refs, name, size, \$SI/\$FN MACB timestamps, flags, ADS) as JSONL or CSV, one folder per table. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="gomft" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/go-ntfs"
