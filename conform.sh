#!/usr/bin/env bash
# ==============================================================================
# conform.sh — check ONE tool directory against the GoDFIR-toolz Container
# Framework white paper (layout + standards).
#
# Static by default (no docker build): validates the directory tree, the
# Dockerfile's declared standards, contract.yml, and name/label consistency.
# With --build it additionally builds the image and verifies the built artifact
# (USER, labels, /etc/dfir-hardened vs the actual filesystem) — a local
# pre-flight for the CI release gate.
#
# This encodes the white paper's MUST-rules:
#   02 tool directory structure · 03 environment contract · 05 hardening standard
#
# Dependencies: bash; python3 for contract.yml (full checks need PyYAML, else a
# reduced key-presence check); docker only for --build.
#
# Usage:
#   conform.sh [options] <tool-dir>
#
# Options:
#   --build              also build the image and run built-artifact checks
#   --schema FILE        contract JSON-schema (default: <repo>/hardening/contract.schema.yml)
#   --images-yml FILE    inventory for name/ref consistency (default: <repo>/images.yml)
#   --strict             treat WARN as FAIL for the exit code
#   -q, --quiet          print only the summary and failures
#   -h, --help
#
# Exit: 0 conformant · 1 non-conformant (one or more FAIL; also WARN under
#       --strict) · 2 usage / environment error.
# ==============================================================================
set -o pipefail

# ---- styling -----------------------------------------------------------------
if [[ -t 1 && -z "${NO_COLOR:-}" && "${TERM:-dumb}" != dumb ]]; then
    C_RST=$'\033[0m'; C_B=$'\033[1m'; C_OK=$'\033[38;5;172m'
    C_WARN=$'\033[38;5;173m'; C_ERR=$'\033[38;5;167m'; C_DIM=$'\033[38;5;245m'
else C_RST= C_B= C_OK= C_WARN= C_ERR= C_DIM=; fi

QUIET=false; STRICT=false; DO_BUILD=false
SCHEMA=""; IMAGES_YML=""; TOOL_DIR=""
die() { printf '%serror:%s %s\n' "$C_ERR" "$C_RST" "$*" >&2; exit 2; }
usage() { sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit "${1:-2}"; }

while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)      DO_BUILD=true; shift ;;
        --schema)     SCHEMA="${2:?}"; shift 2 ;;
        --images-yml) IMAGES_YML="${2:?}"; shift 2 ;;
        --strict)     STRICT=true; shift ;;
        -q|--quiet)   QUIET=true; shift ;;
        -h|--help)    usage 0 ;;
        -*)           die "unknown option: $1 (see --help)" ;;
        *)            [[ -z "$TOOL_DIR" ]] && TOOL_DIR="$1" || die "unexpected arg: $1"; shift ;;
    esac
done
[[ -n "$TOOL_DIR" ]] || usage 2
[[ -d "$TOOL_DIR" ]] || die "not a directory: $TOOL_DIR"

TOOL_DIR="$(cd "$TOOL_DIR" && pwd)"
TOOL="$(basename "$TOOL_DIR")"
REPO_ROOT="$(dirname "$TOOL_DIR")"
PREFIX="$(printf '%s' "$TOOL" | tr '[:lower:]-' '[:upper:]_')"
DF="$TOOL_DIR/Dockerfile"
CONTRACT="$TOOL_DIR/contract.yml"
SCHEMA="${SCHEMA:-$REPO_ROOT/hardening/contract.schema.yml}"
IMAGES_YML="${IMAGES_YML:-$REPO_ROOT/images.yml}"

# ---- reporters ---------------------------------------------------------------
pass=0; warn=0; fail=0
_p() { ((pass++)); [[ "$QUIET" == true ]] || printf '  %sPASS%s %s\n' "$C_OK"   "$C_RST" "$*"; }
_w() { ((warn++));                          printf '  %sWARN%s %s\n' "$C_WARN" "$C_RST" "$*"; }
_f() { ((fail++));                          printf '  %sFAIL%s %s\n' "$C_ERR"  "$C_RST" "$*"; }
section() { [[ "$QUIET" == true ]] || printf '%s%s%s\n' "$C_B" "$*" "$C_RST"; }
have()  { command -v "$1" >/dev/null 2>&1; }
dfgrep(){ grep -Eq "$1" "$DF" 2>/dev/null; }   # test a pattern in the Dockerfile

