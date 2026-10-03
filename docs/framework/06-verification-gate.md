# 06 — The verification gate

[← 05 — Hardening standard](05-hardening-standard.md) · [07 — Supply chain and versioning →](07-supply-chain-and-versioning.md)

## 6.0 The ownership move

The per-image verification is the producer's job. The checks that DX_DFIR
performs after each build — `dxdfir_images/tasks/build.yml`'s asserts and
`get_sybers_dxdfir.images` — belong in GoDFIR-toolz CI, where they run on
**every image, every pull request, every release**. An image is **releasable
only if it passes the full gate.** DX_DFIR is then reduced to the preflight
version check ([07](07-supply-chain-and-versioning.md)).

The gate is one script, `verify-all.sh`, run locally by a maintainer and by
`.github/workflows/ci.yml` in CI. Per image it performs the following steps in
order.

## 6.1 Build

Build the image with BuildKit (the heredoc `COPY` and the `# syntax=` directive
require it) at `DFIR_UID=2000 DFIR_GID=2000`, applying any build-args and `ref`
pins from `images.yml`.

## 6.2 Static contract

`docker image inspect` and assert:

- `Config.User == "2000:2000"` (exact);
- `Config.Labels["com.get-sybers.hardened"] == "true"`;
- the version labels are present: `org.opencontainers.image.version`,
  `org.opencontainers.image.revision`, `com.get-sybers.godfir-release`,
  `com.get-sybers.contract`.

## 6.3 Filesystem surface scan

The image has no shell, so the scan is shell-free:

```
cid=$(docker create <img> __export_only__); docker export "$cid" | tar -t; docker rm -f "$cid"
```

Assert on the member list:

- **Every image:** `usr/bin/apt-get`, `usr/bin/dpkg`, `usr/bin/sudo` absent; no
  `bin/pip[0-9.]*`; no `/ansible[-/]` anywhere (the build-time provisioner is
  gone).
- **Tool-only images** — those whose `/etc/dfir-hardened` declares
  `shell=false` and `python=false` (the Go/scratch tools, zeek, anamnesis):
  additionally **no shell** (`bin/(sh|bash|dash)`) and **no Python**
  (`bin/python3(\.N)?`).

The assertion set is derived from each image's own `shell=` / `python=`
booleans ([§5.6](05-hardening-standard.md)), not from a hand-maintained name
list, so it cannot drift. Images that legitimately keep an interpreter (plaso,
byakugan) declare `python=true` and are exempt from the Python assertion but
remain asserted shell-free unless they also declare `shell=true` (plaso and
signatures do, with justification).

## 6.4 Self-declaration cross-check

Read `/etc/dfir-hardened` from the export and assert that its `shell=`,
`python=`, `pkg_mgr=`, and `static_binary=` booleans MATCH what the filesystem
scan actually found. This catches a Dockerfile that claims hardened but
shipped a shell.

## 6.5 Contract and summary conformance

This step proves [03](03-environment-contract.md) and
[04](04-self-orchestration.md):

- Validate `contract.yml` against `hardening/contract.schema.yml`.
- Run the image over `test/fixtures/` in **batch mode (no arguments)** with the
  environment contract set. Assert: the exit code is in the contract's table;
  **stdout is exactly one line**; that line is JSON matching `summary_schema`.
- Re-run with the same inputs and assert **idempotency**: the second run's
  status is `nothing` (or every item skipped), and no duplicate output exists.
- Run with a bad environment (a missing required mount) and assert exit `2`.
- Assert README ↔ `contract.yml` agreement (variables and exit codes).
- Where the tool implements `--print-contract`, assert its output equals the
  checked-in file.

## 6.6 SBOM

Generate the SBOM ([§5.8](05-hardening-standard.md)) with `syft` and store it at
`sbom/<tool>.spdx.json`. The SBOM is a release artifact in its own right and
rides the bundle.

## 6.7 CVE gate — final phase only

The CVE gate (`cve-scan.sh`, [08](08-zero-cve-program.md)) is the seventh step
of the gate **once every image has migrated into the framework, and not
before**. It is never run as a release criterion against a partially-migrated
image set. Until the migration is complete the step is absent from
`verify-all.sh` and `ci.yml`; `cve-scan.sh` may be run by hand for triage.

## 6.8 Release criterion

A GoDFIR-toolz release is cut only when **all images pass §6.1–6.6** (and §6.7
once enabled). The release produces the versioned `docker save` bundle and its
checksum manifest ([07](07-supply-chain-and-versioning.md)). CI runs the full
matrix (the `build-all.sh` names) on pull requests touching `<tool>/` or
`hardening/`.

## 6.9 Net effect on DX_DFIR

`dxdfir_images/tasks/build.yml`'s post-build asserts and
`get_sybers_dxdfir.images.check_config` become **redundant** with the producer
gate. DX_DFIR keeps only:

- **`ensure_built` self-heal** — build from the submodule when an image is
  missing locally;
- the **preflight version check** ([§7.7](07-supply-chain-and-versioning.md));
- the **`images.py` inventory audit** — no unexpected `get-sybers/*` image on
  the host. This is a host-hygiene check, not an image-internals audit; the
  internals audit is the producer's job.

The redundant asserts are deleted in the final consumer cutover
([§10.4](10-migration-roadmap.md)), after every image is inside the framework.

[← 05 — Hardening standard](05-hardening-standard.md) · [07 — Supply chain and versioning →](07-supply-chain-and-versioning.md)
