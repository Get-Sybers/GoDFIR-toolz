#!/usr/bin/env bash
# ==============================================================================
# build-all.sh — build the get-sybers/* images from images.yml, STANDALONE ONLY.
#
# This script exists for exactly one situation: this repository cloned by
# itself, with no consumer around. It launches the collection playbook
# (playbooks/build_images.yml → the godfir_build role): the inventory lives in
# images.yml and the build logic in the role; the script only prepares the
# host, forwards names and the optional stamp overrides.
#
# It is NEVER used by another repository. A consumer builds through the
# godfir_build role from its own tooling (DX_DFIR: `dxdfir build-docker`), and
# this script refuses to run from a checkout that is a submodule of another
# repository. Nothing it provisions is shared with a consumer: every pin it
# needs is written HERE, it creates no file a consumer's tooling could pick
# up, and what it installs stays in two gitignored, artifact-ignored trees
# of this checkout —
#   - <repo>/.venv                the pinned controller layer (ansible-core +
#                                 the docker SDK community.docker's modules
#                                 import), created by the invoking user,
#                                 reinstalled only when the pins below change;
#   - <repo>/.ansible/collections the pinned community.docker collection,
#                                 installed only when it does not already
#                                 resolve, put on ANSIBLE_COLLECTIONS_PATH for
#                                 the run (never written to ansible.cfg).
# It then preflights what it cannot install — python3 >= 3.11 with the venv
# module, the docker CLI, a daemon this user may talk to, BuildKit — and
# names the fix on every failure. A host provisioned once builds offline.
#
# Usage:
#   build-all.sh [options] [NAME...]
#
#   NAME...        images to build (case-insensitive; aliases and sub-tool
#                  names resolve to their image). None = every manifest image,
#                  in manifest order.
#
# Options:
#   --preflight    prepare + verify the host, build nothing (exit 0 when ready)
#   -h, --help
#
# Environment:
#   GODFIR_REVISION / GODFIR_RELEASE   override the source stamps
#   <env_args from images.yml>         per-image clone-at-build pins, e.g.
#                                      BYAKUGAN_REF=v1.2 ./build-all.sh byakugan
#   GODFIR_VENV                        the controller venv (default <repo>/.venv)
#   GODFIR_ANSIBLE                     an ansible-playbook to use INSTEAD of the
#                                      venv (a host with its own ansible); its
#                                      python must import docker
#
# Exit: the playbook's exit code · 2 usage / host not ready.
# ==============================================================================
set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)"
cd "$REPO_ROOT"

# ---- the pins: THE one place this script's dependencies are set -------------
# The controller layer, an exact lock (ansible-core itself, the docker SDK +
# requests that community.docker's modules import on the controller, and
# their resolution). Regenerate after a deliberate bump on the OLDEST python
# supported (3.11 — ansible-core 2.19.x installs on 3.11 through 3.13):
#   python3 -m venv /tmp/lock && /tmp/lock/bin/pip install ansible-core requests docker PyYAML \
#     && /tmp/lock/bin/pip freeze
PIP_LOCK='
ansible-core==2.19.13
certifi==2026.7.22
cffi==2.1.1
charset-normalizer==3.5.1
cryptography==50.0.1
docker==7.2.0
idna==3.19
Jinja2==3.1.6
MarkupSafe==3.0.3
packaging==26.3
pycparser==3.0
PyYAML==6.0.3
requests==2.34.2
resolvelib==1.2.1
urllib3==2.7.0
'
# The collection the godfir_build role's modules come from: exact, never a
# range or :latest. It satisfies galaxy.yml's declared dependency range.
COLLECTION_PIN='community.docker:3.10.3'
COLLECTION_NAME='community.docker'

# ---- styling (conform.sh's) --------------------------------------------------
if [[ -t 1 && -z "${NO_COLOR:-}" && "${TERM:-dumb}" != dumb ]]; then
    C_RST=$'\033[0m'; C_OK=$'\033[38;5;172m'; C_ACT=$'\033[38;5;214m'
    C_ERR=$'\033[38;5;167m'; C_DIM=$'\033[38;5;245m'
else C_RST= C_OK= C_ACT= C_ERR= C_DIM=; fi
ok()   { printf '%s[ ok ]%s %s\n' "$C_OK"  "$C_RST" "$*"; }
step() { printf '%s[ >> ]%s %s\n' "$C_ACT" "$C_RST" "$*"; }
note() { printf '       %s%s%s\n' "$C_DIM" "$*" "$C_RST"; }
die()  { printf '%s[fail]%s %s\n' "$C_ERR" "$C_RST" "$1" >&2; shift; for _l in "$@"; do printf '       %s\n' "$_l" >&2; done; exit 2; }
usage() { sed -n '2,52p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-2}"; }

# ---- arguments ---------------------------------------------------------------
PREFLIGHT_ONLY=false
NAMES=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --preflight) PREFLIGHT_ONLY=true ;;
        -h|--help)   usage 0 ;;
        --)          shift; NAMES+=("$@"); break ;;
        -*)          die "unknown option: $1 (see --help)" ;;
        *)           NAMES+=("$1") ;;
    esac
    shift
