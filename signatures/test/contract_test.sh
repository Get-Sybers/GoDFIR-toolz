#!/usr/bin/env bash
# The framework conformance smoke test for the signatures multi-tool image
# (docs/framework 06.5): yara/suricata/hayabusa each over their fixtures,
# plus idempotency and the config-error exits. `scan` needs an NTFS image
# and is covered by the Go unit tests with a stub pipe. IMAGE=<ref> reuses
# a built image.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
contract="$tool_dir/contract.yml"
command -v python3 >/dev/null 2>&1 || { echo "contract_test: python3 is required" >&2; exit 2; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
mkdir -p "$scratch/staged" "$scratch/pcaps" "$scratch/evtx" "$scratch/out-yara" "$scratch/out-suricata" "$scratch/out-hayabusa" "$scratch/work"
cp -R "$here/fixtures/staged/." "$scratch/staged/"
cp -R "$here/fixtures/pcaps/." "$scratch/pcaps/"
cp -R "$here/fixtures/evtx/." "$scratch/evtx/"
chmod -R a+rX "$scratch/staged" "$scratch/pcaps" "$scratch/evtx"
chmod 777 "$scratch/out-yara" "$scratch/out-suricata" "$scratch/out-hayabusa" "$scratch/work"

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

# timeline <summary> <item dir>: the one fixture item was processed with
# detections, its timeline.jsonl holds exactly the records the summary counts
# (one JSON detection record per line) and its hayabusa.jsonl index is beside it
timeline() {
    python3 - "$1" "$2" <<'PY' || exit 1
import json, os, sys
s = json.load(open(sys.argv[1])); item = sys.argv[2]
if s["inputs"] != 1 or s["processed"] != 1 or s["records"] < 1:
    sys.exit(f"FAIL hayabusa must process the one fixture item with detections: {s}")
timeline = os.path.join(item, "timeline.jsonl")
lines = [l for l in open(timeline).read().splitlines() if l.strip()]
if len(lines) != s["records"]:
    sys.exit(f"FAIL {timeline} has {len(lines)} lines, the summary counts {s['records']} records")
for l in lines:
    d = json.loads(l)
    if not isinstance(d, dict) or not d.get("RuleTitle") or not d.get("Timestamp"):
        sys.exit(f"FAIL not a detection record: {l[:200]}")
idx = json.loads(open(os.path.join(item, "hayabusa.jsonl")).readline())
if idx.get("timeline") != "timeline.jsonl" or idx.get("records") != s["records"]:
    sys.exit(f"FAIL hayabusa.jsonl index {idx} does not match the summary {s}")
print(f"hayabusa: {len(lines)} detections in {timeline}")
PY
}

for sub in yara suricata hayabusa; do
    P="SIGNATURES_$(printf '%s' "$sub" | tr '[:lower:]' '[:upper:]')"
    envfile="$scratch/env-$sub"
    printf '%s_INPUT_DIR=/input\n%s_OUT_DIR=/output\n%s_WORK_DIR=/work\n%s_FORCE=0\n%s_LOG_LEVEL=info\n' "$P" "$P" "$P" "$P" "$P" >"$envfile"
    case "$sub" in
        yara)     in="$scratch/staged"; out="$scratch/out-yara" ;;
        suricata) in="$scratch/pcaps";  out="$scratch/out-suricata" ;;
        hayabusa) in="$scratch/evtx";   out="$scratch/out-hayabusa" ;;
    esac
    echo "== $sub: batch run, expect exit 0"
    rc=0; run "$sub" "$in" "$out" "$envfile" "$scratch/$sub.out1" "$scratch/$sub.err1" || rc=$?
    check "$scratch/$sub.out1" 0 "$rc" ok || { cat "$scratch/$sub.err1" >&2; exit 1; }
    [[ "$sub" != hayabusa ]] || timeline "$scratch/$sub.out1" "$out/hostA"
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
