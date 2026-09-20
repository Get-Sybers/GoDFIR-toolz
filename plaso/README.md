# `get-sybers/plaso` — Plaso behind a multi-tool dispatcher

[Plaso](https://github.com/log2timeline/plaso) at a pinned PyPI release
(`PLASO_VERSION`, libyal built from source) with its three entry tools
(`log2timeline`, `psort`, `image_export`) and the psort wrapper
(`psort_wrapper.py`, which imports a mounted custom output module so psort
discovers it, then hands psort the remaining argv). Plaso is python, so python
and `sh` stay in the image as declared deviations (`bash` is removed); ansible,
apt, pip, sudo and setuid binaries are gone and the image runs as uid 2000.

The ENTRYPOINT is **plaso-entry**, the dispatcher of
[`contract.yml`](contract.yml): named with a sub-tool and nothing else, it
self-orchestrates that sub-tool from the sub-tool's own env block and prints
one JSON summary line.

```
plaso-entry log2timeline     evidence tree -> one .plaso storage file per item
plaso-entry psort            .plaso files  -> one rendered timeline per item
plaso-entry image_export     disk images   -> one extracted artefact tree per item
```

## Input

Each sub-tool walks its own `PLASO_<SUBTOOL>_INPUT_DIR` (default `/input`,
mounted read-only):

- `log2timeline`: every disk image (`.E01 .Ex01 .raw .dd .img .vmdk .vhd .vhdx .qcow2 .aff4 .001 .bin`) anywhere under the tree, plus every immediate subdirectory (a staged evidence tree), is one item.
- `psort`: every `*.plaso` storage file is one item.
- `image_export`: every disk image is one item.

## Env

Every sub-tool reads the reserved variables under its own prefix, plus `_ARGS`
(extra argv appended to the tool's command line, space-separated):

| Variable | Default | Meaning |
|---|---|---|
| `PLASO_<SUBTOOL>_INPUT_DIR` | `/input` | the sub-tool's input tree |
| `PLASO_<SUBTOOL>_OUT_DIR` | `/output` | output root, one folder per item |
| `PLASO_<SUBTOOL>_WORK_DIR` | `/work` | plaso's temporary directory (writable tmpfs) |
| `PLASO_<SUBTOOL>_FORCE` | `0` | `1/true/yes/on`: rerun items that already have valid output |
| `PLASO_<SUBTOOL>_ARGS` | *(empty)* | extra argv for the tool |
| `PLASO_<SUBTOOL>_LOG_LEVEL` | `info` | `error|warn|info|debug` (debug echoes the tool's log), stderr only |

Sub-tool specific — `log2timeline`: `PLASO_LOG2TIMELINE_INPUT_DIR`,
`PLASO_LOG2TIMELINE_OUT_DIR`, `PLASO_LOG2TIMELINE_WORK_DIR`,
`PLASO_LOG2TIMELINE_FORCE`, `PLASO_LOG2TIMELINE_ARGS`,
`PLASO_LOG2TIMELINE_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `PLASO_LOG2TIMELINE_VSS` | `1` | process every VSS store of an image (`--vss-stores all`) |
| `PLASO_LOG2TIMELINE_WINREG_BINARY` | `1` | keep binary registry values (`--extract_winreg_binary`) |
| `PLASO_LOG2TIMELINE_PARSERS` | *(empty)* | parser preset or list (`--parsers`); empty = plaso's default |

`psort`: `PLASO_PSORT_INPUT_DIR`, `PLASO_PSORT_OUT_DIR`, `PLASO_PSORT_WORK_DIR`,
`PLASO_PSORT_FORCE`, `PLASO_PSORT_ARGS`, `PLASO_PSORT_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `PLASO_PSORT_OUTPUT_FORMAT` | `json_line` | psort output module (`-o`) |
| `PLASO_PSORT_OUTPUT_MODULE` | *(empty)* | path of a mounted custom output module; when set, psort runs through `/opt/dxdfir/psort_wrapper.py` |

`image_export`: `PLASO_IMAGE_EXPORT_INPUT_DIR`, `PLASO_IMAGE_EXPORT_OUT_DIR`,
`PLASO_IMAGE_EXPORT_WORK_DIR`, `PLASO_IMAGE_EXPORT_FORCE`,
`PLASO_IMAGE_EXPORT_ARGS`, `PLASO_IMAGE_EXPORT_LOG_LEVEL`, and:

| Variable | Default | Meaning |
|---|---|---|
| `PLASO_IMAGE_EXPORT_VSS` | `1` | export from every VSS store (`--vss-stores all`) |
| `PLASO_IMAGE_EXPORT_FILTER_FILE` | *(empty)* | a mounted plaso filter file (`--filter_file`); takes precedence |
| `PLASO_IMAGE_EXPORT_ARTIFACT_FILTERS` | `WindowsEventLogs` | comma-separated artifact definitions (`--artifact_filters`) |

## Output

`<item>` is the input path relative to the sub-tool's `INPUT_DIR` with path
separators and whitespace folded to `_`. Each sub-tool writes its index
(`<subtool>.jsonl`) last; an item whose index exists is skipped on the next
run unless `FORCE` is set.

| Sub-tool | Per item | `records` |
|---|---|---|
| `log2timeline` | `<OUT_DIR>/<item>/<item>.plaso`, `log2timeline.log`, `log2timeline.jsonl` | storage files produced |
| `psort` | `<OUT_DIR>/<item>/timeline.jsonl`, `psort.log`, `psort.jsonl` | rendered events |
| `image_export` | `<OUT_DIR>/<item>/export/…`, `image_export.log`, `image_export.jsonl` | files exported |

stdout is exactly one JSON object: `tool`, `subtool`, `version`, `status`,
`inputs`, `processed`, `skipped`, `failed`, `records`, `outputs`, `exit`,
`started`, `duration_s`, plus `failures` (item + error) when something failed
and `error` on a config error. Plaso's own output goes to the per-item log
(and to stderr at `debug`); progress and errors go to stderr.

## Exit codes

| Code | Status | Meaning |
|---|---|---|
| 0 | `ok` | every item processed, or already up to date |
| 1 | `nothing` | no input found, or every item failed |
| 2 | `config_error` | no sub-tool named, bad variable, missing or unreadable input, unwritable output |
| 3 | `partial` | at least one item processed and at least one failed |

## Run

```sh
docker build -t get-sybers/plaso:latest -f plaso/Dockerfile .
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/evidence:/input:ro" -v "$PWD/out:/output" \
  -e PLASO_LOG2TIMELINE_PARSERS=win7 \
  get-sybers/plaso:latest log2timeline
docker run --rm … -v "$PWD/out:/input:ro" -v "$PWD/timelines:/output" \
  get-sybers/plaso:latest psort
```

Builds with the repo root as context so `COPY hardening/harden.yml` consumes
the canonical hardener directly. `test/contract_test.sh` builds the image,
runs `log2timeline` over `test/fixtures/` (a small staged tree) and `psort`
over the resulting storage file, and asserts the summary line, the exit
codes, idempotency and the config-error exit; `test/entry_test.py` exercises
the dispatcher's batch loop on the host with stub tools.

## argv pass-through (debug only)

`plaso-entry <tool> <args…>` execs one of plaso's own entry tools
(`log2timeline`, `psort`, `image_export`, `pinfo`, `psteal`, with or without
`.py`) or `python3 /opt/dxdfir/psort_wrapper.py <module.py> <psort args…>`
verbatim, with that tool's exit code; `--version` prints the plaso version and
`--print-contract` prints `contract.yml`. Any other first argument is a
config error (exit 2).
