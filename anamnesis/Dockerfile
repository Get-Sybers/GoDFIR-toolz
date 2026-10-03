# syntax=docker/dockerfile:1
# get-sybers/anamnesis — the Anamnesis memory-forensics engine (pure Go on
# MemProcFS) in one hardened image. The engine is imported from its published
# module at build time (github.com/Get-Sybers/Anamnesis via the Go module
# proxy, checksum-verified against sum.golang.org) — no git clone, no Go
# source in this repo. Runtime interface: contract.yml; symbol cache, offline
# recovery and hardening shape: README.md.
ARG GO_VERSION=1.27
ARG BASE=debian:trixie-slim
# the ONE place the engine version is set: a release tag of the published
# module; consumers build without a REF arg and inherit it
ARG ANAMNESIS_REF=v1.1.0
# MemProcFS pin: the release TAG is major.minor but the asset name carries the full
# patch + date stamp — bump tag, asset and checksum together
ARG MEMPROCFS_VERSION=5.18.11
ARG MEMPROCFS_TAG=v5.18
ARG MEMPROCFS_ASSET=MemProcFS_files_and_binaries_v5.18.11-linux_x64-20260914.tar.gz
ARG MEMPROCFS_SHA256=55648725838cae111c428131b619845b88fee45be61ea2d962e869ec0e8012aa
ARG DFIR_UID=2000
ARG DFIR_GID=2000
# what the engine binary self-reports (`anamnesis --version`); the module tag
# stays v1.x because go.mod carries no /v2 major suffix
ARG TOOL_VERSION=2.0.0
ARG GODFIR_REVISION=unknown
# the get-sybers image release this Dockerfile ships as: the ONE version the
# workflow tags the pushed container with (synchronised across images, §31)
ARG GODFIR_RELEASE=v0.2.1

# glibc builder, NOT alpine: purego's cgo-free dlopen still links libc, so the
# binary carries a dynamic loader reference — built on musl it cannot exec on
# the trixie (glibc) runtime rootfs
FROM golang:${GO_VERSION}-trixie AS build
ARG ANAMNESIS_REF
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
RUN go install -trimpath -ldflags='-s -w' -tags memprocfs \
      "github.com/Get-Sybers/Anamnesis/cmd/anamnesis@${ANAMNESIS_REF}"
# build-stage sanity gate
RUN /go/bin/anamnesis --version
# the engine's seed data travels with the published module; stage it out of the
# module cache. Globbed, not spelled from ANAMNESIS_REF: a commit-sha override
# lands in the cache under its resolved pseudo-version, not the raw ref, and
# this stage's cache holds exactly one anamnesis version either way.
RUN set -eux; \
    MODDIR="$(ls -d "$(go env GOMODCACHE)"/github.com/\!get-\!sybers/\!anamnesis@* | head -n 1)"; \
    cp -a "$MODDIR/seeds/anamnesis-offsets" /seed-offsets; \
    cp -a "$MODDIR/seeds/anamnesis-signatures" /seed-signatures

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
    # libpdbcrust*: THE Linux PDB parser vmm.so dlopens (no symbols at all without it);
    # libMSCompression: Win10 XPRESS compressed-memory pages — all three must exist in the release
    cp "$(find /extract -name libpdbcrust.so | head -1)" /opt/anamnesis/lib/; \
    cp "$(find /extract -name libpdbcrust2.so | head -1)" /opt/anamnesis/lib/; \
    cp "$(find /extract -name libMSCompression.so | head -1)" /opt/anamnesis/lib/; \
    if find /extract -name info.db | head -1 | grep -q .; then \
      cp "$(find /extract -name info.db | head -1)" /opt/anamnesis/lib/; \
    fi

# glibc + libusb-1.0 stay (the .so need them); the hardener installs into a throwaway dir and leaves no python
FROM ${BASE} AS hardened
ARG DFIR_UID
ARG DFIR_GID
COPY --from=build /go/bin/anamnesis /opt/anamnesis/anamnesis
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
COPY --from=build /seed-offsets/ /opt/anamnesis/seed-offsets/
COPY --from=build /seed-signatures/ /opt/anamnesis/seed-signatures/
COPY anamnesis/contract.yml /opt/anamnesis/contract.yml
COPY hardening/harden.yml hardening/requirements.txt /tmp/hardening/
RUN set -eux; \
    # the symbol-cache mount point (contract mount `symbols`; README: Symbols)
    mkdir -p /opt/anamnesis/lib/Symbols; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-pip libusb-1.0-0; \
    # keep libusb next to the bundled MemProcFS libs so it survives the scratch copy via LD_LIBRARY_PATH
    cp -a /usr/lib/*/libusb-1.0.so.0* /opt/anamnesis/lib/ 2>/dev/null || true; \
    if ! find /opt/anamnesis/lib -maxdepth 1 -name 'libusb-1.0.so.0*' -print -quit | grep -q .; then \
      libusb_path="$(find / -name 'libusb-1.0.so.0*' 2>/dev/null | head -1)"; \
      test -n "$libusb_path"; \
      cp -a "$libusb_path" /opt/anamnesis/lib/; \
    fi; \
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible \
      -r /tmp/hardening/requirements.txt; \
    chmod -R a+rX /opt/anamnesis; \
    PYTHONPATH=/tmp/ansible /tmp/ansible/bin/ansible-playbook -c local -i localhost, \
      /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=anamnesis -e harden_static_binary=false -e harden_shell=false -e harden_python=false; \
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
# vmm.so is dlopen'd from ANAMNESIS_VMM_LIB; LD_LIBRARY_PATH resolves leechcore.so beside it
ENV PATH=/opt/anamnesis:/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp \
    XDG_CACHE_HOME=/tmp/.cache \
    LD_LIBRARY_PATH=/opt/anamnesis/lib \
    ANAMNESIS_VMM_LIB=/opt/anamnesis/lib/vmm.so
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/opt/anamnesis/anamnesis"]
LABEL org.opencontainers.image.title="get-sybers/anamnesis" \
      org.opencontainers.image.description="Anamnesis (pure-Go memory forensics on MemProcFS) for the DX_DFIR memory lane: the engine imported from its published Go module at the pinned release tag (-tags memprocfs, CGO_ENABLED=0, proxy + checksum-db verified), the MemProcFS vmm/leechcore/libpdbcrust/libMSCompression libraries bundled at a pinned, sha256-verified release, and the PDB symbol cache supplied by a read-write bind mount at /opt/anamnesis/lib/Symbols that persists across runs (always offline — no symbol variable, no PDB download, no network). Self-orchestrating, env-driven batch over a mounted memory-image tree, one JSON summary line. Hardened: uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as uid 2000; glibc + libusb-1.0 kept for the native libraries." \
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
