# godfir_godaemonhunter

The Linux daemon lane of [`get_sybers.godfir_run`](../../README.md):
godaemonhunter's layered matrix over every host, under the `godfir_run`
confinement skeleton.

- A **loose host** gets one confined run per (parser, host): **Layer 1**
  (`gohost`, `gousers`, `gonetwork`, …) builds the host's knowledge store, then
  **Layer 2** (the daemon parsers) runs with that store mounted read-only at
  `/knowledge`.
- A **disk image** is hunted ON directly (`hunt`, one run per image): the baked
  gomount pulls the linux-core surface into `/work` while the layers run.

The layer lists come from the shipped `contract.yml` (`layer1`/`layer2` keys).
The lane takes **caller-discovered item lists** (`_images` as `[{tree, rel, name}]`,
`_hosts` as `[{name, export}]`) — evidence *discovery* is the caller's. Exits
1/3 are tolerated (`godfir_run_require_no_failures: false`, gate on output).
Variables: `meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: given one host and one image item, it
asserts the built Layer 1 / Layer 2 / hunt argvs (subtool dispatch, the
`/knowledge` read-only mount on Layer 2, `/work` on hunt) without a daemon or the
real image.

```bash
molecule test
```

## License

Apache-2.0.
