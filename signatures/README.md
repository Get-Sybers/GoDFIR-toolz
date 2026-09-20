# `get-sybers/signatures` — the detection lane in one image

One hardened image for the whole detection lane: **YARA + Suricata**
(Debian) **+ Hayabusa** (pinned release zip, sha256-verified at build time —
`HAYABUSA_VERSION` + `HB_SHA_*` build args) **+ the gomount→goyara
userspace NTFS scan pipe** (both built from their own top-level modules;
[goyara](../goyara/README.md) links libyara through cgo and has no image of its
own). The DetectRaptor YARA merge and the ET Open Suricata ruleset are baked
at build time. `sh`/`dash` stay as a declared deviation for the legacy
per-file YARA loop (`/opt/dxdfir/scan-list.sh`); bash, python, ansible, apt,
pip, sudo and setuid binaries are gone and the image runs as uid 2000.

The ENTRYPOINT is **signatures-entry**, a static Go dispatcher on the shared
batch runtime ([`contract.yml`](contract.yml)): named with a sub-tool and
nothing else, it self-orchestrates that sub-tool from the sub-tool's own env
block and prints one JSON summary line.

```
signatures-entry yara       staged trees   -> YARA hits per target
signatures-entry suricata   captures       -> EVE JSON per capture
signatures-entry hayabusa   .evtx trees    -> sigma detections per tree
signatures-entry scan       disk images    -> gomount stream | goyara hits per image
```

## Input

Each sub-tool walks its own `SIGNATURES_<SUBTOOL>_INPUT_DIR` (default
`/input`, mounted read-only):

- `yara`: every immediate child of the input root (a staged tree or a single file) is one scan target, scanned recursively.
- `suricata`: every `*.pcap`/`*.pcapng`/`*.cap` anywhere under the tree is one item.
- `hayabusa`: every immediate subdirectory holding `.evtx` files (anywhere below it) is one item; when the root itself holds `.evtx` files directly, the root is the single item.
- `scan`: every disk image (`.E01 .Ex01 .raw .dd .img .vmdk .vhd .vhdx .001 .bin`) is one item.

## Env

