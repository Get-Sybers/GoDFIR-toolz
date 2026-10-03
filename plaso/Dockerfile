# syntax=docker/dockerfile:1
# get-sybers/plaso — Plaso at a pinned PyPI release (libyal built from source)
# behind the plaso-entry dispatcher. Shape B; declared deviations python=true,
# shell=true (sh/dash for the console-script wrappers; bash removed).
# Dispatch semantics, env blocks and run docs: README.md.
ARG BASE=python:3.12-slim
ARG PLASO_VERSION=20260720
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

FROM ${BASE} AS wheels
ARG PLASO_VERSION
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends build-essential pkg-config \
        zlib1g-dev libssl-dev libfuse3-dev; \
    pip wheel --no-cache-dir --wheel-dir /wheels "plaso==${PLASO_VERSION}"

FROM ${BASE} AS hardened
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG PLASO_VERSION
COPY --from=wheels /wheels /wheels
RUN set -eux; \
    pip install --no-cache-dir --no-index --find-links /wheels "plaso==${PLASO_VERSION}"; \
    pip install --no-cache-dir ansible-core; \
    rm -rf /wheels; \
    ln -s /usr/local/bin/log2timeline /usr/local/bin/log2timeline.py; \
    ln -s /usr/local/bin/psort /usr/local/bin/psort.py; \
    ln -s /usr/local/bin/image_export /usr/local/bin/image_export.py; \
    printf 'plaso %s (pypi, libyal built from source)\n' "${PLASO_VERSION}" > /etc/dxdfir-plaso-version
# modes pinned below, not inherited: the build context's umask varies and uid 2000 must read these
COPY plaso/psort_wrapper.py /opt/dxdfir/psort_wrapper.py
COPY plaso/plaso-entry.py /opt/dxdfir/plaso-entry.py
COPY plaso/contract.yml /opt/dxdfir/contract.yml
COPY hardening/harden.yml /tmp/hardening/harden.yml
RUN set -eux; \
    chmod 0644 /opt/dxdfir/psort_wrapper.py /opt/dxdfir/contract.yml; \
    chmod 0755 /opt/dxdfir/plaso-entry.py; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
      -e harden_uid=${DFIR_UID} -e harden_gid=${DFIR_GID} \
      -e harden_tool=plaso -e harden_static_binary=false -e harden_shell=true -e harden_python=true; \
    rm -rf /usr/local/bin/ansible* /usr/local/lib/python3*/site-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /bin/bash /usr/bin/bash /bin/rbash /usr/bin/rbash /usr/bin/bashbug
# build-stage sanity gate: the dispatcher answers --version and plaso's tools load
RUN python3 /opt/dxdfir/plaso-entry.py --version && log2timeline --version

FROM scratch
COPY --from=hardened / /
ENV PATH=/usr/local/bin:/usr/local/sbin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp XDG_CACHE_HOME=/tmp/.cache LANG=C.UTF-8 PYTHONDONTWRITEBYTECODE=1
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG PLASO_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["python3", "/opt/dxdfir/plaso-entry.py"]
LABEL org.opencontainers.image.title="get-sybers/plaso" \
      org.opencontainers.image.description="Plaso (pinned PyPI, libyal from source) behind a multi-tool dispatcher: plaso-entry log2timeline|psort|image_export self-orchestrates that sub-tool from its env block, one output folder per item, one JSON summary line. Hardened: uid0 renamed+locked, no ansible/apt/pip/sudo/setuid, runs as uid 2000; python and sh kept (declared)." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${PLASO_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="plaso" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}" \
      com.get-sybers.upstream="https://github.com/log2timeline/plaso"
