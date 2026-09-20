# `get-sybers/byakugan` — the Byakugan MITRE CAR engine

The external [Byakugan](https://github.com/Get-Sybers/byakugan) MITRE CAR
engine in one hardened python image (plus its static Go parse binary,
`byakugan-parse`). The engine is cloned recursively at build time at
`--build-arg BYAKUGAN_REF` — its nested `third_party/car` +
`attack-datasources` submodules rebuild the object model — so the consuming
DX_DFIR checkout holds nothing but how it invokes this image. DX_DFIR passes
its `sources.yml` pin (the default here is `main`), and the built image
carries it as `com.get-sybers.engine-ref`. Python (`python3` + `python3-yaml`)
stays as a declared deviation; every shell, ansible, apt, pip, sudo and setuid
binaries are gone and the image runs as uid 2000.

The ENTRYPOINT is the engine's own dispatcher, `byakugan.cli`
(`byakugan-entry.py` is a shim that delegates to it), which selects the
operation on its first argument ([`contract.yml`](contract.yml)). The
sub-tool set and the processed-layout discovery are the engine's, so they
follow the cloned ref:

```
byakugan build        processed evidence tree -> one car.db per source
byakugan timeline     a car tree               -> timeline.jsonl
byakugan verify       a materialised car tree  -> the CAR correctness gate (verify.txt)
byakugan car-vocab    the car_action vocabulary, one JSON line on stdout
```

## Input

- `build` walks `BYAKUGAN_BUILD_INPUT_DIR` (default `/input`, mounted read-only) as the engine's `--batch` root: every processed source under it is one item. The engine discovers the framework layouts (`windows_logs/<item>/goevtx.jsonl`, `jsonl/<source>/timeline.jsonl`, `godfir-toolz/<tool>/<item>/`, …).
- `timeline` reads `BYAKUGAN_TIMELINE_INPUT_DIR` (default `/input`): a source's car directory, or a tree of them to aggregate — one item.
- `verify` reads `BYAKUGAN_VERIFY_INPUT_DIR` (default `/input`, mounted read-only): a materialised CAR tree — every directory holding `car_<object>.jsonl` / `car_relationships.jsonl` under it is one item.
- `car-vocab` reads nothing.

## Env

| Variable | Default | Meaning |
|---|---|---|
| `BYAKUGAN_BUILD_INPUT_DIR` | `/input` | the processed-evidence tree (`--batch`) |
| `BYAKUGAN_BUILD_OUT_DIR` | `/output` | the car/ root, one store per source (`--out`) |
| `BYAKUGAN_BUILD_FORCE` | `0` | `1/true/yes/on`: rebuild sources whose `car.db` exists (`--force`) |
| `BYAKUGAN_BUILD_DERIVE` | `0` | also run the derived relationship pass into `superset.db` (`--derive`) |
| `BYAKUGAN_BUILD_STIX` | `0` | also derive the STIX 2.1 bundle (`--stix`) |
| `BYAKUGAN_BUILD_ARGS` | *(empty)* | extra `byakugan.pipeline` argv |
| `BYAKUGAN_BUILD_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_TIMELINE_INPUT_DIR` | `/input` | a car directory or a tree of them |
| `BYAKUGAN_TIMELINE_OUT_DIR` | `/output` | where `timeline.jsonl` is written |
| `BYAKUGAN_TIMELINE_FORCE` | `0` | `1/true/yes/on`: rewrite an existing `timeline.jsonl` |
| `BYAKUGAN_TIMELINE_HOST` | *(empty)* | only events whose `source_host` matches (`--host`) |
| `BYAKUGAN_TIMELINE_AFTER` | *(empty)* | only events at/after this ISO-8601 timestamp (`--after`) |
| `BYAKUGAN_TIMELINE_BEFORE` | *(empty)* | only events at/before this ISO-8601 timestamp (`--before`) |
| `BYAKUGAN_TIMELINE_ARGS` | *(empty)* | extra `byakugan.timeline` argv (`--objects-only`, `--edges-only`) |
| `BYAKUGAN_TIMELINE_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_VERIFY_INPUT_DIR` | `/input` | the materialised CAR tree the gate reads |
| `BYAKUGAN_VERIFY_OUT_DIR` | `/output` | where the report `verify.txt` is written; the default applies only when `/output` is a mounted, writable directory — otherwise the report goes to stderr alone and the gate still runs |
| `BYAKUGAN_VERIFY_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |

## Output

| Sub-tool | Output | `records` |
|---|---|---|
| `build` | `<OUT_DIR>/<source>/car.db` (+ `superset.db` with `DERIVE`, `stix_bundle.json` with `STIX`); a source whose `car.db` exists is skipped unless `FORCE` | CAR events |
| `timeline` | `<OUT_DIR>/timeline.jsonl`; skipped when it exists unless `FORCE` | timeline entries |
| `verify` | `<OUT_DIR>/verify.txt` — the gate report (every check, the tally, the verdict), also on stderr; written (replacing any earlier report) only when at least one materialised CAR source is found (status `ok` or `failed`) — an empty tree (status `nothing`) writes no report | CAR rows read |
| `car-vocab` | stdout: `{object: [car_actions]}` as one JSON line | — |

For `build`, `timeline` and `verify` stdout is exactly one JSON object:
`tool`, `subtool`, `version`, `engine_ref`, `status`, `inputs`, `processed`,
`skipped`, `failed`, `records`, `outputs`, `exit`, `started`, `duration_s`,
`engine` (the engine's own summary: the per-source result list, the
entries/objects/relationships counts, or verify's
`passed`/`failed`/`not_exercised`/`os_families_covered`/`os_families_total`
tally), plus `failures` when a source or a check failed and `error` on a
config error. The engine's own stdout is captured into `engine`; progress,
errors and the verify report go to stderr.

## Exit codes

| Code | Status | Meaning |
|---|---|---|
| 0 | `ok` | every source processed, or already up to date; `verify`: the gate passed |
| 1 | `nothing` or `failed` | no output produced: no source produced events, no `car.db` found, every source failed, or no materialised CAR under the input dir (`nothing`) — or, `verify` only, the gate failed (`failed`: `failed` counts the failed checks, `failures` names them) |
| 2 | `config_error` | no sub-tool named, bad variable, missing or unreadable input, unwritable output, an engine argument error |
| 3 | `partial` | at least one source processed and at least one failed |

Exit 1 is the general non-success of a run that produced no output; `verify`
uses it for both an empty tree and a failed gate, so a consumer reads the
summary `status` — never the code alone — to tell `nothing` from `failed`.
`ok` is the only pass.

## Run

```sh
docker build -t get-sybers/byakugan:latest \
  --build-arg BYAKUGAN_REF=<40-hex sha> -f byakugan/Dockerfile .
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
  -v "$PWD/processed:/input:ro" -v "$PWD/car:/output" \
  -e BYAKUGAN_BUILD_DERIVE=1 \
  get-sybers/byakugan:latest build
docker run --rm … -v "$PWD/car:/input:ro" -v "$PWD/timeline:/output" \
  get-sybers/byakugan:latest timeline
docker run --rm … -v "$PWD/car:/input:ro" -v "$PWD/car:/output" \
  get-sybers/byakugan:latest verify
docker run --rm get-sybers/byakugan:latest car-vocab
```

Builds with the repo root as context so `COPY hardening/harden.yml` consumes
the canonical hardener directly. `test/contract_test.sh` builds the image,
runs `build` and `verify` over an empty tree and `car-vocab`, and asserts the
summary line, the exit codes, idempotency and the config-error exit.

## argv pass-through (debug only)

`byakugan timeline <car_dir> [--out …] [--host …] [--after …] [--before …]
[--objects-only|--edges-only]`, `byakugan verify [car_dir]` and `byakugan
[build] --in FILE --out DIR [--host …] [--artefacts …] | --batch DIR [--out DIR]
[--force] [--derive] [--stix]` run the engine's own CLI with its own stdout and
exit code;
`--version` prints the image version and the pinned engine ref, and
`--print-contract` prints `contract.yml`.
