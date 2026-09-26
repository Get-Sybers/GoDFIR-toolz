#!/usr/bin/env bash
# ==============================================================================
# build-all.sh — build the get-sybers/* images from images.yml, STANDALONE.
#
# This script exists so the repository works on its own (cloned by itself, no
# consumer around). It is a launcher of the collection playbook
# (playbooks/build_images.yml → the godfir_build role): the inventory lives in
# images.yml and the build logic in the role; the script only prepares the
# host, forwards names and the optional stamp overrides. An integrating
# consumer uses the role, never this script — DX_DFIR's dxdfir_images role is
# the reference — and nothing here is reachable from a consumer's path.
#
# Being standalone, it provisions what the role needs itself, per checkout,
# without touching the system, and it is safe to re-run:
#   - the pinned controller layer (requirements.txt: ansible-core + the docker
#     SDK community.docker's modules import) into <repo>/.venv, created by the
#     invoking user; reinstalled only when the lock changes;
#   - the pinned community.docker collection (requirements.yml) into
#     <repo>/.ansible/collections, only when it does not already resolve;
#   - a preflight on everything it cannot install: python3 >= 3.11 with the
#     venv module, the docker CLI, a daemon this user may talk to, BuildKit.
# The messages name the fix. Both trees are gitignored and excluded from the
# collection artifact. A host provisioned once builds offline afterwards.
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

# ---- styling (conform.sh's) --------------------------------------------------
if [[ -t 1 && -z "${NO_COLOR:-}" && "${TERM:-dumb}" != dumb ]]; then
    C_RST=$'\033[0m'; C_OK=$'\033[38;5;172m'; C_ACT=$'\033[38;5;214m'
    C_ERR=$'\033[38;5;167m'; C_DIM=$'\033[38;5;245m'
else C_RST= C_OK= C_ACT= C_ERR= C_DIM=; fi
ok()   { printf '%s[ ok ]%s %s\n' "$C_OK"  "$C_RST" "$*"; }
step() { printf '%s[ >> ]%s %s\n' "$C_ACT" "$C_RST" "$*"; }
note() { printf '       %s%s%s\n' "$C_DIM" "$*" "$C_RST"; }
die()  { printf '%s[fail]%s %s\n' "$C_ERR" "$C_RST" "$1" >&2; shift; for _l in "$@"; do printf '       %s\n' "$_l" >&2; done; exit 2; }
usage() { sed -n '2,46p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-2}"; }

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

# ---- the controller layer: ansible-core + the docker SDK, pinned -------------
# GODFIR_ANSIBLE: a host that brings its own ansible skips the venv entirely.
# Otherwise the checkout's own venv, (re)installed from requirements.txt when
# the venv is missing, incomplete, or the lock changed since the last install.
VENV="${GODFIR_VENV:-$REPO_ROOT/.venv}"
if [[ -n "${GODFIR_ANSIBLE:-}" ]]; then
    [[ -x "$GODFIR_ANSIBLE" ]] || die "GODFIR_ANSIBLE is not an executable: $GODFIR_ANSIBLE"
    ANSIBLE_PLAYBOOK="$GODFIR_ANSIBLE"
    ANSIBLE_BIN_DIR="$(cd "$(dirname "$(readlink -f "$ANSIBLE_PLAYBOOK")")" && pwd)"
    ok "ansible from GODFIR_ANSIBLE: $ANSIBLE_PLAYBOOK"
else
    [[ -f requirements.txt ]] || die "requirements.txt is missing — incomplete checkout?"
    _lock_sum="$(sha256sum requirements.txt | cut -d' ' -f1)"
    _stamp="$VENV/.requirements.sha256"
    if [[ ! -x "$VENV/bin/ansible-playbook" || "$(cat "$_stamp" 2>/dev/null)" != "$_lock_sum" ]]; then
        step "Installing the pinned controller layer (requirements.txt) into $VENV ..."
        note "first run, or the lock changed — needs PyPI; later runs are offline"
        python3 -m venv "$VENV" || die "could not create the venv at $VENV"
        "$VENV/bin/pip" install --quiet --upgrade pip \
            || die "pip upgrade in $VENV failed (no route to PyPI?)"
        "$VENV/bin/pip" install --quiet -r requirements.txt \
            || die "installing requirements.txt into $VENV failed (no route to PyPI?)"
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
           "$([[ -n "${GODFIR_ANSIBLE:-}" ]] && echo "$_ansible_py -m pip install -r $REPO_ROOT/requirements.txt" || echo "remove $VENV and re-run to reinstall the lock")"

# ---- the community.docker collection, pinned ---------------------------------
# ansible.cfg (read from the repo root) puts <repo>/.ansible/collections first
# on collections_path, the user/system paths after: a host that already has
# community.docker resolves it, a bare clone gets the pin installed in-tree.
# (`collection list NAME` exits 0 whether or not NAME is installed, so the
# JSON listing is parsed instead — the resolved version, or nothing.)
COLLECTIONS="$REPO_ROOT/.ansible/collections"
collection_version() {
    ansible-galaxy collection list --format json 2>/dev/null \
        | "$_ansible_py" -c 'import json, sys
for paths in json.load(sys.stdin).values():
    if sys.argv[1] in paths:
        print(paths[sys.argv[1]].get("version", "?")); break' "$1" 2>/dev/null
}
_cd_ver="$(collection_version community.docker)"
if [[ -z "$_cd_ver" ]]; then
    [[ -f requirements.yml ]] || die "requirements.yml is missing — incomplete checkout?"
    step "Installing the pinned community.docker collection (requirements.yml) into $COLLECTIONS ..."
    note "needs galaxy.ansible.com once; later runs are offline"
    ansible-galaxy collection install -r requirements.yml -p "$COLLECTIONS" \
        || die "installing the community.docker collection failed (no route to galaxy.ansible.com?)" \
               "an offline host: install it once on a connected host and carry $COLLECTIONS across"
    _cd_ver="$(collection_version community.docker)"
    [[ -n "$_cd_ver" ]] \
        || die "community.docker still does not resolve after the install — check collections_path in ansible.cfg"
fi
ok "community.docker $_cd_ver"

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
