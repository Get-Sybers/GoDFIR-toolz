# syntax=docker/dockerfile:1
# get-sybers/signatures — ONE hardened image for the whole detection lane:
# YARA + Suricata + Hayabusa, plus the gomount→goyara userspace NTFS scan pipe,
# fronted by signatures-entry, a static Go dispatcher (docs/framework 04 §4.3).
# Multi-tool image: `signatures-entry <yara|suricata|hayabusa|scan>` with no
# further arguments self-orchestrates that sub-tool from its
# SIGNATURES_<SUBTOOL>_* env block (one output folder per item, one JSON
# summary line); any other argv is passed through to the image's own tools
# verbatim (yara, suricata, hayabusa, gomount, goyara, scan-list.sh, sh).
# Shape B, declared deviations: shell=true (sh/dash stay for the legacy
# per-file YARA loop /opt/dxdfir/scan-list.sh and the pass-through; bash is
# removed) and static_binary=false (glibc for yara, suricata, hayabusa and
# libyara). No python: the ruleset provisioners run at build time only.
#
#   docker build -t get-sybers/signatures:latest -f signatures/Dockerfile .
ARG BASE=debian:trixie-slim
ARG HAYABUSA_VERSION=4.1.0
ARG GO_IMAGE=golang:trixie
ARG TOOL_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

# gomount + goyara — the userspace NTFS→YARA pipe baked into this image. gomount
# walks an NTFS disk image in-process and streams a tar of every file; goyara
# reads that tar on stdin and scans each file with libyara. Both build from their
# own top-level modules (the build context is the repo root).

# gomount is CGO-free (it only EXECs ntfs-3g for the mount backend; the stream
# verb links nothing external), so it is a static binary.
FROM ${GO_IMAGE} AS gomount-build
WORKDIR /src/gomount
COPY gomount/go.mod gomount/go.sum ./
RUN go mod download
COPY gomount/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/gomount .

