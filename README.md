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

## The Linux side: godaemonhunter (docs/linux — the same shape, second OS)

**One structured binary** — [godaemonhunter/](godaemonhunter/README.md) —
carrying the whole Linux matrix: twelve parser packages, **every one a
daemon parser** (each reads what one daemon writes — journald, the relay
every other daemon logs through; auditd; the login machinery; the shells —
or what one daemon is told: systemd's units, crond's tabs, sshd's account
material, the kernel's sysctl). **The parameter is the stream**: called
with a byakugan-model word (`authentication`, `user_session`, `process`,
`service`, `flow`, `file`, `module`) it runs the layered one-shot scoped
to the parsers feeding that model — Layer 1 (gohost, gousers, gonetwork)
always runs first and builds the image's knowledge store, then the
selected daemon parsers run enriched by it. **No arguments is every
stream, the default.** Each parser also runs granularly as
`godaemonhunter <subtool>`. Built on the shared
[`pinfo/`](pinfo/) module (batch runtime, record envelope,
provenance stamping, the typed-event families the journal pathway and the
flat logs share), and therefore built with the **repo root as context**.
Same hardening contract: `FROM scratch`, one static binary,
`USER 2000:2000`.

- [gowtmp](godaemonhunter/wtmp/README.md) — Linux logins: utmp/wtmp/btmp `struct utmp` records + the sparse lastlog table
- [gojournal](godaemonhunter/journal/README.md) — systemd journal: binary `.journal` files (dirty/compact included), XZ/LZ4/ZSTD payloads, streamed
- [goauditd](godaemonhunter/auditd/README.md) — audit.log records coalesced into one record per event, hex fields decoded, execve argv reassembled
- [gosyslog](godaemonhunter/syslog/README.md) — syslog-family text logs, three timestamp dialects, sshd/sudo/pam/cron families typed by the parser
- [goshell](godaemonhunter/shell/README.md) — bash/zsh/fish/sh/python/mysql/psql histories, one record per command
- [gousers](godaemonhunter/users/README.md) — passwd/shadow/group, sudoers, sshd_config, authorized_keys, known_hosts as typed records
- [gocron](godaemonhunter/cron/README.md) — system/user crontabs, cron.d, run-parts, anacrontab, at jobs
- [gounit](godaemonhunter/unit/README.md) — systemd units, timers and drop-ins, with the vendor/admin/runtime/user scope
- [gotrash](godaemonhunter/trash/README.md) — XDG Trash: original path, deletion time, paired content file
- [gohost](godaemonhunter/host/README.md) — host identity (os-release, hostname, machine-id, timezone, locale) + fstab/crypttab volume-to-name mapping
- [gonetwork](godaemonhunter/network/README.md) — hosts, resolv, nsswitch, TCP wrappers, interface/connection profiles, persisted firewall state
- [goctl](godaemonhunter/ctl/README.md) — sysctl, module policy (modprobe.d install/blacklist), ld.so.preload and ld.so.conf

The plan they implement — the pinfo module, snapshots, filesystem residue,
byakugan alignment, the phased sequence — is
[docs/linux/README.md](docs/linux/README.md).

## Pipeline images

- [byakugan/](byakugan/README.md) — `get-sybers/byakugan`: the external
  Byakugan MITRE CAR engine, cloned recursively at the sha pinned in its
  Dockerfile (`BYAKUGAN_REF`)
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

## The image inventory

**`images.yml`** at the repo root is the single source of truth for every
image this repository builds — name, build context, dockerfile, build args,
aliases and the engine-pin markers — plus the `get-sybers/*` namespace's
known non-tool repos. `conform.sh` checks each tool directory against it, and
the **`godfir_build` role** — the collection's build engine (ansible tasks end
to end) — builds and hardening-verifies every entry from it.
`build-all.sh` exists solely so this repo works **standalone** (cloned on its
own, no consumer around): a thin launcher of the collection playbook
(`playbooks/build_images.yml`), nothing more — an integrating consumer uses
the role, never the script. A consumer plugs this repo in one of two
ways, both reading the same files:

