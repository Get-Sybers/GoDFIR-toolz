# syntax=docker/dockerfile:1
#
# get-sybers/flashback — the flashback memory-forensics tool in ONE hardened
# image. flashback is a pure-Go tool built on MemProcFS (no Volatility, no
# Python): it parses a memory image natively and renders flat per-plugin JSONL +
# a CAR store (car.db) for the DX_DFIR CAR lane. It replaces get-sybers/piiat-mem.
#
# The flashback SOURCE is cloned at build time at the pinned commit (no vendored
# checkout in DX_DFIR); the Go binary is built with `-tags memprocfs` (still
# CGO_ENABLED=0 — purego). The MemProcFS vmm/leechcore shared libraries are
# fetched from a pinned upstream release and bundled at /opt/flashback/lib; the
# binary dlopen's them at runtime, so no C toolchain and no Python ship. glibc is
# kept (the .so need it), like plaso/byakugan keep their interpreter.
#
#   docker build -t get-sybers/flashback:latest \
#     --build-arg FLASHBACK_REF=<40-hex sha> \
#     --build-arg MEMPROCFS_VERSION=5.18 \
#     --build-arg MEMPROCFS_SHA256=<sha256 of the linux tarball> \
#     -f flashback/Dockerfile .
#
# SELF-ORCHESTRATING, ENV-DRIVEN. The ENTRYPOINT is the flashback binary; with NO
# arguments it runs its built-in batch orchestrator: it discovers every memory
# image under the mounted memory dir and processes each, per-plugin idempotent,
# printing ONE JSON summary line on stdout for the caller to gate on. No wrapper
# on the host: the DX_DFIR ansible volatility lane just runs
#
#   docker run --rm --network none --read-only --tmpfs /tmp \
#     -e FLASHBACK_PLUGINS= -e FLASHBACK_FORCE=0 -e FLASHBACK_SYMBOLS_ONLINE=0 \
#     -v "$mem_dir:/mem:ro" -v "$out:/out" -v "$symbols:/symbols" \
#     get-sybers/flashback:latest
#
# Env-var contract (all optional; defaults match the mount points):
#   FLASHBACK_MEMORY_DIR      memory image tree, recursed          default /mem
#   FLASHBACK_OUT_DIR         output root, one folder per image    default /out
#   FLASHBACK_SYMBOLS_DIR     PDB/symbol cache (read-write)        default /symbols
#   FLASHBACK_PLUGINS         comma-separated collectors; empty = the default CAR set
#   FLASHBACK_FORCE           1/true/yes/on = rerun collectors with valid output
#   FLASHBACK_SYMBOLS_ONLINE  1/true/yes/on = this container has network for PDB fetch
# Outputs: <out>/<clean name>/plugins/<plugin>.jsonl + car.db + flashback.log;
# stdout = one JSON summary line; stderr = progress. Exit 0 normal, 1 nothing
# produced/nothing done, 2 config error.
#
# Any CLI argument switches to single-image pass-through: `... get-sybers/flashback
# -f /mem/<image> -o /out`.
ARG GO_VERSION=1.25
ARG BASE=debian:trixie-slim
ARG FLASHBACK_REPO=https://github.com/Get-Sybers/flashback
ARG FLASHBACK_REF=main
ARG MEMPROCFS_VERSION=5.18
ARG MEMPROCFS_SHA256=
ARG DFIR_UID=2000
ARG DFIR_GID=2000

# --- stage 1: fetch the pinned flashback source ------------------------------
FROM debian:trixie-slim AS src
ARG FLASHBACK_REPO
ARG FLASHBACK_REF
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends git ca-certificates; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    git clone "${FLASHBACK_REPO}" /src; \
    git -C /src checkout "${FLASHBACK_REF}"; \
    find /src -name .git -prune -exec rm -rf {} +

# --- stage 2: build the static Go binary (native MemProcFS engine) ------------
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY --from=src /src /src
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -tags memprocfs -o /flashback ./cmd/flashback
# Sanity-gate: the binary runs and prints its version.
RUN /flashback --version

