# Memory forensics — `get-sybers/piiat-mem`

Beyond the EZ CLI family, this repo also builds **`get-sybers/piiat-mem`**
(`piiat-mem/Dockerfile`): [PIIAT-Mem](https://github.com/Get-Sybers/PIIAT-Mem)
(Volatility 3 memory forensics) fused into one hardened python image. Volatility
runs **in-process** via piiat_mem's `--native` backend — confined by the image,
with no nested `docker run`. The PIIAT-Mem source is cloned at build time at
`--build-arg PIIAT_MEM_REF` (DX_DFIR passes its `sources.yml` pin). Build it with
`./build-all.sh piiat-mem`.

The image is **self-orchestrating and driven entirely by environment
variables** — the caller (DX_DFIR's ansible volatility lane) needs no wrapper
on the host. Its ENTRYPOINT is the baked batch orchestrator
(`piiat-mem/piiat_mem_batch.py`, at `/opt/piiat-mem-batch/`): with no arguments
it discovers every memory image under the mounted memory dir, runs the CAR
plugin set over each one through PIIAT-Mem's public CLI (`python3 -m piiat_mem
--native … --no-timeline`, Volatility in-process), and prints one JSON summary
line:

```
docker run --rm --network none --read-only --tmpfs /tmp \
  -e PIIAT_PLUGINS= -e PIIAT_FORCE=0 -e PIIAT_SYMBOLS_ONLINE=0 \
  -v "$mem_dir:/mem:ro" -v "$out:/out" -v "$symbols:/symbols" \
  get-sybers/piiat-mem:latest
```

| Variable | Default | Meaning |
|---|---|---|
| `PIIAT_MEMORY_DIR` | `/mem` | memory image tree (recursed; symlinks ignored). Matched by extension `.raw .mem .dmp .lime .vmem .bin .dump .vmsn .crash` or `*dramimage` |
| `PIIAT_OUT_DIR` | `/out` | output root — one folder per image, named by the image's path relative to the memory dir with `/` and spaces folded to `_` |
| `PIIAT_SYMBOLS_DIR` | `/symbols` | Volatility ISF symbol cache (read-write) |
| `PIIAT_PLUGINS` | *(empty)* | comma-separated Volatility plugin names; empty/unset = the default CAR set (18 plugins, `banners.Banners` first, see the orchestrator's `DEFAULT_PLUGINS`) |
| `PIIAT_FORCE` | `0` | `1/true/yes/on`: rerun plugins that already have valid output |
| `PIIAT_SYMBOLS_ONLINE` | `0` | `1/true/yes/on`: the caller has given *this container* network for the ISF fetch. **Informational** — recorded in the summary and log; the container's `--network` is what actually gates it |

Outputs, per image: `<out>/<clean name>/plugins/<plugin>.jsonl` (raw per-plugin
JSON Lines, one flat object per TreeGrid node) and `<out>/<clean name>/piiat_mem.log`
(that image's full PIIAT-Mem stdout+stderr, overwritten per run; PIIAT-Mem also
rebuilds its own `car.db` there). **Idempotent per plugin**: a plugin whose
`.jsonl` exists and whose first line parses as JSON is skipped (unless
`PIIAT_FORCE`); only the still-missing plugins are passed to `--plugins`; an image
with none missing is not invoked at all; empty/failed outputs are deleted, never
counted as done. The legacy flat `<out>/<clean name>/<plugin>.jsonl` layout an
earlier DX_DFIR lane wrote is also honoured as "done".

stdout is exactly one JSON object; stderr carries progress lines and, when
nothing at all was produced, the tail of each image's log as `diagnostics`:

```json
{"tool": "piiat-mem", "memory_dir": "/mem", "out_dir": "/out", "symbols_dir": "/symbols",
 "symbols_online": false, "force": false, "images": 2, "plugins": 18,
 "processed": 17, "skipped": 18, "failed": 1,
 "results": [{"image": "case1/win10.raw", "produced": ["banners.Banners", "..."], "empty": ["windows.malfind"]},
             {"image": "case2/box.vmem", "produced": [], "empty": []}],
 "diagnostics": "--- ... (last lines of piiat_mem.log) ---\n..."}
```

`processed`/`skipped`/`failed` count plugin **outputs** (gate `changed_when` on
`processed > 0`). Exit code: `0` normal — including a fully idempotent re-run
where everything is skipped; `1` when the run produced nothing, nothing was
already done, and something failed (the retryable Windows-without-symbols case);
`2` on a configuration error (memory dir missing, output dir unwritable —
`error` is set in the summary).

Any CLI argument switches to **pass-through**: `… get-sybers/piiat-mem -f
/mem/<image> -o /out --symbols /symbols` runs PIIAT-Mem's own single-image CLI
(`--native` still forced) — the previous run shape, for manual use.

`/out` and `/symbols` must be writable by uid 2000; `/tmp` is `HOME` and
Volatility's cache, so a read-only rootfs needs the tmpfs. Windows plugins fetch
ISF symbols on first use — under `--native`, piiat_mem's `--symbols-online` only
lifts *its own* container's isolation (a no-op here), so **the default posture is
offline** (`--network none`, pre-seed `/symbols`); give this container network
(`--network bridge`, and set `PIIAT_SYMBOLS_ONLINE=1` so the summary records it)
only for a run that must fetch symbols. Format-agnostic plugins
(`banners.Banners`) need no symbols. Runtime site-packages hold only
`volatility3`, `yara-python` and `pefile`: the build-time hardener is installed
into a throwaway dir and removed with its whole dependency closure. The batch
mode ideally belongs upstream in PIIAT-Mem (`piiat-mem --batch <dir>`); until it
grows one, this image carries the orchestrator.
