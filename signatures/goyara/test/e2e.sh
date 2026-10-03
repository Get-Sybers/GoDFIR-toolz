#!/usr/bin/env bash
#
# goyara end-to-end PIPE test: gomount stream <image> | goyara --rules <...>
# over an mkntfs fixture — no evidence image, no mount, no privilege, and NO
# environment SKIP path. Full design, assertions and the non-resident marker
# subtlety: ../README.md "The end-to-end pipe test". Exit 0 proven, 2 failed.
set -u -o pipefail

# ----------------------------------------------------------------------------- config
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOYARA_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"          # the goyara Go module root
REPO_DIR="$(cd "$GOYARA_DIR/../.." && pwd)"         # GoDFIR-toolz repo root
GOMOUNT_DIR="${GOMOUNT_DIR:-$REPO_DIR/gomount}"     # the gomount Go module root
WORK="${WORK:-/work}"                               # writable scratch (also holds the image)
IMG="${IMG:-$WORK/ntfs.img}"
SECTORS="${SECTORS:-40960}"                         # 40960 * 512 = 20 MiB superfloppy
SECTOR_SIZE="${SECTOR_SIZE:-512}"
LABEL="${LABEL:-GOYARAE2E}"
MARKER="GOYARA_HIT_MARKER"
RULE_NAME="detectraptor_smoke"
HIT_NAME="hit.txt"
MISS_NAME="miss.txt"
HIT_FILL_BYTES="${HIT_FILL_BYTES:-262144}"          # 256 KiB filler => non-resident $DATA
RULES="$WORK/trivial.yar"
# Where a built signatures image bakes the real DetectRaptor ruleset. If present
# (e.g. this test runs inside get-sybers/signatures), a compile-only smoke runs.
REAL_RULES="${REAL_RULES:-/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar}"

GOMOUNT_BIN="${GOMOUNT_BIN:-$WORK/gomount}"
GOYARA_BIN="${GOYARA_BIN:-$WORK/goyara}"

log()  { printf '[e2e] %s\n' "$*"; }
ok()   { printf '[e2e] PASS: %s\n' "$*"; }
fail() { printf '[e2e] FAIL: %s\n' "$*" >&2; }
die()  { fail "$*"; exit 2; }

# ----------------------------------------------------------------------------- env report
environment_report() {
  log "================= ENVIRONMENT REPORT ================="
  log "kernel:            $(uname -sr)"
  log "id:                $(id)"
  [ -e /dev/fuse ] && log "/dev/fuse:         present (NOT USED — pure pipe)" \
                    || log "/dev/fuse:         absent  (NOT NEEDED — pure pipe)"
  [ -e /dev/kvm  ] && log "/dev/kvm:          present (NOT USED — pure pipe)" \
                    || log "/dev/kvm:          absent  (NOT NEEDED — pure pipe)"
  log "mkntfs:            $(command -v mkntfs || echo MISSING)"
  log "ntfscp:            $(command -v ntfscp || echo MISSING)"
  log "go toolchain:      $(command -v go || echo MISSING)"
  log "pkg-config yara:   $(pkg-config --exists yara 2>/dev/null && pkg-config --modversion yara || echo 'no (.pc absent; -lyara fallback)')"
  log "real ruleset:      $( [ -s "$REAL_RULES" ] && echo "$REAL_RULES" || echo 'absent (compile-only smoke skipped)')"
  log "====================================================="
}

require_tools() {
  command -v sha256sum >/dev/null 2>&1 || die "sha256sum not found (coreutils)."
  command -v mkntfs   >/dev/null 2>&1 || die "mkntfs not found — install ntfs-3g (apt-get install ntfs-3g)."
  command -v ntfscp   >/dev/null 2>&1 || die "ntfscp not found — install ntfs-3g (apt-get install ntfs-3g)."
  command -v go       >/dev/null 2>&1 || die "go toolchain not found."
}

