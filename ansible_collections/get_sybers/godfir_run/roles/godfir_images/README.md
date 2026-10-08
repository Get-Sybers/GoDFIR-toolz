# godfir_images

The image lifecycle for the get-sybers DFIR tool images on the **consumer**
side, and part of the [`get_sybers.godfir_run`](../../README.md) collection.
It **pulls** the published images and **verifies** their hardened posture.
Building is not here — that is [`get_sybers.godfir_build`](../../../godfir_build).

## Entry points

| `tasks_from` | what it does |
| --- | --- |
| `main` | Pull the images in `godfir_images_set` (a thin wrapper over `ensure_present`) — provision a host up front. |
| `ensure_present` | Read the local image facts and pull any required ref that is missing (or all, under `godfir_images_force`), then tag each pulled registry image to its local name. Idempotent and self-healing; `godfir_run`'s preflight calls it at the head of every lane. |
| `verify` | The runtime supply-chain gate: every namespace ref about to run must be a **known manifest image** (membership) and still carry the hardened contract — the non-root `USER` (`uid:gid`) and the `com.get-sybers.hardened` label (posture). Refs outside `get-sybers/*` pass untouched. |
| `audit` | The full namespace inventory audit: every manifest image present and hardened, no unexpected `get-sybers/*` image on the host (allow-list = the manifest images + its `non_tool_repos`). One aggregated verdict. |

The manifest (`images.yml`) that `verify`'s membership gate and `audit` read is shipped under `files/` (override `godfir_images_manifest_path` to drive them off another manifest — `get_sybers.godfir_build` delegates its own verify/audit here, passing the repo-root manifest).

Variables are specified in `meta/argument_specs.yml`.

## Registry pull, local name

The lanes and each tool's `contract.yml` reference the **local** name
`get-sybers/<tool>:latest`. `ensure_present` pulls
`{{ godfir_images_registry }}/<tool>@<digest>` when the manifest records the
image's `digest:` — the content digest the Build & Push workflow pushed for
its release, so the pull is pinned to that exact content — and
`{{ godfir_images_registry }}/<tool>:<release>` (the image's `release:`)
otherwise; the default registry is `ghcr.io/get-sybers/godfir-toolz`. The
pulled image is tagged to that local name, so nothing downstream carries a
registry-qualified ref and the contract machinery is unchanged. Point
`godfir_images_registry` at a mirror to move the whole family at once; set
`godfir_images_tag` to pull every image at one tag instead (an older wave, a
fixture) — it bypasses the digests.

There is no build path and no tar load/save here: image delivery is registry
pull. Building and the offline air-gap packaging live elsewhere
(`get_sybers.godfir_build` and the deployment layer respectively).

## Testing

The **Molecule** scenario is offline and daemon-only (no registry): it builds
two `FROM scratch` fixture images — one hardened (`USER 2000:2000` +
`com.get-sybers.hardened=true`), one not — then asserts `verify` accepts the
hardened one and refuses the other by name, that a non-namespace ref is ignored
by the scope, and that `ensure_present` is a no-op when the image is already
present (it attempts no pull). The pull-from-registry path itself is exercised
in integration, not offline molecule.

```bash
molecule test
```

## License

Apache-2.0.
