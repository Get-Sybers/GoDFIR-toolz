#!/usr/bin/env bash
#
# Build the hardened get-sybers/* images — a THIN LAUNCHER, nothing more: the
# inventory lives in images.yml and the build logic lives in ansible (the
# godfir_build role, driven by playbooks/build_images.yml). This script only
# forwards names and the optional stamp overrides.
#
#   ./build-all.sh                  # everything in images.yml, in manifest order
#   ./build-all.sh gore gomft       # a subset (names case-insensitive; the
#                                   # substituted EZ-tool aliases resolve)
#   ./build-all.sh sqlecmd bstrings # the .NET per-tool images by name
#   ./build-all.sh byakugan plaso signatures zeek
#
# Clone-at-build engine pins live as ARG defaults in the tool Dockerfiles;
# each entry's env_args in images.yml names the pins overridable from the
# environment (e.g. BYAKUGAN_REF=v1.2 ./build-all.sh byakugan — the role reads
# them itself). GODFIR_REVISION / GODFIR_RELEASE override the source stamps.
set -Eeuo pipefail
cd "$(dirname "$0")"

command -v ansible-playbook >/dev/null 2>&1 \
    || { echo "error: ansible-playbook is required — the build logic is ansible (the godfir_build role)" >&2; exit 2; }

EXTRA=()
if [ "$#" -gt 0 ]; then
    names=$(printf '"%s",' "$@")
    EXTRA+=(-e "{\"godfir_build_set\": [${names%,}]}")
fi
[ -n "${GODFIR_REVISION:-}" ] && EXTRA+=(-e "godfir_build_src_sha=${GODFIR_REVISION}")
[ -n "${GODFIR_RELEASE:-}" ] && EXTRA+=(-e "godfir_build_release=${GODFIR_RELEASE}")

exec ansible-playbook playbooks/build_images.yml "${EXTRA[@]}"
