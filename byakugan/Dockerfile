# syntax=docker/dockerfile:1
# get-sybers/byakugan — the external Byakugan MITRE CAR engine, cloned + built
# into the image at the pinned ref, behind the engine's own dispatcher
# (byakugan build|timeline|verify|load|car-vocab, plus the STIX/CTI exchange:
# stix-export|stix-behaviour|cti-pull|cti-sightings). The engine repo ships
# the Elastic detection rules-as-code (its rules/), baked from the clone at
# /rules and gated at build by its own rules/validate.py; a runtime mount
# overrides. Shape B; declared deviation python=true; no shell. Dispatch
# semantics, env blocks and run docs: README.md.
ARG BASE=debian:trixie-slim
ARG GO_BASE=golang:1.24-bookworm
ARG BYAKUGAN_REPO=https://github.com/Get-Sybers/byakugan
# the ONE place the engine version is set; consumers build without a REF arg and inherit it
ARG BYAKUGAN_REF=eb4ef6e675edd11bcd868a4841ebcc49d6fcd02d
ARG TOOL_VERSION=0.3.1
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

# stdlib-only Go → static binary; GOTOOLCHAIN=local keeps the build offline.
# The clone is RECURSIVE: the engine rebuilds its object model from the pinned
# nested submodules (third_party/car + attack-datasources).
FROM ${GO_BASE} AS engine
ARG BYAKUGAN_REPO
ARG BYAKUGAN_REF
ENV GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-trimpath
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends git ca-certificates; \
    rm -rf /var/lib/apt/lists/*; \
    git clone "${BYAKUGAN_REPO}" /src; \
    git -C /src checkout "${BYAKUGAN_REF}"; \
    git -C /src submodule update --init --recursive; \
    (cd /src/go && go build -o bin/byakugan-parse ./cmd/byakugan-parse); \
    test -x /src/go/bin/byakugan-parse; \
    # ship the model + binary, never the history (repo AND submodule .git dirs)
    find /src -name .git -prune -exec rm -rf {} +

FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG BYAKUGAN_REF
ARG TOOL_VERSION
# python3-yaml from apt (no pip); ca-certificates = TLS trust for the load sub-tool's push mode
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-yaml ca-certificates ansible-core; \
    rm -rf /var/lib/apt/lists/*
# the engine resolves its model dir relative to the package: run from this tree via PYTHONPATH, never site-packages
COPY --from=engine /src /opt/byakugan
COPY byakugan/byakugan-entry.py /usr/local/bin/byakugan
COPY byakugan/contract.yml /opt/byakugan/contract.yml
# the engine repo ships the rules-as-code: its rules/ rides the clone, baked
# at /rules where stix-export defaults its pattern resolution
COPY --from=engine /src/rules/ /rules/
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN chmod 0755 /usr/local/bin/byakugan && chmod 0644 /opt/byakugan/contract.yml
# the baked rules-as-code gate (the engine's own rules/validate.py, riding
# the clone): a malformed, dropped or unpinned rule FAILS THE BUILD; a
# runtime mount over /rules overrides the set
RUN set -eux; \
    python3 /opt/byakugan/rules/validate.py --rules-dir /rules >/dev/null; \
    find /rules -type d -exec chmod 0755 {} +; \
    find /rules -type f -exec chmod 0644 {} +
# sanity gate before the hardener strips the shell this RUN needs
RUN BYAKUGAN_VERSION=${TOOL_VERSION} BYAKUGAN_REF=${BYAKUGAN_REF} PYTHONPATH=/opt/byakugan /usr/local/bin/byakugan --version
RUN set -eux; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=byakugan -e harden_static_binary=false -e harden_shell=false -e harden_python=true; \
    PYTHONPATH=/opt/byakugan python3 -c 'import byakugan.cli, byakugan.pipeline, byakugan.timeline, byakugan.verify, byakugan.carmodel, byakugan.elastic.load, byakugan.elastic.projection, byakugan.elastic.inverse_projection, byakugan.exchange.cli, byakugan.exchange.behaviour'; \
    # every shell goes; python3 + python3-yaml + the baked engine tree stay (python IS the tool)
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
# PYTHONPATH = the baked source tree; BYAKUGAN_PARSE_BIN is read by the engine first; HOME=/tmp is the one writable path
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
      org.opencontainers.image.description="The Byakugan MITRE CAR engine (python + a static Go parse binary) cloned and built at the pinned ref, behind the engine's own multi-tool dispatcher: byakugan build|timeline|verify|load and the STIX/CTI exchange (stix-export|stix-behaviour|cti-pull|cti-sightings) self-orchestrate from their env blocks with one JSON summary line each, byakugan car-vocab prints the car_action vocabulary. The deployment's detection rules-as-code ride baked at /rules (build-gated; a mount overrides). Hardened: uid0 renamed+locked, no ansible/apt/pip/sudo/setuid/shell, runs as uid 2000; python kept (declared); ca-certificates kept for the load and exchange push/pull TLS trust." \
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
