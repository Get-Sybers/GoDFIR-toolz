# syntax=docker/dockerfile:1
# get-sybers/anamnesis — the Anamnesis memory-forensics engine (pure Go on
# MemProcFS) in ONE hardened image: it parses a memory image natively and
# renders flat per-plugin JSONL + a CAR store (car.db) for the DX_DFIR CAR lane.
#
# The engine SOURCE is cloned at build time at the pinned commit (ANAMNESIS_REF,
# the ref DX_DFIR's sources.yml pins); the Go binary is built with
# `-tags memprocfs` (still CGO_ENABLED=0 — purego). The MemProcFS vmm/leechcore
# shared libraries are fetched from a pinned upstream release, sha256-verified,
# and bundled at /opt/anamnesis/lib; the binary dlopen's them at runtime.
# Shape B: glibc + libusb-1.0 are kept because the MemProcFS .so need them
# (the justified native deviations); no shell, no python, no package manager
# ship — the build-time hardener
# (ansible-core) runs in a throwaway install and is removed with python.
#
# Self-orchestrating, env-driven, run-once-exit (docs/framework 03/04): the
# ENTRYPOINT is the anamnesis binary; with NO arguments it discovers every memory
# image under ANAMNESIS_INPUT_DIR, processes each into ANAMNESIS_OUT_DIR
# (idempotent per collector) and prints ONE JSON summary line on stdout. The
# variables, mounts, exit codes and output layout are declared in contract.yml.
# Any argument switches to the single-image pass-through (`-f /input/<image> -o /out`).
#
#   docker build -t get-sybers/anamnesis:latest \
#     --build-arg ANAMNESIS_REF=<40-hex sha> \
#     -f anamnesis/Dockerfile .
#
# The MemProcFS release (tag, asset filename and sha256) is pinned by the
# MEMPROCFS_* ARG defaults below; override them only to move to a new release.
# The upstream tag (v5.18) and the asset filename (v5.18.11 + a date stamp)
# differ, so the tag and the full asset name are pinned separately.
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
# The per-image version label: the engine release the pinned ref corresponds to.
ARG TOOL_VERSION=2.0.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

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
# Build-stage sanity gate: the binary runs and prints its version.
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
# glibc + libusb-1.0 kept (the .so need them); the build-time hardener (ansible-core) is
# installed into a THROWAWAY dir and removed with python afterwards, so the
# runtime holds no python, no pip, no apt, no shell — only the anamnesis binary,
# the MemProcFS libraries and the required native shared libraries.
FROM ${BASE} AS hardened
ARG DFIR_UID
ARG DFIR_GID
COPY --from=build /anamnesis /opt/anamnesis/anamnesis
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
COPY anamnesis/contract.yml /opt/anamnesis/contract.yml
COPY hardening/harden.yml /tmp/hardening/harden.yml
# harden.yml writes the /etc/dfir-hardened self-declaration (schema=1) from the
# values below; the shell and python it declares absent are removed right after.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-pip libusb-1.0-0; \
    # keep libusb next to bundled MemProcFS libs so it survives scratch copy and resolves via LD_LIBRARY_PATH
    cp -a /usr/lib/*/libusb-1.0.so.0* /opt/anamnesis/lib/ 2>/dev/null || \
      cp -a "$(find / -name 'libusb-1.0.so.0*' 2>/dev/null | head -1)" /opt/anamnesis/lib/; \
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible ansible-core; \
    chmod -R a+rX /opt/anamnesis; \
    PYTHONPATH=/tmp/ansible /tmp/ansible/bin/ansible-playbook -c local -i localhost, \
      /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=anamnesis -e harden_static_binary=false -e harden_shell=false -e harden_python=false; \
    # remove the build-time hardener, python entirely (anamnesis needs none), and
    # every shell — the ENTRYPOINT is the anamnesis binary.
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
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
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
      org.opencontainers.image.description="Anamnesis (pure-Go memory forensics on MemProcFS) for the DX_DFIR memory lane: the engine cloned + built at the pinned ref (-tags memprocfs, CGO_ENABLED=0), the MemProcFS vmm/leechcore libraries bundled at a pinned, sha256-verified release. Self-orchestrating, env-driven batch over a mounted memory-image tree, one JSON summary line. Hardened: uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as uid 2000; glibc + libusb-1.0 kept for the native libraries." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/Anamnesis" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="anamnesis" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.engine-ref="${ANAMNESIS_REF}" \
      com.get-sybers.anamnesis.ref="${ANAMNESIS_REF}" \
      com.get-sybers.memprocfs.version="${MEMPROCFS_VERSION}"
