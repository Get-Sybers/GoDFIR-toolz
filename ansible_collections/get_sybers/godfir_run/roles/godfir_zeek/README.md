# godfir_zeek

The Zeek lane of [`get_sybers.godfir_run`](../../README.md): a tree of PCAPs ->
Zeek JSON, via the self-orchestrating `get-sybers/zeek` image under the
`godfir_run` confinement skeleton. The image discovers every capture under
`/input`, parses each into its own `/output` folder, and skips a capture that
already has valid output unless `ZEEK_FORCE`.

The lane takes the **pcap tree** and **output root** as inputs — evidence
*discovery* (which tree, where the output lands) is the caller's
(DX_DFIR). It ships the tool's `contract.yml` (`files/contract.yml`) so the
collection is self-contained on install. Variables: `meta/argument_specs.yml`.

## Testing

The **Molecule** scenario is offline (`godfir_run_build_only`): it runs the lane
against empty fixture dirs and asserts the built `docker run` argv — the image,
the `/input` (ro) and `/output` (rw) binds, `--network none`, and the
confinement flags — without a daemon or the real image.

```bash
molecule test
```

## License

Apache-2.0.
