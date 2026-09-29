# get_sybers.godfir_run

The **consumer** collection for the get-sybers DFIR tool images: pull the
published images from the registry and run each tool under its confinement
contract. This is the collection downstreams (DX_DFIR) pin and delegate to —
they consume the images, they do not build them.

It is the counterpart of [`get_sybers.godfir_build`](../godfir_build): the build
collection produces and pushes the hardened images; this one pulls and runs
them. The image inventory (`images.yml`) and each tool's `contract.yml` are the
shared source of truth.

## Roles

| Role | What it does |
|---|---|
| `godfir_run` | The generic, contract-driven confined docker-run engine. A lane hands it a run spec (image, item list, paths, env, confinement) and it runs the container hardened — dropped capabilities, read-only rootfs, tmpfs scratch, no network unless asked — then gates the result (expected exit, output present, no per-item failures). |
| `godfir_images` | The image lifecycle: **pull** the published image from the registry (`ghcr.io/get-sybers/<tool>:<tag>`), and verify/audit its hardened posture (runs as the DFIR uid, carries `com.get-sybers.hardened=true`, is a known manifest image). Build is **not** here — that is `get_sybers.godfir_build`. |
| `godfir_<tool>` | One lane per image (`godfir_gowindowlicker`, `godfir_godaemonhunter`, `godfir_signatures`, `godfir_zeek`, `godfir_plaso`, `godfir_byakugan`, `godfir_anamnesis`). Each carries only its tool's **run spec** — the env block, output layout and confinement its `contract.yml` declares — and delegates to `godfir_run`. Evidence *discovery* (walking a data store, disk-image detection, taxonomy) stays in the consumer (DX_DFIR); a lane takes the items/paths it is given. |

## Image delivery

Images are delivered by **registry pull**, not by building or by loading a tar.
`godfir_images` pulls `ghcr.io/get-sybers/<tool>:<tag>` and verifies the pulled
image before any lane runs it. The tag tracks one release train shared with
`get_sybers.godfir_build` (the image's `com.get-sybers.godfir-release` label).

## Layout (ansible-standards §1)

```
ansible_collections/get_sybers/godfir_run/
├── galaxy.yml            # + community.docker dependency
├── requirements.yml      # the single source of exact Ansible pins (§2)
└── roles/
    ├── godfir_run/        # the confined docker-run engine
    ├── godfir_images/     # pull + verify/audit
    └── godfir_<tool>/     # one lane per image
```

Resolved uninstalled from the repo root in-repo; downstreams pin this collection
via their own `requirements.yml` (§2) and pull the images.

## Lane conventions

Each `godfir_<tool>` lane is thin and follows the same shape:

- it **ships the tool's `contract.yml`** under `roles/godfir_<tool>/files/` so the
  collection is self-contained on install (`conform.sh` sha256-checks each copy
  against the repo-root `<tool>/contract.yml`);
- it **freezes** any `role_path`-derived path (the contract, a `gate_extra` file)
  with a `set_fact` before delegating — `role_path` evaluated lazily inside
  `godfir_run` would resolve to the *engine's* directory, not the lane's;
- it takes **inputs** (paths, item lists), never doing evidence discovery — that
  stays in the consumer (DX_DFIR);
- its **molecule** scenario sets `godfir_run_build_only: true` and asserts the
  built `docker run` argv, so it runs offline with no daemon and no image.

## Dependencies

- `community.docker` (declared in `galaxy.yml`, pinned in `requirements.yml`).

## License

Apache-2.0.