printf '%sconform:%s %s  (tool=%s, prefix=%s_)\n\n' "$C_B" "$C_RST" "$TOOL_DIR" "$TOOL" "$PREFIX"

# ---- 1. directory structure (white paper 02) ---------------------------------
section "directory structure (02)"
[[ -f "$DF" ]]        && _p "Dockerfile present"        || _f "Dockerfile missing (MUST)"
[[ -f "$CONTRACT" ]]  && _p "contract.yml present"      || _f "contract.yml missing (MUST)"
[[ -f "$TOOL_DIR/README.md" ]] && _p "README.md present" || _f "README.md missing (MUST)"
if [[ -d "$TOOL_DIR/test" ]]; then
    _p "test/ present"
    [[ -d "$TOOL_DIR/test/fixtures" ]] && _p "test/fixtures/ present" \
        || _w "test/fixtures/ missing (a deterministic generator is allowed instead)"
    compgen -G "$TOOL_DIR/test/contract_test.*" >/dev/null \
        && _p "test/contract_test.* present" || _f "test/contract_test.* missing (MUST)"
else
    _f "test/ missing (MUST)"
fi
[[ -f "$TOOL_DIR/.gitignore" ]] && _p ".gitignore present" || _w ".gitignore missing (SHOULD)"

# Go tool? (has go.mod, or any .go source)
IS_GO=false
if [[ -f "$TOOL_DIR/go.mod" ]] || compgen -G "$TOOL_DIR/*.go" >/dev/null; then IS_GO=true; fi
if [[ "$IS_GO" == true ]]; then
    section "Go tool files (02)"
    [[ -f "$TOOL_DIR/go.mod" ]] && _p "go.mod present" || _f "go.mod missing (MUST for Go tools)"
    [[ -f "$TOOL_DIR/go.sum" ]] && _p "go.sum present" || _f "go.sum missing (MUST for Go tools)"
    compgen -G "$TOOL_DIR/*_test.go" >/dev/null \
        && _p "*_test.go present" || _f "*_test.go missing (MUST for Go tools)"
fi

