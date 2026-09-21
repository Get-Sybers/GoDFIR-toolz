#!/usr/bin/env bash
# Open (or update) the PR that moves ONE engine pin — the ARG <TOOL>_REF
# default in <tool>/Dockerfile — to the commit just merged on the engine's
# main. Run by the engine repos' bump-godfir-pin workflows with THIS repo
# checked out on main; deterministic, no operator input. The PR carries the
# source merge's own PR title and body (falling back to the commit message),
# and this repo's PR checks (conform --build + cve-scan) gate it.
#
#   usage: SRC_REPO=<owner/engine-repo> SRC_SHA=<40-hex> GH_TOKEN=<token> \
#          bump-engine-pin.sh <tool>
set -euo pipefail

tool="${1:?usage: bump-engine-pin.sh <tool>}"
: "${SRC_REPO:?SRC_REPO (owner/repo of the engine) is required}"
: "${SRC_SHA:?SRC_SHA (the merged engine commit) is required}"
[[ "$SRC_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "SRC_SHA is not a 40-hex sha: $SRC_SHA" >&2; exit 2; }

arg="$(printf '%s' "$tool" | tr '[:lower:]-' '[:upper:]_')_REF"
dockerfile="$tool/Dockerfile"

grep -qE "^ARG ${arg}=[0-9a-f]{40}$" "$dockerfile" \
  || { echo "no 40-hex ARG ${arg} default in $dockerfile" >&2; exit 2; }
if grep -q "^ARG ${arg}=${SRC_SHA}$" "$dockerfile"; then
  echo "pin already at ${SRC_SHA}"
  exit 0
fi
sed -i -E "s/^ARG ${arg}=[0-9a-f]{40}$/ARG ${arg}=${SRC_SHA}/" "$dockerfile"

# An engine bump is an image change: bump the image's patch version so
# org.opencontainers.image.version distinguishes the builds.
ver="$(sed -nE 's/^ARG TOOL_VERSION=([0-9]+)\.([0-9]+)\.([0-9]+)$/\1 \2 \3/p' "$dockerfile")"
if [ -n "$ver" ]; then
  read -r maj min pat <<<"$ver"
  newver="${maj}.${min}.$((pat + 1))"
  sed -i -E "s/^ARG TOOL_VERSION=[0-9]+\.[0-9]+\.[0-9]+$/ARG TOOL_VERSION=${newver}/" "$dockerfile"
  echo "TOOL_VERSION -> ${newver}"
fi

# The source merge's own PR message is the bump's message. Reads of the engine
# repo use SRC_GH_TOKEN when set (a public engine needs none beyond GH_TOKEN;
# a private engine's workflow passes a token that can read it).
src_api() { GH_TOKEN="${SRC_GH_TOKEN:-$GH_TOKEN}" gh api "$@"; }
title="$(src_api "repos/${SRC_REPO}/commits/${SRC_SHA}/pulls" --jq '.[0].title // empty')"
body="$(src_api "repos/${SRC_REPO}/commits/${SRC_SHA}/pulls" --jq '.[0].body // empty')"
if [ -z "$title" ]; then
  msg="$(src_api "repos/${SRC_REPO}/commits/${SRC_SHA}" --jq '.commit.message')"
  title="${msg%%$'\n'*}"
  body="${msg#*$'\n'}"
  [ "$body" = "$msg" ] && body=""
fi

branch="pin/${tool}-${SRC_SHA:0:7}"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git switch -C "$branch"
git commit -am "${tool}: engine -> ${SRC_SHA:0:7}${ver:+ (image ${newver})}

${title}
(${SRC_REPO}@${SRC_SHA})"
git push -f origin "$branch"

open="$(gh pr list --head "$branch" --base main --state open --json number --jq 'length')"
if [ "$open" = "0" ]; then
  gh pr create --base main --head "$branch" --title "$title" --body "${body}

---
Pin bump: \`${tool}\` engine → ${SRC_REPO}@${SRC_SHA} — automated from the merge to the engine's main; message carried over from its PR."
else
  echo "PR for ${branch} already open — updated by the push"
fi
