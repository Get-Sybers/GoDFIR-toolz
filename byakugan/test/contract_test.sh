#!/usr/bin/env bash
# test/contract_test.sh — the framework conformance smoke test for the byakugan
# multi-tool image (docs/framework 06.5). Processed evidence cannot be
# committed, so the fixture set is empty: the test builds the image (unless
# IMAGE is given), runs `build` over an empty processed tree and asserts the
# nothing-to-do exit 1 with one JSON summary line carrying every summary_schema
# key, reruns it (same result, no files), runs with a bad environment
# (missing input mount) asserting exit 2, runs with no sub-tool named asserting
# exit 2, and runs `car-vocab` asserting exit 0 and one JSON line.
#
#   test/contract_test.sh                                   # docker build + run
#   IMAGE=get-sybers/byakugan:latest test/contract_test.sh      # reuse a built image
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
contract="$tool_dir/contract.yml"
command -v python3 >/dev/null 2>&1 || { echo "contract_test: python3 is required" >&2; exit 2; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
input="$scratch/input"; output="$scratch/output"
mkdir -p "$input" "$output"
chmod 777 "$output"

IMAGE="${IMAGE:-contract-test/byakugan:latest}"
if [[ -z "${IMAGE_PREBUILT:-}" && "$IMAGE" == contract-test/* ]]; then
    echo "== building $IMAGE (context $(basename "$repo"))"
    docker build -q -t "$IMAGE" -f "$tool_dir/Dockerfile" "$repo" >/dev/null
fi

run() {
    local sub="$1" envfile="$2" so="$3" se="$4" rc=0
    docker run --rm --network none --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
        --cap-drop ALL --security-opt no-new-privileges \
        -v "$input:/input:ro" -v "$output:/output" --env-file "$envfile" "$IMAGE" $sub >"$so" 2>"$se" || rc=$?
    return "$rc"
}

check() {
    python3 - "$@" "$contract" <<'PY'
import json, re, sys
out, want, got, status, contract = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4], sys.argv[5]
raw = open(out).read(); lines = raw.splitlines()
if len(lines) != 1 or not raw.endswith("\n"):
    sys.exit(f"FAIL stdout must be exactly one line, got {len(lines)}: {raw!r}")
s = json.loads(lines[0])
if status == "vocab":
    if not isinstance(s, dict) or not s: sys.exit(f"FAIL car-vocab must print a non-empty object: {raw[:200]!r}")
    if got != want: sys.exit(f"FAIL exit {got}, want {want}")
    print(json.dumps({"objects": len(s)})); sys.exit(0)
m = re.search(r"summary_schema:\s*\n\s*required:\s*\[(.*?)\]", open(contract).read(), re.S)
required = [k.strip() for k in m.group(1).split(",")]
missing = [k for k in required if k not in s and not (status == "config_error" and k in ("subtool", "engine_ref", "inputs", "processed", "skipped", "failed", "records", "outputs", "started", "duration_s"))]
if missing: sys.exit(f"FAIL summary lacks keys {missing}: {s}")
if got != want: sys.exit(f"FAIL exit {got}, want {want}: {s}")
if s.get("exit") != got: sys.exit(f"FAIL summary.exit != process exit: {s}")
if status and s.get("status") != status: sys.exit(f"FAIL status {s.get('status')!r}, want {status!r}")
print(json.dumps({k: s[k] for k in ("subtool", "status", "inputs", "processed", "skipped", "failed", "exit") if k in s}))
PY
}

envfile="$scratch/env"
printf 'BYAKUGAN_BUILD_INPUT_DIR=/input\nBYAKUGAN_BUILD_OUT_DIR=/output\nBYAKUGAN_BUILD_FORCE=0\nBYAKUGAN_BUILD_LOG_LEVEL=info\n' >"$envfile"

echo "== build: empty processed tree, expect exit 1 (nothing)"
rc=0; run build "$envfile" "$scratch/out1" "$scratch/err1" || rc=$?
check "$scratch/out1" 1 "$rc" nothing || { cat "$scratch/err1" >&2; exit 1; }
before="$(cd "$output" && find . -type f | sort)"
echo "== build: rerun, idempotent"
rc=0; run build "$envfile" "$scratch/out2" "$scratch/err2" || rc=$?
check "$scratch/out2" 1 "$rc" nothing || { cat "$scratch/err2" >&2; exit 1; }
[[ "$before" == "$(cd "$output" && find . -type f | sort)" ]] || { echo "FAIL rerun changed the output tree" >&2; exit 1; }

echo "== build: bad environment (missing input mount), expect exit 2"
sed 's|^BYAKUGAN_BUILD_INPUT_DIR=.*|BYAKUGAN_BUILD_INPUT_DIR=/does-not-exist|' "$envfile" >"$scratch/env-bad"
rc=0; run build "$scratch/env-bad" "$scratch/out3" "$scratch/err3" || rc=$?
check "$scratch/out3" 2 "$rc" config_error || { cat "$scratch/err3" >&2; exit 1; }

echo "== no sub-tool named, expect exit 2"
rc=0; run "" "$envfile" "$scratch/out4" "$scratch/err4" || rc=$?
check "$scratch/out4" 2 "$rc" config_error || { cat "$scratch/err4" >&2; exit 1; }

echo "== car-vocab: one JSON line, exit 0"
rc=0; run car-vocab "$envfile" "$scratch/out5" "$scratch/err5" || rc=$?
check "$scratch/out5" 0 "$rc" vocab || { cat "$scratch/err5" >&2; exit 1; }

echo "PASS byakugan contract test"
