#!/usr/bin/env bash
# gocron contract smoke (docs/framework 02 §2.3, 06 §6.2): runs the tool in
# batch mode over generated fixtures and asserts the exit codes, that stdout
# is exactly one JSON summary line, idempotency, and the config-error exit.
#
# Default: builds and runs the hardened image (needs docker; run from the
# repo root, the image copies pinfo/). CONTRACT_LOCAL=1 builds the binary
# with the host Go toolchain instead — the same entrypoint, no container.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

in="$work/in" out="$work/out" scratch="$work/scratch"
mkdir -p "$in" "$out" "$scratch"
(cd "$tool_dir" && go run test/fixtures/gen.go "$in")

run() { # run [extra env...] -> stdout captured, exit code in $code
    local envs=("$@") code=0
    if [[ "${CONTRACT_LOCAL:-0}" == "1" ]]; then
        env "${envs[@]}" "$work/gocron" >"$work/stdout" 2>"$work/stderr" || code=$?
    else
        local docker_env=()
        for e in "${envs[@]}"; do docker_env+=(-e "$e"); done
        docker run --rm --network none --read-only --cap-drop ALL \
            --security-opt no-new-privileges \
            --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
            -v "$in:/input:ro" -v "$out:/output" "${docker_env[@]}" \
            get-sybers/gocron:latest >"$work/stdout" 2>"$work/stderr" || code=$?
    fi
    return $code
}

if [[ "${CONTRACT_LOCAL:-0}" == "1" ]]; then
    (cd "$tool_dir" && CGO_ENABLED=0 go build -o "$work/gocron" .)
    export GOCRON_INPUT_DIR="$in" GOCRON_OUT_DIR="$out" GOCRON_WORK_DIR="$scratch"
    envs=("GOCRON_INPUT_DIR=$in" "GOCRON_OUT_DIR=$out" "GOCRON_WORK_DIR=$scratch")
else
    (cd "$repo" && docker build -q -t get-sybers/gocron:latest -f gocron/Dockerfile .)
    chmod 777 "$out"
    envs=()
fi

fail() { echo "FAIL: $*" >&2; cat "$work/stderr" >&2 || true; exit 1; }
summary() { python3 -c "import json,sys; s=json.load(open('$work/stdout')); print(s['$1'])"; }

# 1. batch run: exit 0, one JSON line, records produced
code=0; run "${envs[@]}" || code=$?
[[ $code -eq 0 ]] || fail "batch exit $code, want 0"
[[ "$(wc -l <"$work/stdout")" == "1" ]] || fail "stdout is not one line"
[[ "$(summary status)" == "ok" ]] || fail "status $(summary status)"
[[ "$(summary records)" == "4" ]] || fail "records $(summary records)"
ls "$out"/*/gocron.jsonl >/dev/null 2>&1 || fail "record file missing"

# 2. rerun converges: everything skipped, still exit 0
code=0; run "${envs[@]}" || code=$?
[[ $code -eq 0 && "$(summary skipped)" == "3" ]] || fail "rerun not idempotent"

# 3. config error: missing input dir -> exit 2
code=0; run "${envs[@]/GOCRON_INPUT_DIR=$in/GOCRON_INPUT_DIR=$in/missing}" GOCRON_INPUT_DIR="$in/missing" || code=$?
[[ $code -eq 2 ]] || fail "config-error exit $code, want 2"

echo "gocron contract smoke: PASS"
