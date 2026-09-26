# `goyara` — scan gomount's tar stream with YARA

goyara reads a tar archive on stdin — one entry per file, as `gomount stream`
emits — and scans each file's bytes with a compiled YARA ruleset, writing one
JSON record per rule hit. It does no disk or NTFS parsing itself; `gomount`
walks the image and streams the files, goyara scans them:

```
gomount stream evidence.E01 | goyara --rules detectraptor.yar
```

- `--rules FILE` — compiled YARA ruleset (default `/opt/dxdfir/yara-rules/detectraptor/detectraptor.yar`).
- `--json OUT` — write JSONL match records here; `-` is stdout (default `-`).
- `--max-bytes N` — per-file scan cap in bytes (default 33554432); a larger file is scanned up to the cap.

Each hit is one JSON object:

```
{"tool":"yara","source":"disk","rule":<name>,"namespace":<ns>,"target":<file path>,"tags":[...],"strings":[{"name":...,"offset":...,"data":...}]}
```

A `{"rules","files_scanned","matches","errors"}` summary prints on stderr. The
exit code is 0 on a clean scan, 1 on a fatal error (bad rules or arguments), and
2 when the scan completed but some files could not be read.

goyara links libyara through cgo (`github.com/hillu/go-yara/v4`), so unlike the
static go\* tools it needs libyara at build and runtime. It ships in the
`get-sybers/signatures` image, which installs libyara and bakes the DetectRaptor
ruleset, alongside the `gomount` binary it consumes.

## The end-to-end pipe test

`test/e2e.sh` proves the whole signature-scanning path —
`gomount stream <image> | goyara --rules <rules>` — without any real evidence
image and without any mount, FUSE, kernel driver, or privilege: an `mkntfs`
superfloppy in a plain file is walked in-process by gomount into a tar on
stdout, and goyara scans each entry's bytes with compiled libyara rules.
gomount owns ALL disk/NTFS parsing; goyara only consumes the tar (stdin —
it takes no positional tar argument; `--json -` puts records on stdout, the
summary JSON on stderr). A parser bug is a Go panic and a rules/file bug a
libyara return code, never host RCE, so like gomount's userspace test it has
**no environment SKIP path**: given `ntfs-3g` (fixture builders), libyara-dev
(the cgo build) and a Go toolchain it must reach a verdict, in CI included.
Unprivileged recipe:

```sh
docker run --rm -v "$PWD:/src" -w /src golang:trixie bash -c '
  export DEBIAN_FRONTEND=noninteractive PATH=$PATH:/usr/local/go/bin
  apt-get update -qq
  apt-get install -y -qq ntfs-3g libyara-dev pkg-config
  bash goyara/test/e2e.sh'
```

It asserts exactly ONE JSON match record, for rule `detectraptor_smoke` on
`/hit.txt`, a stderr summary with `matches>=1` and `files_scanned>=2`, and
the source image's sha256 unchanged across the pipe (exit 0 proven, exit 2
failed). The marker file is deliberately LARGE (>1 cluster) so its NTFS
`$DATA` is **non-resident**: the marker bytes then live only in the file's
data clusters, not inside the `$MFT` record — gomount's walk streams the
NTFS metafiles too, and a small resident `hit.txt` would surface the marker
in `/$MFT` as a second match. A diagnostic pass prints the marker's true
entry-count across the stream so the property is visible, not assumed.
