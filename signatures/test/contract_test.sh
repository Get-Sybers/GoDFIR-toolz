#!/usr/bin/env bash
# test/contract_test.sh — the framework conformance smoke test for the
# signatures multi-tool image (docs/framework 06.5). It builds the image
# (unless IMAGE is given), runs `yara` over test/fixtures/staged/ and
# `suricata` over test/fixtures/pcaps/, and asserts for each run:
#   1. the exit code (0);
#   2. stdout is exactly one JSON line carrying every summary_schema key;
#   3. a rerun is idempotent: same exit, every item skipped, no new files;
# then runs with a bad environment (missing input mount) and with no sub-tool
# named, asserting exit 2 / status config_error for both. hayabusa and scan
# need real evidence (an .evtx tree, an NTFS image) and are covered by the Go
# unit tests with stub tools.
#
#   test/contract_test.sh                                   # docker build + run
#   IMAGE=get-sybers/signatures:latest test/contract_test.sh    # reuse a built image
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
contract="$tool_dir/contract.yml"
command -v python3 >/dev/null 2>&1 || { echo "contract_test: python3 is required" >&2; exit 2; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/staged" "$scratch/pcaps" "$scratch/out-yara" "$scratch/out-suricata" "$scratch/work"
cp -R "$here/fixtures/staged/." "$scratch/staged/"
cp -R "$here/fixtures/pcaps/." "$scratch/pcaps/"
chmod -R a+rX "$scratch/staged" "$scratch/pcaps"; chmod 777 "$scratch/out-yara" "$scratch/out-suricata" "$scratch/work"

IMAGE="${IMAGE:-contract-test/signatures:latest}"
if [[ -z "${IMAGE_PREBUILT:-}" && "$IMAGE" == contract-test/* ]]; then
    echo "== building $IMAGE (context $(basename "$repo"))"
    docker build -q -t "$IMAGE" -f "$tool_dir/Dockerfile" "$repo" >/dev/null
fi

# run <subtool> <in> <out> <envfile> <stdout> <stderr>
run() {
    local sub="$1" in="$2" out="$3" envfile="$4" so="$5" se="$6" rc=0
    docker run --rm --network none --read-only \
        --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 --tmpfs /tmp:rw,uid=2000,gid=2000 \
        --tmpfs /var/run/suricata:rw,nosuid,nodev,uid=2000,gid=2000 --tmpfs /var/log/suricata:rw,nosuid,nodev,uid=2000,gid=2000 \
        --cap-drop ALL --security-opt no-new-privileges \
        -v "$in:/input:ro" -v "$out:/output" --env-file "$envfile" "$IMAGE" $sub >"$so" 2>"$se" || rc=$?
    return "$rc"
}

# check <stdout> <expected exit> <actual exit> <expected status|"">
check() {
    python3 - "$@" "$contract" <<'PY'
import json, re, sys
out, want, got, status, contract = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4], sys.argv[5]
raw = open(out).read(); lines = raw.splitlines()
if len(lines) != 1 or not raw.endswith("\n"):
    sys.exit(f"FAIL stdout must be exactly one line, got {len(lines)}: {raw!r}")
s = json.loads(lines[0])
m = re.search(r"summary_schema:\s*\n\s*required:\s*\[(.*?)\]", open(contract).read(), re.S)
required = [k.strip() for k in m.group(1).split(",")]
missing = [k for k in required if k not in s and not (k == "subtool" and s.get("status") == "config_error")]
if missing: sys.exit(f"FAIL summary lacks keys {missing}: {s}")
if got != want: sys.exit(f"FAIL exit {got}, want {want}: {s}")
if s.get("exit") != got: sys.exit(f"FAIL summary.exit != process exit: {s}")
if status and s.get("status") != status: sys.exit(f"FAIL status {s.get('status')!r}, want {status!r}")
print(json.dumps({k: s[k] for k in ("subtool", "status", "inputs", "processed", "skipped", "failed", "records", "exit") if k in s}))
PY
}

idempotent() {
    python3 - "$1" <<'PY' || exit 1
import json, sys
s = json.load(open(sys.argv[1]))
if s["skipped"] != s["inputs"] or s["processed"] != 0:
    sys.exit(f"FAIL rerun must skip every item: {s}")
PY
}

for sub in yara suricata; do
    P="SIGNATURES_$(printf '%s' "$sub" | tr '[:lower:]' '[:upper:]')"
    envfile="$scratch/env-$sub"
    printf '%s_INPUT_DIR=/input\n%s_OUT_DIR=/output\n%s_WORK_DIR=/work\n%s_FORCE=0\n%s_LOG_LEVEL=info\n' "$P" "$P" "$P" "$P" "$P" >"$envfile"
    case "$sub" in
        yara)     in="$scratch/staged"; out="$scratch/out-yara" ;;
        suricata) in="$scratch/pcaps";  out="$scratch/out-suricata" ;;
    esac
    echo "== $sub: batch run, expect exit 0"
    rc=0; run "$sub" "$in" "$out" "$envfile" "$scratch/$sub.out1" "$scratch/$sub.err1" || rc=$?
    check "$scratch/$sub.out1" 0 "$rc" ok || { cat "$scratch/$sub.err1" >&2; exit 1; }
    before="$(cd "$out" && find . -type f | sort)"
    echo "== $sub: rerun, idempotent"
    rc=0; run "$sub" "$in" "$out" "$envfile" "$scratch/$sub.out2" "$scratch/$sub.err2" || rc=$?
    check "$scratch/$sub.out2" 0 "$rc" ok || { cat "$scratch/$sub.err2" >&2; exit 1; }
    idempotent "$scratch/$sub.out2"
    after="$(cd "$out" && find . -type f | sort)"
    [[ "$before" == "$after" ]] || { echo "FAIL rerun changed the output tree" >&2; exit 1; }
done

echo "== yara: bad environment (missing input mount), expect exit 2"
sed 's|^SIGNATURES_YARA_INPUT_DIR=.*|SIGNATURES_YARA_INPUT_DIR=/does-not-exist|' "$scratch/env-yara" >"$scratch/env-bad"
rc=0; run yara "$scratch/staged" "$scratch/out-yara" "$scratch/env-bad" "$scratch/bad.out" "$scratch/bad.err" || rc=$?
check "$scratch/bad.out" 2 "$rc" config_error || { cat "$scratch/bad.err" >&2; exit 1; }

echo "== no sub-tool named, expect exit 2"
rc=0; run "" "$scratch/staged" "$scratch/out-yara" "$scratch/env-yara" "$scratch/none.out" "$scratch/none.err" || rc=$?
check "$scratch/none.out" 2 "$rc" config_error || { cat "$scratch/none.err" >&2; exit 1; }

echo "PASS signatures contract test"
