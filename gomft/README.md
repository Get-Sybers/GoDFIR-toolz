# `get-sybers/gomft` — raw `$MFT` parser

Static Go binary on Velociraptor's `go-ntfs`. Parses a raw `$MFT` and emits one record per entry — entry/sequence, parent reference, file
name + extension, size, the `$STANDARD_INFORMATION` (0x10) and `$FILE_NAME`
(0x30) MACB timestamps, flags and ADS — as JSONL or CSV. Fields go-ntfs does not expose (ReparseTarget, SecurityId, ObjectId,
ZoneId) are omitted, never faked. `-d` finds the table by its `FILE` signature,
so a raw-mount `$MFT` and Plaso's `image_export` rename (`_MFT`) both parse.
Parse-verified on a real 128 MB `$MFT` (130k entries: the NTFS metadata files at
entries 0–3, real system files resolved to their full paths and MACB times).

```sh
docker build -t get-sybers/gomft:latest -f gomft/Dockerfile gomft
docker run --rm --cap-drop ALL --security-opt no-new-privileges --network none \
  --read-only -v "$PWD/in:/input:ro" -v "$PWD/out:/output" \
  get-sybers/gomft:latest -d /input --json /output --jsonf mft.json
```
