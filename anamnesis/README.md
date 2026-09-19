# Memory forensics — `get-sybers/anamnesis`

This repo builds **`get-sybers/anamnesis`** (`anamnesis/Dockerfile`):
[anamnesis](https://github.com/Get-Sybers/Anamnesis) (pure-Go memory forensics on
[MemProcFS](https://github.com/ufrisk/MemProcFS)) in one hardened image — **no
Volatility, no Python**. It replaces the old `get-sybers/piiat-mem` image. The
anamnesis source is cloned at build time at `--build-arg ANAMNESIS_REF` (DX_DFIR
passes its `sources.yml` pin); the Go binary is built `-tags memprocfs`
(CGO_ENABLED=0, purego) and the MemProcFS `vmm`/`leechcore` shared libraries are
fetched from a pinned upstream release and bundled at `/opt/anamnesis/lib`. glibc is
kept for the native libraries; python/apt/pip/shell are stripped by the shared
build-time hardener. Build it with `./build-all.sh anamnesis`.

The image is **self-orchestrating and env-driven** — the ENTRYPOINT is the anamnesis
binary, and with no arguments it runs its built-in batch orchestrator: discover every
memory image under the mounted memory dir, run the CAR collector set over each, and
print one JSON summary line.

```
docker run --rm --network none --read-only --tmpfs /tmp \
  -e ANAMNESIS_PLUGINS= -e ANAMNESIS_FORCE=0 -e ANAMNESIS_SYMBOLS_ONLINE=0 \
  -v "$mem_dir:/mem:ro" -v "$out:/out" -v "$symbols:/symbols" \
  get-sybers/anamnesis:latest
```

| Variable | Default | Meaning |
|---|---|---|
| `ANAMNESIS_MEMORY_DIR` | `/mem` | memory image tree (recursed; symlinks ignored). Matched by extension `.raw .mem .dmp .lime .vmem .bin .dump .vmsn .crash` or `*dramimage` |
| `ANAMNESIS_OUT_DIR` | `/out` | output root — one folder per image, `/` and spaces folded to `_` |
| `ANAMNESIS_SYMBOLS_DIR` | `/symbols` | PDB/symbol cache (read-write) |
| `ANAMNESIS_PLUGINS` | *(empty)* | comma-separated collector names; empty = the default CAR set |
| `ANAMNESIS_FORCE` | `0` | `1/true/yes/on`: rerun collectors that already have valid output |
| `ANAMNESIS_SYMBOLS_ONLINE` | `0` | `1/true/yes/on`: this container has network for the PDB fetch |

Outputs, per image: `<out>/<clean name>/plugins/<plugin>.jsonl` (raw per-collector
JSON Lines), `<out>/<clean name>/car.db` (the CAR store byakugan consumes 1:1), and
`<out>/<clean name>/anamnesis.log`. **Idempotent per collector**: a collector whose
`.jsonl` exists and whose first line parses as JSON is skipped (unless
`ANAMNESIS_FORCE`). stdout is exactly one JSON object; exit `0` normal, `1` nothing
produced and something failed, `2` a configuration error.

Any CLI argument switches to **single-image pass-through**: `… get-sybers/anamnesis -f
/mem/<image> -o /out`.

Build args: `ANAMNESIS_REF` (source pin), `MEMPROCFS_VERSION` (default `5.18`) and
`MEMPROCFS_SHA256` (the Linux tarball's checksum — set it via DX_DFIR's `sources.yml`
to pin the download). `/out` and `/symbols` must be writable by uid 2000; `/tmp` is
`HOME` and the cache, so a read-only rootfs needs the tmpfs.
