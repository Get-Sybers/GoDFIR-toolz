# `get-sybers/byakugan` — the Byakugan MITRE CAR engine

The external [Byakugan](https://github.com/Get-Sybers/byakugan) MITRE CAR
engine in one hardened python image (plus its static Go parse binary,
`byakugan-parse`). The engine is cloned recursively at build time at
the `BYAKUGAN_REF` pin baked into the Dockerfile — its nested
`third_party/car` + `attack-datasources` submodules rebuild the object model —
so the consuming DX_DFIR checkout holds nothing but how it invokes this image.
The Dockerfile's `ARG BYAKUGAN_REF` default is the ONE place the engine
version is set (`--build-arg` overrides it), and the built image carries it as
`com.get-sybers.engine-ref`. Python (`python3` + `python3-yaml`)
stays as a declared deviation, and `ca-certificates` stays too (TLS trust for
`load`'s push mode and the exchange's OpenCTI wire); every shell, ansible,
apt, pip, sudo and setuid binaries are gone and the image runs as uid 2000.
The dispatcher carries nine sub-tools: `build`, `timeline`, `verify`,
`car-vocab`, `load`, and the STIX/CTI exchange — `stix-export`,
`stix-behaviour`, `cti-pull`, `cti-sightings` (the engine's
`docs/STIX-Exchange.md` is the design doc).

The ENTRYPOINT is the engine's own dispatcher, `byakugan.cli`
(`byakugan-entry.py` is a shim that delegates to it), which selects the
operation on its first argument ([`contract.yml`](contract.yml)). The
sub-tool set and the processed-layout discovery are the engine's, so they
follow the cloned ref:

```
byakugan build        processed evidence tree -> the materialised CAR JSONL, one set per source
byakugan timeline     a car tree               -> timeline.jsonl
byakugan verify       a materialised car tree  -> the CAR correctness gate (verify.txt)
byakugan car-vocab    the car_action vocabulary, one JSON line on stdout
byakugan load         a materialised car tree  -> the DX_DFIR Elastic stack (bundles, or pushed)
byakugan stix-export     detection hits          -> a STIX 2.1 bundle (sightings + indicators, projections merged)
byakugan stix-behaviour  detections x a car tree -> behaviour sightings over spindle-keyed observations
byakugan cti-pull        OpenCTI indicators      -> the cti-* Elasticsearch _bulk copy (no input mount)
byakugan cti-sightings   indicator-match alerts  -> sightings of the platform's own indicators
```

## Input

- `build` walks `BYAKUGAN_BUILD_INPUT_DIR` (default `/input`, mounted read-only) as the engine's `--batch` root: every processed source under it is one item. The engine discovers the framework layouts (`windows_logs/<item>/goevtx.jsonl`, `jsonl/<source>/timeline.jsonl`, `godfir-toolz/<tool>/<item>/`, …).
- `timeline` reads `BYAKUGAN_TIMELINE_INPUT_DIR` (default `/input`): a source's car directory, or a tree of them to aggregate — one item.
- `verify` reads `BYAKUGAN_VERIFY_INPUT_DIR` (default `/input`, mounted read-only): a materialised CAR tree — every directory holding `car_<object>.jsonl` / `car_relationships.jsonl` under it is one item.
- `car-vocab` reads nothing.
- `load` reads `BYAKUGAN_LOAD_INPUT_DIR` (default `/input`, mounted read-only): a materialised CAR tree — every directory holding `car_<object>.jsonl` / `car_relationships.jsonl` (and, from a `BYAKUGAN_BUILD_DERIVE` build, `car_inferred.jsonl` and `car_content.jsonl`) under it is one source, bulk-loaded into the `logs-car.*` data streams.
- `stix-export` reads `BYAKUGAN_STIX_EXPORT_INPUT_DIR` (default `/input`, read-only): EVERY regular file under it is a hits input (detect JSONL, an ES `_search` response, alert documents); `_BUNDLES_DIR` names a materialised CAR tree whose `stix_bundle.json` projections pass through; `_RULES_DIR` (default `/rules`) is the rules-as-code resolving indicator patterns — the engine repo ships the canonical set ([its `rules/`](https://github.com/Get-Sybers/byakugan/blob/main/rules/README.md), baked there from the clone and build-validated), and the `rules` mount overrides it with an operator set.
- `stix-behaviour` reads `BYAKUGAN_STIX_BEHAVIOUR_INPUT_DIR` (default `/input`, read-only): a materialised CAR tree; `_DETECTIONS_DIR` (required) is the detection-lane output dir (`suricata/`, `hayabusa/`, `yara/`).
- `cti-pull` reads nothing on disk (the one input-less sub-tool) — OpenCTI over the wire, or `_FROM_BUNDLE` for the offline re-normalise.
- `cti-sightings` reads `BYAKUGAN_CTI_SIGHTINGS_INPUT_DIR` (default `/input`, read-only): EVERY regular file under it is an alerts input.

## The baked rules-as-code (`/rules`)

The image carries the Elastic detection rules-as-code at `/rules`, baked
from the engine clone itself — the byakugan repo ships them
([`rules/`](https://github.com/Get-Sybers/byakugan/blob/main/rules/README.md) at the `BYAKUGAN_REF` pin): the pinned top-level
detection set (one YAML per rule, ES|QL/EQL, each with its
tagged-evidence-line contract), the `car-detections/` lookup-index contract
and the `cti/` indicator-match rule. The engine's own `rules/validate.py`
gates them during the build — a malformed rule, or any drift from the pinned
id set, fails the image — and the engine's test suite holds the same gate
plus the cti-* template cross-checks. At run time `stix-export` resolves
indicator patterns from them; mounting the `rules` mount shadows the baked
set with an operator one.

## Env

| Variable | Default | Meaning |
|---|---|---|
| `BYAKUGAN_BUILD_INPUT_DIR` | `/input` | the processed-evidence tree (`--batch`) |
| `BYAKUGAN_BUILD_OUT_DIR` | `/output` | the car/ root, one materialised source each (`--out`): the `car_*.jsonl` set |
| `BYAKUGAN_BUILD_FORCE` | `0` | `1/true/yes/on`: rebuild sources whose `car_relationships.jsonl` already exists (`--force`) |
| `BYAKUGAN_BUILD_DERIVE` | `0` | also run the derived relationship pass into `car_inferred.jsonl` (`--derive`) |
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
| `BYAKUGAN_LOAD_INPUT_DIR` | `/input` | the materialised CAR tree to load (`car_<object>.jsonl` + `car_relationships.jsonl` per source; `car_inferred.jsonl` and `car_content.jsonl` when the build ran `BYAKUGAN_BUILD_DERIVE`) |
| `BYAKUGAN_LOAD_OUT_DIR` | `/output` | where the `elastic/` bulk bundles, `manifest.json` and the load report are written |
| `BYAKUGAN_LOAD_ES_URL` | *(empty)* | empty = bundle mode, no network; set = push mode, POSTs the bundles to this Elasticsearch base URL over HTTPS — the explicit network opt-in behind this contract's `network: optional` |
| `BYAKUGAN_LOAD_ES_API_KEY` | *(empty)* | Elasticsearch API key for push mode |
| `BYAKUGAN_LOAD_ES_USER` | *(empty)* | Elasticsearch basic-auth username for push mode, paired with `_ES_PASSWORD`/`_ES_PASSWORD_FILE` |
| `BYAKUGAN_LOAD_ES_PASSWORD` | *(empty)* | Elasticsearch basic-auth password for push mode; ignored when `_ES_PASSWORD_FILE` is set |
| `BYAKUGAN_LOAD_ES_PASSWORD_FILE` | *(empty)* | file holding the basic-auth password for push mode; wins over `_ES_PASSWORD` |
| `BYAKUGAN_LOAD_ES_CA_FILE` | `/certs/ca/ca.crt` | CA bundle to verify the Elasticsearch TLS certificate in push mode; the default applies only when the `certs` mount is present, otherwise the system trust store is used |
| `BYAKUGAN_LOAD_KIBANA_URL` | *(empty)* | empty = skip the Kibana saved-objects import; set = also import the engine's rendered saved objects, push mode only, requires `_SETUP` |
| `BYAKUGAN_LOAD_NAMESPACE` | `default` | the Elastic data-stream namespace: `logs-car.<object>-<namespace>`, `logs-car.rel-<namespace>`, `logs-car.inferred-<namespace>`, `logs-car.content-<namespace>` |
| `BYAKUGAN_LOAD_SETUP` | `0` | `1/true/yes/on`: apply the rendered index/component templates before loading (and, with `_KIBANA_URL` set, import the Kibana saved objects), push mode only |
| `BYAKUGAN_LOAD_FORCE` | `0` | `1/true/yes/on`: re-render bundles and, in push mode, re-push even when the manifest/load report already show the run complete |
| `BYAKUGAN_LOAD_ARGS` | *(empty)* | extra `byakugan.elastic.load` argv |
| `BYAKUGAN_LOAD_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_OPENCTI_URL` | *(empty)* | the OpenCTI endpoint, shared by the exchange sub-tools — with `_PUSH` (or a pull without `_FROM_BUNDLE`) the network opt-in behind `network: optional` |
| `BYAKUGAN_OPENCTI_TOKEN` | *(empty)* | the OpenCTI bearer token — a secret: env only (the lane's `secret_env`), never argv |
| `BYAKUGAN_OPENCTI_CONNECTOR_ID` | *(empty)* | the OpenCTI connector id for pushes; a deterministic default otherwise |
| `BYAKUGAN_STIX_EXPORT_INPUT_DIR` | `/input` | the hits tree — every regular file under it is a hits input |
| `BYAKUGAN_STIX_EXPORT_OUT_DIR` | `/output` | where `bundle.json` is written |
| `BYAKUGAN_STIX_EXPORT_BUNDLES_DIR` | *(empty)* | a materialised CAR tree whose `stix_bundle.json` projections pass through |
| `BYAKUGAN_STIX_EXPORT_RULES_DIR` | `/rules` | the rules-as-code (pattern resolution); the engine repo's set bakes at `/rules`, the `rules` mount overrides it |
| `BYAKUGAN_STIX_EXPORT_CASE` | *(empty)* | the case id scoping observation ids (default: the hits' run id) |
| `BYAKUGAN_STIX_EXPORT_TLP` | *(empty)* | `white|green|amber|red|none` (engine default: `amber`) |
| `BYAKUGAN_STIX_EXPORT_CONFIG` | *(empty)* | a JSON/YAML exchange config file |
| `BYAKUGAN_STIX_EXPORT_PUSH` | `0` | `1/true/yes/on`: also push the bundle to OpenCTI |
| `BYAKUGAN_STIX_EXPORT_ARGS` | *(empty)* | extra exchange argv |
| `BYAKUGAN_STIX_EXPORT_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_STIX_BEHAVIOUR_INPUT_DIR` | `/input` | the materialised CAR tree the join reads |
| `BYAKUGAN_STIX_BEHAVIOUR_OUT_DIR` | `/output` | where `behaviour-sightings.json` is written |
| `BYAKUGAN_STIX_BEHAVIOUR_DETECTIONS_DIR` | — | **required**: the detection-lane output dir (bind it read-only) |
| `BYAKUGAN_STIX_BEHAVIOUR_CASE` | — | **required**: the case id scoping sighting/observation ids |
| `BYAKUGAN_STIX_BEHAVIOUR_TLP` | *(empty)* | `white|green|amber|red|none` (engine default: `amber`) |
| `BYAKUGAN_STIX_BEHAVIOUR_PRODUCER` | *(empty)* | the producer identity name (engine default: `DX_DFIR`) |
| `BYAKUGAN_STIX_BEHAVIOUR_ATTACK_INDEX` | *(empty)* | an ATT&CK index/bundle path (empty = the committed index) |
| `BYAKUGAN_STIX_BEHAVIOUR_ARGS` | *(empty)* | extra exchange argv |
| `BYAKUGAN_STIX_BEHAVIOUR_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_CTI_PULL_OUT_DIR` | `/output` | where `cti-bulk.ndjson` is written (no input mount needed) |
| `BYAKUGAN_CTI_PULL_SINCE` | *(empty)* | incremental: only indicators modified after this ISO-8601 watermark |
| `BYAKUGAN_CTI_PULL_PAGE_SIZE` | *(empty)* | indicators per GraphQL page (engine default 200) |
| `BYAKUGAN_CTI_PULL_MAX_PAGES` | *(empty)* | stop after this many pages (safety valve) |
| `BYAKUGAN_CTI_PULL_INDEX` | *(empty)* | the `cti-*` index the bulk lines target (engine default: `cti-opencti`) |
| `BYAKUGAN_CTI_PULL_FROM_BUNDLE` | *(empty)* | offline re-normalise of an already-pulled bundle at this path |
| `BYAKUGAN_CTI_PULL_BUNDLE_OUT` | *(empty)* | also keep the pulled indicator bundle here |
| `BYAKUGAN_CTI_PULL_CONFIG` | *(empty)* | a JSON/YAML exchange config file |
| `BYAKUGAN_CTI_PULL_ARGS` | *(empty)* | extra exchange argv |
| `BYAKUGAN_CTI_PULL_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |
| `BYAKUGAN_CTI_SIGHTINGS_INPUT_DIR` | `/input` | the alerts tree — every regular file under it is an alerts input |
| `BYAKUGAN_CTI_SIGHTINGS_OUT_DIR` | `/output` | where `sightings.json` is written |
| `BYAKUGAN_CTI_SIGHTINGS_CASE` | *(empty)* | the case id scoping the sighting ids (default: the alerts' rule execution id) |
| `BYAKUGAN_CTI_SIGHTINGS_TLP` | *(empty)* | `white|green|amber|red|none` (engine default: `amber`) |
| `BYAKUGAN_CTI_SIGHTINGS_CONFIG` | *(empty)* | a JSON/YAML exchange config file |
| `BYAKUGAN_CTI_SIGHTINGS_PUSH` | `0` | `1/true/yes/on`: also push the sightings back to OpenCTI |
| `BYAKUGAN_CTI_SIGHTINGS_ARGS` | *(empty)* | extra exchange argv |
| `BYAKUGAN_CTI_SIGHTINGS_LOG_LEVEL` | `info` | `error|warn|info|debug`, stderr only |

## Output

| Sub-tool | Output | `records` |
|---|---|---|
| `build` | `<OUT_DIR>/<source>/car_<object>.jsonl` (13 CAR objects, populated ones only) + `car_relationships.jsonl` (always written, even empty — the done/skip marker) + `sources.yaml` — the materialised JSONL tree is the build's ONLY on-disk product, no `car.db` or `superset.db` anywhere; also `car_inferred.jsonl` and `car_content.jsonl` with `DERIVE`, `stix_bundle.json` with `STIX`; this is what `timeline`/`verify`/`load` and downstream ingest read; a source whose `car_relationships.jsonl` already exists is skipped unless `FORCE` | CAR events |
| `timeline` | `<OUT_DIR>/timeline.jsonl`; skipped when it exists unless `FORCE` | timeline entries |
| `verify` | `<OUT_DIR>/verify.txt` — the gate report (every check, the tally, the verdict), also on stderr; written (replacing any earlier report) only when at least one materialised CAR source is found (status `ok` or `failed`) — an empty tree (status `nothing`) writes no report | CAR rows read |
| `car-vocab` | stdout: `{object: [car_actions]}` as one JSON line | — |
| `load` | `<OUT_DIR>/elastic/*.ndjson` — one Elasticsearch `_bulk` NDJSON bundle per `logs-car.*` data stream, deterministic per-document `_id`; `<OUT_DIR>/elastic/manifest.json` — per-stream counts, ids and each bundle's sha256; `<OUT_DIR>/load.txt` — the load report, also on stderr; bundle mode only renders these (no network), push mode also POSTs them (409 = `already_present`) and verifies per-stream counts | documents bundled or indexed |

For `build`, `timeline`, `verify` and `load` stdout is exactly one JSON
object: `tool`, `subtool`, `version`, `engine_ref`, `status`, `inputs`,
`processed`, `skipped`, `failed`, `records`, `outputs`, `exit`, `started`,
`duration_s`, `engine` (the engine's own summary: the per-source result list,
the entries/objects/relationships counts, verify's
`passed`/`failed`/`not_exercised`/`os_families_covered`/`os_families_total`
tally, or load's per-stream document counts and its mode `bundle`/`push`),
plus `failures` when a source or a check failed and `error` on a config
error. The engine's own stdout is captured into `engine`; progress, errors
and the verify/load reports go to stderr.

## Exit codes

| Code | Status | Meaning |
|---|---|---|
| 0 | `ok` | every source processed, or already up to date; `verify`: the gate passed |
| 1 | `nothing` or `failed` | no output produced: no source produced events, no `car_relationships.jsonl` written, every source failed, or no materialised CAR under the input dir (`nothing`) — or, `verify` only, the gate failed (`failed`: `failed` counts the failed checks, `failures` names them) |
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
docker run --rm … -v "$PWD/car:/input:ro" -v "$PWD/elastic-out:/output" \
  get-sybers/byakugan:latest load
# the exchange: hits -> a validated STIX 2.1 bundle (the baked /rules set;
# add -v "$PWD/rules:/rules:ro" to override it with an operator set)
docker run --rm … -v "$PWD/hits:/input:ro" \
  -v "$PWD/exchange:/output" get-sybers/byakugan:latest stix-export
# detections joined to CAR entities -> behaviour sightings
docker run --rm … -v "$PWD/car:/input:ro" -v "$PWD/detections:/detections:ro" \
  -v "$PWD/exchange:/output" -e BYAKUGAN_STIX_BEHAVIOUR_DETECTIONS_DIR=/detections \
  -e BYAKUGAN_STIX_BEHAVIOUR_CASE=CASE-17 get-sybers/byakugan:latest stix-behaviour
```

Builds with the repo root as context so `COPY hardening/harden.yml` consumes
the canonical hardener directly. `test/contract_test.sh` builds the image,
runs `build` and `verify` over an empty tree, `car-vocab`, and `load` in
bundle mode over an empty tree, and asserts the summary line, the exit codes,
idempotency and the config-error exit.

## Loading into Elastic

`load` bulk-loads a materialised CAR tree (the output of `build`) into the
DX_DFIR Elastic stack as `logs-car.<object>-<namespace>` (13 CAR objects),
`logs-car.rel-<namespace>` (relationship instances),
`logs-car.inferred-<namespace>` (inferred nodes) and
`logs-car.content-<namespace>` (content nodes). Like every sub-tool it runs
once and exits — never a daemon.

- **Bundle mode (default, offline)** — `BYAKUGAN_LOAD_ES_URL` unset: renders
  ready-to-POST Elasticsearch `_bulk` NDJSON bundles plus a manifest
  (per-stream counts, deterministic document ids, sha256 per bundle) under
  `<OUT_DIR>/elastic/`. No network. This is the air-gap path: an operator
  ships the bundles and POSTs them stack-side.
- **Push mode** — `BYAKUGAN_LOAD_ES_URL` set: POSTs the same bundles itself
  over HTTPS, treating document-already-exists (409) as `already_present`
  (the deterministic ids make a re-load an idempotent no-op), then verifies
  per-stream counts. With `BYAKUGAN_LOAD_SETUP=1` it first applies the
  engine's rendered index/component templates, and, when
  `BYAKUGAN_LOAD_KIBANA_URL` is also set, imports the engine's Kibana saved
  objects too.

```sh
# offline bundle mode: no network, ships the bundles for someone else to POST
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
  -v "$PWD/car:/input:ro" -v "$PWD/elastic-out:/output" \
  get-sybers/byakugan:latest load

# push mode: this container reaches the Elastic stack directly over TLS
docker run --rm --cap-drop ALL --security-opt no-new-privileges \
  --network <stack network> \
  --read-only --tmpfs /tmp:rw,uid=2000,gid=2000 \
  -v "$PWD/car:/input:ro" -v "$PWD/elastic-out:/output" \
  -v "$PWD/certs:/certs:ro" \
  -e BYAKUGAN_LOAD_ES_URL=https://elasticsearch:9200 \
  -e BYAKUGAN_LOAD_ES_API_KEY="$ES_API_KEY" \
  -e BYAKUGAN_LOAD_SETUP=1 \
  get-sybers/byakugan:latest load
```

## argv pass-through (debug only)

`byakugan timeline <car_dir> [--out …] [--host …] [--after …] [--before …]
[--objects-only|--edges-only]`, `byakugan verify [car_dir]` and `byakugan
[build] --in FILE --out DIR [--host …] [--artefacts …] | --batch DIR [--out DIR]
[--force] [--derive] [--stix]` run the engine's own CLI with its own stdout and
exit code;
`--version` prints the image version and the pinned engine ref, and
`--print-contract` prints `contract.yml`.
