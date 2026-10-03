# Changelog — GoDFIR-toolz

## Unreleased

- **Every cross-repo pin points at published, verifiable history.** The clean
  import rewrote the engine repos' history and left every pin dangling: the
  three Go-module images pinned `@v0.2.0` tags that never existed, byakugan
  pinned a commit absent from its repo, and anamnesis pinned an orphaned
  commit. gopinfo/gomount/godaemonhunter/gowindowlicker are now tagged
  `v0.2.0` on their rewritten main (with the dependents' stale gopinfo
  checksums refreshed to the canonical module hashes first), anamnesis is
  imported from its published module at tag `v1.1.0` (`go install
  …/cmd/anamnesis@v1.1.0`, seeds staged from the module cache — the git
  clone stage is gone), and `BYAKUGAN_REF` pins the commit tagged `v0.2.0`.
  Floating `golang:trixie` builder tags are pinned to `golang:1.26-trixie`.
- **The in-image hardening controller is pinned in one place.** Every
  Dockerfile that runs `harden.yml` installed `ansible-core` unpinned (apt in
  zeek/byakugan/signatures, pip in anamnesis/plaso). All five now pip-install
  from `hardening/requirements.txt` (`ansible-core==2.21.4`, the single
  source of that pin per ansible-standards §2) into a throwaway
  `/tmp/ansible` the hardener removes behind itself.
- **`community.docker` raised to 4.8.8** in both collections'
  `requirements.yml` — the ceiling the `<5.0.0` `galaxy.yml` bound allows
  (§13 `upgrade`).
- **Build & Push workflow hardened.** The checkout no longer persists the
  job token into the work tree (`persist-credentials: false`); the runner is
  pinned to `ubuntu-24.04`; the free-text `tag` input is validated against
  the OCI tag grammar through an env var (never shell-interpolated); the
  workflow token is empty by default with the job granting only
  `contents: read` + `packages: write`; and a per-tool concurrency group
  serialises builds without cancelling a push mid-flight.

- **READMEs state fact after the engine extraction.** The root README and the
  gowindowlicker / godaemonhunter / gomount image READMEs still narrated the
  pre-extraction monolith: 35 links to parser packages that now live in the
  engine repositories (gowindowlicker, godaemonhunter, gomount, gopinfo), and
  build claims — "self-contained module", "copies the sibling `pinfo/`",
  "builds with the repo root as context because it bakes the sibling
  gomount" — that contradict the published-module imports the Dockerfiles
  actually perform. Every parser link now points at its engine repository,
  the shared batch runtime is credited to gopinfo, and the license note no
  longer claims the Go parsers ride this repo.

- **This registry is the public pull endpoint.** A `v*` release tag on
  `Get-Sybers/godfir-containers` (the private build+verify pipeline) now
  triggers a promotion pipeline here, per tool: `promote` resolves the
  handed tag to its digest once and `crane cp`s that digest
  byte-identically into `ghcr.io/get-sybers/GoDFIR-toolz/<tool>`
  (plus a moving `latest`), and `sign` keyless-signs the promoted digest
  as this pipeline's OIDC identity (Sigstore; cosign pinned by release
  checksum, crane by image digest). Job tokens only — no stored
  credentials on either side; the conform gates are excluded from
  promotion pipelines and promotion jobs from every other pipeline. The
  verify command was in the former GitLab CI promotion pipeline.
- **Ansible split into native collections (ansible-standards §1).** The single
  root `get_sybers.godfir_toolz` collection is retired in favour of native
  `ansible_collections/get_sybers/` collections, split by role:
  `get_sybers.godfir_build` (the producer — the `godfir_build` role and
  `build_images` playbook, now at
  `ansible_collections/get_sybers/godfir_build/`) and, next,
  `get_sybers.godfir_run` (the consumer — pull the published images from the
  registry and run each under its confinement contract). The root `galaxy.yml`,
  `roles/`, `playbooks/` and `meta/` are gone; each collection carries its own
  `galaxy.yml`, `requirements.yml` (the single source of exact pins, §2),
  `README.md`, `CHANGELOG.md` and `.ansible-lint`. `ansible.cfg` now sets
  `collections_path` for in-repo resolution (no submodule, no galaxy pull for
  `get_sybers.*`); `build-all.sh` launches the FQCN playbook
  `get_sybers.godfir_build.build_images` and installs `community.docker` from
  `requirements.yml`.

## 0.4.0 (unreleased)