# ---- 2. Dockerfile standards (white paper 05) --------------------------------
if [[ -f "$DF" ]]; then
    section "Dockerfile standards (05)"
    head -n 3 "$DF" | grep -Eq '^#\s*syntax=docker/dockerfile:1' \
        && _p "syntax directive (# syntax=docker/dockerfile:1)" \
        || _f "missing '# syntax=docker/dockerfile:1' on line 1 (MUST)"

    # FROM analysis: multi-stage + final stage is scratch
    mapfile -t FROMS < <(grep -iE '^[[:space:]]*FROM[[:space:]]' "$DF" | awk '{print $2}')
    if (( ${#FROMS[@]} >= 2 )); then _p "multi-stage (${#FROMS[@]} FROM stages)"
    else _f "not multi-stage (need builder + minimal final stage) (MUST)"; fi
    if [[ "${FROMS[-1]:-}" == "scratch" ]]; then _p "final stage is FROM scratch (distroless)"
    else _f "final stage is '${FROMS[-1]:-<none>}', not scratch (MUST be distroless)"; fi

    # USER 2000:2000 (literal, or ARG-defaulted)
    if dfgrep '^[[:space:]]*USER[[:space:]]+2000:2000'; then
        _p "USER 2000:2000"
    elif dfgrep '^[[:space:]]*USER[[:space:]]+\$\{DFIR_UID\}:\$\{DFIR_GID\}' \
         && dfgrep '^[[:space:]]*ARG[[:space:]]+DFIR_UID=2000' \
         && dfgrep '^[[:space:]]*ARG[[:space:]]+DFIR_GID=2000'; then
        _p "USER \${DFIR_UID}:\${DFIR_GID} with 2000:2000 defaults"
    else
        _f "no USER resolving to 2000:2000 (MUST — nonroot fixed identity)"
    fi

    dfgrep '^[[:space:]]*ENTRYPOINT' && _p "ENTRYPOINT present" || _f "ENTRYPOINT missing (MUST)"

    dfgrep 'RUN[[:space:]].*--version' || dfgrep '\$\?[[:space:]]*-eq[[:space:]]*1' \
        && _p "build-stage sanity gate (--version / usage-exit)" \
        || _w "no obvious build-stage sanity gate (RUN /<tool> --version) (MUST — heuristic)"

    dfgrep '/etc/dfir-hardened' \
        && _p "/etc/dfir-hardened declaration in Dockerfile" \
        || _w "/etc/dfir-hardened not in Dockerfile (Shape B writes it via harden.yml; verify with --build)"

    section "required labels (05.7)"
    # always-in-Dockerfile labels
    for L in org.opencontainers.image.title \
             org.opencontainers.image.description \
             org.opencontainers.image.source \
             com.get-sybers.tool com.get-sybers.contract; do
        dfgrep "$L" && _p "label $L" || _f "label $L missing (MUST)"
    done
    dfgrep 'com\.get-sybers\.hardened="?true"?' \
        && _p 'label com.get-sybers.hardened="true"' \
        || _f 'label com.get-sybers.hardened="true" missing (MUST — the trust anchor)'
    # release-stamped labels: may be injected at build; warn if absent statically
    for L in org.opencontainers.image.licenses \
             org.opencontainers.image.version \
             org.opencontainers.image.revision \
             com.get-sybers.godfir-release; do
        dfgrep "$L" && _p "label $L" || _w "label $L not in Dockerfile (release-stamped? verify with --build)"
    done

    # com.get-sybers.tool value == directory name
    lbl_tool="$(grep -oE 'com\.get-sybers\.tool="?[^"[:space:]]+' "$DF" | head -1 | sed -E 's/.*tool="?//')"
    if [[ -n "$lbl_tool" ]]; then
        [[ "$lbl_tool" == "$TOOL" ]] && _p "com.get-sybers.tool == directory ($TOOL)" \
            || _f "com.get-sybers.tool='$lbl_tool' != directory '$TOOL' (MUST match)"
    fi
fi

# ---- 3. engine-ref (05.7) — only for engine-built images ---------------------
IS_ENGINE=false
if [[ -f "$IMAGES_YML" ]] && grep -Eq "name:[[:space:]]*$TOOL\b" "$IMAGES_YML" 2>/dev/null; then
    # crude: an entry with a `ref:` near this name is engine-built
    if awk -v t="$TOOL" '
        $0 ~ ("name:[[:space:]]*"t"([[:space:]]|$|#)") {f=1; next}
        f && /name:[[:space:]]*/ {f=0}
        f && /ref:[[:space:]]*/ {print "yes"; exit}' "$IMAGES_YML" | grep -q yes; then
        IS_ENGINE=true
    fi
fi
if [[ "$IS_ENGINE" == true && -f "$DF" ]]; then
    section "engine image (05.7)"
    dfgrep 'com\.get-sybers\.engine-ref' \
        && _p "label com.get-sybers.engine-ref (engine-built image)" \
        || _w "engine image lacks com.get-sybers.engine-ref label (MUST for engine images; may be build-stamped)"
fi

# ---- 4. contract.yml (white paper 03) ----------------------------------------
if [[ -f "$CONTRACT" ]]; then
    section "contract.yml (03)"
    while IFS=$'\t' read -r st msg; do
        case "$st" in PASS) _p "$msg";; WARN) _w "$msg";; FAIL) _f "$msg";; esac
    done < <(python3 - "$CONTRACT" "$TOOL" "$PREFIX" <<'PY'
import sys, re
path, tool, prefix = sys.argv[1], sys.argv[2], sys.argv[3]
def out(s, m): print(f"{s}\t{m}")
raw = open(path).read()
data = None
try:
    import yaml
    data = yaml.safe_load(raw)
except ModuleNotFoundError:
    out("WARN", "PyYAML not installed — reduced checks (top-level keys only); install pyyaml for full validation")
except Exception as e:
    out("FAIL", f"contract.yml is not valid YAML: {e}")
    sys.exit(0)

REQUIRED_TOP = ["tool","image","entrypoint","env","mounts","exit_codes","summary_schema"]
if data is None:
    # reduced: key presence by regex
    present = set(re.findall(r'(?m)^([a-z_]+):', raw))
    for k in REQUIRED_TOP:
        out("PASS" if k in present else "FAIL", f"top-level key '{k}'" + ("" if k in present else " missing (MUST)"))
    m = re.search(r'(?m)^tool:\s*(\S+)', raw)
    if m and m.group(1) != tool:
        out("FAIL", f"tool: '{m.group(1)}' != directory '{tool}' (MUST match)")
    sys.exit(0)

if not isinstance(data, dict):
    out("FAIL", "contract.yml top level is not a mapping"); sys.exit(0)
for k in REQUIRED_TOP:
    out("PASS" if k in data else "FAIL", f"top-level key '{k}'" + ("" if k in data else " missing (MUST)"))

if data.get("tool") != tool:
    out("FAIL", f"tool: '{data.get('tool')}' != directory '{tool}' (MUST match)")
else:
    out("PASS", f"tool == directory ({tool})")

img = str(data.get("image",""))
if img and f"/{tool}:" not in img and not img.endswith(f"/{tool}"):
    out("WARN", f"image '{img}' does not reference tool '{tool}'")

ep = data.get("entrypoint")
if ep in ("self-orchestrating","multi-tool","argv"):
    out("PASS", f"entrypoint: {ep}")
else:
    out("FAIL", f"entrypoint '{ep}' invalid (self-orchestrating|multi-tool|argv)")

env = data.get("env") or {}
if isinstance(env, dict):
    bad = [k for k in env if not str(k).startswith(prefix + "_")]
    if bad:
        out("WARN", f"env vars not prefixed {prefix}_: {', '.join(bad[:6])}" + (" …" if len(bad)>6 else ""))
    else:
        out("PASS", f"all env vars prefixed {prefix}_")

mounts = data.get("mounts") or []
if isinstance(mounts, list) and mounts:
    ro  = [m for m in mounts if str(m.get("mode"))=="ro"]
    outp= [m for m in mounts if str(m.get("mode"))=="rw" and m.get("required") is True]
    out("PASS" if ro else "WARN", "read-only input mount declared" if ro else "no ro input mount declared")
    out("PASS" if outp else "FAIL", "required rw output mount declared" if outp else "no required rw output mount (MUST)")
else:
    out("FAIL", "mounts missing or empty (MUST)")

ec = data.get("exit_codes") or {}
keys = set(str(k) for k in ec) if isinstance(ec, dict) else set()
miss = [c for c in ("0","1","2") if c not in keys]
out("PASS" if not miss else "WARN", "exit_codes cover 0/1/2" if not miss else f"exit_codes missing {miss}")

ss = data.get("summary_schema") or {}
req = ss.get("required") if isinstance(ss, dict) else None
base = {"tool","version","status","exit"}
if isinstance(req, list) and base.issubset(set(req)):
    out("PASS", "summary_schema.required covers tool/version/status/exit")
else:
    out("WARN", "summary_schema.required should include tool, version, status, exit")
PY
    )

    # schema validation (only if schema + a validator exist)
    if [[ -f "$SCHEMA" ]]; then
        if python3 -c 'import jsonschema,yaml' 2>/dev/null; then
            if python3 - "$CONTRACT" "$SCHEMA" <<'PY' 2>/dev/null
import sys, yaml, jsonschema
c = yaml.safe_load(open(sys.argv[1])); s = yaml.safe_load(open(sys.argv[2]))
jsonschema.validate(c, s)
PY
            then _p "contract.yml validates against $(basename "$SCHEMA")"
            else _f "contract.yml fails schema $(basename "$SCHEMA")"; fi
        else
            _w "contract.schema.yml present but python jsonschema/yaml not installed — schema check skipped"
        fi
    else
        _w "hardening/contract.schema.yml not found — schema check skipped (MUST ADD per 02.2)"
    fi
fi

# ---- 5. README ↔ contract drift (03/06.5) ------------------------------------
if [[ -f "$TOOL_DIR/README.md" && -f "$CONTRACT" ]]; then
    section "README ↔ contract (06.5)"
    missing=()
    while read -r v; do [[ -n "$v" ]] && ! grep -q "$v" "$TOOL_DIR/README.md" && missing+=("$v"); done \
        < <(grep -oE "^[[:space:]]+${PREFIX}_[A-Z0-9_]+" "$CONTRACT" | tr -d ' ')
    if (( ${#missing[@]} == 0 )); then _p "README mentions every contract env var"
    else _w "README omits contract env vars: ${missing[*]}"; fi
fi

# ---- 6. optional --build deep checks (06.2–06.4) -----------------------------
if [[ "$DO_BUILD" == true ]]; then
    section "built-image checks (--build; 06.2–06.4)"
    if ! have docker; then _f "docker not available for --build"; else
        # Build context mirrors the build convention (matches images.yml's per-tool
        # `context` and build-all.sh): Shape B / harden.yml tools need the submodule
        # root (they COPY hardening/harden.yml); Shape A Go tools build from their own
        # directory (COPY go.mod go.sum ./). Always using the repo root broke the
        # latter (e.g. gomft).
        if grep -qE 'COPY[[:space:]]+hardening/|harden\.yml' "$DF"; then CTX="$REPO_ROOT"; else CTX="$TOOL_DIR"; fi
        IMG="conform-check/$TOOL:latest"
        if docker build -q -t "$IMG" -f "$DF" \
             --build-arg DFIR_UID=2000 --build-arg DFIR_GID=2000 "$CTX" >/dev/null 2>"$TOOL_DIR/.conform-build.log"; then
            _p "image builds (context: $(basename "$CTX"))"
            usr="$(docker image inspect -f '{{.Config.User}}' "$IMG" 2>/dev/null)"
            [[ "$usr" == "2000:2000" ]] && _p "built USER == 2000:2000" || _f "built USER='$usr' != 2000:2000"
            hardened="$(docker image inspect -f '{{index .Config.Labels "com.get-sybers.hardened"}}' "$IMG" 2>/dev/null)"
            [[ "$hardened" == "true" ]] && _p "built label com.get-sybers.hardened=true" || _f "built label com.get-sybers.hardened='$hardened' (06.2)"
            # version-bearing labels required by the gate (06.2)
            for L in org.opencontainers.image.version org.opencontainers.image.revision \
                     com.get-sybers.godfir-release com.get-sybers.contract; do
                v="$(docker image inspect -f "{{index .Config.Labels \"$L\"}}" "$IMG" 2>/dev/null)"
                [[ -n "$v" && "$v" != "<no value>" ]] && _p "built label $L=$v" \
                    || _f "built label $L missing/empty (06.2)"
            done
            # filesystem surface scan (06.3) cross-checked against /etc/dfir-hardened (06.4)
            cid="$(docker create "$IMG" __conform__ 2>/dev/null)"
            if [[ -n "$cid" ]]; then
                fs="$(docker export "$cid" 2>/dev/null | tar -t 2>/dev/null)"
                decl=""
                if grep -q '^etc/dfir-hardened$' <<<"$fs"; then
                    decl="$(docker export "$cid" 2>/dev/null | tar -xO etc/dfir-hardened 2>/dev/null)"
                fi
                docker rm -f "$cid" >/dev/null 2>&1
                grep -qE '(^|/)(usr/)?bin/(apt-get|dpkg|sudo)$' <<<"$fs" \
                    && _f "removed surface present (apt-get/dpkg/sudo) (06.3)" || _p "no apt-get/dpkg/sudo"
                grep -qE '(^|/)(usr/)?bin/pip[0-9.]*$' <<<"$fs" \
                    && _f "pip present (06.3)" || _p "no pip"
                grep -qE '/ansible([-/]|$)' <<<"$fs" \
                    && _f "ansible present in runtime image (06.3)" || _p "no ansible in runtime"
                [[ -n "$decl" ]] && _p "/etc/dfir-hardened present" || _w "/etc/dfir-hardened not found in image"
                if grep -q 'shell=false' <<<"$decl"; then
                    grep -qE '(^|/)bin/(sh|bash|dash)$' <<<"$fs" \
                        && _f "declares shell=false but a shell is present (06.4)" || _p "shell=false matches filesystem"
                fi
                if grep -q 'python=false' <<<"$decl"; then
                    grep -qE '(^|/)bin/python3(\.[0-9]+)?$' <<<"$fs" \
                        && _f "declares python=false but python is present (06.4)" || _p "python=false matches filesystem"
                fi
            fi
            docker image rm -f "$IMG" >/dev/null 2>&1
            rm -f "$TOOL_DIR/.conform-build.log"
        else
            _f "image build FAILED (see $TOOL_DIR/.conform-build.log)"
        fi
    fi
fi

# ---- summary -----------------------------------------------------------------
printf '\n%ssummary%s  %sPASS %d%s  %sWARN %d%s  %sFAIL %d%s\n' \
    "$C_B" "$C_RST" "$C_OK" "$pass" "$C_RST" "$C_WARN" "$warn" "$C_RST" "$C_ERR" "$fail" "$C_RST"
rc=0
if (( fail > 0 )); then rc=1
elif [[ "$STRICT" == true && $warn -gt 0 ]]; then rc=1; fi
if (( rc == 0 )); then printf '%s%s conforms%s\n' "$C_OK" "$TOOL" "$C_RST"
else printf '%s%s does NOT conform%s\n' "$C_ERR" "$TOOL" "$C_RST"; fi
exit "$rc"
