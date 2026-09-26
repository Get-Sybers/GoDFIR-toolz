#!/usr/bin/env bash
#
# Standalone-only thin launcher of the collection playbook — usage, env pins
# and the consumer story: README.md "Building". An integrating consumer uses
# the godfir_build role, never this script.
set -Eeuo pipefail
cd "$(dirname "$0")"

command -v ansible-playbook >/dev/null 2>&1 \
    || { echo "error: ansible-playbook is required — the build logic is ansible (the godfir_build role)" >&2; exit 2; }

EXTRA=()
if [ "$#" -gt 0 ]; then
    # JSON-escape each name so a quoted/malformed argument can't break or extend the extra-vars JSON
    names=""
    for _name in "$@"; do
        _esc=${_name//\\/\\\\}
        _esc=${_esc//\"/\\\"}
        names+="\"${_esc}\","
    done
    EXTRA+=(-e "{\"godfir_build_set\": [${names%,}]}")
fi
[ -n "${GODFIR_REVISION:-}" ] && EXTRA+=(-e "godfir_build_src_sha=${GODFIR_REVISION}")
[ -n "${GODFIR_RELEASE:-}" ] && EXTRA+=(-e "godfir_build_release=${GODFIR_RELEASE}")

exec ansible-playbook playbooks/build_images.yml "${EXTRA[@]}"
