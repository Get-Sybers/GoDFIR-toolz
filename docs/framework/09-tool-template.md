# 09 — The tool template (`_template/`)

[← 08 — Zero-CVE program](08-zero-cve-program.md) · [10 — Migration roadmap →](10-migration-roadmap.md)

Adding a tool is `cp -r _template <newtool>/`, rename the binary, fill
`contract.yml`. The skeleton is a complete, gate-passing Shape-A tool with the
tool logic left as a stub.

## 9.1 Skeleton

```
_template/
├── Dockerfile
├── contract.yml
├── README.md
├── main.go            # (Go tools) entrypoint stub implementing 03 and 04
├── main_test.go
└── test/
    ├── fixtures/.keep
    └── contract_test.sh
```

## 9.2 `_template/Dockerfile`

Shape A by default; the header comment points to Shape B for tools that need
libraries or an interpreter.

```dockerfile
# syntax=docker/dockerfile:1
# get-sybers/<TOOL> — <one-line factual description>.
# Self-orchestrating, env-driven, run-once-exit. Batch ENTRYPOINT built in.
# FROM scratch: static binary, no shell/python/pkg-mgr/libc, runs as uid 2000.
# (Needs glibc/.so or an interpreter? switch to the anamnesis Shape-B pattern
#  and JUSTIFY the kept interpreter in this header.)
ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /<TOOL> .
RUN /<TOOL> --version

FROM scratch
COPY --from=build /<TOOL> /<TOOL>
COPY <<EOF /etc/dfir-hardened
schema=1
tool=<TOOL>
user=dfir uid=2000 gid=2000
static_binary=true
shell=false python=false pkg_mgr=false
EOF
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ENV HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/<TOOL>"]
LABEL org.opencontainers.image.title="get-sybers/<TOOL>" \
      org.opencontainers.image.description="<factual sentence>" \
      org.opencontainers.image.source="<upstream URL>" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="0.1.0" \
      com.get-sybers.tool="<TOOL>" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1"
```

`org.opencontainers.image.revision` and `com.get-sybers.godfir-release` are
stamped by the build (`build-all.sh` passes them as build-args from the
checkout and the release tag) rather than hand-written in the template.

## 9.3 `_template/contract.yml`

The [§3.7](03-environment-contract.md) skeleton with `<TOOL>` placeholders and
the reserved variables pre-filled to their defaults:

```yaml
tool: <TOOL>
image: get-sybers/<TOOL>:latest
entrypoint: self-orchestrating
description: >
  <one factual sentence>

env:
  <TOOL>_INPUT_DIR: {required: false, default: /input,  desc: evidence tree (recursed, read-only)}
  <TOOL>_OUT_DIR:   {required: false, default: /output, desc: output root, one folder per input item}
  <TOOL>_WORK_DIR:  {required: false, default: /work,   desc: scratch (writable tmpfs)}
  <TOOL>_FORCE:     {required: false, default: "0", type: bool, desc: rerun items that already have valid output}
  <TOOL>_FORMAT:    {required: false, default: json,    desc: output format (json|csv)}
  <TOOL>_LOG_LEVEL: {required: false, default: info,    desc: error|warn|info|debug, stderr only}

mounts:
  - {name: input,  path: /input,  mode: ro, env: <TOOL>_INPUT_DIR, required: true}
  - {name: output, path: /output, mode: rw, env: <TOOL>_OUT_DIR,   required: true}
  - {name: work,   path: /work,   mode: rw, env: <TOOL>_WORK_DIR,  required: false}

network: none
exit_codes: {0: success, 1: nothing_produced, 2: config_error, 3: partial}

summary_schema:
  required: [tool, version, status, inputs, processed, failed, outputs, exit]

outputs:
  layout: "<OUT_DIR>/<input item>/<TOOL>.jsonl"
```

## 9.4 `_template/main.go`

The entrypoint stub encodes [04](04-self-orchestration.md): read the
environment → discover inputs under `INPUT_DIR` → for each item,
skip-if-done-unless-`FORCE` → process → collect counts → print one JSON summary
line to stdout → exit per the [§3.5](03-environment-contract.md) table. Any
argv switches to the single-item pass-through. The stub is derived from the
shared refactor of the Tier-1 Go tools
([§10.2](10-migration-roadmap.md)), so a new tool inherits the same
discovery, idempotency, and summary code paths.

## 9.5 `_template/test/contract_test.sh`

Builds the image, runs it over `fixtures/` in batch mode, asserts
one-JSON-line stdout, the schema, and the exit code; re-runs to assert
idempotency; then runs once with a bad environment to assert exit `2`. It is
the same sequence the gate performs in [§6.5](06-verification-gate.md), so a
tool that passes its own test passes the gate's conformance step.

## 9.6 `_template/README.md`

Headings mirroring the contract — Input / Env / Output / Exit codes / Run —
in factual present tense, with no issue references. The gate asserts the
README agrees with `contract.yml`.

[← 08 — Zero-CVE program](08-zero-cve-program.md) · [10 — Migration roadmap →](10-migration-roadmap.md)
