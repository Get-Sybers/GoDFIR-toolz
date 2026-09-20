#!/usr/bin/env bash
# test/contract_test.sh — the framework conformance smoke test for this tool
# (docs/framework 06.5 / 09.5). It runs the image over test/fixtures/ in batch
# mode with the environment contract set and asserts:
#   1. the exit code (0 when the fixtures hold at least one item, else 1);
#   2. stdout is exactly one line, JSON, carrying every summary_schema key;
#   3. a rerun is idempotent: same exit, every item skipped, no new files;
#   4. a bad environment (missing input mount) exits 2 / status config_error.
#
#   test/contract_test.sh                              # docker build + run
#   IMAGE=get-sybers/<tool>:latest test/contract_test.sh   # reuse a built image
#   GODFIR_BIN=/path/to/<tool> test/contract_test.sh       # run a host binary
#
# This file is shared verbatim by every self-orchestrating tool; the tool name
# and its variable prefix derive from the directory it sits in.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
tool="$(basename "$tool_dir")"
prefix="$(printf '%s' "$tool" | tr '[:lower:]-' '[:upper:]_')"
contract="$tool_dir/contract.yml"
have_python() { command -v python3 >/dev/null 2>&1; }
have_python || { echo "contract_test: python3 is required" >&2; exit 2; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
# The fixtures are copied so the container's uid 2000 can read them whatever
# the checkout's permissions are; the output root is world-writable for the
# same reason.
input="$scratch/input"; output="$scratch/output"; work="$scratch/work"
mkdir -p "$input" "$output" "$work"
if [[ -d "$here/fixtures" ]]; then cp -R "$here/fixtures/." "$input/"; fi
rm -f "$input/.keep"
chmod -R a+rX "$input"; chmod 777 "$output" "$work"
items="$(find "$input" -type f | wc -l)"
expect=0; [[ "$items" -gt 0 ]] || expect=1
[[ -n "${EXPECT_EXIT:-}" ]] && expect="$EXPECT_EXIT"

# run <envfile> <stdout> <stderr>: one batch invocation; returns its exit code.
run() {
    local envfile="$1" out="$2" err="$3" rc=0
    if [[ -n "${GODFIR_BIN:-}" ]]; then
        (set -a; . "$envfile"; set +a; "$GODFIR_BIN") >"$out" 2>"$err" || rc=$?
    else
        docker run --rm --network none --read-only \
            --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 --tmpfs /tmp:rw,uid=2000,gid=2000 \
            --cap-drop ALL --security-opt no-new-privileges \
            -v "$input:/input:ro" -v "$output:/output" \
            --env-file "$envfile" "$IMAGE" >"$out" 2>"$err" || rc=$?
    fi
    return "$rc"
}

# check <stdout> <expected exit> <actual exit> [<expected status>]
check() {
    python3 - "$@" "$contract" <<'PY'
import json, re, sys
out, want, got, status, contract = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4], sys.argv[5]
raw = open(out).read()
lines = raw.splitlines()
if len(lines) != 1 or not raw.endswith("\n"):
    sys.exit(f"FAIL stdout must be exactly one line, got {len(lines)}: {raw!r}")
try:
    summary = json.loads(lines[0])
except Exception as e:
    sys.exit(f"FAIL stdout is not JSON: {e}: {raw!r}")
m = re.search(r"summary_schema:\s*\n\s*required:\s*\[(.*?)\]", open(contract).read(), re.S)
required = [k.strip() for k in m.group(1).split(",")] if m else ["tool", "version", "status", "exit"]
missing = [k for k in required if k not in summary]
if missing:
    sys.exit(f"FAIL summary lacks keys {missing}: {summary}")
if got != want:
    sys.exit(f"FAIL exit {got}, want {want}: {summary}")
if summary.get("exit") != got:
    sys.exit(f"FAIL summary.exit {summary.get('exit')} != process exit {got}")
if status and summary.get("status") != status:
    sys.exit(f"FAIL status {summary.get('status')!r}, want {status!r}")
print(json.dumps({k: summary[k] for k in ("status", "inputs", "processed", "skipped", "failed", "exit") if k in summary}))
PY
}

if [[ -z "${GODFIR_BIN:-}" ]]; then
    IMAGE="${IMAGE:-contract-test/$tool:latest}"
    if [[ -z "${IMAGE_PREBUILT:-}" && "$IMAGE" == contract-test/* ]]; then
        # Build context mirrors build-all.sh: Shape B / harden.yml tools build from
        # the repo root, Shape A Go tools from their own directory.
        ctx="$tool_dir"
        grep -qE 'COPY[[:space:]]+hardening/|harden\.yml' "$tool_dir/Dockerfile" && ctx="$(dirname "$tool_dir")"
        echo "== building $IMAGE (context $(basename "$ctx"))"
        docker build -q -t "$IMAGE" -f "$tool_dir/Dockerfile" "$ctx" >/dev/null
    fi
fi

envfile="$scratch/env"
if [[ -n "${GODFIR_BIN:-}" ]]; then
    printf '%s_INPUT_DIR=%s\n%s_OUT_DIR=%s\n%s_WORK_DIR=%s\n' "$prefix" "$input" "$prefix" "$output" "$prefix" "$work" >"$envfile"
else
    printf '%s_INPUT_DIR=/input\n%s_OUT_DIR=/output\n%s_WORK_DIR=/work\n' "$prefix" "$prefix" "$prefix" >"$envfile"
fi
printf '%s_FORCE=0\n%s_LOG_LEVEL=info\n' "$prefix" "$prefix" >>"$envfile"

echo "== 1. batch run over $items fixture file(s), expect exit $expect"
rc=0; run "$envfile" "$scratch/out1" "$scratch/err1" || rc=$?
check "$scratch/out1" "$expect" "$rc" "" || { cat "$scratch/err1" >&2; exit 1; }
before="$(cd "$output" && find . -type f | sort)"

echo "== 2. rerun: idempotent"
rc=0; run "$envfile" "$scratch/out2" "$scratch/err2" || rc=$?
check "$scratch/out2" "$expect" "$rc" "" || { cat "$scratch/err2" >&2; exit 1; }
after="$(cd "$output" && find . -type f | sort)"
[[ "$before" == "$after" ]] || { echo "FAIL rerun changed the output tree" >&2; diff <(echo "$before") <(echo "$after") >&2; exit 1; }
if [[ "$expect" -eq 0 ]]; then
    python3 - "$scratch/out2" <<'PY' || exit 1
import json, sys
s = json.load(open(sys.argv[1]))
if s["skipped"] != s["inputs"] or s["processed"] != 0:
    sys.exit(f"FAIL rerun must skip every item: {s}")
PY
fi

echo "== 3. bad environment: missing input mount, expect exit 2"
sed "s|^${prefix}_INPUT_DIR=.*|${prefix}_INPUT_DIR=/does-not-exist|" "$envfile" >"$scratch/env-bad"
rc=0; run "$scratch/env-bad" "$scratch/out3" "$scratch/err3" || rc=$?
check "$scratch/out3" 2 "$rc" "config_error" || { cat "$scratch/err3" >&2; exit 1; }

echo "PASS $tool contract test"
