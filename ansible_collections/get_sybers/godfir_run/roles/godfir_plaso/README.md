# godfir_plaso

The Plaso lane of [`get_sybers.godfir_run`](../../README.md): disk images / VM
exports / loose trees -> Plaso JSON Lines, via the `get-sybers/plaso` image under
the `godfir_run` confinement skeleton. It declares one `log2timeline` run per
present source tree (the primary input plus the optional VM and loose trees),
then a final `psort` render over the same tree, so `timeline.jsonl` lands beside
each `<host>.plaso`.

Per-image failures are tolerated (`godfir_run_require_no_failures: false`; gate on
output). The lane takes the source trees and output root as inputs — evidence
*discovery* is the caller's — and ships the tool's `contract.yml`. Variables:
`meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: asserts the built argv for the
log2timeline + psort runs (image, `/input` ro, `/output` rw, `PLASO_*` env)
without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
