#!/usr/bin/env bash
# test/contract_test.sh — the framework conformance smoke test for the
# anamnesis image (docs/framework 06.5). A memory image cannot be committed, so
# the fixture set is empty: the test builds the image (unless IMAGE is given),
# runs it in batch mode over an empty input dir and asserts the exit code and
# that stdout is exactly one JSON line carrying every summary_schema key,
# reruns to assert idempotency (no files appear), runs with a bad
# environment (missing input mount) and asserts exit 2, and asserts the baked
# PDB symbol cache ships in the image (the engine is always offline; the
# first build downloads + seeds from the pinned memory image, so it is slow
# and needs network + ~6.5 GB of transient build disk).
#
#   test/contract_test.sh                                  # docker build + run
#   IMAGE=get-sybers/anamnesis:latest test/contract_test.sh    # reuse a built image
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
contract="$tool_dir/contract.yml"
command -v python3 >/dev/null 2>&1 || { echo "contract_test: python3 is required" >&2; exit 2; }

scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
input="$scratch/input"; out="$scratch/out"
mkdir -p "$input" "$out"
if [[ -d "$here/fixtures" ]]; then cp -R "$here/fixtures/." "$input/"; fi
rm -f "$input/.keep"
chmod -R a+rX "$input"; chmod 777 "$out"
expect=0   # the engine reports an empty input dir as nothing-to-do with exit 0

IMAGE="${IMAGE:-contract-test/anamnesis:latest}"
if [[ -z "${IMAGE_PREBUILT:-}" && "$IMAGE" == contract-test/* ]]; then
    echo "== building $IMAGE (context $(basename "$repo"))"
    docker build -q -t "$IMAGE" -f "$tool_dir/Dockerfile" "$repo" >/dev/null
fi

run() {
    local envfile="$1" so="$2" se="$3" rc=0
    docker run --rm --network none --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
        --cap-drop ALL --security-opt no-new-privileges \
        -v "$input:/input:ro" -v "$out:/out" \
        --env-file "$envfile" "$IMAGE" >"$so" 2>"$se" || rc=$?
    return "$rc"
}

check() {
    python3 - "$@" "$contract" <<'PY'
import json, re, sys
out, want, got, contract = sys.argv[1], int(sys.argv[2]), int(sys.argv[3]), sys.argv[4]
raw = open(out).read(); lines = raw.splitlines()
if len(lines) != 1 or not raw.endswith("\n"):
    sys.exit(f"FAIL stdout must be exactly one line, got {len(lines)}: {raw!r}")
s = json.loads(lines[0])
m = re.search(r"summary_schema:\s*\n\s*required:\s*\[(.*?)\]", open(contract).read(), re.S)
required = [k.strip() for k in m.group(1).split(",")]
missing = [k for k in required if k not in s]
if missing: sys.exit(f"FAIL summary lacks keys {missing}: {s}")
if got != want: sys.exit(f"FAIL exit {got}, want {want}: {s}")
print(json.dumps({k: s[k] for k in ("tool", "images", "processed", "skipped", "failed") if k in s}))
PY
}

envfile="$scratch/env"
printf 'ANAMNESIS_INPUT_DIR=/input\nANAMNESIS_OUT_DIR=/out\nANAMNESIS_FORCE=0\n' >"$envfile"

echo "== 1. batch run over the input dir, expect exit $expect"
rc=0; run "$envfile" "$scratch/out1" "$scratch/err1" || rc=$?
check "$scratch/out1" "$expect" "$rc" || { cat "$scratch/err1" >&2; exit 1; }
before="$(cd "$out" && find . -type f | sort)"

echo "== 2. rerun: idempotent"
rc=0; run "$envfile" "$scratch/out2" "$scratch/err2" || rc=$?
check "$scratch/out2" "$expect" "$rc" || { cat "$scratch/err2" >&2; exit 1; }
after="$(cd "$out" && find . -type f | sort)"
[[ "$before" == "$after" ]] || { echo "FAIL rerun changed the output tree" >&2; exit 1; }

echo "== 3. bad environment: missing input mount, expect exit 2"
sed -e 's|^ANAMNESIS_INPUT_DIR=.*|ANAMNESIS_INPUT_DIR=/does-not-exist|' \
    "$envfile" >"$scratch/env-bad"
rc=0; run "$scratch/env-bad" "$scratch/out3" "$scratch/err3" || rc=$?
check "$scratch/out3" 2 "$rc" || { cat "$scratch/err3" >&2; exit 1; }
grep -q '"error"' "$scratch/out3" || { echo "FAIL config error summary lacks an error key" >&2; exit 1; }

echo "== 4. the baked PDB symbol cache ships in the image (the engine is always offline)"
cid="$(docker create "$IMAGE")"
# tar member names may or may not carry a ./ prefix depending on the archiver.
docker export "$cid" | tar -t | grep -Eq '^(\./)?opt/anamnesis/lib/Symbols/.+\.pdb$' \
    || { echo "FAIL image ships no baked PDB under /opt/anamnesis/lib/Symbols" >&2; docker rm -f "$cid" >/dev/null; exit 1; }
docker rm -f "$cid" >/dev/null

echo "PASS anamnesis contract test"
