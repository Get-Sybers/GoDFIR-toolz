# syntax=docker/dockerfile:1
# get-sybers/signatures — the whole detection lane in one hardened image (YARA +
# Suricata + Hayabusa + the gomount→goyara NTFS scan pipe) behind the static
# signatures-entry dispatcher. Shape B; declared deviations shell=true,
# static_binary=false; no python at runtime. Semantics, env blocks and run
# docs: README.md.
ARG BASE=debian:trixie-slim
ARG HAYABUSA_VERSION=4.1.0
ARG GO_IMAGE=golang:trixie
ARG TOOL_VERSION=0.2.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

# gomount: CGO-free (the stream verb links nothing; mount only execs ntfs-3g)
FROM ${GO_IMAGE} AS gomount-build
WORKDIR /src/gomount
COPY gomount/go.mod gomount/go.sum ./
RUN go mod download
COPY gomount/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/gomount .

# goyara: cgo against libyara (headers from libyara-dev; runtime .so from the yara package)
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

# signatures-entry: the static dispatcher (this directory's module)
FROM ${GO_IMAGE} AS entry-build
ARG TOOL_VERSION
WORKDIR /src/signatures
COPY signatures/go.mod signatures/go.sum ./
RUN go mod download
COPY signatures/*.go signatures/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /out/signatures-entry .
# build-stage sanity gate
RUN /out/signatures-entry --version

FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG HAYABUSA_VERSION
ARG TARGETARCH
# pinned Hayabusa, downloaded + sha256-verified at build time (the release's own published checksums)
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
    rm -rf /opt/dxdfir/hayabusa/rules/.git /tmp/hb.zip; \
    # sanity gate: exactly the pinned version (fixed-string match on the banner's first two words),
    # and the dfir-timeline command signatures-entry drives exists (hayabusa 4 renamed it)
    /opt/dxdfir/hayabusa/hayabusa help | head -n 1 | cut -d' ' -f1-2 | grep -Fqx "Hayabusa v${HAYABUSA_VERSION}"; \
    /opt/dxdfir/hayabusa/hayabusa dfir-timeline --help >/dev/null
COPY signatures/scan-list.sh /opt/dxdfir/scan-list.sh
COPY signatures/contract.yml /opt/dxdfir/contract.yml
# bake the pinned DetectRaptor YARA ruleset (download, per-asset sha256, dedup-merge) at build time
COPY signatures/detectraptor.py /tmp/detectraptor.py
RUN set -eux; \
    python3 /tmp/detectraptor.py --rules-dir /opt/dxdfir/yara-rules; \
    test -s /opt/dxdfir/yara-rules/detectraptor/detectraptor.yar; \
    rm -f /tmp/detectraptor.py
# bake the pinned ET Open Suricata ruleset the same way
COPY signatures/suricata_rules.py /tmp/suricata_rules.py
RUN set -eux; \
    python3 /tmp/suricata_rules.py --rules-dir /opt/dxdfir/suricata-rules; \
    test -s /opt/dxdfir/suricata-rules/suricata.rules; \
    rm -f /tmp/suricata_rules.py
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN set -eux; \
    chmod 0755 /opt/dxdfir/scan-list.sh; \
    chmod 0644 /opt/dxdfir/contract.yml; \
    chmod -R a+rX /opt/dxdfir/yara-rules /opt/dxdfir/suricata-rules /opt/dxdfir/hayabusa; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=signatures -e harden_static_binary=false -e harden_shell=true -e harden_python=false \
      -e '{"harden_extra_remove": ["/usr/bin/suricata-update", "/usr/bin/suricatasc", "/usr/bin/suricatactl", "/var/lib/suricata/update"]}'; \
    # ansible, python and bash go; sh/dash stay (declared); glibc/libgcc stay for the gnu Hayabusa build
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* /usr/bin/bash /bin/bash /usr/bin/perl* \
           /usr/bin/curl /usr/bin/unzip

FROM scratch
COPY --from=hardened / /
# goyara's libyara.so and its deps ride in with the hardened rootfs above; nothing extra needed
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
# re-declared so the com.get-sybers.hayabusa.version label carries it
ARG HAYABUSA_VERSION
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
