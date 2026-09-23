# 10 — Migration roadmap

[← 09 — Tool template](09-tool-template.md) · [11 — Decisions and open questions →](11-decisions-and-open-questions.md)

> ## Sequencing rule
>
> **The CVE gate is the final phase.** It is enabled and enforced only after
> every image has migrated into the framework, and it is never run as a
> release criterion against a partially-migrated image set. During migration
> `cve-scan.sh` may be run by hand for triage; it is not a release gate until
> the migration is complete ([08](08-zero-cve-program.md)).

## 10.1 Classification

Existing tools are classified by their distance from the standard. "Conforms"
means the tool passes the gate ([06](06-verification-gate.md)) as-is, modulo
the new labels and `contract.yml`, which every tool adds.

## 10.2 The tiers

### Tier 0 — the reference (already the target shape)

- **anamnesis** — multi-stage, env-driven self-orchestrating batch entrypoint,
  single JSON summary, `0/1/2` exit codes, no shell and no Python (glibc
  justified). *Work:* add `contract.yml` (extracted from its Dockerfile
  header); add the version labels; keep `0/1/2` as is and adopt `3` for
  partial; adopt the `/etc/dfir-hardened` schema; and, by declaring
  `shell=false python=false`, land in the gate's no-shell/no-Python assertion
  (the hand-maintained consumer list omits it today; the declaration-derived
  set does not).

### Tier 1 — close (static `FROM scratch`; need env-orchestration and a contract)

- **goprefetch, goese, gorb, gomft, goamcache, goappcompat, goevtx, gore,
  gosbe, gole, gojle, gowxt** — already `FROM scratch`, static, `USER 2000`,
  hardened by construction, `-d` directory batch, JSONL/CSV output. **The gap
  is the environment contract and self-orchestration:** they are driven by
  argv (`-d /input --json /output --jsonf x.json`), so DX_DFIR builds argv on
  the host — the thing [03](03-environment-contract.md) forbids. *Work per
  tool:*
  1. add `<TOOL>_INPUT_DIR` / `_OUT_DIR` / `_FORMAT` / `_FORCE` reading; a
     no-argument run batches over `INPUT_DIR` (keep `-d` / `-f` as the
     [§4.2](04-self-orchestration.md) pass-through);
  2. emit the single JSON summary line on stdout — records move to files under
     `OUT_DIR` (they stream to stdout by default today), with `OUT_DIR`
     effectively required in batch mode;
  3. remap the partial exit to `3`;
  4. add `contract.yml`, the version labels, and the `/etc/dfir-hardened`
     schema.

  Low risk: one shared refactor pattern across twelve tools, and the template's
  `main.go` is derived from it.

### Tier 2 — needs real work (interpreters, multi-tool, no contract shape)

- **plaso** — Python, three entry tools plus the psort wrapper, no single
  `ENTRYPOINT`, keeps Python and a shell (justified). *Work:* wrap in a
  [§4.3](04-self-orchestration.md) **dispatcher entrypoint** (`plaso <subtool>`
  → self-orchestrate that subtool from `PLASO_<SUBTOOL>_*`); add `contract.yml`
  with `entrypoint: multi-tool` and per-subtool env blocks; one JSON summary
  per run; version labels. Keep the Python/shell justification and declare
  `python=true shell=true`.
- **zeek** — single `ENTRYPOINT`, Python and shell already removed, hardened.
  Closer than plaso. *Work:* env-orchestrate (`ZEEK_INPUT_DIR` batch over pcaps
  → `ZEEK_OUT_DIR`), single JSON summary, `contract.yml`, version labels.
  Declares `shell=false python=false` and lands in the no-shell/no-Python
  assertion.
- **signatures** (YARA + Suricata + Hayabusa + the gomount→goyara pipe) —
  multi-tool, keeps a shell for the YARA scan loop. *Work:* the largest — a
  dispatcher entrypoint covering four sub-tools (`SIGNATURES_YARA_*`,
  `SIGNATURES_SURICATA_*`, `SIGNATURES_HAYABUSA_*`, `SIGNATURES_SCAN_*` for
  the gomount|goyara pipe); `contract.yml` with `entrypoint: multi-tool`; one
  JSON summary per sub-run; the kept shell justified and declared
  (`shell=true`); version labels. Moving the shell scan loop into a small
  static Go batch runner to reach shell-free is a later option.
- **godfir-tool** (the parameterized .NET image builder — sqlecmd, bstrings,
  iisgeolocate, recentfilecacheparser, rla) — one Dockerfile, argv-driven,
  ~300 MB .NET runtime, no self-orchestrating `ENTRYPOINT`. *Work:* each built
  image needs its own `contract.yml` (they share the recipe but differ in flags
  and output); a dispatcher or per-tool env→argv shim so DX_DFIR passes
  environment, not argv; the version labels; and a decision on whether the
  .NET tools join the in-progress Go port (they are the reason gomft, goevtx,
  and their siblings exist). Until ported, they conform through the shim and
  contract.
- **byakugan** — Python CAR engine, dispatched on its first argument (build /
  timeline / verify / car-vocab / load) by the engine's own `byakugan.cli`, keeps
  Python. byakugan is a separate repository cloned in at a pin; DX_DFIR only
  orchestrates and links out. Conformant: the ENTRYPOINT delegates to the
  engine dispatcher, `contract.yml` (`entrypoint: multi-tool`, `python=true`)
  carries one env block per sub-tool, each run prints a single JSON summary,
  and the image carries the version labels plus `com.get-sybers.engine-ref`
  with the pinned ref. It is exempt from the Go self-orchestration expectation
  (it is an external engine).

