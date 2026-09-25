# `get-sybers/godaemonhunter` — the Linux matrix as one structured binary

Every daemon parser of the Linux matrix ([docs/linux](../docs/linux/README.md),
decision 15) embedded as a sub-tool in a single static binary, plus **`hunt`**
— the layered one-shot:

1. **Layer 1** runs first — `gohost`, `gousers`, `gonetwork` — and their
   output *is* the image's knowledge store (identity, naming, layout:
   uid/gid→name, hostname, machine-id, timezone, mounts).
2. **Layer 2** — `gojournal`, `goauditd`, `gowtmp`, `gosyslog`, `gounit`,
   `gocron`, `goshell`, `gotrash`, `goctl` — then runs with that store
   mounted, so every record comes out enriched per decision 14: resolved
   names **beside** the native values (`UIDName` next to `UID`, never
   replacing it), the `Host` block stamped, naive syslog timestamps placed
   in the image's own zone. Correlation and joining stay byakugan's job.

One binary, one run, one structured output tree, one JSON summary line.
This is the multi-tool dispatcher shape of
[docs/framework/04 §4.3](../docs/framework/04-self-orchestration.md)
(the plaso and signatures precedent); the per-tool images remain the
pipeline's granular units — godaemonhunter adds the packaging, not a new
parser.

```
godaemonhunter hunt          the layered run, GODAEMONHUNTER_* driven
godaemonhunter <subtool>     one parser's env-driven batch mode, under its
                             canonical <SUBTOOL>_* block
godaemonhunter --version | --print-contract
```

Sub-tools: `hunt gohost gousers gonetwork gojournal goauditd gowtmp gosyslog
gounit gocron goshell gotrash goctl`.

## Input

The evidence tree at `GODAEMONHUNTER_INPUT_DIR` (default `/input`), mounted
read-only and shared by every sub-run: a staged `linux-core` materialise
tree, a mounted root filesystem, or any directory laid out like one. A
`materialise.jsonl` manifest at the input root, when present, is joined onto
every record as `Origin`/`Snapshot`/`Residue` — rule-2 provenance comes from
the stage, never from the parser.

## Env (`hunt`)

| Variable | Default | Meaning |
|---|---|---|
| `GODAEMONHUNTER_INPUT_DIR` | `/input` | evidence tree, recursed read-only, shared by every sub-run |
| `GODAEMONHUNTER_OUT_DIR` | `/output` | output root: `knowledge/` (Layer 1) + one `<subtool>/` tree per daemon parser |
| `GODAEMONHUNTER_KNOWLEDGE_DIR` | *(empty)* | override the knowledge store location; empty = `<OUT_DIR>/knowledge`, built by Layer 1 in the same run |
| `GODAEMONHUNTER_WORK_DIR` | `/work` | scratch (writable tmpfs), shared by every sub-run |
| `GODAEMONHUNTER_FORCE` | `0` | `1/true/yes/on`: rerun items that already have valid output, in every sub-run |
| `GODAEMONHUNTER_LOG_LEVEL` | `info` | `error\|warn\|info\|debug`, stderr only, applied to every sub-run |

A single-parser sub-run (`godaemonhunter gowtmp`) ignores the
`GODAEMONHUNTER_*` block and reads that tool's canonical `<SUBTOOL>_*`
variables instead, exactly as its own contract declares them
([gowtmp/contract.yml](../gowtmp/contract.yml) and siblings).

## Output

`hunt` writes one structured tree under `GODAEMONHUNTER_OUT_DIR`:

```
<OUT_DIR>/knowledge/<item>/{gohost,gousers,gonetwork}.jsonl   Layer 1 = the store
<OUT_DIR>/<subtool>/<item>/<subtool>.jsonl                    per daemon parser, enriched
```

and prints **one** aggregate JSON summary line with every sub-tool's own
summary embedded under `subtools` (plus `knowledge_dir`, the roll-up
counters, `status`, `exit`). A single-parser sub-run prints that parser's
ordinary summary line and writes as its own contract declares.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success — every sub-run succeeded (or was already up-to-date) |
| 1 | nothing produced — no sub-run found anything to parse |
| 2 | config error — any sub-run hit a bad variable / missing input / unwritable output |
| 3 | partial — work was produced but at least one item or sub-run failed |

Roll-up order: any config error → 2; else any partial → 3; else any work → 0;
else 1.

## Run

```sh
docker build -t get-sybers/godaemonhunter:latest -f godaemonhunter/Dockerfile .
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/godaemonhunter:latest hunt
```

(Build from the **repo root**: the image copies the sibling `pinfo/` module
and all twelve parser modules.)

## argv pass-through

`godaemonhunter <subtool>` runs that parser's env-driven batch mode only.
The per-tool `-f FILE | -d DIR | --tar` argv debug modes stay with the
standalone binaries (`gowtmp`, `gojournal`, …) — the one binary keeps one
run shape. `--version` prints the version; `--print-contract` prints
[`contract.yml`](contract.yml).
