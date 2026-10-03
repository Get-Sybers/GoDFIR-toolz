# 07 — Supply chain and versioning

[← 06 — Verification gate](06-verification-gate.md) · [08 — Zero-CVE program →](08-zero-cve-program.md)

## 7.1 The guarantee, in two halves

GoDFIR-toolz guarantees the supply chain by (a) verifying every build-time
dependency during its own build and (b) emitting the built image's checksum in
a release manifest that travels with the `docker save` bundle. The consumer
verifies that checksum — and the version labels — and nothing more. There is no
registry, no signing infrastructure, and no re-audit of image internals on the
consumer side: the gate ([06](06-verification-gate.md)) ran before the bundle
was cut.

## 7.2 Build-time dependency verification

Every input to a build is pinned and verified before it is used. A build that
cannot verify an input fails.

| Input | Pinned by | Verified by |
|---|---|---|
| Go toolchain | `GO_VERSION` build-arg (image tag) | pinned tag; `GOTOOLCHAIN=local` where the build stays offline |
| Go module graph | `go.mod` / `go.sum` | `go mod download` checks every module against `go.sum` |
| base images | tag in the Dockerfile `FROM` | pinned tag; refreshed deliberately, never floating |
| upstream binary releases (MemProcFS for anamnesis; the godfir-tool release) | version in a build-arg or `images.yml` | sha256 compared in the Dockerfile; mismatch fails the build |
| upstream package repositories (zeek) | signing-key fingerprint | fingerprint asserted before any package is installed |
| engine sources from external repositories (anamnesis, byakugan) | `ref:` in `images.yml` | cloned at the pinned commit; the built image's `com.get-sybers.engine-ref` label is asserted against the pin in CI |

This is the first half of the guarantee: what goes into an image is known, and
the build proves it.

## 7.3 Version labels

Every image carries the label set of [§5.7](05-hardening-standard.md). The
version-bearing labels are:

| Label | Value |
|---|---|
| `org.opencontainers.image.version` | the per-image version; bumped when the tool's behaviour or contract changes |
| `org.opencontainers.image.revision` | the GoDFIR-toolz commit sha the image was built from |
| `com.get-sybers.godfir-release` | the GoDFIR-toolz release tag (e.g. `2026.09`), stamped on the whole matrix in one release |
| `com.get-sybers.contract` | the contract schema version |
| `com.get-sybers.engine-ref` | for images built from an external engine repository: the pinned source ref |

`images.yml` records the same `version` and `godfir-release` values per tool,
and CI asserts the labels match the inventory. Whether
`org.opencontainers.image.version` is an independent per-image semver or
simply mirrors the release tag is an open question
([11](11-decisions-and-open-questions.md)); the consumer-side pin is one value
either way (§7.5).

## 7.4 The release bundle and its checksum manifest

The release job produces four artifacts:

```
godfir-toolz-<release>.tar              # docker save of every image in images.yml
godfir-toolz-<release>.tar.sha256       # checksum of the tarball, for transfer integrity
godfir-toolz-<release>.RELEASE.yml      # the checksum manifest (below)
sbom/<tool>.spdx.json                   # per-image SBOMs
```

The manifest:

```yaml
# godfir-toolz-<release>.RELEASE.yml — checksum manifest; unsigned.
release: "2026.09"
commit: "<GoDFIR-toolz commit sha>"
contract: 1
images:
  - name: get-sybers/anamnesis:latest
    tool: anamnesis
    version: "1.4.0"
    id: "sha256:<image id>"
  - name: get-sybers/gomft:latest
    tool: gomft
    version: "0.4.0"
    id: "sha256:<image id>"
  # ... one entry per image in images.yml
sboms:
  - {tool: anamnesis, path: sbom/anamnesis.spdx.json, sha256: "<sha256 of the file>"}
  # ...
```

The manifest is **checksum-only and unsigned**: no cosign, no minisign, no key
custody, no public key to provision on the air-gapped host. The image checksum
and the OCI version labels are what ride the tarball.

The checksum is the **image ID** — the sha256 of the image configuration.
`docker save` preserves it, so `docker image inspect --format '{{.Id}}'` after
`docker load` yields exactly the value the release job recorded. Registry
digests (`RepoDigests`) require a push and are not used.

A detached signature over the manifest is a possible future hardening; it is
not part of the standard (§7.8).

## 7.5 The single transitive pin

DX_DFIR pins **one** GoDFIR-toolz release. Concretely, the pin is the
submodule commit at `docker/GoDFIR-toolz/` together with the expected
`com.get-sybers.godfir-release` value in DX_DFIR's `sources.yml`. Everything
beneath that one value is pinned inside GoDFIR-toolz and travels transitively:

- per-tool versions (`images.yml` `version:`; the image `version` label);
- the anamnesis source ref;
- the byakugan source ref;
- upstream release checksums, key fingerprints, the Go toolchain, base images.

Consequences:

- Bumping the toolbox on the consumer side is a one-line pin bump plus
  dropping in the new bundle. DX_DFIR never pins a per-image sha or a
  per-image version.
- The byakugan-engine-pin assertion that lives in DX_DFIR's test suite today
  moves into GoDFIR-toolz CI, where it asserts that the built byakugan image's
  `com.get-sybers.engine-ref` equals the `ref:` pinned in `images.yml`.
  DX_DFIR's tests assert only the one release.
- `images.yml` lives in GoDFIR-toolz ([§2.2](02-tool-directory-structure.md));
  DX_DFIR reads it from the submodule and never carries a divergent copy.

## 7.6 The air-gap flow

1. **Build and gate** on the connected build host: CI runs the full gate over
   every image ([06](06-verification-gate.md)).
2. **Release**: the release job runs `docker save` over the matrix, writes
   `RELEASE.yml` with each image's ID, computes the tarball's sha256, and
   collects the SBOMs.
3. **Transfer**: the operator carries the four artifacts to the air-gapped
   DX_DFIR host.
4. **Integrity on arrival**: `sha256sum -c godfir-toolz-<release>.tar.sha256`.
5. **Load**: `docker load < godfir-toolz-<release>.tar`.
6. **Preflight** (DX_DFIR `dxdfir_images`): for every image in `RELEASE.yml`,
   inspect and compare (§7.7). Any mismatch fails preflight before any lane
   runs.

The preflight is cheap, offline, and never re-opens image internals.

## 7.7 Exactly what the consumer checks

| Check | How | Passes when |
|---|---|---|
| bundle integrity | `sha256sum -c` on the tarball | the checksum matches |
| image identity | `docker image inspect --format '{{.Id}}' <image>` | equals the manifest `id` for that image |
| release | label `com.get-sybers.godfir-release` | equals the pinned release |
| trust anchor | label `com.get-sybers.hardened` | `"true"` |
| inventory hygiene | `images.py` audit | every `get-sybers/*` image on the host is in `images.yml`, and none is missing |

What the consumer does **not** check: filesystem contents, shells, users,
setuid bits, labels beyond the three above, SBOM contents, CVEs. All of that is
producer-side and was proven before the release was cut.

## 7.8 Possible future hardening

A detached signature over `RELEASE.yml` (cosign or minisign, with the public
key provisioned on the air-gapped host) and, further out, in-toto/SLSA
provenance are possible future hardenings; neither is part of the standard.

[← 06 — Verification gate](06-verification-gate.md) · [08 — Zero-CVE program →](08-zero-cve-program.md)
