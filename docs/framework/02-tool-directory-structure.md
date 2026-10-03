# 02 — Tool directory structure

[← 01 — Architecture and boundary](01-architecture-and-boundary.md) · [03 — Environment contract →](03-environment-contract.md)

## 2.1 The mandated `<tool>/` layout

Every tool lives in `GoDFIR-toolz/<tool>/`. A tool is **framework-compliant**
only when every MUST file is present and the tool passes the CI gate
([06](06-verification-gate.md)).

```
<tool>/
├── Dockerfile            # MUST — multi-stage, hardened final stage (05)
├── contract.yml          # MUST — the machine-readable env-var + I/O contract (03)
├── README.md             # MUST — human doc, generated-checkable against contract.yml
├── .gitignore            # SHOULD — ignore build artefacts
│   # --- source-bearing tools (Go tools built in-repo) also carry: ---
├── go.mod / go.sum       # MUST for in-repo Go tools — pinned module graph
├── main.go (+*.go)       # the tool source
├── *_test.go             # MUST for in-repo Go tools — unit tests (pure funcs)
└── test/                 # MUST — fixtures + a contract/entrypoint smoke test
    ├── fixtures/         #   tiny committed evidence samples (or a generator)
    └── contract_test.*   #   asserts exit codes + JSON summary schema (§3.5)
```

## 2.2 Repo-level files

The shared files at the repository root. Those marked *present* exist today;
the framework formalizes them. Those marked *MUST ADD* are introduced by the
migration ([10](10-migration-roadmap.md)).

```
GoDFIR-toolz/
├── images.yml                 # MUST ADD (moves here from DX_DFIR) — the inventory (§7.5)
├── hardening/
│   ├── harden.yml             # present — the canonical build-time hardener (§5.5)
│   ├── contract.schema.yml    # MUST ADD — JSON-schema for every contract.yml
│   └── cve-waivers.yml        # MUST ADD — the dated CVE waiver ledger (§8.5)
├── _template/                 # MUST ADD — copy-me skeleton (09)
├── build-all.sh               # present — builds each image
├── verify-all.sh              # MUST ADD — runs the CI gate locally (06)
├── cve-scan.sh                # MUST ADD — per-image SBOM + CVE gate (08)
├── sbom/                      # generated SBOMs land here (and ride the release bundle)
└── .github/workflows/ci.yml   # MUST ADD — the release gate (06)
```

`images.yml` is the producer's inventory and lives in the producer's
repository. DX_DFIR reads it from the submodule. It keeps its `tool:` /
`non_tool_repos:` split and gains `version` and `godfir-release` fields
([§7.3](07-supply-chain-and-versioning.md)).

## 2.3 What each MUST file contains and does

**`Dockerfile`** — produces a hardened image conforming to
[05](05-hardening-standard.md): multi-stage; minimal final stage;
`USER 2000:2000`; uid 0 renamed and locked; no shell, package manager, or
interpreter unless justified in the file header; the required OCI and
`com.get-sybers.*` labels; the `/etc/dfir-hardened` self-declaration; and a
build-stage sanity gate (`RUN /<tool> --version`, or the usage-exit check).

**`contract.yml`** — the *single source of truth* for how the tool is driven:
every `<TOOL>_*` variable (required / optional / default), the volume mounts,
the exit-code table, and the stdout JSON summary schema. DX_DFIR reads it to
know which `-e` and `-v` arguments to pass; CI reads it to test the image; the
README is derived from it. It replaces documentation that lives only in a
Dockerfile header comment.

**`README.md`** — human documentation. It is factual and present-tense, carries
no verification narratives and no issue references, and its Input / Env /
Output / Exit codes / Run sections agree with `contract.yml`. CI checks for
drift between the two ([§6.5](06-verification-gate.md)).

**`test/`** — fixtures small enough to commit (or a deterministic generator),
plus a smoke test that runs the built image over a fixture in batch mode and
asserts the exit code and that stdout is exactly one JSON line matching the
summary schema. The same test re-runs to prove idempotency and runs once with a
bad environment to prove the config-error exit.

**`*_test.go`** (Go tools) — unit tests over the pure functions: parsers,
CSV/JSON row shaping. `gomft/main_test.go` is the existing model.

**`go.mod` / `go.sum`** (Go tools) — the pinned module graph. The build runs
`go mod download` against it before any source is copied, so the dependency
closure is fixed by the lockfile, not by the build host.

[← 01 — Architecture and boundary](01-architecture-and-boundary.md) · [03 — Environment contract →](03-environment-contract.md)
