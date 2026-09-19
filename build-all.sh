#!/usr/bin/env bash
#
# Build the hardened DFIR images: the twelve Go parsers (goprefetch/ … gowxt/),
# one per-tool image per remaining .NET tool (godfir-tool/Dockerfile), and the
# DX_DFIR pipeline images (byakugan/, plaso/, signatures/, zeek/, anamnesis/).
#
#   ./build-all.sh                  # everything
#   ./build-all.sh gore gomft       # a subset (names case-insensitive;
#                                   # legacy tool-name aliases still resolve)
#   ./build-all.sh sqlecmd bstrings # the .NET per-tool images by name
#   ./build-all.sh byakugan plaso signatures zeek
set -Eeuo pipefail
cd "$(dirname "$0")"

# Zip basenames on download.ericzimmermanstools.com/net9/ — casing matters for
# the download URL, so keep these exactly as published. Only the five tools the
# repo still builds from .NET releases are listed; everything else is a native
# Go parser.
LINUX_TOOLS=(
  bstrings iisGeolocate RecentFileCacheParser rla SQLECmd
)

lc() { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

build_godfir_tool() {
  local tool="$1"
  echo "==> get-sybers/$(lc "${tool}") (godfir-tool/Dockerfile, GODFIR_TOOL=${tool})"
  docker build -t "get-sybers/$(lc "${tool}"):latest" \
    --build-arg GODFIR_TOOL="${tool}" -f godfir-tool/Dockerfile .
}

build_goprefetch() {
  echo "==> get-sybers/goprefetch (Go, Windows prefetch .pf)"
  docker build -t get-sybers/goprefetch:latest -f goprefetch/Dockerfile goprefetch
}

build_goese() {
  echo "==> get-sybers/goese (Go, ESE databases: SRUM / SUM)"
  docker build -t get-sybers/goese:latest -f goese/Dockerfile goese
}

build_gorb() {
  echo "==> get-sybers/gorb (Go, Recycle Bin \$I records)"
  docker build -t get-sybers/gorb:latest -f gorb/Dockerfile gorb
}

build_gomft() {
  echo "==> get-sybers/gomft (Go, raw \$MFT)"
  docker build -t get-sybers/gomft:latest -f gomft/Dockerfile gomft
}

build_goamcache() {
  echo "==> get-sybers/goamcache (Go, Amcache.hve)"
  docker build -t get-sybers/goamcache:latest -f goamcache/Dockerfile goamcache
}

build_goappcompat() {
  echo "==> get-sybers/goappcompat (Go, ShimCache from SYSTEM hives)"
  docker build -t get-sybers/goappcompat:latest -f goappcompat/Dockerfile goappcompat
}

build_goevtx() {
  echo "==> get-sybers/goevtx (Go, .evtx event logs)"
  docker build -t get-sybers/goevtx:latest -f goevtx/Dockerfile goevtx
}

build_gore() {
  echo "==> get-sybers/gore (Go, registry batch dumps)"
  docker build -t get-sybers/gore:latest -f gore/Dockerfile gore
}

build_gosbe() {
  echo "==> get-sybers/gosbe (Go, ShellBags BagMRU)"
  docker build -t get-sybers/gosbe:latest -f gosbe/Dockerfile gosbe
}

build_gole() {
  echo "==> get-sybers/gole (Go, .lnk shell links)"
  docker build -t get-sybers/gole:latest -f gole/Dockerfile gole
}

build_gojle() {
  echo "==> get-sybers/gojle (Go, AutomaticDestinations jump lists)"
  docker build -t get-sybers/gojle:latest -f gojle/Dockerfile gojle
}

build_gowxt() {
  echo "==> get-sybers/gowxt (Go, Windows Timeline ActivitiesCache.db)"
  docker build -t get-sybers/gowxt:latest -f gowxt/Dockerfile gowxt
}

build_anamnesis() {
  # anamnesis (pure-Go memory forensics on MemProcFS — no Volatility, no Python).
  # Context is the repo root so hardening/harden.yml is in reach; the source is
  # cloned at build time at ANAMNESIS_REF (default main here — DX_DFIR passes its
  # sources.yml pin). The MemProcFS libraries are fetched at MEMPROCFS_VERSION and
  # verified against MEMPROCFS_SHA256 when set. The batch ENTRYPOINT is built into
  # the binary (no host wrapper).
  echo "==> get-sybers/anamnesis (Go, MemProcFS memory forensics, env-driven batch)"
  docker build -t get-sybers/anamnesis:latest \
    ${ANAMNESIS_REF:+--build-arg ANAMNESIS_REF="${ANAMNESIS_REF}"} \
    ${MEMPROCFS_VERSION:+--build-arg MEMPROCFS_VERSION="${MEMPROCFS_VERSION}"} \
    ${MEMPROCFS_SHA256:+--build-arg MEMPROCFS_SHA256="${MEMPROCFS_SHA256}"} \
    -f anamnesis/Dockerfile .
}

build_byakugan() {
  # Byakugan MITRE CAR engine, python + a static Go parse binary. Context is the
  # repo root so hardening/harden.yml and byakugan/byakugan-entry.py are in
  # reach; the engine is cloned RECURSIVELY at build time at BYAKUGAN_REF
  # (default main here — DX_DFIR passes its sources.yml pin).
  echo "==> get-sybers/byakugan (python, MITRE CAR engine, cloned at BYAKUGAN_REF)"
  docker build -t get-sybers/byakugan:latest \
    ${BYAKUGAN_REF:+--build-arg BYAKUGAN_REF="${BYAKUGAN_REF}"} \
    -f byakugan/Dockerfile .
}

build_plaso() {
  echo "==> get-sybers/plaso (python, pinned-PyPI Plaso + psort wrapper)"
  docker build -t get-sybers/plaso:latest -f plaso/Dockerfile .
}

build_signatures() {
  echo "==> get-sybers/signatures (YARA + Suricata + Hayabusa detection lane)"
  docker build -t get-sybers/signatures:latest -f signatures/Dockerfile .
}

build_zeek() {
  echo "==> get-sybers/zeek (Zeek LTS, offline capture parsing)"
  docker build -t get-sybers/zeek:latest -f zeek/Dockerfile .
}

resolve() {
  local want
  want="$(lc "$1")"
  case "${want}" in
    goprefetch|prefetch|pecmd) build_goprefetch; return ;;
    goese|esedump|srum|srumecmd|sumecmd) build_goese; return ;;
    gorb|rbcmd) build_gorb; return ;;
    gomft|mftecmd) build_gomft; return ;;
    goamcache|amcacheparser) build_goamcache; return ;;
    goappcompat|appcompatcacheparser) build_goappcompat; return ;;
    goevtx|evtxecmd) build_goevtx; return ;;
    gore|recmd) build_gore; return ;;
    gosbe|sbecmd) build_gosbe; return ;;
    gole|lecmd) build_gole; return ;;
    gojle|jlecmd) build_gojle; return ;;
    gowxt|wxtcmd) build_gowxt; return ;;
    anamnesis|memory) build_anamnesis; return ;;
    byakugan|mitrecar|car) build_byakugan; return ;;
    plaso|log2timeline|psort) build_plaso; return ;;
    signatures|yara|suricata|hayabusa) build_signatures; return ;;
    zeek) build_zeek; return ;;
    vscmount)
      echo "VSCMount manipulates the Windows VSS device namespace and has no" >&2
      echo "Linux container; use libvshadow (vshadowinfo/vshadowmount) on the host." >&2
      exit 1 ;;
  esac
  local tool
  for tool in "${LINUX_TOOLS[@]}"; do
    if [ "$(lc "${tool}")" = "${want}" ]; then
      build_godfir_tool "${tool}"
      return
    fi
  done
  echo "unknown tool '$1' — valid: ${LINUX_TOOLS[*]} goprefetch goese gorb gomft goamcache goappcompat goevtx gore gosbe gole gojle gowxt anamnesis byakugan plaso signatures zeek" >&2
  echo "  (the substituted EZ-tool names also work: pecmd, srumecmd/sumecmd, rbcmd, mftecmd, amcacheparser, appcompatcacheparser, evtxecmd, recmd, sbecmd, lecmd, jlecmd, wxtcmd)" >&2
  exit 1
}

if [ "$#" -gt 0 ]; then
  for arg in "$@"; do resolve "${arg}"; done
else
  for tool in "${LINUX_TOOLS[@]}"; do build_godfir_tool "${tool}"; done
  build_goprefetch
  build_goese
  build_gorb
  build_gomft
  build_goamcache
  build_goappcompat
  build_goevtx
  build_gore
  build_gosbe
  build_gole
  build_gojle
  build_gowxt
  build_anamnesis
  build_byakugan
  build_plaso
  build_signatures
  build_zeek
fi
echo "done."