# ----------------------------------------------------------------------------- build
build_binaries() {
  mkdir -p "$WORK"
  # gomount is CGO-free (it only ever EXECs ntfs-3g; here we use its userspace
  # stream verb, which links nothing external).
  log "building gomount (CGO off) from $GOMOUNT_DIR -> $GOMOUNT_BIN"
  ( cd "$GOMOUNT_DIR" && CGO_ENABLED=0 go build -o "$GOMOUNT_BIN" . ) \
    || die "go build of gomount failed."

  # goyara links libyara through cgo. Prefer pkg-config; fall back to -lyara with
  # the no-pkg-config build tag when the distro libyara-dev ships no yara.pc.
  log "building goyara (cgo + libyara) from $GOYARA_DIR -> $GOYARA_BIN"
  if pkg-config --exists yara 2>/dev/null; then
    ( cd "$GOYARA_DIR" && CGO_ENABLED=1 go build -o "$GOYARA_BIN" . ) \
      || die "go build of goyara (pkg-config yara) failed."
  else
    ( cd "$GOYARA_DIR" && CGO_ENABLED=1 CGO_LDFLAGS="-lyara" \
        go build -tags yara_no_pkg_config -o "$GOYARA_BIN" . ) \
      || die "go build of goyara (-lyara fallback) failed."
  fi
}

# ----------------------------------------------------------------------------- fixture
build_fixture() {
  log "creating a $((SECTORS * SECTOR_SIZE / 1024 / 1024)) MiB partitionless (superfloppy) NTFS volume: $IMG"
  truncate -s "$((SECTORS * SECTOR_SIZE))" "$IMG"
  if ! mkntfs -F -Q -L "$LABEL" -s "$SECTOR_SIZE" "$IMG" "$SECTORS" >/tmp/mkntfs.log 2>&1; then
    fail "mkntfs failed:"; cat /tmp/mkntfs.log >&2; exit 2
  fi

  # hit.txt: marker at offset 0, then 256 KiB filler so $DATA is NON-RESIDENT
  # (marker lives in data clusters, not the $MFT record). miss.txt: no marker.
  printf '%s\n' "$MARKER" > "$WORK/$HIT_NAME"
  head -c "$HIT_FILL_BYTES" /dev/zero | tr '\0' 'A' >> "$WORK/$HIT_NAME"
  printf 'this file has no token whatsoever\n' > "$WORK/$MISS_NAME"

  for n in "$HIT_NAME" "$MISS_NAME"; do
    if ! ntfscp "$IMG" "$WORK/$n" "$n" >/tmp/ntfscp.log 2>&1; then
      fail "ntfscp of $n failed:"; cat /tmp/ntfscp.log >&2; exit 2
    fi
  done
  log "seeded $HIT_NAME ($(stat -c %s "$WORK/$HIT_NAME") bytes, non-resident) and $MISS_NAME into the image (no mount, unprivileged)"

  # trivial rules: the smoke rule the assertions key on.
  cat > "$RULES" <<EOF
rule $RULE_NAME
{
    strings:
        \$a = "$MARKER" ascii wide
    condition:
        \$a
}
EOF
  log "wrote rules: $RULES"
}

# ----------------------------------------------------------------------------- pipe run
run_pipe() {
  IMG_BEFORE="$(sha256sum "$IMG" | cut -d' ' -f1)"
  log "sha256(image) before pipe: $IMG_BEFORE"

  log "RUN: $GOMOUNT_BIN stream $IMG | $GOYARA_BIN --rules $RULES --json -"
  "$GOMOUNT_BIN" stream "$IMG" 2>"$WORK/gomount.err" \
    | "$GOYARA_BIN" --rules "$RULES" --json - >"$WORK/goyara.out" 2>"$WORK/goyara.err"
  # Snapshot the whole status array in one assignment: reading PIPESTATUS[0] into
  # its own var would reset PIPESTATUS before PIPESTATUS[1] is read (set -u trap).
  local ps=("${PIPESTATUS[@]}")
  GM_RC=${ps[0]}; GY_RC=${ps[1]}

  log "gomount rc=$GM_RC  (0 = no per-file errors, 2 = some entries skipped; both are valid)"
  log "gomount summary (stderr, last line): $(tail -1 "$WORK/gomount.err")"
  log "goyara  rc=$GY_RC"
  log "goyara  match records (stdout):"
  sed 's/^/[e2e]   /' "$WORK/goyara.out"
  log "goyara  summary (stderr, last line): $(tail -1 "$WORK/goyara.err")"
}

# ----------------------------------------------------------------------------- diagnostic
# Independent of the assertions: dump the raw stream to a tar and count how many
# streamed entries actually contain the marker. Proves (not assumes) the
# non-resident fixture keeps the marker to a single entry despite /$MFT et al.
diagnostic_marker_spread() {
  local tar="$WORK/stream.tar" extract="$WORK/extract"
  "$GOMOUNT_BIN" stream "$IMG" >"$tar" 2>/dev/null || true
  local entries; entries="$(tar -tf "$tar" 2>/dev/null | wc -l | tr -d ' ')"
  rm -rf "$extract"; mkdir -p "$extract"
  ( cd "$extract" && tar -xf "$tar" 2>/dev/null ) || true
  local hits; hits="$(grep -rlF "$MARKER" "$extract" 2>/dev/null | wc -l | tr -d ' ')"
  log "diagnostic: stream held $entries entries; the marker is present in $hits of them:"
  grep -rlF "$MARKER" "$extract" 2>/dev/null | sed "s#$extract#[e2e]     #"
}

