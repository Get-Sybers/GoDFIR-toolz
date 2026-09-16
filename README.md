# GoDFIR-toolz

**Minimal hardened Docker images for Eric Zimmerman's forensic tools, built to
actually parse artefacts on Linux** — no shell, no python, no package manager,
uid0 renamed+locked, runs as uid 2000. One parameterized Dockerfile builds a
per-tool image for every Linux-viable EZ tool; a single all-in-one image
carries the whole family behind a static launcher; and the tools that
*cannot* work off-Windows are replaced by static Go parsers that read the same
artefacts natively.

Every image's full documentation (what it parses, build one-liner, run shape,
flags, verification evidence) lives in a `README.md` **inside that image's
directory** — this page is the index.

## Coverage: the full EZ CLI family on Linux

| Requested tool | Image | Linux status |
| --- | --- | --- |
| **AmcacheParser** | **[`get-sybers/goamcache`](goamcache/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (regparser) to drop the .NET runtime — real-hive verified |
| **AppCompatCacheParser** | **[`get-sybers/goappcompat`](goappcompat/README.md) (Go substitute)** | ✅ Linux-viable under .NET, ported to a static Go binary (regparser) to drop .NET — real-SYSTEM-hive verified |
| bstrings | [`get-sybers/bstrings`](eztool/README.md) | ☑️ pure managed .NET — build-verified; parse-verify on first use |
| **EvtxECmd** | **[`get-sybers/goevtx`](goevtx/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (go-evtx) to drop the .NET runtime — real-.evtx verified end-to-end through byakugan's evtx maps |
| iisGeolocate | [`get-sybers/iisgeolocate`](eztool/README.md) | ☑️ pure managed .NET — mount/refresh its GeoLite2 `.mmdb` databases if the release doesn't bundle current ones |
| **JLECmd** | **[`get-sybers/gojle`](gojle/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (mscfb) to drop the .NET runtime — real jump lists verified end-to-end through byakugan's `jlecmd_dest` map |
| **LECmd** | **[`get-sybers/gole`](gole/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (golnk) to drop the .NET runtime — real `.lnk` verified |
| **MFTECmd** | **[`get-sybers/gomft`](gomft/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (go-ntfs) to drop the .NET runtime — real-$MFT verified |
| **PECmd** | **[`get-sybers/goprefetch`](goprefetch/README.md) (Go substitute)** | ❌ PECmd itself cannot parse on Linux → `goprefetch` parses XP→Win11 `.pf` natively, MAM-compressed included |
| **RBCmd** | **[`get-sybers/gorb`](gorb/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary to drop the .NET runtime — parses v1/v2 `$I` records |
| RecentFileCacheParser | [`get-sybers/recentfilecacheparser`](eztool/README.md) | ☑️ pure managed .NET — build-verified; parse-verify on first use |
| **RECmd** | **[`get-sybers/gore`](gore/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (regparser) to drop the .NET runtime — real-hive verified (SYSTEM/SOFTWARE/NTUSER.DAT with `.LOG` replay, through byakugan's `recmd_batch` map) |
| RLA | [`get-sybers/rla`](eztool/README.md) | ☑️ pure managed .NET (same Registry library whose LOG replay already works on Linux via goamcache/goappcompat) |
| **SBECmd** | **[`get-sybers/gosbe`](gosbe/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (regparser) to drop the .NET runtime — real-UsrClass.dat shellbags verified (BagMRU tree, BEEF0004 long names, `.LOG` replay) |
| SQLECmd | [`get-sybers/sqlecmd`](eztool/README.md) | ✅ parse-verified (Maps/ baked in) |
| **SrumECmd** | **[`get-sybers/goese`](goese/README.md) (Go substitute)** | ❌ SrumECmd cannot parse on Linux → `goese` parses SRUDB.dat natively with IdMap/SID enrichment |
| **SumECmd** | **[`get-sybers/goese`](goese/README.md) (Go substitute)** | ❌ SumECmd cannot parse on Linux → `goese` reads SUM `Current.mdb` (any ESE database) |
| **VSCMount** | *(no container possible)* | ❌ manipulates the Windows VSS device namespace; on Linux use libvshadow (`vshadowinfo`/`vshadowmount`) on the host |
| **WxTCmd** | **[`get-sybers/gowxt`](gowxt/README.md) (Go substitute)** | ✅ Linux-viable under .NET, but ported to a static Go binary (modernc sqlite) to drop the .NET runtime — parses Windows Timeline ActivitiesCache.db |

### Why three tools are substituted, not packaged

"Installs on Linux" and "parses artefacts on Linux" are different claims.
Linux installer scripts for the EZ tools set up all 19 and validate them with
`--help` — which genuinely succeeds for every tool. But the four tools above
refuse at *parse* time, and (verified against v2026.5.0 built from upstream
source, run against real artefacts) they print their refusal and **exit 0**,
so a pipeline that only checks exit codes records a successful run that
produced nothing:

- **PECmd** — `Non-Windows platforms not supported due to the need to load
  decompression specific Windows libraries! Exiting...` on *any* input, even
  uncompressed XP-era prefetch (blanket guard in `Program.cs`; the Prefetch
  library P/Invokes `ntdll!RtlDecompressBufferEx` for Win8+/Win10 MAM files).
- **SrumECmd / SumECmd** — `Non-Windows platforms not supported due to the
  need to load ESI specific Windows libraries! Exiting...`; both depend on
  `Microsoft.Database.ManagedEsent`, a P/Invoke wrapper over Windows' native
  `esent.dll`.
- **VSCMount** — creates symlinks to
  `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopyN`; the concept it
  manipulates does not exist off-Windows.

`eztool/Dockerfile` therefore fails fast if asked to build one of the four
(`--build-arg EZTOOL_ALLOW_WINDOWS_ONLY=1` overrides, e.g. to unpack a
release), and this repo ships native substitutes instead.

## The Go substitutes (FROM scratch, a few MB, no runtime at all)

All twelve Go images are `FROM scratch`: one static binary, no shell, no
python, no libc, `USER 2000:2000` — the hardening contract holds by
construction, and the `docker export` scan verifies it the same way as for
the .NET images. Each one's docs:

- [goprefetch/](goprefetch/README.md) — replaces PECmd; XP→Win11 `.pf`, MAM decompression in pure Go
- [goese/](goese/README.md) — replaces SrumECmd + SumECmd; SRUM/SUM (any ESE database) with IdMap/SID enrichment
- [gorb/](gorb/README.md) — replaces RBCmd; Recycle Bin `$I` records, v1 + v2
- [gomft/](gomft/README.md) — replaces MFTECmd; raw `$MFT`, MACB from 0x10 + 0x30
- [goamcache/](goamcache/README.md) — replaces AmcacheParser; Amcache.hve with `.LOG` replay
- [goappcompat/](goappcompat/README.md) — replaces AppCompatCacheParser; ShimCache from SYSTEM hives with `.LOG` replay
- [goevtx/](goevtx/README.md) — replaces EvtxECmd; `.evtx` → EvtxECmd-shape JSON
- [gore/](gore/README.md) — replaces RECmd; batch-driven registry key/value dumps
- [gosbe/](gosbe/README.md) — replaces SBECmd; ShellBags (BagMRU) with reconstructed paths
- [gole/](gole/README.md) — replaces LECmd; `.lnk` shell links
- [gojle/](gojle/README.md) — replaces JLECmd; AutomaticDestinations jump lists
- [gowxt/](gowxt/README.md) — replaces WxTCmd; Windows Timeline ActivitiesCache.db

## The .NET images

- [eztool/](eztool/README.md) — one parameterized Dockerfile building a hardened
  per-tool image for every Linux-viable EZ tool still on .NET
- [eztools-all/](eztools-all/README.md) — the all-in-one image
  (`get-sybers/eztools`): every Linux-viable EZ tool behind a static Go
  launcher, selected at run time

## Beyond the EZ family

- [piiat-mem/](piiat-mem/README.md) — `get-sybers/piiat-mem`: PIIAT-Mem +
  Volatility 3 memory forensics fused into one self-orchestrating, env-driven
  hardened image
- [byakugan/](byakugan/README.md) — `get-sybers/byakugan`: the external
  Byakugan MITRE CAR engine, cloned recursively at a DX_DFIR-pinned sha
- [plaso/](plaso/README.md) — `get-sybers/plaso`: minimal hardened Plaso at a
  pinned PyPI release, plus the psort wrapper
- [signatures/](signatures/README.md) — `get-sybers/signatures`: the whole
  detection lane in one image (YARA + Suricata + Hayabusa)
- [zeek/](zeek/README.md) — `get-sybers/zeek`: minimal hardened Zeek LTS for
  offline capture parsing, deterministically fetched and pinned

The four DX_DFIR pipeline images (byakugan, plaso, signatures, zeek) moved
here from DX_DFIR's `docker/` — that repo now keeps only its Elastic stack.
Like `piiat-mem`, they build with the **repo root as context**, so each
consumes the canonical `hardening/harden.yml` directly — no synced copy.

## Building

```sh
./build-all.sh                              # every .NET per-tool image + all the Go tools + the pipeline images
./build-all.sh gore mftecmd                 # a subset (names case-insensitive)
./build-all.sh all-in-one                   # the single get-sybers/eztools image
./build-all.sh byakugan plaso signatures zeek
```

Each image's README carries its standalone `docker build` one-liner (run from
the repo root, like the commands above).

## Two run modes

Every parser here is fed one of two ways, and the flags are the same shape in
both:

1. **A mounted disk image** — mount the image on the host (ewfmount/losetup +
   mount, or your image-export stage) and bind the filesystem root read-only
   into the container; every tool that takes `-d` walks the mounted root and
   content-detects its artefacts.
2. **Extracted / loose files** — a staged directory of `.evtx`, hives, `.pf`,
   or a single database: same containers, `-d` at the staged directory or
   `-f` at the file.

## Parse-time efficiency: how to run these

The images are offline parsers — run them with nothing but mounts:

```sh
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only --tmpfs /work:rw,nosuid,nodev,uid=2000,gid=2000 \
  -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/<tool>:latest -d /input --csv /output --work-dir /work
```

Two things dominate wall-clock time on real evidence:

1. **Batch per directory, not per file.** Measured here: ~280–340 ms of
   container start overhead per `docker run` before any parsing, plus .NET
   assembly load/JIT warm-up per invocation. Every EZ tool takes `-d`; one
   container over a directory of 400 event logs pays that cost once instead
   of 400 times.
2. **The substitutes are cheap.** The Go images are 4–5 MB (vs ~300 MB for a
   .NET tool image), start as fast as the container runtime allows, and
   parsed the reference `SRUDB.dat` (10 tables, 27k rows, enrichment on) in
   under a second — artefact classes that previously had **no** working
   Linux container now cost less than any other lane.

## The hardening contract

`hardening/harden.yml` (Ansible, build-time only — Ansible itself is removed
afterwards) plus the Dockerfile's strip step leave each .NET image with:

- `USER 2000:2000` (or the `DFIR_UID`/`DFIR_GID` build args), uid0 renamed and
  locked
- no `apt`/`dpkg`/`pip`/`sudo`, no setuid binaries
- **no shell** (`sh`/`bash`/`dash` removed) and **no python**
- label `com.get-sybers.hardened=true` for downstream verification

The Go substitute images satisfy the same contract by construction (`FROM
scratch` — there is nothing to remove) and carry the same label. A consuming
pipeline can verify the contract without a shell in the image by exporting the
filesystem and asserting the absence of the removed binaries — that is exactly
what the DX_DFIR pipeline's image role does after every build.

## License

MIT (this recipe and the Go tools). Eric Zimmerman's tools are themselves
MIT-licensed and are fetched from their published releases at build time;
`go-prefetch` and `go-ese` are Velociraptor components fetched as pinned Go
modules (`go.sum`) at build time.
