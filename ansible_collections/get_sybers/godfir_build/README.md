# get_sybers.godfir_build

The **producer** collection for the get-sybers DFIR tool images. It builds and
hardening-verifies every image declared in `images.yml` (the repo-root
inventory) from its build context and `contract.yml`, and is meant to run
**in-repo against the GoDFIR-toolz checkout** — not installed by consumers.

Consumers do not build. They pin [`get_sybers.godfir_run`](../godfir_run) and
**pull the published images** from the registry; this collection is the release
side that produces and pushes them.

## Layout (ansible-standards §1)

```
ansible_collections/get_sybers/godfir_build/
├── galaxy.yml            # namespace/name/version + community.docker dependency
├── requirements.yml      # the single source of exact Ansible pins (§2)
├── playbooks/
│   └── build_images.yml   # get_sybers.godfir_build.build_images
└── roles/
    └── godfir_build/      # the one role — build + verify from images.yml
```

The collection resolves uninstalled from the repo root (`ansible.cfg`:
`collections_path = .:.ansible/collections`). There is no submodule and no
galaxy pull for `get_sybers.*`.

## Run it

Standalone, from a bare checkout of this repo:

```sh
./build-all.sh                 # every image in images.yml
./build-all.sh gowindowlicker  # one image (aliases / sub-tool names resolve)
./build-all.sh --preflight     # prepare + verify the host, build nothing
```

`build-all.sh` provisions a pinned controller (ansible-core + the docker SDK),
installs `community.docker` from `requirements.yml`, then launches the
collection playbook:

```sh
ansible-playbook get_sybers.godfir_build.build_images -e godfir_build_root="$PWD"
```

`godfir_build_root` is the build tree (the repo root: `images.yml`, `hardening/`
and every build context live there). Left empty it defaults to the checkout, so
a direct playbook run from the repo root needs no argument.

## The role

`godfir_build` builds `get-sybers/*` images from `images.yml` and verifies the
hardening contract on every result. Its variables are documented per entry
point in `roles/godfir_build/meta/argument_specs.yml` (`ansible-doc -t role
get_sybers.godfir_build.godfir_build`), and the tree/inventory contract in
`roles/godfir_build/README.md`.

## Dependencies

- `community.docker` (declared in `galaxy.yml`, pinned in `requirements.yml`) —
  the docker build/inspect modules the role drives.

## Testing

The role ships a molecule scenario (`roles/godfir_build/molecule/default`) that
builds fixture images from a FROM-scratch context and asserts the verify gate,
plus `ansible-lint` at the `production` profile. See the repo `CHANGELOG.md`.

## License

Apache-2.0.