- **`byakugan` bakes the post-schema-mission engine** — the engine pin
  advances 56 commits to Byakugan `608879d`: the STIX v6 projection
  contract, the Phase 1–7 schema/model layers of the schema-mission epic,
  and attack-datasources vendored into the engine tree in place of the
  retired submodule (the model's external source submodule is now
  `model/sources/forensicartifacts`). The dispatch surface is unchanged —
  the nine sub-tools of `contract.yml` — and `python3-yaml` still covers
  the runtime, so only the pin, the image version and the submodule note
  move. Image 0.4.0.
- **The parsers run ON the disk image.** `gowindowlicker` and
  `godaemonhunter` take a disk image as an item (`GOWINDOWLICKER_IMAGE` /
  `GODAEMONHUNTER_IMAGE`, or every image directly under the input tree; a
  sub-tool run reads `<SUBTOOL>_IMAGE`): the baked-in gomount pulls the
  parsers' artefact sets out of the OS volume into the work dir, the batch
  loop runs over that as over a loose folder, records land under
  `<OUT_DIR>/<subtool>/<image>/…` (the hunt's knowledge store under
  `<KNOWLEDGE_DIR>/<image>/`), the scratch goes — nothing is exported, nothing
  is mounted. Both images bake gomount from its published module
  (`GOMOUNT_VERSION`), and their summaries carry `images` and `failures`.
