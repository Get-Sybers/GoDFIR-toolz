# syntax=docker/dockerfile:1
#
# get-sybers/anamnesis — the anamnesis memory-forensics tool in ONE hardened
# image. anamnesis is a pure-Go tool built on MemProcFS (no Volatility, no
# Python): it parses a memory image natively and renders flat per-plugin JSONL +
# a CAR store (car.db) for the DX_DFIR CAR lane. It supersedes the previous
# Volatility-based memory image.
#
# The anamnesis SOURCE is cloned at build time at the pinned commit (no vendored
# checkout in DX_DFIR); the Go binary is built with `-tags memprocfs` (still
# CGO_ENABLED=0 — purego). The MemProcFS vmm/leechcore shared libraries are
# fetched from a pinned upstream release and bundled at /opt/anamnesis/lib; the
# binary dlopen's them at runtime, so no C toolchain and no Python ship. glibc is
# kept (the .so need it), like plaso/byakugan keep their interpreter.
#
#   docker build -t get-sybers/anamnesis:latest \
#     --build-arg ANAMNESIS_REF=<40-hex sha> \
#     -f anamnesis/Dockerfile .
#
# The MemProcFS release (tag, asset filename and sha256) is pinned by the
# MEMPROCFS_* ARG defaults below; override them only to move to a new release.
# The upstream tag (v5.18) and the asset filename (v5.18.11 + a date stamp)
# differ, so the tag and the full asset name are pinned separately.
#
# SELF-ORCHESTRATING, ENV-DRIVEN. The ENTRYPOINT is the anamnesis binary; with NO
# arguments it runs its built-in batch orchestrator: it discovers every memory
# image under the mounted memory dir and processes each, per-plugin idempotent,
# printing ONE JSON summary line on stdout for the caller to gate on. No wrapper
# on the host: the DX_DFIR ansible memory lane just runs
#
#   docker run --rm --network none --read-only --tmpfs /tmp \
#     -e ANAMNESIS_PLUGINS= -e ANAMNESIS_FORCE=0 -e ANAMNESIS_SYMBOLS_ONLINE=0 \
#     -v "$mem_dir:/mem:ro" -v "$out:/out" -v "$symbols:/symbols" \
#     get-sybers/anamnesis:latest
#
# Env-var contract (all optional; defaults match the mount points):
#   ANAMNESIS_MEMORY_DIR      memory image tree, recursed          default /mem
#   ANAMNESIS_OUT_DIR         output root, one folder per image    default /out
#   ANAMNESIS_SYMBOLS_DIR     PDB/symbol cache (read-write)        default /symbols
#   ANAMNESIS_PLUGINS         comma-separated collectors; empty = the default CAR set
#   ANAMNESIS_FORCE           1/true/yes/on = rerun collectors with valid output
#   ANAMNESIS_SYMBOLS_ONLINE  1/true/yes/on = this container has network for PDB fetch
# Outputs: <out>/<clean name>/plugins/<plugin>.jsonl + car.db + anamnesis.log;
# stdout = one JSON summary line; stderr = progress. Exit 0 normal, 1 nothing
# produced/nothing done, 2 config error.
#
# Any CLI argument switches to single-image pass-through: `... get-sybers/anamnesis
# -f /mem/<image> -o /out`.
ARG GO_VERSION=1.25
ARG BASE=debian:trixie-slim
ARG ANAMNESIS_REPO=https://github.com/Get-Sybers/Anamnesis
ARG ANAMNESIS_REF=main
# MemProcFS upstream release pin. The release TAG is major.minor (v5.18) but the
# published asset filename carries the full patch version and a date stamp, so
# the tag and the asset name are pinned as separate values. Bump all three (and
# the checksum) together to move to a new MemProcFS release.
ARG MEMPROCFS_VERSION=5.18.11
ARG MEMPROCFS_TAG=v5.18
ARG MEMPROCFS_ASSET=MemProcFS_files_and_binaries_v5.18.11-linux_x64-20260914.tar.gz
ARG MEMPROCFS_SHA256=55648725838cae111c428131b619845b88fee45be61ea2d962e869ec0e8012aa
ARG DFIR_UID=2000
ARG DFIR_GID=2000

