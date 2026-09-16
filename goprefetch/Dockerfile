# syntax=docker/dockerfile:1
#
# goprefetch — Linux-native Windows Prefetch parser for the DX_DFIR pipeline.
#
# This image ships a static Go binary built on Velociraptor's go-prefetch,
# whose pure-Go LZXpress-Huffman implementation parses XP-era through Win11
# prefetch — MAM compressed included — natively on Linux.
#
# The final image is FROM scratch: one static binary, nothing else. The
# hardening contract is met by construction — no shell, no python, no package
# manager, no setuid, no libc even; runs as uid 2000.
#
#   docker build -t get-sybers/goprefetch:latest -f goprefetch/Dockerfile goprefetch
#   docker run --rm --cap-drop ALL --security-opt no-new-privileges \
#     --network none --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
#     get-sybers/goprefetch:latest -d /input --json /output
#
# Output: JSONL (or --csv) — SourceFilename, Executable, Path, Hash, Version,
# RunCount, LastRun, PreviousRuns, FilesAccessed. See goprefetch/main.go for
# the field notes.

# go-prefetch requires go >= 1.25 (module directive), and the official golang
# images set GOTOOLCHAIN=local, so the builder must be at least 1.25 itself.
ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /goprefetch . \
    && /goprefetch -f /dev/null 2>/dev/null || true

FROM scratch
COPY --from=build /goprefetch /goprefetch
COPY <<EOF /etc/dfir-hardened
user=dfir uid=2000
static_binary=true
EOF
# Single run-as uid/gid, passed by the DX_DFIR image role (dxdfir_runtime_uid/gid)
# so this scratch image honours the same fixed non-root USER as the .NET images.
ARG DFIR_UID=2000
ARG DFIR_GID=2000
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/goprefetch"]
LABEL org.opencontainers.image.title="get-sybers/goprefetch" \
      org.opencontainers.image.description="Linux-native Windows Prefetch parser (static Go, go-prefetch): parses XP through Win11 .pf including MAM/LZXpress-Huffman compressed files, natively on Linux. FROM scratch: no shell, no python, no libc, runs as uid 2000." \
      org.opencontainers.image.licenses="MIT" \
      com.get-sybers.tool="goprefetch" \
      com.get-sybers.hardened="true"
