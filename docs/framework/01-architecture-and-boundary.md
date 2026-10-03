# 01 — Architecture and the ownership boundary

[← README](README.md) · [02 — Tool directory structure →](02-tool-directory-structure.md)

## 1.1 Two parties, one boundary

GoDFIR-toolz is the **producer**. It builds, hardens, verifies, and releases
every `get-sybers/*` tool image. DX_DFIR is the **consumer**. It loads a released
bundle, checks it, and drives each tool through its environment contract from
inside the pipeline lanes.

The boundary is drawn so that nothing about an image's internals is ever
re-examined on the consumer side. Verification of hardening is the producer's
job and runs in GoDFIR-toolz CI on every image, every pull request, every
release ([06](06-verification-gate.md)). DX_DFIR has exactly two
responsibilities toward a tool image:

1. **Pin and check.** DX_DFIR carries one GoDFIR-toolz release pin. At
   preflight it verifies that each loaded image's checksum matches the release
   manifest and that its version labels name the pinned release
   ([07](07-supply-chain-and-versioning.md)).
2. **Drive by environment.** DX_DFIR constructs every `docker run` purely from
   the tool's `contract.yml`: `-e NAME=VALUE` for each declared variable and
   `-v` for each declared mount, and nothing else. It never builds argv on the
   host ([03](03-environment-contract.md)).

| Concern | Producer — GoDFIR-toolz | Consumer — DX_DFIR |
|---|---|---|
| Dockerfile, build shape, hardening | owns | — |
| Verifying hardening (surface scan, self-declaration, labels) | owns, in CI | — |
| Contract conformance, idempotency, summary schema | owns, in CI | — |
| SBOM generation and the CVE gate | owns, in CI | never runs it |
| Inventory (`images.yml`) | owns | reads it from the submodule |
| Build-input verification (pinned, checksum-verified inputs) | owns | — |
| Release bundle and checksum manifest | produces | verifies checksum and labels at preflight |
| Version pinning | pins per-tool versions and engine source refs internally | pins one release |
| Driving a tool | — | environment contract only (`-e`, `-v`) |
| Host hygiene (`ensure_built` self-heal; no unexpected `get-sybers/*` images) | — | keeps |

## 1.2 The container: a self-provisioning, env-driven, run-once black box

Each GoDFIR-toolz tool container is self-provisioning: hardening is applied at
build time by an in-image Ansible playbook, and a no-argument entrypoint
orchestrates the tool at run time from its environment contract. There is no
external control node in either phase, and there is no long-running service.

A tool image has two phases with nothing shared between them:

| Phase | What runs | Reads | Produces |
|---|---|---|---|
| **Build** | `hardening/harden.yml` through a throwaway Ansible install (Shape B), or nothing beyond a static `go build` (Shape A) | pinned build-args, `images.yml` source refs, checksum-verified upstream inputs | a distroless runtime stage; the `/etc/dfir-hardened` self-declaration; the label set |
| **Run** | the tool binary as `ENTRYPOINT`, invoked with no arguments | the `<TOOL>_*` environment and the declared mounts | one output subfolder per discovered input, exactly one JSON summary line on stdout, a uniform exit code |

The properties that follow from this split:

- **Provisioning never ships.** Ansible, Python, pip, the package manager, and
  every shell the tool does not need are removed in the same layer that used
  them, and the runtime stage is a fresh `FROM scratch` copy of what remains
  ([05](05-hardening-standard.md)).
- **Configuration is data.** Every variable the tool reads is declared with its
  default in `contract.yml`; an environment variable overrides the default. The
  checked-in contract is the source of truth for the producer's tests, the
  consumer's driver, and the tool's README alike.
- **The run is idempotent.** Re-running a container over the same inputs
  converges: items with valid output are skipped unless forced; nothing is
  duplicated or corrupted ([04](04-self-orchestration.md)).
- **The run ends.** The entrypoint discovers inputs, batches over them, prints
  one JSON summary line, and exits. There is no daemon, no watch loop, and no
  `tail -f`.

## 1.3 What the producer owns

GoDFIR-toolz owns:

- the Dockerfile and build shape of every image, and the build-time hardener
  `hardening/harden.yml`;
- the machine-readable interface of every tool (`contract.yml`) and its schema
  (`hardening/contract.schema.yml`);
- the inventory `images.yml`, including per-tool versions, build-args, and the
  source refs of engines built from external repositories (anamnesis, byakugan);
- the verification gate (`verify-all.sh` locally, `.github/workflows/ci.yml` in
  CI) and the release criterion;
- the SBOMs, the CVE scan, and the waiver ledger;
- the release bundle and its checksum manifest.

## 1.4 What the consumer does

DX_DFIR keeps three things, none of which opens an image:

- **`ensure_built` self-heal** — build an image from the submodule when it is
  missing locally.
- **The preflight version check** — for each image in the pinned release,
  checksum and labels match the manifest ([§7.7](07-supply-chain-and-versioning.md)).
- **The `images.py` inventory audit** — no `get-sybers/*` image exists on the
  host that the inventory does not name. This is host hygiene, not an
  image-internals audit.

`dxdfir_images/tasks/build.yml`'s post-build asserts and
`get_sybers_dxdfir.images.check_config` duplicate the producer gate once every
image is inside the framework, and are removed in the final consumer cutover
([§10.4](10-migration-roadmap.md)).

## 1.5 The image is a declared artifact, not a snapshot

An image is defined by its declaration, and CI proves the built image matches
it. The declaration is the union of:

| Part | Declares |
|---|---|
| `contract.yml` | the interface: variables, mounts, exit codes, summary schema, output layout |
| `images.yml` | the inventory entry: image name, per-tool version, build-args, source refs |
| pinned Dockerfile inputs | Go toolchain version, base images, upstream release checksums and key fingerprints |
| `/etc/dfir-hardened` | the hardening posture the image claims for itself |
| the label set | title, source, licence, version, revision, release tag, contract version, hardened flag |

A build that diverges from any part of the declaration fails the gate
([06](06-verification-gate.md)).

## 1.6 The reference implementation

`anamnesis` is the reference implementation of the end state: multi-stage
build, env-driven self-orchestrating batch entrypoint, one JSON summary line,
uniform exit codes, no shell and no Python in the runtime stage, glibc retained
under a written justification, `HOME` and caches pointed at a writable tmpfs.
Where this paper gives a worked example, it is anamnesis.

## 1.7 Supply chain shape

The supply chain is registry-free. GoDFIR-toolz guarantees it in two halves:
every build-time dependency is verified during the build (pinned versions,
sha256 checksums, key fingerprints), and the built image's checksum is emitted in
a release manifest that travels with the `docker save` bundle. The consumer
verifies that checksum and nothing more. [07](07-supply-chain-and-versioning.md)
specifies the bundle, the manifest, the single transitive pin, and the air-gap
flow.

[← README](README.md) · [02 — Tool directory structure →](02-tool-directory-structure.md)
