# Third-party notices

What the image builds fetch, and under whose terms. **This repository ships
none of the content below** — it carries only fetchers (URLs, commit pins,
sha256 manifests); the content lands inside images an operator builds
locally. The build galaxy pushes nothing to a registry, so no redistribution
obligation attaches to the repository itself. An operator who publishes a
built image is redistributing what it bakes — these terms then bind them.

## DetectRaptor YARA content — fetched at the `signatures` image build

[`signatures/detectraptor.py`](signatures/detectraptor.py) downloads the YARA
rulesets from [mgreen27/DetectRaptor](https://github.com/mgreen27/DetectRaptor)
— commit-pinned, per-asset sha256-verified — and merges them into the image at
`/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar`. Terms are layered:

- **DetectRaptor itself declares no repository-level licence.** Treat the
  aggregation as all-rights-reserved beyond the fetching-for-use its README
  invites; do not redistribute the merged file.
- **Each rule carries its own provenance** — upstream is a YARA-Forge-style
  aggregation and every rule's `meta` block records `author`, `source_url`
  and `license_url` (Neo23x0 signature-base, Mandiant, Arkbird_SOLG, …). The
  merge keeps those blocks byte-for-byte; the per-rule licences (mostly
  DRL/CC/Apache) govern the rules.
- DetectRaptor's **VQL artifacts and CSV lookups are not fetched** — they
  need a Velociraptor server this pipeline does not run.

## Emerging Threats Open (Suricata rules) — fetched at the `signatures` image build

[`signatures/suricata_rules.py`](signatures/suricata_rules.py) downloads the
[ET Open](https://rules.emergingthreats.net/open/) ruleset — the
version-pinned tarball published per Suricata engine release — and
concatenates its `*.rules` into the image at
`/opt/dxdfir/suricata-rules/suricata.rules`. ET Open is published under the
BSD-style licence in the tarball's `LICENSE` file; it travels with the rules
inside the image.

## Hayabusa and its Sigma rules — fetched at the `signatures` image build

The pinned [Yamato-Security/hayabusa](https://github.com/Yamato-Security/hayabusa)
release zip (sha256-verified against the release's published checksums)
carries the binary (AGPL-3.0) and its bundled Sigma/Hayabusa rules (per-rule
licences, largely DRL); both stay inside the image at `/opt/dxdfir/hayabusa`.

## Elastic detection rules-as-code — the Byakugan engine's own

The rules baked into the `byakugan` image at `/rules` ship with the engine
repository itself ([`rules/`](https://github.com/Get-Sybers/byakugan/blob/main/rules/README.md), riding the clone at the
`BYAKUGAN_REF` pin) — first-party content of the program; each rule's
`source` block records the registry entry it was ported from. No third-party
terms attach.
