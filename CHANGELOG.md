# Changelog — get_sybers.godfir_toolz

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
- **`build-all.sh` provisions a bare standalone clone itself.** It used to
  demand an `ansible-playbook` on PATH and stop there, leaving the
  `community.docker` collection and the docker SDK its modules import for
  the operator to discover one failure at a time. It now installs the
  pinned controller layer (new `requirements.txt`, DX_DFIR's lock) into
  `<repo>/.venv` and the pinned collection (new `requirements.yml`,
  `community.docker 3.10.3`) into `<repo>/.ansible/collections` — per
  checkout, as the invoking user, no sudo, reinstalling only when the lock
  changes — and preflights what it cannot install (python3 ≥ 3.11 with the
  venv module, the docker CLI, a daemon this user may talk to, BuildKit)
  with the fix named on each failure. `--preflight` prepares and verifies
  without building; `GODFIR_ANSIBLE` uses a host's own ansible (its python
  checked for the docker SDK); `GODFIR_VENV` relocates the venv; unknown
  options are refused instead of being forwarded as image names.
  `ansible.cfg` puts the in-tree collection path first on
  `collections_path` with the user/system paths after, so a host that
  already has the collection keeps resolving it; `.venv` and `.ansible`
  are gitignored and `build_ignore`d from the collection artifact. The
  script remains standalone-only: a consumer uses the `godfir_build` role
  and nothing in DX_DFIR calls it.

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
