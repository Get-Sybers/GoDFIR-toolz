# syntax=docker/dockerfile:1
# get-sybers/anamnesis — the Anamnesis memory-forensics engine (pure Go on
# MemProcFS) in ONE hardened image: it parses a memory image natively and
# renders flat per-plugin JSONL + a CAR store (car.db) for the DX_DFIR CAR lane.
#
# The engine SOURCE is cloned at build time at the pinned commit (ANAMNESIS_REF,
# the ref DX_DFIR's sources.yml pins); the Go binary is built with
# `-tags memprocfs` (still CGO_ENABLED=0 — purego). The MemProcFS shared
# libraries (vmm/leechcore, libpdbcrust for the Linux PDB parser,
# libMSCompression for Win10 compressed memory) are fetched from a pinned
# upstream release, sha256-verified, and bundled at /opt/anamnesis/lib; the
# binary dlopen's them at runtime.
#
# PDB SYMBOLS ARE BAKED AT BUILD TIME: the engine is always offline (no
# symbol variable, no /symbols mount, no container network). The seed stage
# fetches the pinned representative memory image(s) (ANAMNESIS_SEED_SOURCES)
# and runs seed/seedsymbols.go over them with the symbol server enabled, so
# MemProcFS populates Symbols/ beside vmm.so with the exact ntoskrnl/ntdll/…
# PDBs those Windows builds need; the populated cache ships read-only in the
# runtime image. Coverage therefore reaches exactly the Windows builds the
# seed images represent — add a seed entry matching your corpus's build to
# widen it (see ANAMNESIS_SEED_SOURCES below).
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
#   docker build -t get-sybers/anamnesis:latest -f anamnesis/Dockerfile .
#   (--build-arg ANAMNESIS_REF=<40-hex sha> overrides the baked pin)
#
# The MemProcFS release (tag, asset filename and sha256) is pinned by the
# MEMPROCFS_* ARG defaults below; override them only to move to a new release.
# The upstream tag (v5.18) and the asset filename (v5.18.11 + a date stamp)
# differ, so the tag and the full asset name are pinned separately.
ARG GO_VERSION=1.25
ARG BASE=debian:trixie-slim
ARG ANAMNESIS_REPO=https://github.com/Get-Sybers/Anamnesis
# The anamnesis engine pin — the ONE place the engine version is set; consumers
# (DX_DFIR included) build without a REF build-arg and inherit it.
ARG ANAMNESIS_REF=0b0bfda424bba1c02acaf2845cf8c20fef561f35
# The memory images the symbol bake seeds from: whitespace-separated
# `url|sha256` entries (URLs percent-encoded, so entries never carry spaces).
# An archive URL (.zip/.tar/.tgz/.tar.gz/.gz/.7z) is extracted and every
# contained file >64M is seeded; any other URL is used as a raw image. The
# default is the Magnet Forensics 2020 CTF Windows memory image
# (memdump-001.mem, Windows 10 x64 build 18362) from the digitalcorpora.org
# corpus — the image the DX_DFIR memory lane was validated against. Append an
# entry with a dump matching your corpus's Windows build (PDBs are matched by
# exact binary revision) to widen coverage. The seed stage transiently needs
# ~6.5 GB of build disk with the default seed.
ARG ANAMNESIS_SEED_SOURCES="https://digitalcorpora.s3.amazonaws.com/corpora/scenarios/magnet/2020%20CTF%20-%20Windows%20Memory.zip|8ba868f49bd33970a1cc6d7144a63f8336d83bf57bfddce3ba34a771a7d75955"
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
ARG TOOL_VERSION=2.2.0
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
    # libpdbcrust is THE Linux PDB parser+downloader vmm.so dlopens from its
    # own directory (pdb.c: libpdbcrust.so, with libpdbcrust2.so as the
    # loader fallback) — without it there is no symbol capability at all and
    # command_line/sid/user stay empty. libMSCompression decompresses Win10
    # XPRESS compressed-memory pages (optional for vmm, load-bearing for
    # reading modern images). All three are in the pinned release; fail the
    # build if not found.
    cp "$(find /extract -name libpdbcrust.so | head -1)" /opt/anamnesis/lib/; \
    cp "$(find /extract -name libpdbcrust2.so | head -1)" /opt/anamnesis/lib/; \
    cp "$(find /extract -name libMSCompression.so | head -1)" /opt/anamnesis/lib/; \
    if find /extract -name info.db | head -1 | grep -q .; then \
      cp "$(find /extract -name info.db | head -1)" /opt/anamnesis/lib/; \
    fi

