#!/usr/bin/env bash
# ==============================================================================
# go-conform.sh — the Go conformance gate (go-standards.md §12).
#
# Runs, across every module in the workspace: a module-path check, gofmt -l,
# go vet, go build, go test, a go:generate drift check, and a structural lint
# (no exported Get* accessors). Exits non-zero on any failure.
#
# A module whose CGO backend is unavailable here (signatures/goyara needs
# libyara) is reported and its build/test/vet skipped — CI with the library
# present is the gate for it.
#
# Usage: scripts/go-conform.sh [module-dir ...]   (default: all workspace modules)
# ==============================================================================
set -uo pipefail
cd "$(dirname "$(readlink -f "$0")")/.." || exit 2
ROOT="$PWD"

if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  OK=$'\033[38;5;172m'; ERR=$'\033[38;5;167m'; DIM=$'\033[38;5;245m'; RST=$'\033[0m'
else OK= ERR= DIM= RST=; fi
pass=0; fail=0
ok()   { printf '%s[ ok ]%s %s\n'   "$OK"  "$RST" "$*"; pass=$((pass+1)); }
bad()  { printf '%s[fail]%s %s\n'   "$ERR" "$RST" "$*" >&2; fail=$((fail+1)); }
note() { printf '       %s%s%s\n'   "$DIM" "$*" "$RST"; }

WANT_PREFIX="github.com/Get-Sybers/GoDFIR-toolz"

# workspace modules (from go.work), or the args given
modules() {
  if [[ $# -gt 0 ]]; then printf '%s\n' "$@"; return; fi
  go work edit -json | sed -n 's/.*"DiskPath": "\(.*\)".*/\1/p'
}

cgo_missing() { # module dir -> 0 if it needs a CGO library we lack
  case "$1" in
    */goyara) pkg-config --exists yara 2>/dev/null && return 1 || return 0 ;;
  esac
  return 1
}

mapfile -t MODS < <(modules "$@")   # array preserves paths with spaces; no word-split/glob
for m in "${MODS[@]}"; do
  d="$ROOT/${m#./}"
  name="${m#./}"
  [[ -f "$d/go.mod" ]] || { bad "$name: no go.mod"; continue; }

  # module path convention (go-standards.md §1)
  mp=$(sed -n 's/^module //p' "$d/go.mod")
  case "$mp" in
    "$WANT_PREFIX"/*) : ;;
    *) bad "$name: module path $mp is not under $WANT_PREFIX" ;;
  esac

  # gofmt — a gofmt error (e.g. a parse error) fails the check, never passes silently
  if ! unformatted=$(gofmt -l "$d" 2>/tmp/gc.fmt.err); then
    bad "$name: gofmt errored:"; head -6 /tmp/gc.fmt.err >&2
  elif [[ -n "$unformatted" ]]; then
    bad "$name: gofmt needed: $unformatted"
  fi

  # no exported Get* accessors (go-standards.md §6)
  getters=$(grep -rnE 'func (\([^)]*\) )?Get[A-Z]' "$d" --include=*.go 2>/dev/null | grep -v '_test.go' || true)
  [[ -z "$getters" ]] || bad "$name: exported Get* accessor(s):"$'\n'"$getters"

  if cgo_missing "$m"; then
    note "$name: CGO backend unavailable here (needs libyara) — build/test run in CI"
    ok "$name: static checks"
    continue
  fi

  ( cd "$d" && go vet ./... ) 2>/tmp/gc.err   && ok "$name: vet"   || { bad "$name: vet";   head -6 /tmp/gc.err >&2; }
  ( cd "$d" && go build ./... ) 2>/tmp/gc.err  && ok "$name: build" || { bad "$name: build"; head -6 /tmp/gc.err >&2; }
  ( cd "$d" && go test ./... >/tmp/gc.out 2>&1 ) && ok "$name: test" || { bad "$name: test"; grep -E '^(FAIL|---|panic)' /tmp/gc.out | head -8 >&2; }

  # go:generate drift: only if the module declares any directive
  if grep -rql '//go:generate' "$d" --include=*.go 2>/dev/null; then
    ( cd "$d" && go generate ./... ) >/dev/null 2>&1
    if ! drift=$(cd "$ROOT" && git status --porcelain -- "${m#./}" 2>/tmp/gc.git.err); then
      bad "$name: generate drift check — git status failed:"; head -3 /tmp/gc.git.err >&2
    elif [[ -z "$drift" ]]; then
      ok "$name: generate (no drift)"
    else
      bad "$name: go:generate drift"; echo "$drift" >&2
    fi
  fi
done

echo
if [[ $fail -eq 0 ]]; then printf '%sconform: %d checks passed%s\n' "$OK" "$pass" "$RST"; exit 0
else printf '%sconform: %d passed, %d FAILED%s\n' "$ERR" "$pass" "$fail" "$RST"; exit 1; fi