# --- stage 1: fetch the pinned anamnesis source ------------------------------
FROM debian:trixie-slim AS src
ARG ANAMNESIS_REPO
ARG ANAMNESIS_REF
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends git ca-certificates; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    git clone "${ANAMNESIS_REPO}" /src; \
    git -C /src checkout "${ANAMNESIS_REF}"; \
    find /src -name .git -prune -exec rm -rf {} +

# --- stage 2: build the static Go binary (native MemProcFS engine) ------------
FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY --from=src /src /src
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -tags memprocfs -o /anamnesis ./cmd/anamnesis
# Sanity-gate: the binary runs and prints its version.
RUN /anamnesis --version

# --- stage 3: fetch + verify the pinned MemProcFS Linux libraries -------------
FROM debian:trixie-slim AS memprocfs
ARG MEMPROCFS_TAG
ARG MEMPROCFS_ASSET
ARG MEMPROCFS_SHA256
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl ca-certificates; \
    rm -rf /var/lib/apt/lists/*
RUN set -eux; \
    test -n "${MEMPROCFS_SHA256}" || { echo "MEMPROCFS_SHA256 must be pinned in this Dockerfile" >&2; exit 1; }; \
    url="https://github.com/ufrisk/MemProcFS/releases/download/${MEMPROCFS_TAG}/${MEMPROCFS_ASSET}"; \
    curl -fsSL "$url" -o /tmp/mpfs.tgz; \
    echo "${MEMPROCFS_SHA256}  /tmp/mpfs.tgz" | sha256sum -c -; \
    mkdir -p /extract /opt/anamnesis/lib; \
    tar -xzf /tmp/mpfs.tgz -C /extract; \
    cp "$(find /extract -name vmm.so | head -1)" /opt/anamnesis/lib/; \
    cp "$(find /extract -name leechcore.so | head -1)" /opt/anamnesis/lib/; \
    if find /extract -name info.db | head -1 | grep -q .; then \
      cp "$(find /extract -name info.db | head -1)" /opt/anamnesis/lib/; \
    fi

# --- stage 4: hardened runtime -----------------------------------------------
# glibc kept (the .so need it); the build-time hardener (ansible-core) is
# installed into a THROWAWAY dir and removed with python afterwards, so the
# runtime holds no python, no pip, no apt, no shell — only the anamnesis binary,
# the MemProcFS libraries and libc.
FROM ${BASE} AS hardened
ARG DFIR_UID
ARG DFIR_GID
COPY --from=build /anamnesis /opt/anamnesis/anamnesis
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-pip; \
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible ansible-core; \
    chmod -R a+rX /opt/anamnesis; \
    PYTHONPATH=/tmp/ansible /tmp/ansible/bin/ansible-playbook -c local -i localhost, \
      /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID}; \
    # remove the build-time hardener, python entirely (anamnesis needs none), and
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
ARG ANAMNESIS_REF
ARG MEMPROCFS_VERSION
# HOME/cache -> /tmp (the only writable path on a read-only rootfs). The vmm
# library is dlopen'd from here; LD_LIBRARY_PATH lets it find leechcore.so.
ENV PATH=/opt/anamnesis:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp \
    XDG_CACHE_HOME=/tmp/.cache \
    LD_LIBRARY_PATH=/opt/anamnesis/lib \
    ANAMNESIS_VMM_LIB=/opt/anamnesis/lib/vmm.so
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/opt/anamnesis/anamnesis"]
LABEL org.opencontainers.image.title="get-sybers/anamnesis" \
      org.opencontainers.image.description="Minimal hardened anamnesis (pure-Go memory forensics on MemProcFS) for the DX_DFIR memory lane: source cloned + built in at the sources.yml pin (-tags memprocfs, CGO_ENABLED=0), MemProcFS vmm/leechcore libraries bundled at a pinned upstream release, uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as a fixed non-root uid (default 2000). glibc kept for the native libraries. Self-orchestrating: env-driven (ANAMNESIS_MEMORY_DIR/ANAMNESIS_OUT_DIR/ANAMNESIS_SYMBOLS_DIR/ANAMNESIS_PLUGINS/ANAMNESIS_FORCE/ANAMNESIS_SYMBOLS_ONLINE) batch over a mounted memory dir." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/Anamnesis" \
      com.get-sybers.hardened="true" \
      com.get-sybers.anamnesis.ref="${ANAMNESIS_REF}" \
      com.get-sybers.memprocfs.version="${MEMPROCFS_VERSION}"