# --- stage 4: bake the PDB symbol cache ---------------------------------------
# 4a: the seeder builds INSIDE the cloned engine source tree so it uses the
# engine's own pinned gomemprocfs (a separate stage keeps the engine-binary
# layer cache untouched when only the seeder changes).
FROM build AS seedbuild
COPY anamnesis/seed/seedsymbols.go /src/cmd/seed-symbols/main.go
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /seed-symbols ./cmd/seed-symbols

# 4b: the ONE networked run — fetch the pinned seed image(s), open each with
# the symbol server enabled, and let MemProcFS populate Symbols/ beside
# vmm.so (it only uses that directory while it is WRITABLE, hence the bake
# happens here and never at runtime). The seeder fails the build unless the
# kernel PDB resolves and command_line/sid actually populate; the find-gate
# below fails it if the cache still ships empty.
FROM ${BASE} AS seed
ARG ANAMNESIS_SEED_SOURCES
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl ca-certificates libarchive-tools libusb-1.0-0; \
    rm -rf /var/lib/apt/lists/*
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
COPY --from=seedbuild /seed-symbols /usr/local/bin/seed-symbols
ENV LD_LIBRARY_PATH=/opt/anamnesis/lib
RUN set -eux; \
    mkdir -p /opt/anamnesis/lib/Symbols /seed/imgs; \
    cd /seed; i=0; \
    for entry in ${ANAMNESIS_SEED_SOURCES}; do \
      i=$((i+1)); url="${entry%%|*}"; sha="${entry##*|}"; \
      if [ -z "$url" ] || [ -z "$sha" ] || [ "$url" = "$entry" ]; then \
        echo "bad ANAMNESIS_SEED_SOURCES entry (want url|sha256): $entry" >&2; exit 1; \
      fi; \
      curl -fL --retry 3 -o "dl$i" "$url"; \
      echo "$sha  dl$i" | sha256sum -c -; \
      case "$url" in \
        *.zip|*.tar|*.tgz|*.tar.gz|*.gz|*.7z) bsdtar -xf "dl$i" -C imgs; rm "dl$i" ;; \
        *) mv "dl$i" "imgs/seed$i.raw" ;; \
      esac; \
    done; \
    find imgs -type f -size +64M -exec seed-symbols -lib /opt/anamnesis/lib/vmm.so '{}' +; \
    find /opt/anamnesis/lib/Symbols -name '*.pdb' | head -1 | grep -q . \
      || { echo "symbol bake produced no PDBs — refusing to ship an empty cache" >&2; exit 1; }; \
    rm -rf /seed

# --- stage 5: hardened runtime -----------------------------------------------
# glibc + libusb-1.0 kept (the .so need them); the build-time hardener (ansible-core) is
# installed into a THROWAWAY dir and removed with python afterwards, so the
# runtime holds no python, no pip, no apt, no shell — only the anamnesis binary,
# the MemProcFS libraries and the required native shared libraries.
FROM ${BASE} AS hardened
ARG DFIR_UID
ARG DFIR_GID
COPY --from=build /anamnesis /opt/anamnesis/anamnesis
COPY --from=memprocfs /opt/anamnesis/lib /opt/anamnesis/lib
# The baked PDB cache, read-only beside vmm.so. At runtime the engine stages
# it into /tmp before opening vmm (MemProcFS only reads a Symbols/ dir it can
# write, else it silently uses /tmp — see the engine's stageSymbols).
COPY --from=seed /opt/anamnesis/lib/Symbols /opt/anamnesis/lib/Symbols
COPY anamnesis/contract.yml /opt/anamnesis/contract.yml
COPY hardening/harden.yml /tmp/hardening/harden.yml
# harden.yml writes the /etc/dfir-hardened self-declaration (schema=1) from the
# values below; the shell and python it declares absent are removed right after.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-pip libusb-1.0-0; \
    # keep libusb next to bundled MemProcFS libs so it survives scratch copy and resolves via LD_LIBRARY_PATH
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
      org.opencontainers.image.description="Anamnesis (pure-Go memory forensics on MemProcFS) for the DX_DFIR memory lane: the engine cloned + built at the pinned ref (-tags memprocfs, CGO_ENABLED=0), the MemProcFS vmm/leechcore/libpdbcrust/libMSCompression libraries bundled at a pinned, sha256-verified release, and the PDB symbol cache baked at build time (always offline at runtime — no symbol variable, no network). Self-orchestrating, env-driven batch over a mounted memory-image tree, one JSON summary line. Hardened: uid0 renamed+locked, no python/apt/pip/sudo/setuid/shell, runs as uid 2000; glibc + libusb-1.0 kept for the native libraries." \
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
