#!/usr/bin/env bash
# test/contract_test.sh — the framework conformance smoke test for the byakugan
# multi-tool image (docs/framework 06.5). Processed evidence cannot be
# committed, so the fixture set is empty: the test builds the image (unless
# IMAGE is given), runs `build` over an empty processed tree and asserts the
# nothing-to-do exit 1 with one JSON summary line carrying every summary_schema
# key, reruns it (same result, no files), runs with a bad environment
# (missing input mount) asserting exit 2, runs with no sub-tool named asserting
# exit 2, runs `car-vocab` asserting exit 0 and one JSON line, runs `verify`
# over an empty car tree asserting the nothing-to-do exit 1 with no report, and
# runs `load` (bundle mode, no BYAKUGAN_LOAD_ES_URL — never a network) over an
# empty car tree asserting the nothing-to-do exit 1 with subtool "load", plus a
# bad environment (bogus log level) asserting exit 2. `load`'s success path
# (a materialised car tree -> elastic/ bundles) is not exercised here: the
# engine-side sub-tool is not implemented yet (a separate phase), and the
# car_<object>.jsonl row shape is owned by the externally-cloned engine, not
# this repo, so no fixture can be authored against it in advance.
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

# check_field <stdout> <summary key> <expected value>: the summary line's one
# JSON object carries <key> == <expected value> (string-compared).
check_field() {
    python3 - "$@" <<'PY'
import json, sys
out, key, want = sys.argv[1], sys.argv[2], sys.argv[3]
s = json.loads(open(out).read().splitlines()[0])
got = s.get(key)
if str(got) != want:
    sys.exit(f"FAIL {key} = {got!r}, want {want!r}: {s}")
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

echo "== verify: empty car tree, expect exit 1 (nothing), no report written"
printf 'BYAKUGAN_VERIFY_INPUT_DIR=/input\nBYAKUGAN_VERIFY_OUT_DIR=/output\nBYAKUGAN_VERIFY_LOG_LEVEL=info\n' >"$scratch/env-verify"
rc=0; run verify "$scratch/env-verify" "$scratch/out6" "$scratch/err6" || rc=$?
check "$scratch/out6" 1 "$rc" nothing || { cat "$scratch/err6" >&2; exit 1; }
[[ ! -e "$output/verify.txt" ]] || { echo "FAIL verify wrote a report over an empty tree" >&2; exit 1; }

echo "== load: bundle mode (no BYAKUGAN_LOAD_ES_URL), empty car tree, expect exit 1 (nothing)"
printf 'BYAKUGAN_LOAD_INPUT_DIR=/input\nBYAKUGAN_LOAD_OUT_DIR=/output\nBYAKUGAN_LOAD_NAMESPACE=default\nBYAKUGAN_LOAD_LOG_LEVEL=info\n' >"$scratch/env-load"
rc=0; run load "$scratch/env-load" "$scratch/out7" "$scratch/err7" || rc=$?
check "$scratch/out7" 1 "$rc" nothing || { cat "$scratch/err7" >&2; exit 1; }
check_field "$scratch/out7" subtool load || { cat "$scratch/err7" >&2; exit 1; }
[[ ! -e "$output/elastic" ]] || { echo "FAIL load wrote elastic/ over an empty tree" >&2; exit 1; }

echo "== load: bad environment (bogus log level), expect exit 2"
sed 's|^BYAKUGAN_LOAD_LOG_LEVEL=.*|BYAKUGAN_LOAD_LOG_LEVEL=bogus|' "$scratch/env-load" >"$scratch/env-load-bad"
rc=0; run load "$scratch/env-load-bad" "$scratch/out8" "$scratch/err8" || rc=$?
check "$scratch/out8" 2 "$rc" config_error || { cat "$scratch/err8" >&2; exit 1; }

echo "PASS byakugan contract test"
