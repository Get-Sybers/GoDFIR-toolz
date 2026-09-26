# syntax=docker/dockerfile:1
# get-sybers/anamnesis — the Anamnesis memory-forensics engine (pure Go on
# MemProcFS) in one hardened image. Runtime interface: contract.yml; symbol
# cache, offline recovery and hardening shape: README.md.
ARG GO_VERSION=1.25
ARG BASE=debian:trixie-slim
ARG ANAMNESIS_REPO=https://github.com/Get-Sybers/Anamnesis
# the ONE place the engine version is set; consumers build without a REF arg and inherit it
ARG ANAMNESIS_REF=5a5175829fe4a7203e8b1e9ab1c7e695fbbdfd9b
# MemProcFS pin: the release TAG is major.minor but the asset name carries the full
# patch + date stamp — bump tag, asset and checksum together
ARG MEMPROCFS_VERSION=5.18.11
ARG MEMPROCFS_TAG=v5.18
ARG MEMPROCFS_ASSET=MemProcFS_files_and_binaries_v5.18.11-linux_x64-20260914.tar.gz
ARG MEMPROCFS_SHA256=55648725838cae111c428131b619845b88fee45be61ea2d962e869ec0e8012aa
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION=2.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

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

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY --from=src /src /src
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -tags memprocfs -o /anamnesis ./cmd/anamnesis
# build-stage sanity gate
RUN /anamnesis --version

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
COPY --from=build /anamnesis /opt/anamnesis/anamnesis
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
COPY --from=src /src/seeds/anamnesis-offsets/ /opt/anamnesis/seed-offsets/
COPY --from=src /src/seeds/anamnesis-signatures/ /opt/anamnesis/seed-signatures/
COPY anamnesis/contract.yml /opt/anamnesis/contract.yml
COPY hardening/harden.yml /tmp/hardening/harden.yml
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
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible ansible-core; \
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
      org.opencontainers.image.description="Anamnesis (pure-Go memory forensics on MemProcFS) for the DX_DFIR memory lane: the engine cloned + built at the pinned ref (-tags memprocfs, CGO_ENABLED=0), the MemProcFS vmm/leechcore/libpdbcrust/libMSCompression libraries bundled at a pinned, sha256-verified release, and the PDB symbol cache supplied by a read-write bind mount at /opt/anamnesis/lib/Symbols that persists across runs (always offline — no symbol variable, no PDB download, no network). Self-orchestrating, env-driven batch over a mounted memory-image tree, one JSON summary line. Hardened: uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as uid 2000; glibc + libusb-1.0 kept for the native libraries." \
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