done

# ---- standalone only ---------------------------------------------------------
# A submodule checkout carries a `.git` FILE whose gitdir points into the
# superproject's .git/modules/ tree: that is another repository's copy of this
# one, and its build goes through that repository's tooling. A worktree carries
# a `.git` file too (gitdir under .git/worktrees/) and is a standalone clone
# like any other, so only the modules/ shape is refused.
if [[ -f "$REPO_ROOT/.git" ]] && grep -qE '^gitdir: .*/\.git/modules/' "$REPO_ROOT/.git" 2>/dev/null; then
    die "this checkout is a submodule of another repository — build-all.sh is standalone-only." \
        "build through that repository's own tooling (DX_DFIR: dxdfir build-docker), which drives the godfir_build role."
fi

# ---- host tools the script cannot install -----------------------------------
# apt names are hints: the script never escalates and never touches the system.
command -v python3 >/dev/null 2>&1 \
    || die "python3 is required (the controller runs ansible)." "Debian/Ubuntu: sudo apt-get install python3 python3-venv"
python3 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)' \
    || die "python3 >= 3.11 is required by the pinned ansible-core ($(python3 --version 2>&1))."
python3 -c 'import venv, ensurepip' 2>/dev/null \
    || die "python3's venv module is incomplete (no ensurepip)." "Debian/Ubuntu: sudo apt-get install python3-venv"
command -v docker >/dev/null 2>&1 \
    || die "the docker CLI is required (the images build with BuildKit)." "https://docs.docker.com/engine/install/"
command -v git >/dev/null 2>&1 \
    || note "git not found — images will be stamped from GODFIR_REVISION or 'unknown', not the checkout's HEAD"

# ---- the controller layer: ansible-core + the docker SDK, pinned above ------
# GODFIR_ANSIBLE: a host that brings its own ansible skips the venv entirely.
# Otherwise the checkout's own venv, (re)installed from PIP_LOCK when the venv
# is missing, incomplete, or the lock changed since the last install.
VENV="${GODFIR_VENV:-$REPO_ROOT/.venv}"
if [[ -n "${GODFIR_ANSIBLE:-}" ]]; then
    [[ -x "$GODFIR_ANSIBLE" ]] || die "GODFIR_ANSIBLE is not an executable: $GODFIR_ANSIBLE"
    ANSIBLE_PLAYBOOK="$GODFIR_ANSIBLE"
    ANSIBLE_BIN_DIR="$(cd "$(dirname "$(readlink -f "$ANSIBLE_PLAYBOOK")")" && pwd)"
    ok "ansible from GODFIR_ANSIBLE: $ANSIBLE_PLAYBOOK"
else
    _lock_sum="$(printf '%s' "$PIP_LOCK" | sha256sum | cut -d' ' -f1)"
    _stamp="$VENV/.lock.sha256"
    if [[ ! -x "$VENV/bin/ansible-playbook" || "$(cat "$_stamp" 2>/dev/null)" != "$_lock_sum" ]]; then
        step "Installing the pinned controller layer into $VENV ..."
        note "first run, or the pins changed — needs PyPI; later runs are offline"
        python3 -m venv "$VENV" || die "could not create the venv at $VENV"
        "$VENV/bin/pip" install --quiet --upgrade pip \
            || die "pip upgrade in $VENV failed (no route to PyPI?)"
        printf '%s\n' "$PIP_LOCK" > "$VENV/lock.txt"
        "$VENV/bin/pip" install --quiet -r "$VENV/lock.txt" \
            || die "installing the pinned controller layer into $VENV failed (no route to PyPI?)"
        printf '%s\n' "$_lock_sum" > "$_stamp"
    fi
    ANSIBLE_PLAYBOOK="$VENV/bin/ansible-playbook"
    ANSIBLE_BIN_DIR="$VENV/bin"
    ok "ansible in the repo venv: $VENV"
fi
[[ -x "$ANSIBLE_PLAYBOOK" && -x "$ANSIBLE_BIN_DIR/ansible-galaxy" ]] \
    || die "ansible-playbook / ansible-galaxy not found beside $ANSIBLE_PLAYBOOK"
# the venv (or the host ansible) leads PATH for the rest of the run: the
# playbook's python, ansible-galaxy and every helper resolve consistently
export PATH="$ANSIBLE_BIN_DIR:$PATH"
_ansible_ver="$("$ANSIBLE_PLAYBOOK" --version 2>/dev/null | head -1)" \
    || die "$ANSIBLE_PLAYBOOK --version failed"
note "$_ansible_ver"

