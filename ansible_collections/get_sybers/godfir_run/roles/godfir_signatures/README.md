# godfir_signatures

The detection lane of [`get_sybers.godfir_run`](../../README.md): the YARA,
Suricata, Hayabusa and disk-`scan` sub-tools of the `get-sybers/signatures` image
over the evidence trees, under the `godfir_run` confinement skeleton. It declares
one run per **selected sub-tool** and **present evidence tree** — a tree left
empty or absent on disk is skipped.

- **yara** — the loose-files and/or memory trees.
- **suricata** — the PCAP tree.
- **hayabusa** — the loose event-log tree and, on the image, the disk-image and
  VM trees (event logs pulled by the baked gomount into `/work`).
- **scan** — every disk image of the disk-image and VM trees, streamed through
  gomount into goyara.

Exits 1/3 are tolerated (`godfir_run_require_no_failures: false`; gate on output).
The lane takes the evidence-tree paths and the output/scratch roots as inputs —
evidence *discovery* is the caller's — and ships the tool's `contract.yml`.
Variables: `meta/argument_specs.yml`.

## Testing

Offline `godfir_run_build_only` molecule: with only the files tree present and
`lanes: [yara]`, it asserts the built `yara` run argv (image, `/input` ro,
`/output` under `yara/`) without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
