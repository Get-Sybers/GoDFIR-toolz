# `get-sybers/signatures` — the detection lane in one image

One hardened image for the whole detection lane: **YARA + Suricata**
(Debian) **+ Hayabusa** (pinned release zip, sha256-verified at build time —
`HAYABUSA_VERSION` + `HB_SHA_*` build args). There is no ENTRYPOINT: it is a
multi-tool image invoked as `container.run(IMAGE, [<tool>, <args>...])` — the
caller names the tool (the YARA scan loop `/opt/dxdfir/scan-list.sh`,
`suricata`, or the baked `hayabusa`). A shell remains only for the YARA
per-file scan loop; the loop scans every file even when one fails, but any
yara invocation error fails the whole run (exit 1) so a partial scan can
never pass as clean.

```sh
docker build -t get-sybers/signatures:latest -f signatures/Dockerfile .
```

Builds with the **repo root as context** so `COPY hardening/harden.yml`
consumes the canonical hardener directly; `get-sybers/*` namespace and
[the hardening contract](../README.md#the-hardening-contract) like every
other image here.
