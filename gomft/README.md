# `get-sybers/gomft` — raw `$MFT` parser

Static Go binary on Velociraptor's `go-ntfs`. Parses a raw `$MFT` and emits one record per entry — entry/sequence, parent reference, file
name + extension, size, the `$STANDARD_INFORMATION` (0x10) and `$FILE_NAME`
(0x30) MACB timestamps, flags and ADS — as JSONL or CSV. Fields go-ntfs does not expose (ReparseTarget, SecurityId, ObjectId,
ZoneId) are omitted, never faked. The `$MFT` is found by its `FILE` record
signature, so a raw-mount `$MFT` and Plaso's `image_export` rename (`_MFT`) both parse.
Parse-verified on a real 128 MB `$MFT` (130k entries: the NTFS metadata files at
entries 0–3, real system files resolved to their full paths and MACB times).

## Input

Exactly one input mode per run:

- `-f FILE` — parse a single `$MFT` file.
- `-d DIR` — scan a directory tree and parse each file carrying the `FILE` signature.
- `--tar` — read a tar archive on stdin (one entry per file, as `gomount stream` emits) and parse each entry carrying the `FILE` signature. A `$MFT` needs random access, so each candidate entry is buffered whole in memory before parsing.

`--tar` lets `gomount` stream a `$MFT` straight out of an NTFS image without a mount:

```sh
gomount stream --filter '$MFT' evidence.E01 | gomft --tar --json out --jsonf mft.json
```

Output goes to stdout as JSONL by default; `--json DIR`/`--csv DIR` write to a
file instead (`--jsonf`/`--csvf` name it, default `MFTECmd_Output.jsonl` /
`MFTECmd_Output.csv`). Exit code is 0 when every `$MFT` parsed, 1 on a usage or
fatal error, 2 when at least one file failed to parse (the rest are still
emitted).

```sh
docker build -t get-sybers/gomft:latest -f gomft/Dockerfile gomft
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gomft:latest -d /input --json /output --jsonf mft.json
```
