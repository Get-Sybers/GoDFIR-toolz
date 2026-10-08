# GoDFIR-toolz

**Minimal hardened Docker images for the DX_DFIR forensic pipeline** — static
Go parsers for the Windows artefact classes, the pipeline lane images, and
memory forensics: no shell, no python, no package manager, uid0
renamed+locked, runs as uid 2000.

Every image's full documentation (what it parses, build one-liner, run shape,
flags, verification evidence) lives in a `README.md` **inside the directory
that builds it** — and this page is the index.

## The Windows side: gowindowlicker (FROM scratch, one binary, no runtime at all)

**One structured binary** — [gowindowlicker/](gowindowlicker/README.md) —
carrying the whole Windows matrix: twelve parser packages, one per artefact
class. **The parameter is the sub-tool**: bare invocation (or `lick`) is
the sweep — every parser over one evidence tree, each into its own
`<OUT_DIR>/<subtool>/` tree, one aggregate JSON summary line — and each
parser also runs granularly as `gowindowlicker <subtool>` under its
canonical `<SUBTOOL>_*` env block. **Each parser keeps its canonical tool
name, env block, record shapes and record-file name**
(`goprefetch.jsonl`, `gore.jsonl`, `goevtx.jsonl`, …) — the interface
byakugan and the pipeline consume. The module lives in
[its own repository](https://github.com/Get-Sybers/gowindowlicker) and uses
[gopinfo](https://github.com/Get-Sybers/gopinfo)'s shared `batch/` runtime; the
image imports the published module and bakes the
[gomount](https://github.com/Get-Sybers/gomount) binary from its published
module — the parsers run **on a disk image**
(`GOWINDOWLICKER_IMAGE`: gomount pulls their artefact sets out of the OS
volume into the work dir, nothing exported, nothing mounted). Still one
static binary plus gomount, no shell, no python, no libc, `USER 2000:2000` —
the hardening contract holds by construction, and the `docker export` scan
verifies it.

- [goprefetch](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/prefetch/README.md) — Windows prefetch: XP→Win11 `.pf`, MAM decompression in pure Go
- [goese](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/ese/README.md) — ESE databases: SRUM `SRUDB.dat` (IdMap/SID enrichment) and SUM `Current.mdb`
- [gorb](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/rb/README.md) — Recycle Bin `$I` records, v1 + v2
- [gomft](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/mft/README.md) — raw `$MFT`: MACB from 0x10 + 0x30, ADS, full paths
- [goamcache](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/amcache/README.md) — `Amcache.hve`, with `.LOG` replay
- [goappcompat](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/appcompat/README.md) — ShimCache from SYSTEM hives, with `.LOG` replay
- [goevtx](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/evtx/README.md) — `.evtx` event logs → the JSON record shape byakugan's evtx maps consume
- [gore](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/re/README.md) — batch-driven registry key/value dumps
- [gosbe](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/sbe/README.md) — ShellBags (BagMRU) with reconstructed paths
- [gole](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/le/README.md) — `.lnk` shell links
- [gojle](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/jle/README.md) — AutomaticDestinations jump lists
- [gowxt](https://github.com/Get-Sybers/gowindowlicker/-/blob/main/wxt/README.md) — Windows Timeline ActivitiesCache.db

The calling vocabulary is the sub-tool names — there is no stream or
model-word vocabulary and no layered knowledge store: the Windows artefacts
are self-contained, so every sub-run is independent.

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
[gopinfo](https://github.com/Get-Sybers/gopinfo) module (batch runtime, record
envelope, provenance stamping, the typed-event families the journal pathway
and the flat logs share) and baking the
[gomount](https://github.com/Get-Sybers/gomount) binary, both pulled as
published modules at image build (the hunt runs **on a disk image**,
`GODAEMONHUNTER_IMAGE`).
Same hardening contract: `FROM scratch`, one static binary,
`USER 2000:2000`.

- [gowtmp](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/wtmp/README.md) — Linux logins: utmp/wtmp/btmp `struct utmp` records + the sparse lastlog table
- [gojournal](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/journal/README.md) — systemd journal: binary `.journal` files (dirty/compact included), XZ/LZ4/ZSTD payloads, streamed
- [goauditd](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/auditd/README.md) — audit.log records coalesced into one record per event, hex fields decoded, execve argv reassembled
- [gosyslog](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/syslog/README.md) — syslog-family text logs, three timestamp dialects, sshd/sudo/pam/cron families typed by the parser
- [goshell](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/shell/README.md) — bash/zsh/fish/sh/python/mysql/psql histories, one record per command
- [gousers](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/users/README.md) — passwd/shadow/group, sudoers, sshd_config, authorized_keys, known_hosts as typed records
- [gocron](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/cron/README.md) — system/user crontabs, cron.d, run-parts, anacrontab, at jobs
- [gounit](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/unit/README.md) — systemd units, timers and drop-ins, with the vendor/admin/runtime/user scope
- [gotrash](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/trash/README.md) — XDG Trash: original path, deletion time, paired content file
- [gohost](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/host/README.md) — host identity (os-release, hostname, machine-id, timezone, locale) + fstab/crypttab volume-to-name mapping
- [gonetwork](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/network/README.md) — hosts, resolv, nsswitch, TCP wrappers, interface/connection profiles, persisted firewall state
- [goctl](https://github.com/Get-Sybers/godaemonhunter/-/blob/main/ctl/README.md) — sysctl, module policy (modprobe.d install/blacklist), ld.so.preload and ld.so.conf

The plan they implement — the pinfo module, snapshots, filesystem residue,
byakugan alignment, the phased sequence — is
[docs/linux/README.md](docs/linux/README.md).

## Pipeline images

- [byakugan/](byakugan/README.md) — `get-sybers/byakugan`: the external
  Byakugan MITRE CAR engine, cloned recursively at the sha pinned in its
  Dockerfile (`BYAKUGAN_REF`), with the engine repo's Elastic detection
  rules-as-code baked from that clone at `/rules`
  ([rules/](https://github.com/Get-Sybers/Byakugan/-/blob/main/rules/README.md), build-gated by the engine's own `validate.py`)
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

## The image inventory

**`images.yml`** at the repo root is the single source of truth for every
image this repository builds — name, build context, dockerfile, build args,
aliases, the engine-pin markers, and the release each image is published as
with the digest of that push — plus the `get-sybers/*` namespace's known
non-tool repos. `conform.sh` checks each tool directory against it, and
the **`godfir_build` role** — the collection's build engine (ansible tasks end
to end) — builds and hardening-verifies every entry from it.
`build-all.sh` exists solely so this repo works **standalone** (cloned on its
own, no consumer around): a thin launcher of the collection playbook
(`get_sybers.godfir_build.build_images`), nothing more.

The Ansible content ships as native collections under
[`ansible_collections/get_sybers/`](ansible_collections/get_sybers)
(ansible-standards §1), split by role:

- [**`get_sybers.godfir_build`**](ansible_collections/get_sybers/godfir_build) —
  the **producer**. The `godfir_build` role and `build_images` playbook build
  and hardening-verify every `images.yml` entry. Runs in-repo against this
  checkout; not installed by consumers.
- **`get_sybers.godfir_run`** — the **consumer**. It pulls the published images
  from the registry and runs each tool under its confinement contract.
  Downstreams (DX_DFIR) pin this collection and pull images — they do not
  build.

No image list exists anywhere else — a new tool is added in `images.yml`
(and its own directory), in one place.

### Manifest schema

Per image:

| field | meaning |
| ----- | ------- |
| `name` | the image identity — `get-sybers/<name>:latest` — and, for single-tool directories, the tool directory name |
| `release` | the `vX.Y.Z` the image is published as — `ghcr.io/get-sybers/godfir-toolz/<name>:<release>` — kept in step with its Dockerfile's `GODFIR_RELEASE` (the one place the version is set; the Build & Push workflow ships that pin). Consumers (`godfir_images`) pull by it when no `digest` is recorded |
| `digest` | the content digest (`sha256:…`) of that release's push — the image index the Build & Push workflow reported — recorded here once the image is built, so a consumer pulls `<registry>/<name>@<digest>`: the exact bytes that shipped, not whatever the tag points at. Absent until the release has been published |
| `context` | build context relative to this repository's root; default `.` — the repo root, which keeps `hardening/harden.yml` and the shared entry scripts in reach of the Dockerfiles that COPY them |
| `dockerfile` | Dockerfile path relative to the context; default `<name>/Dockerfile` |
| `args` | `--build-arg` map baked into the build (a consumer's run-as `DFIR_UID`/`DFIR_GID` args combine on top and win) |
| `env_args` | ARG names a launcher passes through from the environment when set. The default pin **values** live in the Dockerfiles as ARG defaults — never here — so each pin exists in exactly one place |
| `engine_ref` | the ARG carrying a clone-at-build engine pin; marks the entry as an engine image (`conform.sh` then requires the `com.get-sybers.engine-ref` label) |
| `aliases` | alternate names the build set accepts for this image |
| `subtool_aliases` | names that resolve to this image with a "that's a sub-tool now" note (godaemonhunter's daemon parsers, gowindowlicker's Windows parsers and their EZ-tool names) |
| `tool` | `false` = not a tool container, exempt from the hardened-tool contract (default `true`) |

Two top-level maps complete the namespace's story: **`unbuildable`** names
tools people ask for that deliberately have no image — the build role
refuses them with the stated reason instead of "unknown tool" — and
**`non_tool_repos`** lists `get-sybers/*` repos that legitimately exist but
are not tool containers, allow-listed so a namespace audit ignores them
instead of flagging a supply-chain red flag.

Hardening posture is never listed here: every image self-declares it in
`/etc/dfir-hardened` (schema=1: `static_binary` / `shell` / `python` /
`pkg_mgr`) and the build gates verify the declaration against the actual
filesystem, so posture cannot drift from the images either.

## Building

```sh
./build-all.sh                              # everything in images.yml, in manifest order
./build-all.sh gowindowlicker godaemonhunter # a subset (names case-insensitive; a sub-tool or EZ-tool name resolves to its image)
./build-all.sh byakugan plaso signatures zeek
```

`build-all.sh` is the launcher of the `get_sybers.godfir_build.build_images`
playbook for standalone use of this repo **only** — this repository cloned by
itself, no consumer around. It is never used by another repository: consumers
pull the published images from the registry (via `get_sybers.godfir_run`) and
do not build. The script refuses to run from a checkout that
is a submodule of another repository. The inventory lives in `images.yml`
and the build logic in the role; the script prepares the host, then
forwards names and the optional stamp overrides.

A bare clone needs nothing installed beforehand but `python3` (≥ 3.11,
with the `venv` module) and the docker CLI. Every pin the script needs is
written in the script itself — it creates no file a consumer's tooling
could pick up. On first run it installs the pinned controller layer
(ansible-core + the docker SDK `community.docker`'s modules import) into
`<repo>/.venv` and the pinned `community.docker` collection into
`<repo>/.ansible/collections`, put on `ANSIBLE_COLLECTIONS_PATH` for the
run — per checkout, as the invoking user, never touching the system or
`ansible.cfg`; both trees are gitignored and excluded from the collection
artifact, and a host provisioned once builds offline afterwards. It then
checks what it cannot install (a daemon this user may talk to, BuildKit)
and names the fix when something is missing. `./build-all.sh --preflight`
runs exactly that and builds nothing. A host with its own ansible passes
it as `GODFIR_ANSIBLE=/path/to/ansible-playbook` (its python must import
`docker`); `GODFIR_VENV` relocates the venv.

Clone-at-build engine pins live as ARG defaults in the tool Dockerfiles;
each entry's `env_args` in `images.yml` names the pins overridable from the
environment (e.g. `BYAKUGAN_REF=v1.2 ./build-all.sh byakugan` — the role
reads them itself), and `GODFIR_REVISION` / `GODFIR_RELEASE` override the
source stamps.

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
   container start overhead per `docker run` before any parsing. Every parser
   takes `-d`; one container over a directory of 400 event logs pays that
   cost once instead of 400 times.
2. **The Go parsers are cheap.** The Go images are a few MB — gowindowlicker
   carries the whole Windows dozen in one ~8 MB static binary — start as
   fast as the container runtime allows, and parsed the reference
   `SRUDB.dat` (10 tables, 27k rows, enrichment on) in under a second.

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
block; `gomount` is a declared argv deviation.

Per tool, `conform.sh <tool>` checks the layout, the Dockerfile standards,
`contract.yml` and the README against the white paper (`--build` also builds
the image and cross-checks the built artifact), and `<tool>/test/contract_test.sh`
runs the image over `test/fixtures/` in batch mode and asserts the summary
line, the exit code, idempotency and the config-error exit. The build galaxy
itself is molecule-tested
(`ansible_collections/get_sybers/godfir_build/roles/godfir_build/molecule/default`
— `molecule test` runs the whole gate matrix, negatives included, offline
against a committed fixture). The `godfir_build`
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
the absence of the removed binaries — `conform.sh --build` does exactly that,
and the `godfir_build` role runs the same scan after every build.

`harden.yml` runs by ansible **inside** the image build
(`ansible-playbook -c local`, then the stage is squashed) and takes its
knobs with `-e` from each Dockerfile:

| var | default | meaning |
| --- | ------- | ------- |
| `harden_user` | `dfir` | runtime user name |
| `harden_uid` / `harden_gid` | `2000` / the uid | fixed uid/gid |
| `harden_remove_pkg_mgr` | `true` | remove apt/dpkg + pip |
| `harden_extra_remove` | `[]` | image-specific paths to delete |
| `harden_tool` | `unknown` | recorded in `/etc/dfir-hardened` (equals the `com.get-sybers.tool` label) |
| `harden_static_binary` | `false` | the entrypoint is a static binary with no libc (Shape B keeps glibc) |
| `harden_shell` / `harden_python` | `false` | the image intentionally keeps a shell / python — a declared deviation its Dockerfile justifies. The build gate asserts these booleans against the actual filesystem and **fails the build on any mismatch** (a declared-absent shell that is still present, and vice versa), so a Dockerfile that strips the shell *after* this playbook runs still declares `shell=false` here |

## License

MIT (this recipe). The Go engines live in their own repositories with their
own licenses and attribution (gowindowlicker's `go-prefetch` and `go-ese`
are Velociraptor components, pinned in its `go.sum`).
