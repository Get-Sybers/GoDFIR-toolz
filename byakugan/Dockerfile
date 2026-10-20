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
ARG GO_BASE=golang:1.27-bookworm
ARG BYAKUGAN_REPO=https://github.com/Get-Sybers/Byakugan
# the ONE place the engine version is set; consumers build without a REF arg and
# inherit it. Full commit SHA (immutable): the merge of Byakugan #132 — the
# car-relationships lookup index (model/projection/rel_lookup.yml), a
# native-Elastic ES|QL LOOKUP JOIN layer over the CAR relationship graph that
# `load` populates (car-relationships-<namespace>) alongside the logs-car.rel-*
# edge stream — on top of #131's elastic layout (the CAR->ECS contract under
# model/projection/, the rendered elastic/ tree with the Byakugan Kibana space),
# after the commit tagged v0.2.0 on the engine repo; the pinned ref now includes
# the flat lane-layout CAR source discovery (byakugan #134).
ARG BYAKUGAN_REF=14a2812ed925a86199d24f890cd56e3cc6d2dea0
ARG TOOL_VERSION=0.4.3
ARG GODFIR_REVISION=unknown
# the get-sybers image release this Dockerfile ships as: the ONE version the
# workflow tags the pushed container with (synchronised across images, §31)
ARG GODFIR_RELEASE=v0.2.6

# stdlib-only Go → static binary; GOTOOLCHAIN=local keeps the build offline.
# The clone is RECURSIVE: the engine rebuilds its object model from the pinned
# nested submodules, its upstream inputs under model/sources/ (car, the CAR
# model; forensicartifacts; attack-datasources is vendored beside them).
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
# Runtime Python deps from apt (no pip): byakugan's requirements.txt / pyproject
# [project] dependencies are pyyaml>=6 + jsonschema>=4.18, supplied here as
# python3-yaml and python3-jsonschema (trixie ships 4.19.2 and pulls in
# referencing/rpds/attr — jsonschema is the load sub-tool's row/wire schema
# authority, imported lazily in byakugan.elastic.load._row_validators).
# ca-certificates = TLS trust for the load sub-tool's push mode.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends python3 python3-yaml python3-jsonschema python3-pip ca-certificates; \
    rm -rf /var/lib/apt/lists/*
# the engine resolves its model dir relative to the package: run from this tree via PYTHONPATH, never site-packages
COPY --from=engine /src /opt/byakugan
COPY byakugan/byakugan-entry.py /usr/local/bin/byakugan
COPY byakugan/contract.yml /opt/byakugan/contract.yml
# the engine repo ships the rules-as-code: its rules/ rides the clone, baked
# at /rules where stix-export defaults its pattern resolution
COPY --from=engine /src/rules/ /rules/
COPY hardening/harden.yml hardening/requirements.txt /tmp/hardening/
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
    # the hardening controller, pinned by hardening/requirements.txt, installed
    # into a throwaway dir removed below (python itself stays: python IS the tool)
    pip3 install --no-cache-dir --break-system-packages --target /tmp/ansible \
      -r /tmp/hardening/requirements.txt; \
    PYTHONPATH=/tmp/ansible /tmp/ansible/bin/ansible-playbook -c local -i localhost, \
      /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=byakugan -e harden_static_binary=false -e harden_shell=false -e harden_python=true; \
    # import the runtime deps explicitly (jsonschema, yaml): byakugan.elastic.load
    # imports jsonschema LAZILY (in _row_validators), so importing the package
    # alone would not catch a missing runtime dep — it would only surface when
    # the load sub-tool validates its first row, at runtime, in production.
    PYTHONPATH=/opt/byakugan python3 -c 'import jsonschema, yaml, byakugan.cli, byakugan.pipeline, byakugan.timeline, byakugan.verify, byakugan.carmodel, byakugan.elastic.load, byakugan.elastic.projection, byakugan.elastic.inverse_projection, byakugan.exchange.cli, byakugan.exchange.behaviour'; \
    # every shell goes; python3 + python3-yaml + the baked engine tree stay (python IS the tool)
    rm -rf /tmp/ansible /usr/bin/pip3* /usr/lib/python3/dist-packages/pip* \
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
      org.opencontainers.image.source="https://github.com/Get-Sybers/Byakugan" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="byakugan" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.engine-ref="${BYAKUGAN_REF}" \
      com.get-sybers.byakugan.ref="${BYAKUGAN_REF}"