### Tier 3 — inventory reconciliation (not in `images.yml`)

- **gomount** — has its own Dockerfile, deliberately `debian:trixie-slim`
  (execs ntfs-3g; permissive-by-separation), unprivileged userns, needs
  `/dev/fuse`. It is not in `images.yml` and is orchestrated *with* the other
  Go tools, not imported by them. It is a genuine, documented Shape-B
  deviation: non-scratch, keeps a shell for fusermount. *Work:* give it a
  `contract.yml` (`entrypoint: argv`, streaming), the version labels, and a
  `shell=true` declaration with justification; then settle whether it is a
  first-class image in `images.yml` or a formal component of `signatures`
  (where its `stream` verb is already baked) — an open question
  ([11](11-decisions-and-open-questions.md)).
- **goyara** — has no Dockerfile; it is built *inside* `signatures` (libyara
  via cgo). It stays a component of `signatures`, not a standalone image, and
  `signatures/contract.yml` documents it. There is no independent image to
  harden. Its inventory status is settled alongside gomount's.

## 10.3 Cross-cutting tasks (once, repo-wide)

1. **Move `images.yml` into GoDFIR-toolz** (it is the producer's inventory) and
   have DX_DFIR read it from the submodule; keep the `tool:` /
   `non_tool_repos:` split; add `version` and `godfir-release` fields.
2. Add `hardening/contract.schema.yml`, `_template/`, `verify-all.sh`,
   `cve-scan.sh`, `.github/workflows/ci.yml`, and an empty
   `hardening/cve-waivers.yml`.
3. Standardize `/etc/dfir-hardened` to the schema
   ([§5.6](05-hardening-standard.md)) in every Dockerfile.
4. Add the version labels to every `LABEL` block.
5. Write each tool's `contract.yml` and reconcile its README.
6. Delete the now-redundant post-build asserts from DX_DFIR's `dxdfir_images`
   (leave `ensure_built`, the inventory audit, and the new preflight version
   check); move the byakugan-engine-pin assertion into GoDFIR-toolz CI; reduce
   DX_DFIR's pin to the single release value.

## 10.4 The phased sequence

The sequence keeps the DX_DFIR pipeline green at every phase boundary. Three
rules hold throughout:

- **No tool loses its argv mode before its lane switches.** The pass-through
  ([§4.2](04-self-orchestration.md)) keeps the existing argv-driven lane
  working until DX_DFIR's driver for that tool becomes contract-driven; the
  two land as a producer change and a consumer change in the same window.
- **DX_DFIR's existing post-build asserts stay in place until Phase 5.** They
  are removed only when the producer gate covers every image.
- **Every phase ends with a green pipeline run** over the light test evidence
  set on the consumer side.

| Phase | Scope | Producer (GoDFIR-toolz) | Consumer (DX_DFIR) | Green because |
|---|---|---|---|---|
| **0 — Scaffolding** | repo-wide, no runtime behaviour change | `contract.schema.yml`, `_template/`, `verify-all.sh`, `ci.yml` running §6.1–6.4 and §6.6 on every image (§6.5 only for tools that have a `contract.yml`), `cve-scan.sh` + empty waiver ledger (not wired into the gate), `images.yml` moved in, `/etc/dfir-hardened` schema and version labels in every Dockerfile | reads `images.yml` from the submodule; keeps every existing assert | no image's runtime behaviour changes |
| **1 — Tier 0** | anamnesis | `contract.yml`, exit `3` for partial, labels, declaration-derived no-shell/no-Python assertion | memory lane accepts exit `3` (it is already env-driven) | `0/1/2` unchanged; `3` is additive |
| **2 — Tier 1** | the twelve Go tools | one shared refactor: env reading, batch-on-no-args, records to `OUT_DIR`, summary line, exit remap, `contract.yml`, labels; argv pass-through retained | each tool's lane switches from argv-building to contract-driven in the same window as its producer change | argv mode keeps the old lane working until the switch |
| **3 — Tier 2** | zeek → plaso → byakugan → signatures → godfir-tool | dispatcher entrypoints, multi-tool contracts, declared `python=true` / `shell=true` where justified, `engine-ref` label on byakugan | lanes switch per tool as in Phase 2 | same pairing rule |
| **4 — Tier 3** | gomount, goyara | `contract.yml` for gomount, `shell=true` declaration, inventory status settled (open question) | lane unchanged until the status decision lands | gomount's streaming argv shape is a declared deviation |
| **5 — Consumer cutover** | DX_DFIR | first checksum-manifested release bundle; byakugan-engine-pin assertion in CI | delete post-build asserts and `check_config`; add the preflight checksum/version check; single transitive pin | producer gate covers every image; the preflight replaces, not removes, verification |
| **6 — CVE gate (final)** | every image | `cve-scan.sh` becomes §6.7 at default settings (fixable `high`+); the ledger is populated with dated waivers for what fails on day one; the ratchet begins | none | the image set is complete and stable; waivers describe the target, not the transition |

Phase 6 is last by rule, not by convenience: the CVE gate measures the
migrated matrix, and only that.

[← 09 — Tool template](09-tool-template.md) · [11 — Decisions and open questions →](11-decisions-and-open-questions.md)
