#!/usr/bin/env bash
# godaemonhunter contract smoke: a hunt over generated fixtures asserts the
# layered run — Layer 1 into knowledge/, enriched daemon output, exactly
# one aggregate JSON line — plus idempotency and the config-error exit.
#
# Default: builds and runs the hardened image (docker, repo root).
# CONTRACT_LOCAL=1 builds the binary with the host Go toolchain instead.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

in="$work/in" out="$work/out" scratch="$work/scratch"
mkdir -p "$in" "$out" "$scratch"
(cd "$tool_dir" && go run test/fixtures/gen.go "$in")

run() {
    local code=0
    if [[ "${CONTRACT_LOCAL:-0}" == "1" ]]; then
        env GODAEMONHUNTER_INPUT_DIR="$in" GODAEMONHUNTER_OUT_DIR="$out" \
            GODAEMONHUNTER_WORK_DIR="$scratch" "$@" \
            "$work/godaemonhunter" hunt >"$work/stdout" 2>"$work/stderr" || code=$?
    else
        docker run --rm --network none --read-only --cap-drop ALL \
            --security-opt no-new-privileges \
            --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
            -v "$in:/input:ro" -v "$out:/output" \
            get-sybers/godaemonhunter:latest hunt >"$work/stdout" 2>"$work/stderr" || code=$?
    fi
    return $code
}

if [[ "${CONTRACT_LOCAL:-0}" == "1" ]]; then
    (cd "$tool_dir" && CGO_ENABLED=0 go build -o "$work/godaemonhunter" .)
else
    (cd "$repo" && docker build -q -t get-sybers/godaemonhunter:latest -f godaemonhunter/Dockerfile .)
    chmod 777 "$out"
fi

fail() { echo "FAIL: $*" >&2; cat "$work/stderr" >&2 || true; exit 1; }
field() { python3 -c "import json;print(json.load(open('$work/stdout'))['$1'])"; }

code=0; run || code=$?
[[ $code -eq 0 ]] || fail "hunt exit $code, want 0"
[[ "$(wc -l <"$work/stdout")" == "1" ]] || fail "stdout is not one line"
[[ "$(field status)" == "ok" ]] || fail "status $(field status)"
ls "$out"/knowledge/*/gousers.jsonl >/dev/null 2>&1 || fail "knowledge store missing"
grep -q '"UIDName":"alice"' "$out"/goauditd/*/goauditd.jsonl || fail "enrichment missing"
grep -q '"Hostname":"web01"' "$out"/goauditd/*/goauditd.jsonl || fail "Host block missing"

code=0; run || code=$?
[[ $code -eq 0 ]] || fail "rerun exit $code"
python3 -c "
import json; s=json.load(open('$work/stdout'))
assert s['processed']==0 and s['skipped']>0, s" || fail "rerun not idempotent"

code=0
if [[ "${CONTRACT_LOCAL:-0}" == "1" ]]; then
    env GODAEMONHUNTER_INPUT_DIR="$in/missing" GODAEMONHUNTER_OUT_DIR="$out" \
        GODAEMONHUNTER_WORK_DIR="$scratch" \
        "$work/godaemonhunter" hunt >"$work/stdout" 2>"$work/stderr" || code=$?
    [[ $code -eq 2 ]] || fail "config-error exit $code, want 2"

    # argv debug pass-through (decision 16: the modes ride the dispatcher):
    # gosyslog -f on the fixture streams records to stdout.
    "$work/godaemonhunter" gosyslog -f "$in/var/log/syslog" -q \
        >"$work/argv-out" 2>"$work/stderr" || fail "argv pass-through exit $?"
    grep -q '"Tool":"gosyslog"' "$work/argv-out" || fail "argv pass-through emitted no gosyslog record"
fi

echo "godaemonhunter contract smoke: PASS"
