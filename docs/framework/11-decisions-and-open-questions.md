# 11 — Decisions and open questions

[← 10 — Migration roadmap](10-migration-roadmap.md) · [README](README.md)

## 11.1 Locked decisions

These are design facts of the standard. Each is stated in full where the
table points.

| # | Decision | Where |
|---|---|---|
| 1 | GoDFIR-toolz owns hardening end-to-end, including its verification; per-image checks live in GoDFIR-toolz CI and leave DX_DFIR | [01](01-architecture-and-boundary.md), [06](06-verification-gate.md) |
| 2 | The environment contract is the primary interface; argv is a debug pass-through the consumer never uses | [§3.1](03-environment-contract.md), [§4.2](04-self-orchestration.md) |
| 3 | The exit-code table is `0/1/2/3` with `3` = partial; anamnesis's `0/1/2` remain valid and `3` is additive; Go tools remap argv-mode partial `2`→`3` in batch mode | [§3.5](03-environment-contract.md) |
| 4 | stdout carries exactly one JSON summary line; records go to files; `<TOOL>_OUT_DIR` is effectively required in batch mode | [§3.6](03-environment-contract.md) |
| 5 | Ansible is a build-time provisioner only (`hardening/harden.yml`); it is never present in a runtime image | [§1.2](01-architecture-and-boundary.md), [§5.5](05-hardening-standard.md) |
| 6 | Runtime stages are distroless; two sanctioned shapes: static `FROM scratch` (default) and build-time-hardened rootfs re-copied into scratch | [§5.2–5.3](05-hardening-standard.md) |
| 7 | `/etc/dfir-hardened` is a `schema=1` record with explicit `static_binary` / `shell` / `python` / `pkg_mgr` booleans; the gate's assertion set derives from it, not from a name list | [§5.6](05-hardening-standard.md), [§6.3–6.4](06-verification-gate.md) |
| 8 | Shell-bearing multi-tool images (plaso, signatures, gomount) are accepted as declared `shell=true` deviations | [§5.9](05-hardening-standard.md) |
| 9 | Every image carries the OCI + `com.get-sybers.*` label set, including `version`, `revision`, `godfir-release`, `contract`; engine images add `engine-ref` | [§5.7](05-hardening-standard.md), [§7.3](07-supply-chain-and-versioning.md) |
| 10 | The release manifest is checksum-only and unsigned; the image ID and version labels ride the `docker save` bundle; no cosign/minisign, no key custody | [§7.4](07-supply-chain-and-versioning.md) |
| 11 | GoDFIR-toolz guarantees the supply chain by verifying every build-time dependency and emitting the built image's checksum; the consumer verifies only that checksum and the labels | [§7.1, §7.7](07-supply-chain-and-versioning.md) |
| 12 | Version pinning is single and transitive: DX_DFIR pins one GoDFIR-toolz release; per-tool versions and the engine refs (anamnesis, byakugan) are pinned inside GoDFIR-toolz; the byakugan-engine-pin assertion moves from DX_DFIR's tests into GoDFIR-toolz CI | [§7.5](07-supply-chain-and-versioning.md) |
| 13 | `images.yml` lives in GoDFIR-toolz; DX_DFIR reads it from the submodule | [§2.2](02-tool-directory-structure.md) |
| 14 | The CVE gate uses `cve-scan.sh`: fail on fixable findings at or above `high`, unfixed reported only, dated waivers in `hardening/cve-waivers.yml` that block when expired; the threshold ratchets down and never up | [§8.4–8.7](08-zero-cve-program.md) |
| 15 | **The CVE gate is enabled and enforced only after every image has migrated; it is the final phase and is never run against a partially-migrated image set. During migration `cve-scan.sh` is a manual triage tool, not a release gate** | [§8.0](08-zero-cve-program.md), [§10.4](10-migration-roadmap.md) |
| 16 | anamnesis is the Tier-0 reference implementation | [§1.6](01-architecture-and-boundary.md), [§10.2](10-migration-roadmap.md) |
| 17 | goyara is a component of `signatures`, built inside it, with no image of its own to harden | [§10.2](10-migration-roadmap.md) |
| 18 | The migration is phased and keeps the pipeline green: argv modes persist until lanes switch; DX_DFIR's asserts persist until the producer gate covers every image | [§10.4](10-migration-roadmap.md) |

## 11.2 Open questions

Three questions remain genuinely open. Each is a choice between workable
options; nothing in the standard depends on which way it goes.

### 1. Where does CI run, given the air gap?

GoDFIR-toolz has no `.github/workflows/` today and the supply chain is
registry-free. Two shapes are possible:

- **GitHub-hosted CI** (internet): builds, gates, scans, and produces the
  offline bundle; only the *bundle* crosses the air gap.
- **A fully offline runner**: the whole gate runs inside the air gap, which
  requires mirroring the grype vulnerability database (and syft) onto the
  runner and passing `--db-dir`.

`cve-scan.sh` supports both ([§8.6](08-zero-cve-program.md)); the choice
decides the CI tooling and where the release job lives.

### 2. Per-image semver or one monotonic release tag?

The consumer-side pin is settled: one release value
([§7.5](07-supply-chain-and-versioning.md)). What remains is whether
`org.opencontainers.image.version` is an independent per-image semver (bumped
when that tool's behaviour or contract changes, so a tool can be reasoned
about across releases) or simply mirrors `com.get-sybers.godfir-release`. The
question is whether anyone ever needs to identify *one* tool's version
independently of the matrix it shipped in.

### 3. gomount and goyara: first-class images or components of `signatures`?

gomount has its own Dockerfile and a declared Shape-B deviation but is absent
from `images.yml`; goyara is built inside `signatures`. Either they become
first-class entries in the inventory (each gated on its own), or they are
formally listed as components of `signatures` (documented in
`signatures/contract.yml`, gated as part of that image). The `stream` verb
gomount already provides to `signatures` argues for the component reading;
its standalone Dockerfile argues for the inventory reading.

## 11.3 Possible future hardening

Not part of the standard; recorded so they are not re-derived:

- a detached signature over `RELEASE.yml` (cosign or minisign, with the public
  key provisioned on the air-gapped host);
- in-toto/SLSA provenance attestations;
- moving the signatures YARA scan loop and the plaso wrappers into small static
  Go runners so those images reach shell-free.

[← 10 — Migration roadmap](10-migration-roadmap.md) · [README](README.md)
