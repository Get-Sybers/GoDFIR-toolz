# 03 — The environment contract

[← 02 — Tool directory structure](02-tool-directory-structure.md) · [04 — Self-orchestration →](04-self-orchestration.md)

## 3.1 The primary interface

A tool is a black box driven **only** by environment variables and volume
mounts. The environment contract is the primary interface of every tool; it is
the only interface DX_DFIR uses. DX_DFIR passes `-e NAME=VALUE` and `-v` mounts
and nothing else — it never builds argv on the host. argv exists solely as a
debug pass-through ([§4.2](04-self-orchestration.md)).

Every variable has a default declared in `contract.yml`; an environment
variable overrides the default. `anamnesis` is the reference.

## 3.2 Variable naming

- Every variable is prefixed with `<TOOL>_` in SCREAMING_SNAKE_CASE:
  `ANAMNESIS_OUT_DIR`, `GOMFT_INPUT_DIR`. The tool name matches the
  `com.get-sybers.tool` label and the `images.yml` entry.
- A tool MAY use a domain-specific name for its input mount when that reads more
  clearly (e.g. `<TOOL>_MEMORY_DIR` rather than `<TOOL>_INPUT_DIR`), and MUST
  then document it as the input mount in `contract.yml`.

## 3.3 Reserved cross-tool variables

These carry the same meaning in every tool:

| Variable | Meaning | Default |
|---|---|---|
| `<TOOL>_INPUT_DIR` | the read-only evidence tree, recursed | `/input` |
| `<TOOL>_OUT_DIR` | the output root; one subfolder per input item | `/output` |
| `<TOOL>_WORK_DIR` | scratch, writable | `/work` |
| `<TOOL>_FORCE` | `1/true/yes/on` — rerun work that already has valid output | off |
| `<TOOL>_FORMAT` | output format selector where the tool supports several (`json`/`csv`/…) | tool-specific |
| `<TOOL>_LOG_LEVEL` | `error\|warn\|info\|debug`, to stderr only | `info` |

## 3.4 Volume convention

| Mount | Mode | Default path | Meaning |
|---|---|---|---|
| input | **ro** | `/input` (or tool-specific, e.g. `/mem`) | evidence; never written |
| output | rw | `/output` (or `/out`) | one subfolder per input item |
| work | rw (tmpfs) | `/work` or `/tmp` | scratch; survives a `--read-only` rootfs |

The runtime posture DX_DFIR applies, and which the contract assumes:

```
--rm --network none --read-only --tmpfs /work:...uid=2000 \
--cap-drop ALL --security-opt no-new-privileges
```

A tool that needs network access (byakugan's `LOAD_ES_URL` push mode, which
POSTs bundles to an Elastic stack) gates it behind an explicit variable that
defaults to off, and declares `network: optional` in its contract. A cache a
tool needs at runtime is a **baked image dependency or a declared read-write
mount**, never a network fetch: the anamnesis image keeps its MemProcFS PDB
symbol cache in a bind-mounted host directory (contract mount `symbols`) that
persists across runs, and runs fully offline.

## 3.5 Exit-code table

The table is uniform across all tools:

| Code | Meaning |
|---|---|
| 0 | success — work done, or already up-to-date and not forced |
| 1 | nothing produced / nothing to do — no inputs found, empty tree |
| 2 | config error — bad variable, missing required mount, unreadable input |
| 3 | partial — at least one item processed, at least one failed (detail in the summary) |

Code 3 is additive. anamnesis's existing `0/1/2` contract remains valid as is;
it gains `3` for the partial case without changing the meaning of the other
three. The Go tools' argv dialect (`0` ok, `1` usage/fatal, `2` partial) is
remapped in batch mode so that partial becomes `3`; in the argv pass-through
mode ([§4.2](04-self-orchestration.md)) the existing single-item exit meaning
is preserved.

## 3.6 stdout carries one JSON summary line; stderr carries everything else

- **stdout is exactly one line**: a single JSON object, the machine-readable run
  summary. Nothing else appears on stdout, ever. DX_DFIR gates on this line.