- **gomount reads VM disks.** VMDK (monolithic and split sparse extents,
  streamOptimized, text descriptors with flat/zero extents, snapshot chains
  through `parentFileNameHint`, descriptor-less `-sNNN` sets), VHDX and VHD
  (fixed, dynamic, differencing), QCOW2 (backing files, compressed clusters)
  and VDI (dynamic, differencing) — detected by content, never by name.
  The decoders are ported from [VMkatz](https://github.com/nikaiw/VMkatz)
  (MIT); `identify` labels the container.
- **gomount reads APFS.** A clean-room backend (`gomount/fsx/apfs`): the
  container's newest checkpoint, the object maps, fixed and variable
  B-trees, every volume's file-system tree (inodes with extended fields,
  hashed and plain directory records, xattrs, data streams, extents), the
  sealed — hashed, headerless — tree of a System volume with its extents in
  the fext tree, and file content including decmpfs compression (zlib,
  LZVN, LZFSE; inline or resource fork — the LZVN/LZFSE decoders are ported
  from Apple's lzfse, BSD-3, in `gomount/lzfse`). The stack resolver peels a
  container into its volumes (`identify` lists them as `<partition>/apfsN`
  with name, role, UUID and the sealed/FileVault flags; `--volume 0` prefers
  the Data volume, then the System volume; a FileVault volume lists but its
  content does not read), `identify` guesses `macos (data)` / `macos
  (system)` and labels the Apple partition GUIDs. Snapshots, Fusion
  containers and LZBITMAP are out of scope.
- **The contract declares the layers; the Apple disk images are items.**
  `godaemonhunter/contract.yml` carries `layer1` and `layer2` — the
  registry hunt runs — so a driver that runs the parsers one container at
  a time over a staged host (DX_DFIR's loose-host path) reads the lists
  from the contract instead of keeping a copy; a test holds the contract
  and the binary's registry in step. gowindowlicker, godaemonhunter and
  the signatures scan take `.dmg` and `.sparseimage` files as disk-image
  items now that gomount reads them.
- **godaemonhunter hunts a Mac.** The same layered run over a macOS image
  or staged tree: Layer 1 gains `gomachost` (SystemVersion.plist, the
  SystemConfiguration host names and model, the system time zone and
  locale) and `gomacusers` (the OpenDirectory local node — dslocal users
  and groups, the account-policy times, the authentication authority; the
  hash blob reported present, never carried), both emitting the record
  types gohost and gousers do so the knowledge store reads a Mac
  unchanged; Layer 2 gains `golaunchd` (launchd jobs from every
  LaunchDaemons/LaunchAgents domain with the location's kind, domain and
  owner, and the `disabled*.plist` override tables) in the `service`
  stream. The Linux parsers read the macOS shapes of their artefacts:
  gosyslog `system.log`/`install.log`/`wifi.log` (two-digit zone offsets,
  the wifi dialect, bzip2 rotations through `discover.OpenAuto`), gowtmp
  the 628-byte `utmpx`, gocron `var/at/tabs`, `var/at/jobs` and
  `etc/periodic`, goshell the per-session histories, gousers
  `master.passwd`, gohost a staged `localtime` symlink's zone name. A new
  `pinfo/plist` package decodes XML (UTF-8 or UTF-16) and binary property lists (clean-room;
  proven against a reference encoder's bytes). gomount's catalogue gains
  the `macos-core` and `macos-system` sets; the image pull asks for
  `linux-core` and `macos-core`, then — when `gomount identify` shows a
  Data volume beside a System volume — pulls `macos-system` off the
  System volume in a second pass merged into the same staged tree, so the
  OS version and Apple's own launchd jobs are on the record. Not yet: the
  unified log, ASL, BTM, the TCC/KnowledgeC/quarantine databases,
  fseventsd.
- **Image runs carry provenance.** godaemonhunter's pull now asks gomount
  for the manifest, so every record from a disk image carries `Origin`
  (image, volume, path, inode) as the loose-tree path always could.
  `gomount materialise` gives each staged file the volume's modification
  time, so `SourceModified` and the year a yearless syslog stamp is
  anchored on speak of the evidence, not of the pull. gosyslog's own host
  field is now `Hostname` (as gojournal's): as `Host` it hid the
  envelope's knowledge block on every syslog record.
- **gomount reads HFS+.** A clean-room backend (`gomount/fsx/hfsplus`)
  for the Mac filesystem before APFS: the volume header, directly or
  through the classic HFS wrapper; the catalog, extents-overflow and
  attributes B-trees; forks that spill into the overflow tree; symbolic
  links; file and directory hard links through the private metadata
  directories; extended attributes; decmpfs content through the shared
  `gomount/fsx/decmpfs` package (the APFS backend now uses it too). HFSX
  is case-sensitive, HFS+ case-folded on lookup. The partition layer reads
  the **Apple Partition Map** (PowerPC-era disks, older external media,
  uncompressed DMGs), and `--volume 0` prefers an HFS+ volume holding
  `SystemVersion.plist` ahead of the Linux rule. Fixtures come from
  `gomount/fsx/hfsplus/hfstest` (`mkhfs`): no Linux build host can format
  HFS+, so the volume is written from the format — with a real index
  level, fragmented forks, links and every decmpfs shape — and both Mac
  backends are also tested against volumes Apple's own tools wrote: the
  raw disks of Homebrew's `transmission-2.61.dmg` and `container-apfs.dmg`
  cask fixtures (BSD-2, `THIRD_PARTY_NOTICES.md`), committed under
  `gomount/fsx/testdata/`.
- **gomount reads Apple disk images.** A clean-room UDIF reader
  (`gomount/image/dmg.go`): the `koly` trailer, the `mish` block tables
  from the XML plist or the classic resource fork, and the chunks decoded
  on demand behind a small cache — zero-fill, raw, ADC, zlib, bzip2 and
  LZFSE — so a `.dmg` is the raw disk it holds and the partition layer and
  the HFS+/APFS backends read it unchanged (`identify` labels it `dmg`).
  LZMA (ULMO) chunks, segmented `.dmgpart` sets and encrypted images are
  refused with a clear error. `.sparseimage` and `.sparsebundle` (the
  directory as `<image>`) read too, unwritten bands as zeros. The LZFSE
  decoder now lets a match reach into an earlier block of the same stream,
  as Apple's does — a DMG's 1 MiB chunks span several blocks. Tested
  byte for byte against Homebrew's hdiutil-made fixtures (committed under
  `gomount/image/testdata/`) and dfvfs's `hfsplus.sparseimage` (Apache-2.0).
- **gomount `materialise` speaks the batch contract**: ONE JSON summary line
  on stdout and the 0/1/2/3 exit table (0 ok, 1 nothing pulled, 2 config
  error, 3 partial), so a lane can run it per image like any dispatcher.
  New sets: `winevt`, `prefetch`, `mft`, `recent`, `recyclebin`, and
  `windows-core` (every Windows set in one); `linux-core` gains
  `usr/lib/os-release`, `utmp` and `.Trash-*`. The Dockerfile stamps
  `main.version`.
- **Versions.** The galaxy is 0.4.0; the images byakugan 0.4.0, gomount
  0.2.0, gowindowlicker 0.2.0, godaemonhunter 0.2.0, signatures 0.3.0
  (`TOOL_VERSION`; the Go images build the engines' published `v0.2.0`
  module tags, which carry every feature above). The ported VM-disk
  readers and the LZFSE/LZVN decoders are attributed in
  `THIRD_PARTY_NOTICES.md`.
- **plaso `image_export` runs again.** The dispatcher handed it
  `--temporary_directory`, which only `log2timeline`/`psort` accept —
  argparse exit 2 on every image; it now points `TMPDIR` at the work dir
  instead. Plaso's own log defaults to `./image_export-<ts>.log.gz`, opened
  lazily on the first warning, and the container's cwd is the read-only
  rootfs — the first warning was an `OSError` and a failed item; the
  sub-tool passes `--logfile` beside its item (`image_export-plaso.log`).
  An APFS/LVM image stopped at plaso's interactive volume prompt and, with
  stdin closed, exported nothing: the contract gains
  `PLASO_IMAGE_EXPORT_VOLUMES` (default `all`), so `--volumes` is never
  left to the prompt. With an argparse-strict stub test. (#81)

## 0.3.2

- **`byakugan` discovers DX_DFIR's per-collection, per-host processed
  tree** — the engine pin advances to Byakugan #126 (`1d1b837`, merged): sources
  are found by the files each tool writes, at any depth under the
  tool-named leaf (`processed/<tool>/[<collection>/]<host>/…`), one
  isolated store each and named after the path under the leaf so two
  collections never share a store; the older flat leaves keep building
  beside them, and the exchange's behaviour bridge reads the signatures
  image's per-item detection folders. Image 0.3.2.
- **A capture's output folder drops its extension** — `zeek` and the
  `signatures` suricata sub-tool name the per-capture folder
  `captures_cap/`, not `captures_cap.pcap/` (the extension says nothing
  about the evidence and the CAR engine names its source after the
  folder); two captures differing only by extension keep their full names
  rather than one being skipped as the other's "done" output.
- **`plaso`'s `psort` renders beside the storage file.** Its per-item
  folder is the storage file's name without `.plaso`, collapsing
  log2timeline's own `<item>/<item>.plaso` to `<item>`: with `OUT_DIR` at
  the log2timeline output root, `timeline.jsonl` (and `psort.log`,
  `psort.jsonl`) land in the one `<item>/` folder the `.plaso` already sits
  in, never in a second `<item>_<item>.plaso/` tree beside it. A consumer
  that kept psort's output root separate is unaffected (its item names
  only lose the extension).
- **`build-all.sh` provisions a bare standalone clone itself, and is
  standalone ONLY.** It used to demand an `ansible-playbook` on PATH and
  stop there, leaving the `community.docker` collection and the docker SDK
  its modules import for the operator to discover one failure at a time.
  It now installs the pinned controller layer (ansible-core + the docker
  SDK, an exact lock written in the script itself) into `<repo>/.venv` and
  the pinned `community.docker 3.10.3` into `<repo>/.ansible/collections`,
  put on `ANSIBLE_COLLECTIONS_PATH` for the run — per checkout, as the
  invoking user, no sudo, nothing written to `ansible.cfg` or anywhere a
  consumer's tooling could pick it up, reinstalling only when the pins
  change — and preflights what it cannot install (python3 ≥ 3.11 with the
  venv module, the docker CLI, a daemon this user may talk to, BuildKit)
  with the fix named on each failure. `--preflight` prepares and verifies
  without building; `GODFIR_ANSIBLE` uses a host's own ansible (its python
  checked for the docker SDK); `GODFIR_VENV` relocates the venv; unknown
  options are refused instead of being forwarded as image names. The
  script refuses to run from a checkout that is a submodule of another
  repository: a consumer builds through the `godfir_build` role from its
  own tooling (DX_DFIR: `dxdfir build-docker`). `.venv` and `.ansible` are
  gitignored and `build_ignore`d from the collection artifact.

## 0.3.1

- **`byakugan` bakes the engine's Elastic detection rules-as-code** — the
  detections move out of DX_DFIR's retired host-python package into the
  byakugan repo itself (its `rules/`: the pinned top-level rule set, the
  `car-detections/` lookup-index contract, the `cti/` indicator-match rule
  — the image clones that repo at the pin anyway, so the rules ride the
  clone). The engine pin advances to the merged move (Byakugan #125) and
  the image bakes `/rules` from it, where `stix-export` already defaults
  its pattern resolution; the `rules` mount now *overrides* the baked set
  instead of supplying the only one. The engine's own `rules/validate.py`
  gates the set during the build: a malformed rule, or drift from its
  `PINNED_IDS`, fails the image — and the engine's test suite holds the
  same gate plus the cti-* template cross-checks. New
  `THIRD_PARTY_NOTICES.md` records the terms of everything the builds
  fetch (DetectRaptor, ET Open, Hayabusa); image 0.3.1.

## 0.3.0

- **`byakugan` carries the STIX/CTI exchange** — the engine pin advances to
  the merged `byakugan.exchange` (Byakugan #124) and the image version to
  0.3.0: four new dispatcher sub-tools beside
  `build|timeline|verify|load|car-vocab` — `stix-export` (detection hits →
  STIX 2.1 sightings +
  indicators, the projection's `stix_bundle.json` merged through),
  `stix-behaviour` (the detection lanes joined to CAR entities as sightings
  over spindle-keyed observed-data), `cti-pull` (OpenCTI indicators → the
  `cti-*` Elastic `_bulk` copy; the one input-less sub-tool) and
  `cti-sightings` (indicator-match alerts → sightings pushed back). The
  contract declares each sub-tool's env block, the shared
  `BYAKUGAN_OPENCTI_*` wire (the token rides env only, never argv), the
  optional `/rules` mount (the deployment's rules-as-code — the engine
  ships none), and the widened `network: optional` note; the `input` mount
  is `required: false` now, for cti-pull's sake, with the engine itself
  refusing a missing input everywhere else.

## 0.2.0

- **`gowindowlicker`** — the Windows matrix as one structured binary: the
  twelve Windows Go parsers move under one directory, one module and one
  FROM-scratch image, every parser a package (`gowindowlicker/prefetch`,
  `gowindowlicker/rb`, …) and a sub-tool of one ~8 MB static binary. Bare
  invocation (or `lick`) is the sweep — every parser over one evidence
  tree, each into its own `<OUT_DIR>/<subtool>/` tree, one aggregate JSON
  summary line — and the argv debug modes ride the dispatcher
  (`gowindowlicker gorb -f FILE`). Each parser keeps its canonical tool
  name, `<SUBTOOL>_*` env block, record shapes and record-file names — the
  interface byakugan and the pipeline consume. The per-tool `batch.go`
  copies collapse into the module's shared `batch/` package
  (`<TOOL>_FORMAT`/CSV kept). The standalone per-parser images, contracts
  and Dockerfiles are retired; `images.yml` replaces the twelve entries
  with one `gowindowlicker` entry whose `subtool_aliases` carry the parser
  names and their EZ-tool names, so `build-all.sh goprefetch` (or `pecmd`)
  resolves to the one image with the "that's a sub-tool now" note.
- **The .NET per-tool lane is retired** — `godfir-tool/` and its five
  images (`sqlecmd`, `bstrings`, `iisgeolocate`, `recentfilecacheparser`,
  `rla`) are removed as succeeded. The five names move to `images.yml`'s
  `unbuildable`, so a consumer naming one gets the stated refusal instead
  of "unknown tool".

## 0.1.0

The repository becomes the plug-and-play BUILD galaxy for the `get-sybers/*`
hardened tool images:

- **`images.yml`** — the canonical root inventory of every image this repo
  builds (the Windows Go parsers, godaemonhunter, gomount, the `.NET`
  per-tool images from the parameterized `godfir-tool/Dockerfile`, and the
  pipeline images), with contexts, dockerfiles, build args, aliases, the
  environment-overridable engine pins (`env_args`), the `unbuildable`
  refusals, and the namespace's `non_tool_repos`. `conform.sh`'s inventory
  consistency check runs against it, and consumers (DX_DFIR at its submodule
  pin) read exactly this file — no consumer-side image list to drift.
- **`godfir_build` role + `playbooks/build_images.yml`** — the build logic as
  ansible tasks end to end: manifest/alias resolution, BuildKit builds
  stamped with the source revision (`com.get-sybers.src`, staleness-replaced
  without `--force`), and the hardening verification on every result (static
  contract, shell-free filesystem scan, and each image's own
  `/etc/dfir-hardened` posture declaration held to the actual filesystem).
- **`build-all.sh`** — now a thin launcher of the collection playbook: no
  image list, no logic. It exists solely for standalone use of this repo;
  integrating consumers use the role.
- **`godfir_build` runtime entries** — `tasks_from: verify` (the supply-chain
  gate: every namespace ref a caller is about to run must be a manifest image
  still carrying the hardened contract on the host; digest pins normalize for
  membership, non-namespace refs pass) and `tasks_from: audit` (every
  manifest image present + hardened, no unexpected namespace image; allow-list
  = the manifest + `non_tool_repos`) — the ansible successors of DX_DFIR's
  retired python guard.
- **molecule coverage** — `roles/godfir_build/molecule/default` runs the whole
  gate matrix offline against a committed FROM-scratch fixture (`molecule
  test`): the build with stamps and env pins, molecule's own idempotence
  gate, staleness replacement, alias/subtool resolution, the four build
  negatives (unknown, unbuildable, declared-shell violation, missing
  declaration), the runtime verify gate (pass / unknown / digest pin /
  wrong uid) and the audit (clean + aggregated violations with the
  `non_tool_repos` exemption honoured).
- **`galaxy.yml`** — the repo installs as the `get_sybers.godfir_toolz`
  collection (`ansible-galaxy collection install git+https://github.com/Get-Sybers/GoDFIR-toolz.git`),
  carrying the manifest, the build role and every build context.
