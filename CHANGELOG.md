# Changelog — get_sybers.godfir_toolz

## 0.2.0

- **`gowindowlicker`** — the twelve Windows Go parsers move under one
  directory, one module and one FROM-scratch image, the godaemonhunter shape
  (docs/linux decision 16) applied to the Windows side: every parser a
  package (`gowindowlicker/prefetch`, `gowindowlicker/rb`, …) and a sub-tool
  of one ~8 MB static binary; bare invocation (or `lick`) is the sweep —
  every parser over one evidence tree, each into its own
  `<OUT_DIR>/<subtool>/` tree, one aggregate JSON summary line; the argv
  debug modes ride the dispatcher (`gowindowlicker gorb -f FILE`). Tool
  names, `<SUBTOOL>_*` env blocks, record shapes and record-file names are
  unchanged — byakugan sees the same records; only the packaging is one. The
  byte-identical per-tool `batch.go` is promoted to the module's shared
  `batch/` package (`<TOOL>_FORMAT`/CSV kept — pinfo adoption stays a
  separate phase, docs/linux §11.2). The standalone per-parser images,
  contracts and Dockerfiles are retired; `images.yml` replaces the twelve
  entries with one `gowindowlicker` entry whose `subtool_aliases` carry the
  parser names and their EZ-tool names, so `build-all.sh goprefetch` (or
  `pecmd`) resolves to the one image with the "that's a sub-tool now" note.

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
