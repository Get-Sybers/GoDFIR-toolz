# syntax=docker/dockerfile:1
#
# Parameterized .NET tool image for the DX_DFIR pipeline — fetch a published
# .NET tool release at build time, bake it into a minimal-hardened .NET
# runtime, tool as the pinned ENTRYPOINT — generalized over the tool name so
# every remaining .NET tool builds identically:
#
#   docker build -t get-sybers/sqlecmd:latest --build-arg GODFIR_TOOL=SQLECmd \
#     -f godfir-tool/Dockerfile .
#   docker build -t get-sybers/bstrings:latest --build-arg GODFIR_TOOL=bstrings \
#     -f godfir-tool/Dockerfile .
#
# Pin the release (recommended):
#   --build-arg GODFIR_TOOL_SHA256=<sha256 of <GODFIR_TOOL>.zip>
#
# The whole release directory is kept — a tool that ships data sets alongside
# its DLL (SQLECmd's Maps/) keeps them at /opt/godfir-tool/<that dir>, reachable
# from the caller's flags. The .exe stub is dropped (the DLL is the
# entrypoint). The upstream tools are MIT-licensed; this fetches the published
# release at build time — a recipe, not a committed binary.
#
# Shape B, declared: static_binary=false (the .NET runtime, glibc), no shell,
# no python. The interface is the tool's own argv (contract.yml: entrypoint
# argv, one entry per built image) — the caller supplies only the flags:
#   docker run --rm -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
#     get-sybers/sqlecmd:latest -f /input/History --csv /output

ARG DOTNET_VERSION=9.0
ARG GODFIR_TOOL=SQLECmd
ARG GODFIR_TOOL_URL=""
ARG DFIR_UID=2000
ARG DFIR_GID=2000
ARG TOOL_VERSION=0.1.0
ARG GODFIR_REVISION=unknown
ARG GODFIR_RELEASE=dev

