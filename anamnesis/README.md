# `get-sybers/anamnesis` — memory forensics on MemProcFS

[Anamnesis](https://github.com/Get-Sybers/Anamnesis) (pure-Go memory
forensics on [MemProcFS](https://github.com/ufrisk/MemProcFS)) in one
hardened image — no Volatility, no Python. The engine source is cloned at
build time at `--build-arg ANAMNESIS_REF` (DX_DFIR passes its `sources.yml`
pin); the Go binary is built `-tags memprocfs` (CGO_ENABLED=0, purego) and the
MemProcFS `vmm`/`leechcore` shared libraries are fetched from a pinned,
sha256-verified upstream release and bundled at `/opt/anamnesis/lib` alongside
`libusb-1.0` so `leechcore.so` can load in the hardened runtime. glibc and
libusb are kept for the native libraries (the declared deviations); python, apt, pip
and every shell are stripped by the shared build-time hardener. Build it with
`./build-all.sh anamnesis`.

The image is self-orchestrating and env-driven — the reference implementation
of the container framework: the ENTRYPOINT is the anamnesis binary, and with
no arguments it reads the environment contract in
[`contract.yml`](contract.yml), discovers every memory image under the
mounted memory dir, runs the CAR collector set over each, and prints one JSON
summary line.

## Input

`ANAMNESIS_INPUT_DIR` (default `/input`, mounted read-only) is walked
recursively (symlinks ignored); every file matched by extension `.raw .mem
.dmp .lime .vmem .bin .dump .vmsn .crash` or named `*dramimage` is one image.

## Env

| Variable | Default | Meaning |
|---|---|---|
| `ANAMNESIS_INPUT_DIR` | `/input` | memory image tree (recursed; symlinks ignored) — the input mount |
| `ANAMNESIS_OUT_DIR` | `/out` | output root — one folder per image, `/` and spaces folded to `_` |
| `ANAMNESIS_SYMBOLS_DIR` | `/symbols` | PDB/symbol cache (read-write) |
| `ANAMNESIS_PLUGINS` | *(empty)* | comma-separated collector names; empty = the default CAR set |
| `ANAMNESIS_FORCE` | `0` | `1/true/yes/on`: rerun collectors that already have valid output |
| `ANAMNESIS_SYMBOLS_ONLINE` | `0` | `1/true/yes/on`: this container has network for the PDB fetch |

## Output

Per image: `<OUT_DIR>/<clean name>/plugins/<plugin>.jsonl` (raw
per-collector JSON Lines), `<OUT_DIR>/<clean name>/car.db` (the CAR store
byakugan consumes 1:1), and `<OUT_DIR>/<clean name>/anamnesis.log`.
Idempotent per collector: a collector whose `.jsonl` exists and whose first
line parses as JSON is skipped unless `ANAMNESIS_FORCE` is set.

stdout is exactly one JSON object with `tool`, `input_dir`, `out_dir`,
`symbols_dir`, `symbols_online`, `force`, `images`, `plugins`, `processed`,
`skipped`, `failed` and `results` (one entry per image), plus `error` on a
config error. Progress goes to stderr.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | every image processed, or nothing to do (no image found, or all already up to date) |
| 1 | nothing produced and something failed |
| 2 | config error — the memory dir is missing or not a directory, the output dir is not writable |
| 3 | partial — reserved for the framework's at-least-one-processed-and-one-failed case; the engine reports that case as 1 today |

## Run

```sh
docker run --rm --network none --read-only --tmpfs /tmp \
  -e ANAMNESIS_PLUGINS= -e ANAMNESIS_FORCE=0 -e ANAMNESIS_SYMBOLS_ONLINE=0 \
  -v "$input_dir:/input:ro" -v "$out:/out" -v "$symbols:/symbols" \
  get-sybers/anamnesis:latest
```

`/out` and `/symbols` must be writable by uid 2000; `/tmp` is `HOME` and the
cache, so a read-only rootfs needs the tmpfs. Build args: `ANAMNESIS_REF`
(source pin), `MEMPROCFS_VERSION` / `MEMPROCFS_TAG` / `MEMPROCFS_ASSET` /
`MEMPROCFS_SHA256` (the pinned Linux release and its checksum), and
`TOOL_VERSION` (the engine release the pin corresponds to; the
`org.opencontainers.image.version` label). `test/contract_test.sh` builds the
image, runs it over an empty memory dir (a memory image cannot be committed),
and asserts the summary line, the exit code, idempotency and the config-error
exit.

## argv pass-through (debug only)

Any argument switches to the engine's single-image mode:
`… get-sybers/anamnesis -f /input/<image> -o /out`; `--version` prints the
engine version.
