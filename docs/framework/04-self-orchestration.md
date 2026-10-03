# 04 — Self-orchestration

[← 03 — Environment contract](03-environment-contract.md) · [05 — Hardening standard →](05-hardening-standard.md)

## 4.1 The no-argument entrypoint

The `ENTRYPOINT` is the tool binary itself (or, for a multi-tool image, a thin
dispatcher — §4.3). Invoked with no arguments it:

1. **Reads the environment contract.** Every path and flag is resolved from
   `<TOOL>_*`; host-supplied argv is never required.
2. **Discovers inputs.** It walks `<TOOL>_INPUT_DIR` recursively and
   content-detects the artefacts it handles — the same discovery the Go tools
   perform under `-d`, and anamnesis performs over the memory directory.
3. **Batches over every discovered item**, writing one output subfolder per item
   under `<TOOL>_OUT_DIR`.
4. **Is idempotent.** An item whose valid output already exists is skipped unless
   `<TOOL>_FORCE` is set. Re-running the container converges; it never
   duplicates or corrupts prior output. The second run over unchanged inputs
   reports `status: nothing` (or all items skipped) and produces no new files.
5. **Runs once and exits.** No daemon, no watch loop, no `tail -f`. It emits the
   single JSON summary line on stdout, progress on stderr, and returns the exit
   code from the uniform table ([§3.5](03-environment-contract.md)).
6. **Honours the read-only rootfs.** All writes go to `<TOOL>_OUT_DIR`,
   `<TOOL>_WORK_DIR`, or `/tmp`. `HOME` and caches point at a writable tmpfs;
   the image sets `HOME=/tmp` and `XDG_CACHE_HOME=/tmp/.cache` as anamnesis
   does.

Batch mode with no arguments is the contract DX_DFIR uses; it is the mode the
gate exercises ([§6.5](06-verification-gate.md)).

## 4.2 The argv pass-through (debug only)

When the entrypoint is invoked *with* arguments it MAY switch to a single-item
mode (anamnesis: `anamnesis -f /input/<img> -o /out`; the Go tools: `-d`, `-f`,
`--json`). This mode exists for interactive debugging and for lanes that want
per-item parallelism under an operator's hand. The consumer never uses it: the
batch mode is the interface DX_DFIR drives, and argv is demoted to a debug
pass-through. In pass-through mode the tool's existing single-item exit
meanings are preserved ([§3.5](03-environment-contract.md)).

## 4.3 The multi-tool dispatcher

A multi-tool image (plaso, signatures, byakugan) cannot make one binary its
sole entrypoint. Its rule: a thin **dispatcher entrypoint** takes the sub-tool
name as its first argument and *then* self-orchestrates that sub-tool from the
sub-tool's own env block (`SIGNATURES_YARA_*`, `SIGNATURES_SURICATA_*`,
`PLASO_<SUBTOOL>_*`). Each dispatched run obeys §4.1 in full: discover, batch,
converge, one JSON summary line, one exit code.

This keeps the consumer's model as "name the tool, pass the environment"
rather than "pass a full argv." Such an image declares
`entrypoint: multi-tool` in its contract, with one env block per sub-tool
([§3.7](03-environment-contract.md)).

Multi-tool images that keep a shell for their dispatcher or scan loops (plaso,
signatures, gomount) are accepted deviations from the shell-free rule: they
declare `shell=true` in `/etc/dfir-hardened`, justify it in the Dockerfile
header, and are gated accordingly ([§5.9](05-hardening-standard.md)).

## 4.4 Behaviour summary

| Invocation | Mode | Who uses it | Exit semantics |
|---|---|---|---|
| `docker run … <image>` | batch over `<TOOL>_INPUT_DIR` | DX_DFIR, CI | uniform table (§3.5) |
| `docker run … <image> <sub-tool>` | dispatcher → batch for that sub-tool | DX_DFIR, CI (multi-tool images only) | uniform table (§3.5) |
| `docker run … <image> -f <item> …` | single-item pass-through | operators, debugging | tool's existing argv meanings |

[← 03 — Environment contract](03-environment-contract.md) · [05 — Hardening standard →](05-hardening-standard.md)
