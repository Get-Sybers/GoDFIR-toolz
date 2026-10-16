# godfir_images

The image lifecycle for the get-sybers DFIR tool images on the **consumer**
side, and part of the [`get_sybers.godfir_run`](../../README.md) collection.
It **pulls** the published images and **verifies** their hardened posture.
Building is not here — that is [`get_sybers.godfir_build`](../../../godfir_build).

## Entry points

| `tasks_from` | what it does |
| --- | --- |
| `main` | Pull the images in `godfir_images_set` (a thin wrapper over `ensure_present`) — provision a host up front. |
| `ensure_present` | Read the local image facts and pull any required ref that is missing — or present but whose `RepoDigests` do not carry the manifest `digest:` (digest-stale) — (or all, under `godfir_images_force`), then tag each pulled registry image to its local name and verify it against the recorded digest. Idempotent and self-healing; `godfir_run`'s preflight calls it at the head of every lane. |
| `resolve` | The side-effect-free pull decision `ensure_present` runs first: read the local facts and set `godfir_images_to_pull` (the missing refs plus the digest-stale present ones) and `godfir_images_stale`. It pulls nothing, so the offline Molecule scenario can assert the decision directly. |
| `verify` | The runtime supply-chain gate: every namespace ref about to run must be a **known manifest image** (membership) and still carry the hardened contract — the non-root `USER` (`uid:gid`) and the `com.get-sybers.hardened` label (posture). Refs outside `get-sybers/*` pass untouched. |
| `audit` | The full namespace inventory audit: every manifest image present and hardened, no unexpected `get-sybers/*` image on the host (allow-list = the manifest images + its `non_tool_repos`). One aggregated verdict. |

The manifest (`images.yml`) that `verify`'s membership gate and `audit` read is shipped under `files/` (override `godfir_images_manifest_path` to drive them off another manifest — `get_sybers.godfir_build` delegates its own verify/audit here, passing the repo-root manifest).

Variables are specified in `meta/argument_specs.yml`.

## Registry pull, local name

The lanes and each tool's `contract.yml` reference the **local** name
`get-sybers/<tool>:latest`. `ensure_present` pulls
`{{ godfir_images_registry }}/<tool>:<release>` (the image's `release:`) and
tags it to that local name, so nothing downstream carries a registry-qualified
ref and the contract machinery is unchanged; the default registry is
`ghcr.io/get-sybers/godfir-toolz`. When the manifest records the image's
`digest:` — the content digest the Build & Push workflow pushed for that
release — the freshly pulled image is then **verified** against it: its
`RepoDigests` must carry `{{ godfir_images_registry }}/<tool>@<digest>`, so a
moved tag is caught and the content refused. A **present** image is held to the
same digest: when its `RepoDigests` do not carry the recorded `digest:` it is
re-pulled by its release tag and re-verified, so a stale local `:latest` (a
dev-built or superseded image) cannot slip the gate. Pulling by the readable tag
and verifying the digest keeps the local `:<release>` tag while still pinning to
the exact bytes the release build pushed. Point `godfir_images_registry` at a mirror
to move the whole family at once; set `godfir_images_tag` to pull every image at
one tag instead (an older wave, a fixture) — it bypasses the digest check.

There is no build path and no tar load/save here: image delivery is registry
pull. Building and the offline air-gap packaging live elsewhere
(`get_sybers.godfir_build` and the deployment layer respectively).

## Testing

The **Molecule** scenario is offline and daemon-only (no registry): it builds
two `FROM scratch` fixture images — one hardened (`USER 2000:2000` +
`com.get-sybers.hardened=true`), one not — then asserts `verify` accepts the
hardened one and refuses the other by name, that a non-namespace ref is ignored
by the scope, and that `ensure_present` is a no-op when the image is already
present with no recorded digest (it attempts no pull), and that a present image
whose `RepoDigests` fail the recorded digest is scheduled for re-pull — asserted
through the side-effect-free `resolve`, so the decision is checked without a pull.
The pull-from-registry path itself is exercised in integration, not offline
molecule.

```bash
molecule test
```

## License

Apache-2.0.
