# syntax=docker/dockerfile:1
# get-sybers/zeek — Zeek LTS for offline capture parsing behind zeek-run, a
# static Go batch entrypoint. Shape B: glibc stays; no shell, no python,
# zkg/zeekctl stripped. Batch semantics, env and run docs: README.md.
ARG BASE=debian:trixie-slim
ARG GO_VERSION=1.25
ARG ZEEK_PACKAGE=zeek-lts
ARG ZEEK_VERSION=8.0.10-0
# the OBS MASTER server (direct content, no mirrorbrain geo-redirect: a mid-sync
# mirror can never serve a stale deb); the repo key is pinned fail-closed below
ARG ZEEK_REPO=https://downloadcontent.opensuse.org/repositories/security:/zeek/Debian_13
ARG ZEEK_KEY_FPR=F9FA0223B56B116C363737EF5DA57BDD6DD785CA
ARG TOOL_VERSION=8.0.10
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM golang:${GO_VERSION}-alpine AS runner
ARG TOOL_VERSION
WORKDIR /src
COPY zeek/go.mod zeek/go.sum ./
RUN go mod download
COPY zeek/*.go zeek/contract.yml ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${TOOL_VERSION}" -o /zeek-run .
# build-stage sanity gate
RUN /zeek-run --version

FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG ZEEK_PACKAGE
ARG ZEEK_VERSION
ARG ZEEK_REPO
ARG ZEEK_KEY_FPR
RUN set -eux; \
    echo 'Acquire::Retries "3";' > /etc/apt/apt.conf.d/80-dxdfir-retries; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl ca-certificates gnupg ansible-core; \
    curl -fsSL -o /tmp/zeek-release.key "${ZEEK_REPO}/Release.key"; \
    # fail-closed: trust the key only if it IS the pinned security:zeek key
    gpg --show-keys --with-fingerprint --with-colons /tmp/zeek-release.key \
      | grep -q "^fpr:::::::::${ZEEK_KEY_FPR}:$"; \
    gpg --dearmor -o /usr/share/keyrings/zeek.gpg /tmp/zeek-release.key; \
    rm /tmp/zeek-release.key; \
    echo "deb [signed-by=/usr/share/keyrings/zeek.gpg] ${ZEEK_REPO}/ /" \
      > /etc/apt/sources.list.d/zeek.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends "${ZEEK_PACKAGE}${ZEEK_VERSION:+=${ZEEK_VERSION}}"; \
    apt-get purge -y curl gnupg; apt-get autoremove -y --purge
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN set -eux; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=zeek -e harden_static_binary=false -e harden_shell=false -e harden_python=false \
      -e '{"harden_extra_remove": ["/opt/zeek/bin/zkg", "/opt/zeek/bin/zeekctl", "/opt/zeek/bin/zeek-client", "/opt/zeek/var"]}'; \
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* \
           /bin/sh /bin/dash /bin/bash /bin/rbash /usr/bin/sh /usr/bin/dash /usr/bin/bash /usr/bin/rbash

FROM scratch
COPY --from=hardened / /
COPY --from=runner /zeek-run /usr/local/bin/zeek-run
ENV PATH=/opt/zeek/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp XDG_CACHE_HOME=/tmp/.cache
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["/usr/local/bin/zeek-run"]
LABEL org.opencontainers.image.title="get-sybers/zeek" \
      org.opencontainers.image.description="Zeek LTS for offline capture parsing, fronted by a static Go batch entrypoint: one zeek pass per capture under the input tree into its own output folder (JSON logs with ISO-8601 timestamps, or TSV), one JSON summary line. Hardened: uid0 renamed+locked, no ansible/apt/pip/sudo/setuid, no shell, no python, no zkg/zeekctl, runs as uid 2000; glibc kept for zeek." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="BSD-3-Clause" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="zeek" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/zeek/zeek"
