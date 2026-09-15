# syntax=docker/dockerfile:1
#
# get-sybers/zeek — minimal hardened Zeek LTS for OFFLINE capture parsing. Stripped to
# the zeek binary + its scripts; ansible (build-time hardener), python and every
# shell removed from the final image; zkg/zeekctl stripped. zeek is the pinned
# ENTRYPOINT.
#
# zeek-lts comes from the OpenSUSE security:zeek OBS repo, fetched from the
# MASTER server (downloadcontent.opensuse.org — direct content, no mirrorbrain
# geo-redirect, so a mid-sync mirror can never serve a stale deb), with the
# package version pinned (ZEEK_VERSION; empty = repo-current) and the repo
# signing key verified against a pinned fingerprint before it is trusted
# (ZEEK_KEY_FPR, fail-closed) — same doctrine as signatures' Hayabusa sha256
# check.
#
#   docker build -t get-sybers/zeek:latest -f zeek/Dockerfile .
ARG BASE=debian:trixie-slim
ARG ZEEK_PACKAGE=zeek-lts
ARG ZEEK_VERSION=8.0.10-0
ARG ZEEK_REPO=https://downloadcontent.opensuse.org/repositories/security:/zeek/Debian_13
ARG ZEEK_KEY_FPR=F9FA0223B56B116C363737EF5DA57BDD6DD785CA

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
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e '{"harden_extra_remove": ["/opt/zeek/bin/zkg", "/opt/zeek/bin/zeekctl", "/opt/zeek/bin/zeek-client", "/opt/zeek/var"]}'; \
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* \
           /bin/sh /bin/dash /bin/bash /bin/rbash /usr/bin/sh /usr/bin/dash /usr/bin/bash /usr/bin/rbash

FROM scratch
COPY --from=hardened / /
ENV PATH=/opt/zeek/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin HOME=/tmp
ARG DFIR_UID=2000
ARG DFIR_GID=2000
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["zeek"]
LABEL org.opencontainers.image.title="get-sybers/zeek" \
      org.opencontainers.image.description="Minimal hardened Zeek LTS (offline pcap parsing) for the DX_DFIR zeek lane: uid0 renamed+locked, no ansible/apt/pip/sudo/setuid, no shell, no python, no zkg/zeekctl, runs as a fixed non-root uid (default 2000)." \
      com.get-sybers.hardened="true"
