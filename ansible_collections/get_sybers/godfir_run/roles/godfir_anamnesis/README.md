# godfir_anamnesis

The memory lane of [`get_sybers.godfir_run`](../../README.md): memory images ->
per-plugin JSONL via the offline `anamnesis` (MemProcFS) image under the
`godfir_run` confinement skeleton. Symbols come from a **persistent host cache**
mounted read-write at `/opt/anamnesis/lib/Symbols` (the image never downloads a
PDB — the contract is `network: none`).

Per-plugin failures without symbols are normal, so the lane sets
`godfir_run_require_no_failures: false` and gates (via `tasks/gate_extra.yml`) on
"some plugin produced output, or there were no images". It takes the memory
tree, output root and symbol cache as inputs — evidence *discovery* is the
caller's — and ships the tool's `contract.yml`. Variables:
`meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: asserts the built `docker run` argv
(image, `/input` ro, `/out` rw, the symbol-cache mount, the `ANAMNESIS_*` env)
without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
