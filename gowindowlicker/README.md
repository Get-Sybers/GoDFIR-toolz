# `get-sybers/gowindowlicker` — the Windows artefact dozen as one structured binary

Every Windows Go parser is a package of this module and a sub-tool of this
single static binary — the godaemonhunter shape
([docs/linux](../docs/linux/README.md) decision 16) applied to the Windows
side, and the multi-tool dispatcher shape of
[docs/framework/04 §4.3](../docs/framework/04-self-orchestration.md) (the
plaso, signatures and godaemonhunter precedent). It is the **only** shipped
shape: the parsers live here as packages, and there are no standalone
per-parser binaries or images. gowindowlicker *is* the Windows tool.

**Tool names, `<SUBTOOL>_*` env blocks, record shapes and record-file names
are unchanged** — byakugan and the pipeline see the same records
(`goprefetch.jsonl`, `gore.jsonl`, `goevtx.jsonl`, …); only the packaging is
one. One binary, one image, one run, one structured output tree, one JSON
summary line.

## The sub-tool is the parameter

```
gowindowlicker                     every parser (the default sweep)
gowindowlicker <subtool>           one parser's env-driven batch mode, under
                                   its canonical <SUBTOOL>_* block
gowindowlicker <subtool> <args>    that parser's argv debug pass-through
gowindowlicker --version | --print-contract
```

`lick` stays accepted as the explicit word for the default run. Unlike
godaemonhunter there is **no stream vocabulary yet**: docs/linux decision 17
admits a model word only when byakugan maps feed on a parser here, and today
byakugan consumes `goevtx`, `goprefetch`, `goese`, `gojle` and `gore`
directly while the other artefact classes arrive through plaso's `l2t_*`
maps — a Windows stream word would strand most of the matrix. There is also
no layered knowledge store: these parsers read self-contained artefacts, not
a host's own record-keeping, so every sub-run is independent and the sweep
has no layers.

Sub-tools — each one a package of this module with its own README
documenting what it parses, its `<SUBTOOL>_*` env block and its record
shapes:

| Sub-tool | Parses |
|---|---|
| [`goprefetch`](prefetch/README.md) | Windows Prefetch: XP→Win11 `.pf`, MAM decompression in pure Go |
| [`goese`](ese/README.md) | ESE databases: SRUM `SRUDB.dat` (IdMap/SID enrichment) and SUM `Current.mdb` |
| [`gorb`](rb/README.md) | Recycle Bin `$I` records, v1 + v2 |
| [`gomft`](mft/README.md) | raw `$MFT`: MACB from 0x10 + 0x30, ADS, full paths |
| [`goamcache`](amcache/README.md) | `Amcache.hve`, with `.LOG` replay |
| [`goappcompat`](appcompat/README.md) | ShimCache from SYSTEM hives, with `.LOG` replay |
| [`goevtx`](evtx/README.md) | `.evtx` event logs → the JSON record shape byakugan's evtx maps consume |
| [`gore`](re/README.md) | batch-driven registry key/value dumps (bundled `/batch/default.reb`) |
| [`gosbe`](sbe/README.md) | ShellBags (BagMRU) with reconstructed paths |
| [`gole`](le/README.md) | `.lnk` shell links |
| [`gojle`](jle/README.md) | AutomaticDestinations jump lists |
| [`gowxt`](wxt/README.md) | Windows Timeline ActivitiesCache.db |

All parsing is content-driven where the artefact allows it, so one evidence
tree — a mounted image root, a `gomount materialise` tree, or a staged
export — feeds every parser without routing.

## Input

The evidence tree at `GOWINDOWLICKER_INPUT_DIR` (default `/input`), mounted
read-only and shared by every sub-run.

## Env (the sweep)

| Variable | Default | Meaning |
|---|---|---|
| `GOWINDOWLICKER_INPUT_DIR` | `/input` | evidence tree, recursed read-only, shared by every sub-run |
| `GOWINDOWLICKER_OUT_DIR` | `/output` | output root: one `<subtool>/` tree per parser |
| `GOWINDOWLICKER_WORK_DIR` | `/work` | scratch (writable tmpfs), shared by every sub-run |
| `GOWINDOWLICKER_FORCE` | `0` | `1/true/yes/on`: rerun items that already have valid output, in every sub-run |
| `GOWINDOWLICKER_LOG_LEVEL` | `info` | `error\|warn\|info\|debug`, stderr only, applied to every sub-run |

A single-parser sub-run (`gowindowlicker gorb`) ignores the
`GOWINDOWLICKER_*` block and reads that tool's canonical `<SUBTOOL>_*`
variables instead (`GORB_INPUT_DIR`, `GORB_OUT_DIR`, …) — each parser
package's README documents its block. Under the sweep a sub-tool's own
non-reserved variables fall through untouched: **format stays per parser**
(`GORB_FORMAT=csv` beside the sweep is honoured; three of the twelve are
JSONL-only), and `GORE_BATCH` keeps selecting gore's `.reb` definition
(default: the bundled `/batch/default.reb`).

## Output

The sweep (bare or `lick`) writes one structured tree under
`GOWINDOWLICKER_OUT_DIR`:

```
<OUT_DIR>/<subtool>/<item>/<subtool>.<jsonl|csv>   per parser, one folder per discovered artefact
```

and prints **one** aggregate JSON summary line with every sub-tool's own
summary embedded under `subtools` (plus the roll-up counters, `status`,
`exit`). A single-parser sub-run prints that parser's ordinary summary line
and writes as its own README declares.

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
docker build -t get-sybers/gowindowlicker:latest -f gowindowlicker/Dockerfile gowindowlicker
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gowindowlicker:latest              # every parser (the default)
```

Scoped, the sub-tool is the only thing that changes:

```sh
docker run --rm … get-sybers/gowindowlicker:latest gorb
```

(The module is self-contained — its own directory is the build context; the
shared batch runtime lives here as the [`batch/`](batch/) package, the
promoted form of the byte-identical `batch.go` the per-tool directories used
to carry.)

## argv pass-through (debug only)

`gowindowlicker <subtool> <args>` hands the rest of the command line to that
parser's own argv debug mode: `-f FILE | -d DIR | --tar` (a `gomount stream`
tar on stdin) and each parser's own flags; records stream to stdout — see
each package README for its exact flags. `--version` prints the version;
`--print-contract` prints [`contract.yml`](contract.yml).

`test/contract_test.sh` builds the image (or the host binary under
`CONTRACT_LOCAL=1`), assembles one evidence tree from the packages'
committed `testdata/`, and asserts the sweep, idempotency, the config-error
exit, the argv pass-through and a single-subtool run.