- **stderr** carries all human, progress, and log output.
- **Records go to files.** In batch mode every record the tool produces is
  written under `<TOOL>_OUT_DIR`, one subfolder per input item. `<TOOL>_OUT_DIR`
  is therefore effectively required in batch mode: it has a default, the output
  mount is declared `required: true` in every contract, and a missing or
  unwritable output mount is a config error (exit `2`).

Required summary keys (a tool MAY add more):

```json
{"tool":"anamnesis","version":"1.4.0","status":"ok",
 "inputs":3,"processed":3,"skipped":0,"failed":0,
 "outputs":["/out/imgA","/out/imgB","/out/imgC"],
 "exit":0,"started":"2026-09-20T10:00:00Z","duration_s":42.1}
```

`status` ∈ `ok | nothing | config_error | partial` mirrors the exit code.
`contract.yml` declares the exact schema so CI validates the line.

## 3.7 The contract file: `contract.yml`

```yaml
# <tool>/contract.yml — the single source of truth for how DX_DFIR drives this tool.
tool: anamnesis
image: get-sybers/anamnesis:latest
entrypoint: self-orchestrating       # or: multi-tool | argv
description: >
  Pure-Go memory forensics on MemProcFS. Batches over a mounted memory-image
  tree, one output folder per image.

env:
  ANAMNESIS_INPUT_DIR:  {required: false, default: /input,    desc: memory image tree (recursed)}
  ANAMNESIS_OUT_DIR:    {required: false, default: /out,      desc: output root, one folder per image}
  ANAMNESIS_PLUGINS:    {required: false, default: "",        desc: comma-list of collectors; empty = default CAR set}
  ANAMNESIS_FORCE:      {required: false, default: "0",       type: bool, desc: rerun collectors with valid output}

mounts:
  - {name: input,   path: /input,   mode: ro,  env: ANAMNESIS_INPUT_DIR,  required: true}
  - {name: output,  path: /out,     mode: rw,  env: ANAMNESIS_OUT_DIR,    required: true}
  - {name: symbols, path: /opt/anamnesis/lib/Symbols, mode: rw, required: false}

network: none            # PDB symbols come from the bind-mounted persistent cache
exit_codes: {0: success, 1: nothing_produced, 2: config_error, 3: partial}

summary_schema:          # keys on the single stdout JSON line
  required: [tool, version, status, inputs, processed, failed, outputs, exit]

outputs:
  layout: "<OUT_DIR>/<clean image name>/plugins/<plugin>.jsonl + car.db + anamnesis.log"
```

Field reference:

| Key | Meaning |
|---|---|
| `tool` | the tool name; equals `com.get-sybers.tool` and the `images.yml` entry |
| `image` | the image reference DX_DFIR runs |
| `entrypoint` | `self-orchestrating` (one binary, no-arg batch), `multi-tool` (dispatcher, [§4.3](04-self-orchestration.md)), or `argv` (declared deviation, streaming tools) |
| `env` | every variable: `required`, `default`, optional `type`, `desc` |
| `mounts` | every mount: `name`, `path`, `mode`, the variable that overrides the path, `required` |
| `network` | `none` (default) or `optional` (gated by a declared variable) |
| `exit_codes` | the table of §3.5, restated so a reader of the contract alone sees it |
| `summary_schema` | the required keys of the stdout summary line |
| `outputs.layout` | the on-disk layout under the output root |

For a `multi-tool` contract, `env` is grouped per sub-tool
(`SIGNATURES_YARA_*`, `SIGNATURES_SURICATA_*`, …) and each sub-tool carries its
own `outputs.layout`.

Every `contract.yml` validates against `hardening/contract.schema.yml`
([§6.5](06-verification-gate.md)).

## 3.8 The consumer's driver is data-driven

DX_DFIR's driver reads `env` and `mounts` and constructs the `docker run` purely
from that data — no per-tool host code, no argv building. Adding a tool to a
lane is "read its `contract.yml`."

## 3.9 `--print-contract`

A tool MAY implement `--print-contract`, writing its contract to stdout so the
interface is discoverable from the binary alone. It is a debug affordance; the
checked-in `contract.yml` is authoritative, and CI asserts the two agree when
the flag exists.

[← 02 — Tool directory structure](02-tool-directory-structure.md) · [04 — Self-orchestration →](04-self-orchestration.md)
