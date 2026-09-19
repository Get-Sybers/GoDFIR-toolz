# GoDFIR-toolz

**Minimal hardened Docker images for the DX_DFIR forensic pipeline** — static
Go parsers for the Windows artefact classes, the pipeline lane images, and
memory forensics: no shell, no python, no package manager, uid0
renamed+locked, runs as uid 2000.

Every image's full documentation (what it parses, build one-liner, run shape,
flags, verification evidence) lives in a `README.md` **inside the directory
that builds it** — the remaining `.NET`-based per-tool images share
[`godfir-tool/`](godfir-tool/README.md), their one recipe — and this page is the index.

## The Go parsers (FROM scratch, a few MB, no runtime at all)

Twelve static Go binaries, one per artefact class. All are `FROM scratch`:
one static binary, no shell, no python, no libc, `USER 2000:2000` — the
hardening contract holds by construction, and the `docker export` scan
verifies it the same way as for the .NET images.

- [goprefetch/](goprefetch/README.md) — Windows prefetch: XP→Win11 `.pf`, MAM decompression in pure Go
- [goese/](goese/README.md) — ESE databases: SRUM `SRUDB.dat` (IdMap/SID enrichment) and SUM `Current.mdb`
- [gorb/](gorb/README.md) — Recycle Bin `$I` records, v1 + v2
- [gomft/](gomft/README.md) — raw `$MFT`: MACB from 0x10 + 0x30, ADS, full paths
- [goamcache/](goamcache/README.md) — `Amcache.hve`, with `.LOG` replay
- [goappcompat/](goappcompat/README.md) — ShimCache from SYSTEM hives, with `.LOG` replay
- [goevtx/](goevtx/README.md) — `.evtx` event logs → the JSON record shape byakugan's evtx maps consume
- [gore/](gore/README.md) — batch-driven registry key/value dumps
- [gosbe/](gosbe/README.md) — ShellBags (BagMRU) with reconstructed paths
- [gole/](gole/README.md) — `.lnk` shell links
- [gojle/](gojle/README.md) — AutomaticDestinations jump lists
- [gowxt/](gowxt/README.md) — Windows Timeline ActivitiesCache.db

## Pipeline images

- [byakugan/](byakugan/README.md) — `get-sybers/byakugan`: the external
  Byakugan MITRE CAR engine, cloned recursively at a DX_DFIR-pinned sha
- [plaso/](plaso/README.md) — `get-sybers/plaso`: minimal hardened Plaso at a
  pinned PyPI release, plus the psort wrapper
- [signatures/](signatures/README.md) — `get-sybers/signatures`: the whole
  detection lane in one image (YARA + Suricata + Hayabusa)
- [zeek/](zeek/README.md) — `get-sybers/zeek`: minimal hardened Zeek LTS for
  offline capture parsing, deterministically fetched and pinned
- [anamnesis/](anamnesis/README.md) — `get-sybers/anamnesis`: anamnesis (pure-Go
  memory forensics on MemProcFS — no Volatility, no Python) in one
  self-orchestrating, env-driven hardened image (replaces the previous Volatility-based memory image)

The four lane images (byakugan, plaso, signatures, zeek) moved here from
DX_DFIR's `docker/` — that repo now keeps only its Elastic stack. Like
`anamnesis`, they build with the **repo root as context**, so each consumes
the canonical `hardening/harden.yml` directly — no synced copy.

## The .NET images

- [godfir-tool/](godfir-tool/README.md) — one parameterized Dockerfile building the
  remaining `.NET`-based per-tool images (`sqlecmd`, `bstrings`,
  `iisgeolocate`, `recentfilecacheparser`, `rla`)

## Building

```sh
./build-all.sh                              # every image: .NET per-tool + Go parsers + pipeline images
./build-all.sh gore gomft                   # a subset (names case-insensitive)
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
   content-detects its artefacts. Volume Shadow Copies: mount each snapshot
   on the host with libvshadow (`vshadowinfo`/`vshadowmount`) and feed the
   mounted snapshot like any other image root.
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
   assembly load/JIT warm-up per invocation of a .NET image. Every parser
   takes `-d`; one container over a directory of 400 event logs pays that
   cost once instead of 400 times.
2. **The Go parsers are cheap.** The Go images are 4–5 MB (vs ~300 MB for a
   .NET tool image), start as fast as the container runtime allows, and
   parsed the reference `SRUDB.dat` (10 tables, 27k rows, enrichment on) in
   under a second.

## The hardening contract

`hardening/harden.yml` (Ansible, build-time only — Ansible itself is removed
afterwards) plus the Dockerfile's strip step leave each .NET image with:

- `USER 2000:2000` (or the `DFIR_UID`/`DFIR_GID` build args), uid0 renamed and
  locked
- no `apt`/`dpkg`/`pip`/`sudo`, no setuid binaries
- **no shell** (`sh`/`bash`/`dash` removed) and **no python**
- label `com.get-sybers.hardened=true` for downstream verification

The Go parser images satisfy the same contract by construction (`FROM
scratch` — there is nothing to remove) and carry the same label. A consuming
pipeline can verify the contract without a shell in the image by exporting the
filesystem and asserting the absence of the removed binaries — that is exactly
what the DX_DFIR pipeline's image role does after every build.

## License

MIT (this recipe and the Go parsers). The `godfir-tool/`-built images fetch Eric
Zimmerman's tools from their published releases at build time — the upstream
tools are themselves MIT-licensed (attribution kept here for that reason);
`go-prefetch` and `go-ese` are Velociraptor components fetched as pinned Go
modules (`go.sum`) at build time.