# goyara links libyara through cgo (github.com/hillu/go-yara/v4): libyara-dev
# provides the headers and yara.pc, pkg-config resolves -lyara at build time. The
# resulting binary needs libyara.so (from the runtime `yara` package installed in
# the hardened stage) plus that package's own libmagic/libjansson/libcrypto/
# libz/libzstd/liblzma/libbz2.
FROM ${GO_IMAGE} AS goyara-build
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends libyara-dev pkg-config; \
    rm -rf /var/lib/apt/lists/*
WORKDIR /src/goyara
COPY goyara/go.mod goyara/go.sum ./
RUN go mod download
COPY goyara/ ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/goyara .

# signatures-entry — the static dispatcher entrypoint (this directory's module).
FROM ${GO_IMAGE} AS entry-build
ARG TOOL_VERSION
WORKDIR /src/signatures
COPY signatures/go.mod signatures/go.sum ./
RUN go mod download
COPY signatures/*.go signatures/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /out/signatures-entry .
# Build-stage sanity gate: the entrypoint runs and prints its version.
RUN /out/signatures-entry --version

FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG HAYABUSA_VERSION
ARG TARGETARCH
# Pinned Hayabusa (Yamato-Security, GPL-3.0) — downloaded + sha256-verified at
# BUILD time, like the apt tools; nothing is committed to the repo. One archive
# per arch (the checksums are the release's own published assets).
ARG HB_SHA_amd64=ed2c4595f95b8e765d37f392212587304ce89ed7cfb6682b425a96d126ed67eb
ARG HB_SHA_arm64=9a112ed5ead6850c0f7f1b2a18b2c9c71c6eddc469cc2e1a31f5ed1170afa628
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        yara suricata ansible-core ca-certificates curl unzip \
        ewf-tools ntfs-3g fuse3; \
    case "${TARGETARCH:-amd64}" in \
      amd64) hbarch=x64;     hbsha="${HB_SHA_amd64}" ;; \
      arm64) hbarch=aarch64; hbsha="${HB_SHA_arm64}" ;; \
      *) echo "unsupported TARGETARCH '${TARGETARCH}'" >&2; exit 1 ;; \
    esac; \
    zip="hayabusa-${HAYABUSA_VERSION}-lin-${hbarch}-gnu.zip"; \
    curl -fsSL "https://github.com/Yamato-Security/hayabusa/releases/download/v${HAYABUSA_VERSION}/${zip}" -o /tmp/hb.zip; \
    printf '%s  /tmp/hb.zip\n' "${hbsha}" | sha256sum -c -; \
    mkdir -p /opt/dxdfir/hayabusa; \
    unzip -q /tmp/hb.zip -d /opt/dxdfir/hayabusa; \
    mv "/opt/dxdfir/hayabusa/hayabusa-${HAYABUSA_VERSION}-lin-${hbarch}-gnu" /opt/dxdfir/hayabusa/hayabusa; \
    chmod 0755 /opt/dxdfir/hayabusa/hayabusa; \
    rm -rf /opt/dxdfir/hayabusa/rules/.git /tmp/hb.zip
COPY signatures/scan-list.sh /opt/dxdfir/scan-list.sh
COPY signatures/contract.yml /opt/dxdfir/contract.yml
# Bake the pinned DetectRaptor YARA ruleset into the image: the vendored provisioner
# downloads the pinned assets, verifies each sha256, and dedup-merges them into
# /opt/dxdfir/yara-rules/detectraptor.yar at BUILD time (exactly what the pipeline's
# --fetch does at runtime). python3 (an ansible-core dependency) is present here and
# is removed with the hardener below, leaving only the merged rules on the image.
COPY signatures/detectraptor.py /tmp/detectraptor.py
RUN set -eux; \
    python3 /tmp/detectraptor.py --rules-dir /opt/dxdfir/yara-rules; \
    test -s /opt/dxdfir/yara-rules/detectraptor/detectraptor.yar; \
    rm -f /tmp/detectraptor.py
# Bake the pinned ET Open Suricata ruleset into the image the same way: the vendored
# provisioner downloads the engine-version tarball, structurally validates it, and
# concatenates its *.rules into /opt/dxdfir/suricata-rules/suricata.rules at BUILD
# time (exactly what the pipeline's --fetch does at runtime).
COPY signatures/suricata_rules.py /tmp/suricata_rules.py
RUN set -eux; \
    python3 /tmp/suricata_rules.py --rules-dir /opt/dxdfir/suricata-rules; \
    test -s /opt/dxdfir/suricata-rules/suricata.rules; \
    rm -f /tmp/suricata_rules.py
COPY hardening/harden.yml /tmp/hardening/harden.yml
# harden.yml writes the /etc/dfir-hardened self-declaration (schema=1) from the
# values below: the shell stays (declared), python, bash and ansible go.
RUN set -eux; \
    chmod 0755 /opt/dxdfir/scan-list.sh; \
    chmod 0644 /opt/dxdfir/contract.yml; \
    chmod -R a+rX /opt/dxdfir/yara-rules /opt/dxdfir/suricata-rules /opt/dxdfir/hayabusa; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=signatures -e harden_static_binary=false -e harden_shell=true -e harden_python=false \
      -e '{"harden_extra_remove": ["/usr/bin/suricata-update", "/usr/bin/suricatasc", "/usr/bin/suricatactl", "/var/lib/suricata/update"]}'; \
    # remove the build-time hardener + everything the tools do not need at runtime,
    # keeping sh/dash for the YARA scan loop (glibc/libgcc stay — Hayabusa's gnu
    # build needs them; the tools are C/Rust, so python goes).
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* /usr/bin/bash /bin/bash /usr/bin/perl* \
           /usr/bin/curl /usr/bin/unzip

FROM scratch
COPY --from=hardened / /
# The gomount→goyara pipe. gomount is a static binary. goyara links libyara.so
# (from the `yara` apt package installed in the hardened stage and carried in with
# its rootfs above), plus that package's libmagic/libjansson/libcrypto/libz/
# libzstd/liblzma/libbz2 — so no extra runtime libraries are needed here. Neither
# tool carries a setuid bit; both run as the fixed non-root uid below.
COPY --from=gomount-build /out/gomount /usr/local/bin/gomount
COPY --from=goyara-build  /out/goyara  /usr/local/bin/goyara
COPY --from=entry-build   /out/signatures-entry /usr/local/bin/signatures-entry
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/usr/local/bin/signatures-entry"]
LABEL org.opencontainers.image.title="get-sybers/signatures" \
      org.opencontainers.image.description="The detection lane in one hardened image (YARA + Suricata + Hayabusa + the gomount->goyara userspace NTFS scan pipe) behind a static Go dispatcher: signatures-entry yara|suricata|hayabusa|scan self-orchestrates that sub-tool from its env block, one output folder per item, one JSON summary line. uid0 renamed+locked, no ansible/apt/pip/sudo/setuid, no python, runs as uid 2000; sh kept (declared) for the legacy per-file YARA loop." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT AND GPL-2.0-only AND GPL-3.0-only AND BSD-3-Clause" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="signatures" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.hayabusa.version="${HAYABUSA_VERSION}"
