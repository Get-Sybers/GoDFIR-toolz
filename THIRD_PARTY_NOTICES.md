# Third-party notices

What the image builds fetch, and under whose terms. **This repository ships
none of the content below** — it carries only fetchers (URLs, commit pins,
sha256 manifests); the content lands inside images an operator builds
locally. The build galaxy pushes nothing to a registry, so no redistribution
obligation attaches to the repository itself. An operator who publishes a
built image is redistributing what it bakes — these terms then bind them.

## VMkatz disk-image readers — ported source in `gomount/image/`

Unlike everything below, this one IS shipped in the repository:
[`gomount/image/vmdk.go`](gomount/image/vmdk.go), `vhd.go`, `vhdx.go`,
`qcow2.go` and `vdi.go` are Go ports of the `src/disk` readers of
[nikaiw/VMkatz](https://github.com/nikaiw/VMkatz) (Copyright (c) 2026
Nicolas Devillers, MIT License). The MIT terms require the copyright notice
and permission notice to accompany copies or substantial portions of the
software: each ported file carries the attribution in its header comment,
and this notice reproduces the licence:

> Permission is hereby granted, free of charge, to any person obtaining a
> copy of this software and associated documentation files (the
> "Software"), to deal in the Software without restriction, including
> without limitation the rights to use, copy, modify, merge, publish,
> distribute, sublicense, and/or sell copies of the Software, and to permit
> persons to whom the Software is furnished to do so, subject to the
> following conditions: The above copyright notice and this permission
> notice shall be included in all copies or substantial portions of the
> Software. THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
> EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
> MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN
> NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
> DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
> OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
> USE OR OTHER DEALINGS IN THE SOFTWARE.

The streamOptimized (compressed grain) VMDK path, the descriptor-less split
set assembly beyond VMkatz's, the QCOW2 compressed-cluster path and the test
encoders (`gomount/image/imagetest/`) are this repository's own, under its
Apache-2.0 licence.

## Apple lzfse — ported decoders in `gomount/lzfse/`

Also shipped in the repository: [`gomount/lzfse/lzvn.go`](gomount/lzfse/lzvn.go)
and `lzfse.go` are Go ports of the decode paths of
[lzfse/lzfse](https://github.com/lzfse/lzfse) (Copyright (c) 2015-2016
Apple Inc., BSD-3-Clause), the compressors behind APFS decmpfs. The
BSD-3-Clause terms require the copyright notice, this list of conditions
and the disclaimer to be reproduced:

> Redistribution and use in source and binary forms, with or without
> modification, are permitted provided that the following conditions are
> met: 1. Redistributions of source code must retain the above copyright
> notice, this list of conditions and the following disclaimer. 2.
> Redistributions in binary form must reproduce the above copyright notice,
> this list of conditions and the following disclaimer in the documentation
> and/or other materials provided with the distribution. 3. Neither the name
> of the copyright holder nor the names of its contributors may be used to
> endorse or promote products derived from this software without specific
> prior written permission. THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT
> HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES,
> INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY
> AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE
> COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT,
> INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
> NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
> DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
> THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
> (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
> THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

The APFS backend itself (`gomount/fsx/apfs/`) is this repository's own
clean-room work, written against the libyal APFS documentation and Apple's
File System Reference, under its Apache-2.0 licence.

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