Every sub-tool reads the reserved variables under its own prefix, plus `_ARGS`
(extra argv appended to the tool's command line, space-separated):

| Variable | Default | Meaning |
|---|---|---|
| `SIGNATURES_<SUBTOOL>_INPUT_DIR` | `/input` | the sub-tool's input tree |
| `SIGNATURES_<SUBTOOL>_OUT_DIR` | `/output` | output root, one folder per item |
| `SIGNATURES_<SUBTOOL>_WORK_DIR` | `/work` | scratch (writable tmpfs) |
| `SIGNATURES_<SUBTOOL>_FORCE` | `0` | `1/true/yes/on`: rerun items that already have valid output |
| `SIGNATURES_<SUBTOOL>_FORMAT` | `json` | record format; `json` (JSONL) is the only format |
| `SIGNATURES_<SUBTOOL>_ARGS` | *(empty)* | extra argv for the tool |
| `SIGNATURES_<SUBTOOL>_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |

`yara` — `SIGNATURES_YARA_INPUT_DIR`, `SIGNATURES_YARA_OUT_DIR`,
`SIGNATURES_YARA_WORK_DIR`, `SIGNATURES_YARA_FORCE`, `SIGNATURES_YARA_FORMAT`,
`SIGNATURES_YARA_ARGS`, `SIGNATURES_YARA_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `SIGNATURES_YARA_RULES` | `/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar` | the ruleset; a mounted operator ruleset overrides the baked DetectRaptor merge |

Each target is scanned with `yara -w -s -N -r <rules> <target>`; match and
no-match both succeed, any yara error fails the item.

`suricata` — `SIGNATURES_SURICATA_INPUT_DIR`, `SIGNATURES_SURICATA_OUT_DIR`,
`SIGNATURES_SURICATA_WORK_DIR`, `SIGNATURES_SURICATA_FORCE`,
`SIGNATURES_SURICATA_FORMAT`, `SIGNATURES_SURICATA_ARGS`,
`SIGNATURES_SURICATA_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `SIGNATURES_SURICATA_RULES` | `/opt/dxdfir/suricata-rules/suricata.rules` | the ruleset (`-S`); a mounted file overrides the baked ET Open merge |
| `SIGNATURES_SURICATA_SET` | *(empty)* | comma- or space-separated `key=value` tuning entries passed as `--set` (`HOME_NET=…`) |

Each capture is replayed with `suricata -r <capture> -l <item dir> -k none -S
<rules> --set unix-command.enabled=no [--set …]`. Under a read-only rootfs
suricata also needs writable `/var/run/suricata` and `/var/log/suricata`
(tmpfs).

`hayabusa` — `SIGNATURES_HAYABUSA_INPUT_DIR`, `SIGNATURES_HAYABUSA_OUT_DIR`,
`SIGNATURES_HAYABUSA_WORK_DIR`, `SIGNATURES_HAYABUSA_FORCE`,
`SIGNATURES_HAYABUSA_FORMAT`, `SIGNATURES_HAYABUSA_ARGS`,
`SIGNATURES_HAYABUSA_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `SIGNATURES_HAYABUSA_RULES` | `/opt/dxdfir/hayabusa/rules` | the sigma rules directory; a mounted directory overrides the baked set |
| `SIGNATURES_HAYABUSA_PROFILE` | `verbose` | output profile; `verbose` carries the MITRE ATT&CK columns |

Each item runs `hayabusa json-timeline --directory <item> --output
<item dir>/timeline.jsonl --JSONL-output --profile <profile> --no-wizard
--UTC --quiet --rules <rules>` from the baked hayabusa home.

`scan` — `SIGNATURES_SCAN_INPUT_DIR`, `SIGNATURES_SCAN_OUT_DIR`,
`SIGNATURES_SCAN_WORK_DIR`, `SIGNATURES_SCAN_FORCE`, `SIGNATURES_SCAN_FORMAT`,
`SIGNATURES_SCAN_ARGS`, `SIGNATURES_SCAN_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `SIGNATURES_SCAN_RULES` | `/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar` | goyara's ruleset (`--rules`) |
| `SIGNATURES_SCAN_FILTER` | *(empty)* | gomount `--filter` glob over the volume paths to stream; empty = every file |
| `SIGNATURES_SCAN_VOLUME` | `0` | 1-based NTFS volume to stream; `0` = the largest |
| `SIGNATURES_SCAN_MAX_BYTES` | `33554432` | goyara's per-file scan cap in bytes |

Each image runs `gomount stream [--filter …] [--volume N] <image> | goyara
--rules <rules> --json - --max-bytes N` in-process; goyara's exit 2 (some
files unreadable) is a warning, the hits are kept.

## Output

`<item>` is the input path relative to the sub-tool's `INPUT_DIR` with path
separators and whitespace folded to `_`. The record or index file named after
the sub-tool is written last and marks the item done; an item whose file
exists is skipped on the next run unless `FORCE` is set.

| Sub-tool | Per item | `records` |
|---|---|---|
| `yara` | `<OUT_DIR>/<item>/yara.jsonl` — `{tool, rule, tags, target, strings[{offset,name,data}]}` per hit | hits |
| `suricata` | `<OUT_DIR>/<item>/eve.json` + suricata's own logs + `suricata.jsonl` (index) | EVE lines |
| `hayabusa` | `<OUT_DIR>/<item>/timeline.jsonl` + `hayabusa.jsonl` (index) | detections |
| `scan` | `<OUT_DIR>/<item>/scan.jsonl` — goyara's hit records | hits |

stdout is exactly one JSON object: `tool`, `subtool`, `version`, `status`,
`inputs`, `processed`, `skipped`, `failed`, `records`, `outputs`, `exit`,
`started`, `duration_s`, plus `failures` (item + error) when something failed
and `error` on a config error. The tools' own output, progress and errors go
to stderr.

## Exit codes

| Code | Status | Meaning |
|---|---|---|
| 0 | `ok` | every item processed, or already up to date |
| 1 | `nothing` | no input found, or every item failed |
| 2 | `config_error` | no sub-tool named, bad variable, missing or unreadable input, unwritable output |
| 3 | `partial` | at least one item processed and at least one failed |

## Run

```sh
docker build -t get-sybers/signatures:latest -f signatures/Dockerfile .
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/staged:/input:ro" -v "$PWD/out:/output" \
  get-sybers/signatures:latest yara
docker run --rm … --tmpfs /var/run/suricata:rw,nosuid,nodev --tmpfs /var/log/suricata:rw,nosuid,nodev \
  -v "$PWD/pcaps:/input:ro" -v "$PWD/out:/output" \
  -e SIGNATURES_SURICATA_SET='HOME_NET=[10.0.0.0/8]' \
  get-sybers/signatures:latest suricata
```

Builds with the repo root as context so `COPY hardening/harden.yml`, the
`gomount/` and `goyara/` modules and this directory's module are in reach.
`test/contract_test.sh` builds the image, runs `yara` over `test/fixtures/`
(a small staged tree) and `suricata` over its capture, and asserts the summary
line, the exit codes, idempotency and the config-error exit; the unit tests
cover every sub-tool's discovery and processing with stub tools.

## argv pass-through (debug only)

`signatures-entry <tool> <args…>` execs `yara`, `suricata`,
`/opt/dxdfir/hayabusa/hayabusa`, `gomount`, `goyara`,
`/opt/dxdfir/scan-list.sh` or `sh` verbatim with that tool's exit code;
`--version` prints the dispatcher's version and `--print-contract` prints
`contract.yml`. Any other first argument is a config error (exit 2).
`/opt/dxdfir/scan-list.sh` is the legacy per-file loop: it reads a mounted
`/list.txt` and scans each file with the mounted `/index.yar`; every file is
scanned even when one fails, and any yara error fails the run.
