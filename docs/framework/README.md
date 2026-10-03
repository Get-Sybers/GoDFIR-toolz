# GoDFIR-toolz Container Framework

**The hardening, provisioning, and verification standard for every `get-sybers/*` tool image.**

**Status:** design standard. **Scope:** the GoDFIR-toolz repository, consumed by
DX_DFIR as the submodule at `docker/GoDFIR-toolz/`. This paper states the target
every tool image conforms to; the repository migrates toward it along the
roadmap in [10 — Migration roadmap](10-migration-roadmap.md). The `anamnesis`
tool is the reference implementation of the end state.

## Abstract

GoDFIR-toolz is a matrix of single-purpose, distroless container images for
batch digital forensics. Each image is a self-provisioning, environment-driven,
run-once black box: hardening is applied at build time by an in-image Ansible
playbook and squashed away before the runtime stage is cut; a no-argument
entrypoint orchestrates the tool at run time from a machine-readable environment
contract; and the image declares its own hardening posture in
`/etc/dfir-hardened`. The producer repository owns hardening and its
verification end-to-end. An in-repo CI gate builds every image, proves the built
artifact matches its declaration, exercises the contract over committed
fixtures, generates an SBOM, and only then cuts a release as a checksum-manifested
`docker save` bundle that crosses an air gap without a registry. The consumer,
DX_DFIR, pins exactly one GoDFIR-toolz release, verifies image checksums and
version labels at preflight, and drives every tool purely through `-e` and `-v`.
A zero-CVE program, enforced by `cve-scan.sh` with dated, expiring waivers, is
switched on as the final phase once every image is inside the framework.

## The north star: one ownership boundary

GoDFIR-toolz **owns hardening end-to-end, including its verification**. The
per-image checks that DX_DFIR's `dxdfir_images` Ansible role and
`get_sybers_dxdfir.images` Python guard perform after each build live in
GoDFIR-toolz CI, where they run on every image, every pull request, every
release. DX_DFIR is left with two responsibilities: (1) pin one GoDFIR-toolz
release and check, at preflight, that the loaded images carry that release's
checksums and version labels; (2) drive each tool purely through its environment
contract. The supply chain is registry-free and air-gap capable: build →
`docker save` bundle → offline `docker load`.

## Design principles

1. **Self-provisioning.** Every image provisions itself; there is no external
   control node. Build-time Ansible (`hardening/harden.yml`) hardens the rootfs;
   a no-argument entrypoint orchestrates the run.
2. **Two phases, cleanly split.** Provisioning happens at build time and is
   removed from the image. Orchestration happens at run time and reads only the
   environment and the mounts.
3. **The environment contract is the primary interface.** `contract.yml`
   declares every variable, mount, exit code, and the stdout summary schema.
   argv is a debug pass-through the consumer never uses.
4. **Run once and exit.** Discover inputs, batch over them, converge
   idempotently, print exactly one JSON summary line on stdout, return a uniform
   exit code. No daemon, no watch loop, no log tailing.
5. **Distroless by default.** The runtime stage is `FROM scratch` around a static
   binary, or a build-time-hardened minimal rootfs re-copied into scratch. It
   carries no shell, package manager, or interpreter unless the tool provably
   needs one, and then it declares it.
6. **Nonroot, fixed identity.** `USER 2000:2000`; uid 0 renamed and locked;
   every setuid/setgid bit stripped.
7. **The image is a declared artifact, not a snapshot.** `contract.yml`,
   `images.yml`, pinned build inputs, `/etc/dfir-hardened`, and the label set
   declare it; CI proves the built image matches every part of the declaration.
8. **Producer-owned verification.** The release gate lives in GoDFIR-toolz CI.
   An image is releasable only when it passes the full gate.
9. **Registry-free, air-gap-capable supply chain.** Checksum-verified build
   inputs in; a checksum-manifested `docker save` bundle out; one transitive pin
   on the consumer side.
10. **Zero CVEs as a ratchet.** An SBOM for every image and a CVE gate with
    dated, expiring waivers that tightens release over release — enabled only
    after every image has migrated into the framework.
11. **Single-purpose images, minimal closure.** One tool per image. Multi-tool
    images are declared deviations fronted by a dispatcher entrypoint.

## Influences and prior art

The self-provisioning shape of a GoDFIR-toolz image — Ansible carried in the
image and run against localhost, an environment-driven entrypoint, idempotent
convergence, configuration as data — descends from the docker-splunk /
splunk-ansible pattern; the framework applies it to one-shot batch tools by
moving provisioning to build time and having the entrypoint exit rather than
tail a service. The hardening posture — distroless runtime stages, static
binaries, nonroot by default, a minimal dependency closure, an SBOM for every
image, and a low-to-zero-CVE target — follows the philosophy established by
Chainguard, Wolfi, and the distroless projects, applied here to hand-written
multi-stage Dockerfiles over Debian and Alpine bases rather than through
apko/melange. This paragraph is the only place the paper speaks of provenance;
everywhere else it speaks as the standard.

## Contents

| # | File | Covers |
|---|---|---|
| 01 | [Architecture and boundary](01-architecture-and-boundary.md) | Producer/consumer boundary; the container as a self-provisioning, env-driven, run-once black box; who owns what |
| 02 | [Tool directory structure](02-tool-directory-structure.md) | The mandated `<tool>/` layout, repo-level files, what each MUST file contains and does |
| 03 | [Environment contract](03-environment-contract.md) | Variable naming, reserved variables, volume convention, exit-code table, the single-JSON-line rule, `contract.yml` format and worked example |
| 04 | [Self-orchestration](04-self-orchestration.md) | No-argument entrypoint behaviours, the debug argv pass-through, the multi-tool dispatcher |
| 05 | [Hardening standard](05-hardening-standard.md) | The two sanctioned build shapes, static/nonroot/no-shell rules, `harden.yml` guarantees, the `/etc/dfir-hardened` schema, required labels, SBOM generation |
| 06 | [Verification gate](06-verification-gate.md) | The in-repo CI gate as the release criterion and the ownership move it represents |
| 07 | [Supply chain and versioning](07-supply-chain-and-versioning.md) | Build-time dependency verification, the checksum-only release manifest, the single transitive pin, the air-gap flow, what the consumer checks |
| 08 | [Zero-CVE program](08-zero-cve-program.md) | SBOM + CVE scanning via `cve-scan.sh`, the dated-waiver ratchet, and the sequencing rule |
| 09 | [Tool template](09-tool-template.md) | The copy-me `_template/` skeleton |
| 10 | [Migration roadmap](10-migration-roadmap.md) | The tiered tool map and the phased, keep-the-pipeline-green sequence |
| 11 | [Decisions and open questions](11-decisions-and-open-questions.md) | Locked decisions and the genuinely remaining open questions |

Cross-references within the paper use `§<file>.<section>` notation: `§3.4` is
section 4 of file 03.
