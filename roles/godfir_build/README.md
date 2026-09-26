# godfir_build

The build galaxy's one role: build `get-sybers/*` images from `images.yml`
(the root inventory) and verify the hardening contract on every result. The
role groups; the caller decides. It is consumed three ways over the same
tasks:

- `build-all.sh` launches the collection playbook (standalone use of this
  repo only);
- DX_DFIR's `dxdfir_images` delegates here at its submodule pin;
- a galaxy install exposes it as `get_sybers.godfir_toolz.godfir_build`.

Variables are documented in `meta/argument_specs.yml` (per entry point);
`ansible-doc -t role` renders them. What follows is the design contract the
variables sit on.

## The tree contract

Everything resolves from one root, `godfir_build_root`:

- **Empty (the default) means this role's own tree.** `roles/godfir_build`
  sits two levels below the repo/collection root in a source checkout and in
  an installed collection alike, so `inventory.yml` normalises empty to
  `role_path/../..` — the manifest and every build context resolve
  identically in both consumption modes, and a galaxy-installed copy is
  fully self-contained.
- A consumer that pins the tree elsewhere (DX_DFIR's submodule checkout, the
  molecule fixture) passes that root instead.

`images.yml` lives **at the tree root by contract** — `conform.sh`,
`build-all.sh`, DX_DFIR and a galaxy install all rely on it. The manifest
lookup is deliberately strict: `inventory.yml` stats the exact path first
and fails with the real reason (wrong root, incomplete checkout,
uninitialised submodule) before anything else runs; nothing hedges a missing
manifest into an empty one. The same early gate requires
`hardening/harden.yml` — the hardening playbook's one canonical home, which
every repo-root-context Dockerfile COPYs directly.

## Calling from another role: freeze your vars first

`include_role` vars are evaluated **lazily, inside the included role**, where
`role_path` and any file lookups resolve against *godfir_build's* tree, not
the caller's. A caller that derives `godfir_build_root` (or anything else)
from its own `role_path` or filesystem must resolve those decisions into
concrete facts (`set_fact`) **before** the include and pass the frozen
values. DX_DFIR's `dxdfir_images` role is the reference implementation.

## Build semantics (entry point: `main`)

- **Resolution first.** `resolve.yml` validates the whole requested set
  before any image builds: manifest-declared `unbuildable` names are refused
  with their reason, unknown names fail listing the valid set, aliases and
  `subtool_aliases` (case-insensitive) map to their canonical image — a
  sub-tool alias logs a note that it builds the parent image. An empty set
  means every manifest image, in manifest order.
- **Source stamps decide staleness.** Every image is labelled
  `com.get-sybers.src` with the source identity: an explicit
  `godfir_build_src_sha` wins, else `git rev-parse HEAD` of the tree, else
  the installed collection's `MANIFEST.json` version, else `unknown` — the
  chain never blocks a build. A present image whose stamp differs is stale
  and gets rebuilt (`rebuild: always`) without `--force`; a re-run at an
  unchanged source is a no-op, which is what molecule's idempotence step
  asserts. The image the rebuild replaces is removed, so stale builds never
  accumulate as dangling layers.
- **BuildKit, not the legacy builder** — the tool Dockerfiles use heredoc
  COPY and the `# syntax=` frontend directive, both BuildKit-only. Build
  args combine in rising precedence: the manifest entry's `args`, then
  `env_args` actually set in the environment, then the single-source knobs
  (`DFIR_UID`/`DFIR_GID`, `GODFIR_REVISION`/`GODFIR_RELEASE`) — the knobs
  always win over a per-image arg.

## Hardening verification (after every build)

1. **Static contract**: `USER <uid>:<gid>` exactly, and
   `com.get-sybers.hardened=true`.
2. **Filesystem scan without a shell in the image**: the image is
   `docker create`d (with the never-executed `__export_only__` command
   token, so images without an ENTRYPOINT work) and `docker export`ed; the
   tar listing must show none of the removed surface (`apt-get`, `dpkg`,
   `sudo`, `pip`, anything ansible). An empty listing fails outright, so
   the asserts can never pass vacuously. The scan pipelines preserve the
   real exit code across the container cleanup. The shell blocks are
   literal scalars, one command per line — a comment inside a folded
   scalar once turned the whole pipeline into a bash comment, and the
   asserts passed against an empty listing.
3. **Self-declared posture**: every image ships `/etc/dfir-hardened`
   (schema=1: `static_binary` / `shell` / `python` / `pkg_mgr`, written by
   its Dockerfile or by `harden.yml`). The declaration drives the surface
   asserts — `shell=false` means no `bin/sh|bash|dash` in the export,
   `python=false` no `bin/python3*` — so no image-name list lives in this
   role and a new manifest image arrives carrying its own posture. An image
   without the declaration fails: the asserts would otherwise have nothing
   to hold it to.

## The runtime gate (entry point: `verify`)

The ansible successor of the retired python guard's `--require`: every
namespace ref in `godfir_build_verify_images` must be a manifest image and
must still carry the hardened contract **on this host**. A substituted or
un-hardened image stops the caller before any evidence is touched.

- Refs outside the namespace (documented operator-supplied images) are out
  of scope and pass untouched.
- A **registry-qualified** namespace ref is in scope: the registry segment
  (docker's own heuristic — a first path component containing a dot or
  colon, or `localhost`) is normalised away for the membership check, so
  `ghcr.io/<ns>/evil` cannot smuggle past the gate behind its prefix.
- Membership is checked on the fully normalised repo: registry stripped,
  then `@sha256:` digest, then a trailing `:tag` (only after the last
  slash — a colon before that is a registry port).
- The ref is inspected **as given**, so a digest pin or registry-qualified
  ref must itself exist locally and carry the contract.

```yaml
- ansible.builtin.include_role:
    name: get_sybers.godfir_toolz.godfir_build
    tasks_from: verify
  vars:
    godfir_build_verify_images: ["get-sybers/goevtx:latest"]
```

## The namespace audit (entry point: `audit`)

The successor of the python guard's `--audit`, in one aggregated report so a
failing audit names everything wrong at once:

- every manifest tool image is **present** and still carries the hardened
  contract on this host;
- **no unexpected namespace image exists** — the allow-list is the manifest
  images plus the manifest's `non_tool_repos`; anything else under the
  namespace is a supply-chain red flag. The sweep applies the same
  registry normalisation as the gate and names each violation by the tag
  as tagged.

## Testing

`molecule/default` is the committed proof: an offline `molecule test`
(`prerun: false`, dependency step disabled — `community.docker` is the
runner's install, declared by `galaxy.yml`) over a FROM-scratch fixture in
`molecule/default/files/fixture`. It covers build + idempotence
(`changed=0`), staleness replacement at a new source stamp (old image gone,
zero dangling), alias and sub-tool resolution, an environment-pin
pass-through (`TESTPIN`), the four build negatives (unknown, unbuildable,
declared-shell-false-with-a-shell, missing declaration), the verify gate
(pass incl. registry-qualified, unknown, registry-prefixed smuggling,
digest-pin presence, wrong-uid) and the audit (clean, then aggregated
violations honouring the `non_tool_repos` exemption). Every negative runs
the role inside `block`/`rescue` and asserts the expected refusal text —
passing when it should fail, or failing for the wrong reason, fails the
scenario.