- **Direct reference** — pin the repo (submodule or checkout) and read
  `images.yml` at the pin. DX_DFIR consumes this way: its `dxdfir_images`
  role builds every entry (CI included) and its runtime guard allow-lists
  exactly these images from the same manifest.
- **Ansible Galaxy** — the repo installs as the `get_sybers.godfir_toolz`
  collection (`galaxy.yml`), carrying the manifest and every build context:

  ```sh
  ansible-galaxy collection install 'git+https://github.com/Get-Sybers/GoDFIR-toolz.git'
  ```

No image list exists anywhere else — a new tool is added in `images.yml`
(and its own directory), in one place.

## Building

```sh
./build-all.sh                              # everything in images.yml, in manifest order
./build-all.sh gore gomft                   # a subset (names case-insensitive; the EZ-tool aliases resolve)
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

## The container framework

Every image conforms to the container framework in
[`docs/framework/`](docs/framework/README.md): it is self-provisioning
(hardened at build time by `hardening/harden.yml`, squashed into a distroless
final stage), env-driven (a no-argument entrypoint batches over
`<TOOL>_INPUT_DIR` into `<TOOL>_OUT_DIR`, one output folder per item, and
prints exactly one JSON summary line on stdout with the uniform exit codes
`0` success / `1` nothing produced / `2` config error / `3` partial), and
declared (`<tool>/contract.yml` is the single source of truth for its
variables, mounts, exit codes and output layout; `/etc/dfir-hardened` is its
`schema=1` self-declaration; the OCI + `com.get-sybers.*` label set carries
its version, revision, release and contract version). argv is a debug
pass-through the consumer never uses. Multi-tool images (`plaso`,
`signatures`, `byakugan`) front a dispatcher that takes the sub-tool name as
its first argument and self-orchestrates that sub-tool from its own env
block; `gomount` and the `godfir-tool`-built images are declared argv
deviations.

Per tool, `conform.sh <tool>` checks the layout, the Dockerfile standards,
`contract.yml` and the README against the white paper (`--build` also builds
the image and cross-checks the built artifact), and `<tool>/test/contract_test.sh`
runs the image over `test/fixtures/` in batch mode and asserts the summary
line, the exit code, idempotency and the config-error exit. The build galaxy
itself is molecule-tested (`roles/godfir_build/molecule/default` — `molecule
test` runs the whole gate matrix, negatives included, offline against a
committed fixture). The `godfir_build`
role stamps every image with the checkout revision and the release tag, and
replaces any image whose `com.get-sybers.src` stamp went stale.

## The hardening contract

`hardening/harden.yml` (Ansible, build-time only — Ansible itself is removed
afterwards) plus the Dockerfile's strip step leave each Shape-B image with:

- `USER 2000:2000` (or the `DFIR_UID`/`DFIR_GID` build args), uid0 renamed and
  locked
- no `apt`/`dpkg`/`pip`/`sudo`, no setuid binaries
- no shell and no python unless the image declares them (`plaso`: python +
  sh; `signatures`: sh; `byakugan`: python; `gomount`: sh) and its Dockerfile
  header justifies them
- the `/etc/dfir-hardened` self-declaration (`schema=1`, `tool`, `user`,
  `static_binary`, `shell`, `python`, `pkg_mgr`) and the label
  `com.get-sybers.hardened=true`

The Go parser images satisfy the same contract by construction (`FROM
scratch` — there is nothing to remove) and carry the same declaration and
label set. A consuming pipeline can verify the contract without a shell in
the image by exporting the filesystem and asserting the declaration against
the absence of the removed binaries — `conform.sh --build` does exactly that.

## License

MIT (this recipe and the Go parsers). The `godfir-tool/`-built images fetch Eric
Zimmerman's tools from their published releases at build time — the upstream
tools are themselves MIT-licensed (attribution kept here for that reason);
`go-prefetch` and `go-ese` are Velociraptor components fetched as pinned Go
modules (`go.sum`) at build time.
