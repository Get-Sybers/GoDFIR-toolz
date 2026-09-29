# godfir_byakugan

The CAR lane of [`get_sybers.godfir_run`](../../README.md): materialise / verify
/ timeline the MITRE **CAR** from processed evidence via the multi-tool
`get-sybers/byakugan` image, under the `godfir_run` confinement skeleton. One
role, three actions (`godfir_byakugan_action`):

- **build** — materialise the per-source CAR stores from the processed tree
  (`car_relationships.jsonl` is the done/skip marker). Per-source failures
  tolerated (gate on output).
- **verify** — the engine's promotion gate; `status: ok` is the only pass
  (`tasks/gate_verify.yml` judges the summary and asserts `verify.txt`).
- **timeline** — union a source's stores into one time-ordered CAR timeline.

The lane takes the processed tree and CAR tree as inputs — evidence *discovery*
is the caller's — and ships the tool's `contract.yml`. Variables:
`meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: runs each action and asserts the built
argv (the byakugan image, the sub-tool dispatch, the `/input` ro and `/output`
rw binds) without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