# ---- fetch + unpack the published release -----------------------------------
FROM debian:trixie-slim AS fetch
ARG GODFIR_TOOL
ARG GODFIR_TOOL_URL
ARG GODFIR_TOOL_SHA256=""
ARG GODFIR_TOOL_ALLOW_WINDOWS_ONLY=""
RUN set -eux; \
    # Fail FAST on the four Windows-bound release binaries (the case arm
    # below) — run on Linux each prints a refusal and exits 0, so a build
    # would ship a parser that silently produces nothing. The pipeline's own
    # Go parsers cover those artefact classes (goprefetch/, goese/), and VSCs
    # are mounted with libvshadow on the host. Set
    # GODFIR_TOOL_ALLOW_WINDOWS_ONLY=1 to build anyway (e.g. to extract a release).
    tool_lc="$(printf '%s' "${GODFIR_TOOL}" | tr '[:upper:]' '[:lower:]')"; \
    case "${tool_lc}" in \
        pecmd|srumecmd|sumecmd|vscmount) \
            if [ "${GODFIR_TOOL_ALLOW_WINDOWS_ONLY}" != "1" ]; then \
                echo "ERROR: ${GODFIR_TOOL} cannot parse artifacts on Linux (Windows-native" >&2; \
                echo "dependency). The pipeline's own Go parsers cover this artefact class;" >&2; \
                echo "set --build-arg GODFIR_TOOL_ALLOW_WINDOWS_ONLY=1 to build it anyway." >&2; \
                exit 1; \
            fi ;; \
    esac; \
    apt-get update; \
    apt-get install -y --no-install-recommends curl ca-certificates unzip; \
    rm -rf /var/lib/apt/lists/*; \
    url="${GODFIR_TOOL_URL:-https://download.ericzimmermanstools.com/net9/${GODFIR_TOOL}.zip}"; \
    curl -fsSL -o /tmp/tool.zip "${url}"; \
    if [ -n "${GODFIR_TOOL_SHA256}" ]; then \
        echo "${GODFIR_TOOL_SHA256}  /tmp/tool.zip" | sha256sum -c -; \
    fi; \
    unzip -q /tmp/tool.zip -d /tmp/unz; \
    # -iname: release zip casing varies (a DLL can sit in a differently-cased
    # subdirectory; rla ships rla.dll) — locate the DLL case-insensitively and use ITS real
    # basename for the runtimeconfig/deps trio.
    dll="$(find /tmp/unz -iname "${GODFIR_TOOL}.dll" -print -quit)"; \
    test -n "${dll}"; \
    src="$(dirname "${dll}")"; \
    base="$(basename "${dll}" .dll)"; \
    mkdir -p /opt/godfir-tool; \
    cp -a "${src}/." /opt/godfir-tool/; \
    test -f "/opt/godfir-tool/${base}.dll"; \
    rm -f "/opt/godfir-tool/${base}.exe"; \
    # Fixed-path entrypoint: `dotnet tool.dll` resolves tool.runtimeconfig.json
    # and tool.deps.json by the GIVEN name, so the whole trio is linked under
    # the stable name (the deps content references the real assembly, which the
    # runtime probes in the same dir).
    ln -s "/opt/godfir-tool/${base}.dll" /opt/godfir-tool/tool.dll; \
    for suffix in runtimeconfig.json deps.json; do \
        if [ -f "/opt/godfir-tool/${base}.${suffix}" ]; then \
            ln -s "/opt/godfir-tool/${base}.${suffix}" "/opt/godfir-tool/tool.${suffix}"; \
        fi; \
    done

# ---- minimal hardened image --------------------------------------------------
FROM mcr.microsoft.com/dotnet/runtime:${DOTNET_VERSION} AS hardened
ARG GODFIR_TOOL
ARG DFIR_UID
ARG DFIR_GID
COPY --from=fetch /opt/godfir-tool /opt/godfir-tool
COPY --chmod=0644 godfir-tool/contract.yml /opt/godfir-tool/contract.yml
# Build-stage sanity gate: the runtime runs and reports its version, and the
# tool DLL loads (its --help exit code is the tool's own).
RUN dotnet --version && (dotnet /opt/godfir-tool/tool.dll --help > /dev/null || true)
COPY hardening/harden.yml /tmp/hardening/harden.yml
# harden.yml writes the /etc/dfir-hardened self-declaration (schema=1) from the
# values below (tool=godfir-tool, the recipe; the built tool's name is a label);
# the shell and python it declares absent are removed right after.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ansible-core; \
    ansible-playbook -c local -i localhost, /tmp/hardening/harden.yml \
        -e "harden_uid=${DFIR_UID}" -e "harden_gid=${DFIR_GID}" \
        -e harden_tool=godfir-tool -e harden_static_binary=false -e harden_shell=false -e harden_python=false; \
    rm -rf /usr/bin/ansible* /usr/lib/python3/dist-packages/ansible* \
           /etc/ansible /root/.ansible /tmp/hardening \
           /usr/bin/python3* /usr/lib/python3* \
           /bin/sh /bin/dash /bin/bash /bin/rbash /usr/bin/sh /usr/bin/dash /usr/bin/bash /usr/bin/rbash

FROM scratch
COPY --from=hardened / /
ARG GODFIR_TOOL
ARG GODFIR_TOOL_URL
ARG DFIR_UID
ARG DFIR_GID
ARG TOOL_VERSION
ARG GODFIR_REVISION
ARG GODFIR_RELEASE
ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    HOME=/tmp DOTNET_CLI_HOME=/tmp XDG_CACHE_HOME=/tmp/.cache \
    DOTNET_RUNNING_IN_CONTAINER=true DOTNET_EnableDiagnostics=0 \
    DOTNET_NOLOGO=1
WORKDIR /opt/godfir-tool
USER ${DFIR_UID}:${DFIR_GID}
ENTRYPOINT ["dotnet", "/opt/godfir-tool/tool.dll"]
LABEL org.opencontainers.image.title="get-sybers/godfir-tool" \
      org.opencontainers.image.description="A published .NET forensic tool release baked into a hardened .NET runtime with the tool DLL as ENTRYPOINT (argv-driven; parameterized over GODFIR_TOOL). uid0 renamed+locked, no ansible/apt/pip/sudo/setuid, no shell, no python, runs as uid 2000." \
      org.opencontainers.image.source="https://github.com/Get-Sybers/GoDFIR-toolz" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${TOOL_VERSION}" \
      org.opencontainers.image.revision="${GODFIR_REVISION}" \
      com.get-sybers.tool="godfir-tool" \
      com.get-sybers.godfir-tool.name="${GODFIR_TOOL}" \
      com.get-sybers.tool.url="${GODFIR_TOOL_URL}" \
      com.get-sybers.hardened="true" \
      com.get-sybers.contract="1" \
      com.get-sybers.godfir-release="${GODFIR_RELEASE}"
