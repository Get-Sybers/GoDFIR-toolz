# Changelog — get_sybers.godfir_toolz

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
- **`galaxy.yml`** — the repo installs as the `get_sybers.godfir_toolz`
  collection (`ansible-galaxy collection install git+https://github.com/Get-Sybers/GoDFIR-toolz.git`),
  carrying the manifest, the build role and every build context.
