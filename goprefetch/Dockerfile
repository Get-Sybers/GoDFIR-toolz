# syntax=docker/dockerfile:1
# get-sybers/goprefetch — Windows Prefetch (.pf) parser (static Go on Velociraptor's go-prefetch).
# Self-orchestrating, env-driven, run-once-exit: with no arguments the binary
# batches over GOPREFETCH_INPUT_DIR into GOPREFETCH_OUT_DIR (one folder per
# prefetch file)
# and prints one JSON summary line; argv (-f/-d …) is the debug pass-through.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
#
#   docker build -t get-sybers/goprefetch:latest -f goprefetch/Dockerfile goprefetch

# go-prefetch requires go >= 1.25 (module directive), and the official golang
# images set GOTOOLCHAIN=local, so the builder must be at least 1.25 itself.
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
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /goprefetch .
# Build-stage sanity gate: the binary runs and prints its version.
RUN /goprefetch --version

FROM scratch
COPY --from=build /goprefetch /goprefetch
COPY <<EOF /etc/dfir-hardened
schema=1
tool=goprefetch
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
ENTRYPOINT ["/goprefetch"]
LABEL org.opencontainers.image.title="get-sybers/goprefetch" \
      org.opencontainers.image.description="Windows Prefetch parser (static Go, go-prefetch): finds every *.pf under the input tree — XP through Win11, MAM/LZXpress-Huffman compressed included — and writes one record per file (executable, hash, version, size, run count, last/previous runs, files accessed) as JSONL or CSV, one folder per file. Env-driven batch entrypoint; FROM scratch, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="goprefetch" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/Velocidex/go-prefetch"
