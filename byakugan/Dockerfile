# syntax=docker/dockerfile:1
# get-sybers/byakugan — the external Byakugan MITRE CAR engine in ONE hardened
# image. The engine is cloned + built INTO the image at the ref DX_DFIR's
# sources.yml pins, so the only thing the DX_DFIR checkout holds is how it
# INVOKES this image.
#
# The ENTRYPOINT is the engine's own multi-tool dispatcher, byakugan.cli
# (docs/framework 04 §4.3), reached through the byakugan-entry.py shim —
# `byakugan build|timeline|verify|load` with no further arguments
# self-orchestrates that operation from its BYAKUGAN_<SUBTOOL>_* env block and
# prints one JSON summary line; `byakugan car-vocab` prints the car_action
# vocabulary; a sub-tool followed by arguments is the pass-through to the
# engine's own CLI. The sub-tool set and the processed-layout discovery are
# the engine's: the cloned ref must ship byakugan/cli.py.
# Shape B, declared deviation: python=true (the engine is python: python3 +
# python3-yaml stay). No shell: bash, dash and sh are removed after the
# build-time hardener runs. static_binary=false (CPython). ca-certificates
# also stays in the runtime stage: TLS trust for the load sub-tool's push
# mode (precedent: signatures/Dockerfile runtime stage).
#
#   docker build -t get-sybers/byakugan:latest -f byakugan/Dockerfile .
#   (--build-arg BYAKUGAN_REF=<40-hex sha> overrides the baked pin)
#
# The byakugan engine pin — the ONE place the engine version is set; consumers
# (DX_DFIR included) build without a REF build-arg and inherit it.
ARG BASE=debian:trixie-slim
ARG GO_BASE=golang:1.24-bookworm
ARG BYAKUGAN_REPO=https://github.com/Get-Sybers/byakugan
ARG BYAKUGAN_REF=2561e38db488948113ea03753353fc88a1b990ed
ARG TOOL_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

# --- stage 1: build the Go parse engine -------------------------------------
# byakugan-parse is stdlib-only Go (go/go.mod has no external requires), so
# CGO_ENABLED=0 gives a fully static binary that runs regardless of the runtime
# stage's glibc, and GOTOOLCHAIN=local keeps the build offline (no toolchain
# download). The engine is cloned RECURSIVELY: it reconstructs its object model
# live from the pinned nested submodules (third_party/car + attack-datasources),
# so a plain clone would leave the model sources missing.
FROM ${GO_BASE} AS engine
ARG BYAKUGAN_REPO
ARG BYAKUGAN_REF
ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-trimpath
# git + CA certs installed explicitly (like anamnesis) — never assume the
# golang base happens to ship them.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends git ca-certificates; \
    rm -rf /var/lib/apt/lists/*; \
    git clone "${BYAKUGAN_REPO}" /src; \
    git -C /src checkout "${BYAKUGAN_REF}"; \
    git -C /src submodule update --init --recursive; \
    (cd /src/go && go build -o bin/byakugan-parse ./cmd/byakugan-parse); \
    test -x /src/go/bin/byakugan-parse; \
    # drop every .git dir (repo + submodules) — the model YAML/JSON and the built
    # binary are all that ship; the history does not.
    find /src -name .git -prune -exec rm -rf {} +

# --- stage 2: python runtime + in-image hardening ---------------------------
FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG BYAKUGAN_REF
ARG TOOL_VERSION
# python3 + pyyaml (the engine's ONLY runtime dependency, from apt so there is no
# pip); ca-certificates gives the load sub-tool's push mode a trust store for its
# Elasticsearch/Kibana TLS calls (signatures/Dockerfile runtime-stage precedent);
# ansible-core is the build-time hardener, removed below.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-yaml ca-certificates ansible-core; \
    rm -rf /var/lib/apt/lists/*
# The engine SOURCE TREE (package + its pinned model submodules + the static Go
# parse binary): the engine resolves its model dir relative to the package, so
# it must run from this tree via PYTHONPATH, never from site-packages.
COPY --from=engine /src /opt/byakugan
COPY byakugan/byakugan-entry.py /usr/local/bin/byakugan
COPY byakugan/contract.yml /opt/byakugan/contract.yml
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN chmod 0755 /usr/local/bin/byakugan && chmod 0644 /opt/byakugan/contract.yml
# Build-stage sanity gate: the dispatcher runs and reports the version and the
# pinned engine ref (before the hardener strips the shell this RUN needs).
RUN BYAKUGAN_VERSION=${TOOL_VERSION} BYAKUGAN_REF=${BYAKUGAN_REF} PYTHONPATH=/opt/byakugan /usr/local/bin/byakugan --version
# harden.yml writes the /etc/dfir-hardened self-declaration (schema=1) from the
# values below: python stays (declared); every shell and ansible go.
RUN set -eux; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=byakugan -e harden_static_binary=false -e harden_shell=false -e harden_python=true; \
    PYTHONPATH=/opt/byakugan python3 -c 'import byakugan.cli, byakugan.pipeline, byakugan.timeline, byakugan.verify, byakugan.carmodel, byakugan.elastic.load, byakugan.elastic.projection, byakugan.elastic.inverse_projection'; \
    # remove the build-time hardener + every shell; KEEP python3 + python3-yaml +
    # the baked engine tree (python is the tool, like plaso).
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/bash /bin/bash /usr/bin/bashbug /usr/bin/perl* \
           /bin/sh /bin/dash /bin/rbash /usr/bin/sh /usr/bin/dash /usr/bin/rbash

FROM scratch
COPY --from=hardened / /
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG BYAKUGAN_REF
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
# PYTHONPATH: run the engine from its baked source tree. BYAKUGAN_PARSE_BIN: the
# static Go parse binary (the engine reads this env first). HOME=/tmp: the only
# writable path on the read-only rootfs. BYAKUGAN_VERSION/BYAKUGAN_REF: what
# `byakugan --version` and the summary line report.
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp \
    XDG_CACHE_HOME=/tmp/.cache \
    PYTHONPATH=/opt/byakugan \
    PYTHONDONTWRITEBYTECODE=1 \
    BYAKUGAN_PARSE_BIN=/opt/byakugan/go/bin/byakugan-parse \
    BYAKUGAN_VERSION=${TOOL_VERSION} \
    BYAKUGAN_REF=${BYAKUGAN_REF}
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["byakugan"]
LABEL org.opencontainers.image.title="get-sybers/byakugan" \
      org.opencontainers.image.description="The Byakugan MITRE CAR engine (python + a static Go parse binary) cloned and built at the pinned ref, behind the engine's own multi-tool dispatcher: byakugan build|timeline|verify|load self-orchestrates from its env block with one JSON summary line, byakugan car-vocab prints the car_action vocabulary. Hardened: uid0 renamed+locked, no ansible/apt/pip/sudo/setuid/shell, runs as uid 2000; python kept (declared); ca-certificates kept for the load sub-tool's push-mode TLS trust." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/byakugan" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="byakugan" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.engine-ref="${BYAKUGAN_REF}" \
      com.get-sybers.byakugan.ref="${BYAKUGAN_REF}"