# --- stage 3: fetch + verify the pinned MemProcFS Linux libraries -------------
FROM debian:trixie-slim AS memprocfs
ARG MEMPROCFS_VERSION
ARG MEMPROCFS_SHA256
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl ca-certificates; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    url="https://github.com/ufrisk/MemProcFS/releases/download/v${MEMPROCFS_VERSION}/MemProcFS_files_and_binaries_v${MEMPROCFS_VERSION}-linux_x64.tar.gz"; \
    curl -fsSL "$url" -o /tmp/mpfs.tgz; \
    if [ -n "${MEMPROCFS_SHA256}" ]; then \
      echo "${MEMPROCFS_SHA256}  /tmp/mpfs.tgz" | sha256sum -c -; \
    else \
      echo "WARNING: MEMPROCFS_SHA256 unset — the download is NOT verified (pin it in DX_DFIR sources.yml)" >&2; \
    fi; \
    mkdir -p /extract /opt/flashback/lib; \
    tar -xzf /tmp/mpfs.tgz -C /extract; \
    cp "$(find /extract -name vmm.so | head -1)" /opt/flashback/lib/; \
    cp "$(find /extract -name leechcore.so | head -1)" /opt/flashback/lib/; \
    if find /extract -name info.db | head -1 | grep -q .; then \
      cp "$(find /extract -name info.db | head -1)" /opt/flashback/lib/; \
    fi

# --- stage 4: hardened runtime -----------------------------------------------
# glibc kept (the .so need it); the build-time hardener (ansible-core) is
# installed into a THROWAWAY dir and removed with python afterwards, so the
# runtime holds no python, no pip, no apt, no shell — only the flashback binary,
# the MemProcFS libraries and libc.
FROM ${BASE} AS hardened
ARG DFIR_UID
ARG DFIR_GID
COPY --from=build /flashback /opt/flashback/flashback
COPY --from=memprocfs /opt/flashback/lib /opt/flashback/lib
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-pip; \
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible ansible-core; \
    chmod -R a+rX /opt/flashback; \
    PYTHONPATH=/tmp/ansible /tmp/ansible/bin/ansible-playbook -c local -i localhost, \
      /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID}; \
    # remove the build-time hardener, python entirely (flashback needs none), and
    # every shell — the ENTRYPOINT is the static binary.
    rm -rf /tmp/ansible /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* /usr/local/lib/python3* \
           /usr/lib/python3.* /etc/python3* \
           /bin/sh /bin/dash /bin/bash /bin/rbash \
           /usr/bin/sh /usr/bin/dash /usr/bin/bash /usr/bin/rbash /usr/bin/perl*

FROM scratch
COPY --from=hardened / /
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG FLASHBACK_REF
# HOME/cache -> /tmp (the only writable path on a read-only rootfs). The vmm
# library is dlopen'd from here; LD_LIBRARY_PATH lets it find leechcore.so.
ENV PATH=/opt/flashback:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp \
    XDG_CACHE_HOME=/tmp/.cache \
    LD_LIBRARY_PATH=/opt/flashback/lib \
    FLASHBACK_VMM_LIB=/opt/flashback/lib/vmm.so
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/opt/flashback/flashback"]
LABEL org.opencontainers.image.title="get-sybers/flashback" \
      org.opencontainers.image.description="Minimal hardened flashback (pure-Go memory forensics on MemProcFS) for the DX_DFIR volatility lane: source cloned + built in at the sources.yml pin (-tags memprocfs, CGO_ENABLED=0), MemProcFS vmm/leechcore libraries bundled at a pinned upstream release, uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as a fixed non-root uid (default 2000). glibc kept for the native libraries. Self-orchestrating: env-driven (FLASHBACK_MEMORY_DIR/FLASHBACK_OUT_DIR/FLASHBACK_SYMBOLS_DIR/FLASHBACK_PLUGINS/FLASHBACK_FORCE/FLASHBACK_SYMBOLS_ONLINE) batch over a mounted memory dir." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/flashback" \
      com.get-sybers.hardened="true" \
      com.get-sybers.flashback.ref="${FLASHBACK_REF}" \
      com.get-sybers.memprocfs.version="${MEMPROCFS_VERSION}"
