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
