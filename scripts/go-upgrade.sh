#!/usr/bin/env bash
# ==============================================================================
# go-upgrade.sh — raise Go modules to the highest versions their hard limits
# allow, then prove the result (go-standards.md §12).
#
# For every workspace module: raise each dependency to the newest release its
# constraints permit (go get -u, which never crosses a floor/ceiling the module
# or its graph declares), raise the `go` directive to the toolchain in use, and
# tidy. Then run go-conform.sh; the changes are only sound if it passes.
#
# The hard limits — a dependency floor, a security minimum, an API ceiling —
# are declared in go.mod and are the only thing that holds a version back.
#
# signatures/goyara needs libyara (CGO); it is skipped where the library is
# absent and upgraded in CI.
#
# Usage: scripts/go-upgrade.sh [module-dir ...]
# ==============================================================================
set -uo pipefail
cd "$(dirname "$(readlink -f "$0")")/.." || exit 2
ROOT="$PWD"

TOOLVER=$(go env GOVERSION | sed 's/^go//')   # e.g. 1.26.0

modules() {
  if [[ $# -gt 0 ]]; then printf '%s\n' "$@"; return; fi
  go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p'
}

mapfile -t MODS < <(modules "$@")   # array preserves paths with spaces; no word-split/glob
for m in "${MODS[@]}"; do
  d="$ROOT/${m#./}"; name="${m#./}"
  [[ -f "$d/go.mod" ]] || continue
  case "$name" in */goyara) pkg-config --exists yara 2>/dev/null || { echo "skip $name (needs libyara)"; continue; };; esac
  echo ">> upgrading $name"
  ( cd "$d" && go get -u ./... && go mod edit -go="$TOOLVER" && go mod tidy ) \
    || { echo "upgrade failed in $name" >&2; exit 1; }
done

# keep the workspace's own directive in step with the toolchain
go work edit -go="$TOOLVER"

echo ">> re-running conform"
exec "$ROOT/scripts/go-conform.sh"