# ----------------------------------------------------------------------------- assertions
json_num() {  # json_num <key> <json-line>  -> the integer value of "key":N
  sed -n "s/.*\"$1\":\([0-9-]\{1,\}\).*/\1/p" <<<"$2" | head -1
}

assert_pipeline() {
  local rc=0
  local out="$WORK/goyara.out"
  local summary; summary="$(tail -1 "$WORK/goyara.err")"

  # ---- A: exactly one match record ----
  local lines; lines="$(grep -c '{' "$out" 2>/dev/null)"; lines="${lines:-0}"
  if [ "$lines" -eq 1 ]; then
    ok "A: goyara emitted exactly one match record"
  else
    fail "A: expected exactly 1 match record, got $lines"; rc=2
  fi

  # ---- B: rule == detectraptor_smoke ----
  if grep -qF "\"rule\":\"$RULE_NAME\"" "$out"; then
    ok "B: match rule == $RULE_NAME"
  else
    fail "B: match rule is not $RULE_NAME"; rc=2
  fi

  # ---- C: match target == /hit.txt ----
  if grep -qF "\"target\":\"/$HIT_NAME\"" "$out"; then
    ok "C: match target == /$HIT_NAME"
  else
    fail "C: match target is not /$HIT_NAME"; rc=2
  fi

  # ---- D: stderr summary matches>=1 and files_scanned>=2 ----
  local m fscan
  m="$(json_num matches "$summary")"; fscan="$(json_num files_scanned "$summary")"
  m="${m:-0}"; fscan="${fscan:-0}"
  if [ "$m" -ge 1 ] && [ "$fscan" -ge 2 ]; then
    ok "D: summary matches=$m (>=1) and files_scanned=$fscan (>=2)"
  else
    fail "D: summary matches=$m (want >=1), files_scanned=$fscan (want >=2)  [$summary]"; rc=2
  fi

  # ---- E: source image sha256 unchanged ----
  local after; after="$(sha256sum "$IMG" | cut -d' ' -f1)"
  log "sha256(image) after  pipe: $after"
  if [ "$IMG_BEFORE" = "$after" ]; then
    ok "E: source image sha256 UNCHANGED across the pipe (read-only proven)"
  else
    fail "E: source image sha256 CHANGED — a read wrote to evidence ($IMG_BEFORE -> $after)"; rc=2
  fi

  return $rc
}

# ----------------------------------------------------------------------------- real-ruleset compile smoke
# Compile-only: goyara compiles the ruleset BEFORE reading the tar, so feeding it
# an EMPTY tar proves the (large) real ruleset compiles under goyara's libyara
# without needing any evidence. Exit 0 => compiled and scanned zero files.
real_ruleset_smoke() {
  if [ ! -s "$REAL_RULES" ]; then
    log "real-ruleset smoke: SKIPPED (no $REAL_RULES on this image)"
    return 0
  fi
  log "real-ruleset smoke: compiling $REAL_RULES via goyara against an empty tar"
  if tar -cf - -T /dev/null | "$GOYARA_BIN" --rules "$REAL_RULES" --json - >/dev/null 2>"$WORK/real.err"; then
    ok "real-ruleset smoke: $REAL_RULES compiles under goyara ($(tail -1 "$WORK/real.err"))"
  else
    fail "real-ruleset smoke: goyara could not compile $REAL_RULES:"; cat "$WORK/real.err" >&2
    return 2
  fi
}

# ----------------------------------------------------------------------------- main
main() {
  environment_report
  require_tools
  build_binaries
  build_fixture
  run_pipe
  diagnostic_marker_spread

  local rc=0
  assert_pipeline || rc=2
  real_ruleset_smoke || rc=2

  if [ "$rc" -eq 0 ]; then
    log "ALL ASSERTIONS PASSED — gomount stream | goyara proven with no mount/fuse/kvm/privilege."
  else
    fail "one or more assertions failed (see above)."
  fi
  exit "$rc"
}

main "$@"