# community.docker's modules import the docker SDK on the controller — with
# ansible's OWN python, which is not necessarily the one on PATH.
_ansible_py="$("$ANSIBLE_PLAYBOOK" --version 2>/dev/null | sed -n 's/^ *python version = .*(\(.*\))$/\1/p' | head -1)"
[[ -n "$_ansible_py" && -x "$_ansible_py" ]] || _ansible_py="$ANSIBLE_BIN_DIR/python"
"$_ansible_py" -c 'import docker, requests' 2>/dev/null \
    || die "ansible's python ($_ansible_py) cannot import the docker SDK community.docker needs." \
           "$([[ -n "${GODFIR_ANSIBLE:-}" ]] && echo "$_ansible_py -m pip install 'docker==7.2.0' 'requests==2.34.2'" || echo "remove $VENV and re-run to reinstall the pins")"

# ---- the community.docker collection, pinned above ---------------------------
# The checkout's own collection tree leads ANSIBLE_COLLECTIONS_PATH for this
# run only — ansible.cfg is not touched — with the user/system paths after: a
# host that already has community.docker resolves it, a bare clone gets the
# pin installed in-tree. (`collection list NAME` exits 0 whether or not NAME
# is installed, so the JSON listing is parsed instead.)
COLLECTIONS="$REPO_ROOT/.ansible/collections"
export ANSIBLE_COLLECTIONS_PATH="$COLLECTIONS:${ANSIBLE_COLLECTIONS_PATH:-$HOME/.ansible/collections:/usr/share/ansible/collections}"
collection_version() {
    ansible-galaxy collection list --format json 2>/dev/null \
        | "$_ansible_py" -c 'import json, sys
for paths in json.load(sys.stdin).values():
    if sys.argv[1] in paths:
        print(paths[sys.argv[1]].get("version", "?")); break' "$1" 2>/dev/null
}
_cd_ver="$(collection_version "$COLLECTION_NAME")"
if [[ -z "$_cd_ver" ]]; then
    step "Installing the pinned $COLLECTION_PIN collection into $COLLECTIONS ..."
    note "needs galaxy.ansible.com once; later runs are offline"
    ansible-galaxy collection install "$COLLECTION_PIN" -p "$COLLECTIONS" \
        || die "installing the $COLLECTION_NAME collection failed (no route to galaxy.ansible.com?)" \
               "an offline host: install it once on a connected host and carry $COLLECTIONS across"
    _cd_ver="$(collection_version "$COLLECTION_NAME")"
    [[ -n "$_cd_ver" ]] \
        || die "$COLLECTION_NAME still does not resolve after the install (ANSIBLE_COLLECTIONS_PATH=$ANSIBLE_COLLECTIONS_PATH)"
fi
ok "$COLLECTION_NAME $_cd_ver"

# ---- the docker engine -------------------------------------------------------
if ! _dinfo="$(docker info 2>&1 >/dev/null)"; then
    case "$_dinfo" in
        *"permission denied"*)
            die "the docker daemon refuses this user ($(id -un))." \
                "join the docker group: sudo usermod -aG docker $(id -un), then 'newgrp docker' or log in again" ;;
        *)
            die "the docker daemon does not answer." \
                "is it running? (systemctl status docker) — $(printf '%s' "$_dinfo" | head -1)" ;;
    esac
fi
docker buildx version >/dev/null 2>&1 \
    || die "docker buildx (BuildKit) is required — the Dockerfiles use heredoc COPY and the syntax frontend." \
           "install the docker-buildx-plugin package"
ok "docker daemon answers, BuildKit present"
[[ -f images.yml && -f hardening/harden.yml ]] \
    || die "images.yml / hardening/harden.yml not found at $REPO_ROOT — incomplete checkout?"

if [[ "$PREFLIGHT_ONLY" == true ]]; then
    ok "host ready — run without --preflight to build"
    exit 0
fi

# ---- forward the request to the playbook -------------------------------------
EXTRA=()
if [[ ${#NAMES[@]} -gt 0 ]]; then
    # JSON-escape each name so a quoted/malformed argument can't break or extend the extra-vars JSON
    names=""
    for _name in "${NAMES[@]}"; do
        _esc=${_name//\\/\\\\}
        _esc=${_esc//\"/\\\"}
        names+="\"${_esc}\","
    done
    EXTRA+=(-e "{\"godfir_build_set\": [${names%,}]}")
fi
[[ -n "${GODFIR_REVISION:-}" ]] && EXTRA+=(-e "godfir_build_src_sha=${GODFIR_REVISION}")
[[ -n "${GODFIR_RELEASE:-}" ]] && EXTRA+=(-e "godfir_build_release=${GODFIR_RELEASE}")

step "Building: ${NAMES[*]:-every image in images.yml}"
exec "$ANSIBLE_PLAYBOOK" playbooks/build_images.yml "${EXTRA[@]}"
