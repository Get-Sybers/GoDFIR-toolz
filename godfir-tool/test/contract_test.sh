#!/usr/bin/env bash
# The framework conformance smoke test for the godfir-tool recipe — its
# images are declared argv deviations, so it asserts the hardening contract
# and that the entrypoint runs, not a batch summary line. GODFIR_TOOL=<name>
# picks the tool (default SQLECmd); IMAGE=<ref> reuses a built image.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tool_dir="$(dirname "$here")"
repo="$(dirname "$tool_dir")"
GODFIR_TOOL="${GODFIR_TOOL:-SQLECmd}"
name="$(printf '%s' "$GODFIR_TOOL" | tr '[:upper:]' '[:lower:]')"
IMAGE="${IMAGE:-contract-test/$name:latest}"
if [[ -z "${IMAGE_PREBUILT:-}" && "$IMAGE" == contract-test/* ]]; then
    echo "== building $IMAGE (GODFIR_TOOL=$GODFIR_TOOL, context $(basename "$repo"))"
    docker build -q -t "$IMAGE" --build-arg "GODFIR_TOOL=$GODFIR_TOOL" -f "$tool_dir/Dockerfile" "$repo" >/dev/null
fi
scratch="$(mktemp -d)"; trap 'rm -rf "$scratch"' EXIT

echo "== 1. built image identity and labels"
usr="$(docker image inspect -f '{{.Config.User}}' "$IMAGE")"
[[ "$usr" == "2000:2000" ]] || { echo "FAIL USER=$usr" >&2; exit 1; }
for L in com.get-sybers.tool com.get-sybers.godfir-tool.name com.get-sybers.hardened com.get-sybers.contract \
         org.opencontainers.image.version org.opencontainers.image.revision com.get-sybers.godfir-release; do
    v="$(docker image inspect -f "{{index .Config.Labels \"$L\"}}" "$IMAGE")"
    [[ -n "$v" && "$v" != "<no value>" ]] || { echo "FAIL label $L missing" >&2; exit 1; }
done
[[ "$(docker image inspect -f '{{index .Config.Labels "com.get-sybers.godfir-tool.name"}}' "$IMAGE")" == "$GODFIR_TOOL" ]] \
    || { echo "FAIL godfir-tool.name label" >&2; exit 1; }

echo "== 2. self-declaration vs filesystem"
cid="$(docker create "$IMAGE" __export__)"
docker export "$cid" | tar -t > "$scratch/fs"
docker export "$cid" | tar -xO etc/dfir-hardened > "$scratch/decl"
docker rm -f "$cid" >/dev/null
grep -q '^schema=1$' "$scratch/decl" && grep -q '^tool=godfir-tool$' "$scratch/decl" && grep -q 'shell=false python=false pkg_mgr=false' "$scratch/decl" \
    || { echo "FAIL declaration:"; cat "$scratch/decl"; exit 1; } >&2
grep -qE '(^|/)bin/(sh|bash|dash)$' "$scratch/fs" && { echo "FAIL shell present" >&2; exit 1; }
grep -qE '(^|/)bin/python3' "$scratch/fs" && { echo "FAIL python present" >&2; exit 1; }
grep -qE '(^|/)(usr/)?bin/(apt-get|dpkg|sudo)$' "$scratch/fs" && { echo "FAIL apt/dpkg/sudo present" >&2; exit 1; }
grep -qE '^opt/godfir-tool/tool\.dll$' "$scratch/fs" || { echo "FAIL tool.dll missing" >&2; exit 1; }

echo "== 3. the entrypoint runs (--help)"
rc=0; docker run --rm --network none --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
    --cap-drop ALL --security-opt no-new-privileges "$IMAGE" --help >"$scratch/o" 2>"$scratch/e" || rc=$?
[[ "$rc" -lt 100 ]] || { echo "FAIL --help exited $rc (runtime crash)" >&2; cat "$scratch/e" >&2; exit 1; }
[[ -s "$scratch/o" || -s "$scratch/e" ]] || { echo "FAIL --help printed nothing" >&2; exit 1; }

echo "PASS godfir-tool ($GODFIR_TOOL) contract test"
