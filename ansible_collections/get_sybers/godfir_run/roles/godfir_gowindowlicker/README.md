# godfir_gowindowlicker

The Windows artefact lane of [`get_sybers.godfir_run`](../../README.md):
gowindowlicker's parsers (goevtx, gore, gomft, goprefetch, …) over every host,
under the `godfir_run` confinement skeleton.

- A **loose event-log host** (a folder of `.evtx`) gets `goevtx`, one run per host.
- A **disk image** is swept ON directly (`lick`, one run per image via
  `GOWINDOWLICKER_IMAGE`): the baked-in gomount pulls each parser's artefact set
  out of the OS volume into `/work` while they run — nothing is exported.

The lane takes **caller-discovered item lists** (`_images` as `[{tree, rel, name}]`
and `_winevt_hosts` as folders) — the disk-image regex walk lives in the consumer
(DX_DFIR), not here. Exits 1/3 are tolerated (`godfir_run_require_no_failures:
false`, gate on output). It ships the tool's `contract.yml`. Variables:
`meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: given one fake image item and one loose
host, it asserts the built `lick` and `goevtx` argvs (image, binds, `/work`)
without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
